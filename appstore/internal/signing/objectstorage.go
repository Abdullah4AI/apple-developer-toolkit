package signing

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/aws/aws-sdk-go-v2/aws"
	awshttp "github.com/aws/aws-sdk-go-v2/aws/transport/http"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go"
	transporthttp "github.com/aws/smithy-go/transport/http"
	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/readonly"
)

const (
	objectStorageMaxRemoteArtifacts = 1024
	objectStorageListPageSize       = 1000
	objectStorageMaxListPages       = 100
	// objectStorageStagedMarker separates a live key from the random rotation
	// identifier of its staged copy. Staged keys never end in .enc, so a
	// concurrent or later fetch never treats them as live artifacts.
	objectStorageStagedMarker = ".asc-rotation-"
	// objectStorageRecoveryTimeout bounds rollback and cleanup requests that
	// run after the command context was canceled.
	objectStorageRecoveryTimeout = 2 * time.Minute
)

var objectStorageBucketPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9.-]{1,61}[a-z0-9]$`)

// ErrObjectStorageConflict reports that an object changed after this store
// fetched it, so publishing would overwrite ciphertext that was never
// validated.
var ErrObjectStorageConflict = errors.New("object changed since it was fetched")

// ObjectStorageOptions configures an experimental S3-compatible object
// storage store for already-encrypted signing artifacts.
type ObjectStorageOptions struct {
	// Bucket names the bucket holding the artifacts.
	Bucket string
	// Prefix optionally scopes the artifacts inside the bucket. One trailing
	// slash is accepted; an empty prefix uses the bucket root.
	Prefix string
	// Region overrides the region from the standard AWS configuration.
	Region string
	// Endpoint optionally selects an S3-compatible HTTPS origin, addressed
	// with path-style requests.
	Endpoint string
	// MaxArtifacts bounds how many prefixed artifacts may be transported.
	MaxArtifacts int
	// RequestContext bounds one object storage request.
	RequestContext RequestContextFunc
}

type observedObject struct {
	etag   string
	sha256 [sha256.Size]byte
}

// ObjectStorageStore transports encrypted signing artifacts as objects laid
// out exactly like the encrypted Git working tree. It never receives the sync
// password and never decrypts an artifact.
type ObjectStorageStore struct {
	client         *s3.Client
	bucket         string
	prefix         string
	maxArtifacts   int
	requestContext RequestContextFunc
	observed       map[string]observedObject
}

type normalizedObjectStorageOptions struct {
	bucket   string
	prefix   string
	region   string
	endpoint string
}

// ValidateObjectStorageOptions checks the bucket, prefix, region, and endpoint
// without reading credentials or contacting the network.
func ValidateObjectStorageOptions(options ObjectStorageOptions) error {
	_, err := normalizeObjectStorageOptions(options)
	return err
}

func normalizeObjectStorageOptions(options ObjectStorageOptions) (normalizedObjectStorageOptions, error) {
	bucket := strings.TrimSpace(options.Bucket)
	if bucket == "" {
		return normalizedObjectStorageOptions{}, errors.New("object storage bucket must not be empty")
	}
	if !objectStorageBucketPattern.MatchString(bucket) || strings.Contains(bucket, "..") {
		return normalizedObjectStorageOptions{}, errors.New("object storage bucket must be 3-63 lowercase letters, digits, dots, or hyphens, starting and ending with a letter or digit")
	}
	if err := validateObjectStorageBucketReservations(bucket); err != nil {
		return normalizedObjectStorageOptions{}, err
	}
	prefix := strings.TrimSpace(options.Prefix)
	prefix = strings.TrimSuffix(prefix, "/")
	if prefix != "" {
		if err := ValidateArtifactPrefix(prefix); err != nil {
			return normalizedObjectStorageOptions{}, fmt.Errorf("object storage %w", err)
		}
	} else if strings.TrimSpace(options.Prefix) != "" {
		return normalizedObjectStorageOptions{}, errors.New("object storage prefix must not be only a slash; omit it to use the bucket root")
	}
	region := strings.TrimSpace(options.Region)
	if region != "" && !awsRegionPattern.MatchString(region) {
		return normalizedObjectStorageOptions{}, errors.New("object storage region must be a lowercase region identifier such as us-east-1")
	}
	endpoint := strings.TrimSpace(options.Endpoint)
	if endpoint != "" {
		normalized, err := validateObjectStorageEndpoint(endpoint)
		if err != nil {
			return normalizedObjectStorageOptions{}, err
		}
		endpoint = normalized
	}
	return normalizedObjectStorageOptions{bucket: bucket, prefix: prefix, region: region, endpoint: endpoint}, nil
}

// validateObjectStorageBucketReservations rejects names S3 never assigns to a
// bucket. Reserved suffixes stay accepted because access point aliases
// (-s3alias, --ol-s3) and directory buckets (--x-s3) are valid in the
// bucket parameter of object requests.
func validateObjectStorageBucketReservations(bucket string) error {
	if address := net.ParseIP(bucket); address != nil && address.To4() != nil {
		return errors.New("object storage bucket must not be formatted as an IP address")
	}
	for _, reserved := range []string{"xn--", "sthree-", "amzn-s3-demo-"} {
		if strings.HasPrefix(bucket, reserved) {
			return fmt.Errorf("object storage bucket must not start with the reserved prefix %q", reserved)
		}
	}
	return nil
}

func validateObjectStorageEndpoint(raw string) (string, error) {
	for _, character := range raw {
		if unicode.IsSpace(character) || unicode.IsControl(character) || unicode.In(character, unicode.Cf) {
			return "", errors.New("object storage endpoint contains whitespace, control, or format characters")
		}
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" {
		return "", errors.New("object storage endpoint must be an HTTPS origin such as https://s3.example.com")
	}
	if parsed.User != nil {
		return "", errors.New("object storage endpoint must not include credentials")
	}
	if parsed.RawQuery != "" || parsed.Fragment != "" || (parsed.Path != "" && parsed.Path != "/") {
		return "", errors.New("object storage endpoint must not include a path, query, or fragment")
	}
	parsed.Path = ""
	return parsed.String(), nil
}

// NewObjectStorageStore validates the locator and resolves credentials and the
// region from the standard AWS configuration chain: environment, shared
// configuration files, web identity, and container or instance metadata.
func NewObjectStorageStore(ctx context.Context, options ObjectStorageOptions) (*ObjectStorageStore, error) {
	normalized, err := normalizeObjectStorageOptions(options)
	if err != nil {
		return nil, err
	}

	loadOptions := []func(*config.LoadOptions) error{
		// A buildable client lets the SDK apply AWS_CA_BUNDLE before the
		// transport is wrapped with the no-redirect policy below.
		config.WithHTTPClient(awshttp.NewBuildableClient()),
	}
	if normalized.region != "" {
		loadOptions = append(loadOptions, config.WithRegion(normalized.region))
	}
	// Credential discovery can reach container or instance metadata, so it
	// shares the per-request budget used by the object requests below.
	loadCtx, cancel := boundedRequestContext(options.RequestContext, ctx)
	configuration, err := config.LoadDefaultConfig(loadCtx, loadOptions...)
	cancel()
	if err != nil {
		return nil, fmt.Errorf("object storage: load AWS configuration: %w", sanitizedObjectStorageError(err))
	}
	if strings.TrimSpace(configuration.Region) == "" {
		return nil, errors.New("object storage: no AWS region is configured; pass --object-region or set AWS_REGION")
	}
	buildable, ok := configuration.HTTPClient.(*awshttp.BuildableClient)
	if !ok {
		return nil, errors.New("object storage: configured HTTP client cannot enforce the no-redirect policy")
	}
	// Signed requests must never follow a redirect to another origin.
	configuration.HTTPClient = &http.Client{
		Transport: buildable.GetTransport(),
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	client := s3.NewFromConfig(configuration, func(s3Options *s3.Options) {
		if normalized.endpoint != "" {
			s3Options.BaseEndpoint = aws.String(normalized.endpoint)
			s3Options.UsePathStyle = true
		}
		// Ciphertext is authenticated by the encryption envelope, and
		// optional flexible checksums are not implemented by every
		// S3-compatible service.
		s3Options.RequestChecksumCalculation = aws.RequestChecksumCalculationWhenRequired
		s3Options.ResponseChecksumValidation = aws.ResponseChecksumValidationWhenRequired
	})
	store := &ObjectStorageStore{
		client:         client,
		bucket:         normalized.bucket,
		prefix:         normalized.prefix,
		maxArtifacts:   options.MaxArtifacts,
		requestContext: options.RequestContext,
		observed:       make(map[string]observedObject),
	}
	if store.maxArtifacts <= 0 {
		store.maxArtifacts = objectStorageMaxRemoteArtifacts
	}
	return store, nil
}

// Locator returns an s3:// description of the bucket and prefix. It contains
// no credential material and omits any custom endpoint host.
func (s *ObjectStorageStore) Locator() string {
	if s.prefix == "" {
		return "s3://" + s.bucket
	}
	return "s3://" + s.bucket + "/" + s.prefix
}

func (s *ObjectStorageStore) objectKey(relPath string) (string, error) {
	if err := validateRemoteArtifactRelativePath(relPath); err != nil {
		return "", fmt.Errorf("object storage: %w", err)
	}
	if s.prefix == "" {
		return relPath + EncryptedArtifactSuffix, nil
	}
	return s.prefix + "/" + relPath + EncryptedArtifactSuffix, nil
}

func (s *ObjectStorageStore) listPrefix() string {
	if s.prefix == "" {
		return ""
	}
	return s.prefix + "/"
}

func (s *ObjectStorageStore) objectTarget(key string) string {
	return "s3://" + s.bucket + "/" + key
}

// Fetch downloads every encrypted artifact under the prefix into the local
// ciphertext store and records each object's generation for later
// conditional writes.
func (s *ObjectStorageStore) Fetch(ctx context.Context, store ArtifactStore) error {
	keys, err := s.listArtifacts(ctx)
	if err != nil {
		return err
	}
	relPaths := make([]string, 0, len(keys))
	for relPath := range keys {
		relPaths = append(relPaths, relPath)
	}
	sort.Strings(relPaths)
	if err := ValidateEncryptedRepositoryPaths(relPaths); err != nil {
		return fmt.Errorf("object storage: %w", err)
	}
	observed := make(map[string]observedObject, len(relPaths))
	for _, relPath := range relPaths {
		key := keys[relPath]
		ciphertext, etag, err := s.getObject(ctx, key)
		if err != nil {
			return err
		}
		if err := store.WriteEncryptedArtifact(relPath, ciphertext); err != nil {
			return fmt.Errorf("object storage: store %s: %w", relPath, err)
		}
		observed[key] = observedObject{etag: etag, sha256: sha256.Sum256(ciphertext)}
	}
	s.observed = observed
	return nil
}

type pendingObject struct {
	relPath    string
	key        string
	ciphertext []byte
	sha256     [sha256.Size]byte
}

func (s *ObjectStorageStore) pendingObjects(store ArtifactStore) ([]pendingObject, error) {
	local, err := store.ListEncryptedFiles()
	if err != nil {
		return nil, err
	}
	sort.Strings(local)
	if len(local) > s.maxArtifacts {
		return nil, fmt.Errorf("object storage: %d encrypted artifacts exceed the %d-artifact limit", len(local), s.maxArtifacts)
	}
	pending := make([]pendingObject, 0, len(local))
	for _, relPath := range local {
		key, err := s.objectKey(relPath)
		if err != nil {
			return nil, err
		}
		ciphertext, err := store.ReadEncryptedArtifact(relPath)
		if err != nil {
			return nil, fmt.Errorf("object storage: read %s: %w", relPath, err)
		}
		if len(ciphertext) == 0 {
			return nil, fmt.Errorf("object storage: encrypted artifact %s is empty", relPath)
		}
		if len(ciphertext) > MaxEncryptedArtifactBytes {
			return nil, fmt.Errorf("object storage: encrypted artifact %s exceeds the %d-byte size limit", relPath, MaxEncryptedArtifactBytes)
		}
		pending = append(pending, pendingObject{relPath: relPath, key: key, ciphertext: ciphertext, sha256: sha256.Sum256(ciphertext)})
	}
	return pending, nil
}

// Publish writes every changed local artifact. A new object is created only
// if no object exists at its key, and an existing object is replaced only if
// it still has the generation observed by Fetch, so a concurrent writer's
// ciphertext is never overwritten. Objects outside the prefix are never
// inspected, and no object is deleted.
func (s *ObjectStorageStore) Publish(ctx context.Context, store ArtifactStore) error {
	pending, err := s.pendingObjects(store)
	if err != nil {
		return err
	}
	for _, object := range pending {
		observed, known := s.observed[object.key]
		if known && observed.sha256 == object.sha256 {
			continue
		}
		condition := objectWriteCondition{ifNoneMatch: "*"}
		if known {
			if observed.etag == "" {
				return fmt.Errorf("object storage: %s has no entity tag, so it cannot be replaced safely", object.key)
			}
			condition = objectWriteCondition{ifMatch: observed.etag}
		}
		etag, err := s.putObject(ctx, object.key, object.ciphertext, condition)
		if err != nil {
			return s.writeError(object.key, err)
		}
		s.observed[object.key] = observedObject{etag: etag, sha256: object.sha256}
	}
	return nil
}

// ObjectStorageRotationReport describes a completed rotation.
type ObjectStorageRotationReport struct {
	// LeftoverStagedKeys lists staged copies that could not be deleted after
	// a successful swap. They are ignored by fetches and safe to delete.
	LeftoverStagedKeys []string
}

// PublishRotation replaces every fetched artifact with its re-encrypted local
// ciphertext in three phases:
//
//  1. Stage: every new ciphertext is written to a create-only staged key next
//     to its live key. A failure removes the staged copies and leaves every
//     live artifact unchanged.
//  2. Verify: every live object must still have the generation observed by
//     Fetch. A change aborts the rotation before any live write.
//  3. Swap: each live object is replaced only if it still has the observed
//     generation. If a swap fails, the already replaced artifacts are
//     restored from previous, again conditionally. If restoration also
//     fails, the staged copies are kept and the error lists which artifacts
//     use the new password and how to finish the rotation.
//
// Staged copies are deleted after a successful swap or a complete rollback.
// previous must hold the fetched ciphertext of every artifact.
func (s *ObjectStorageStore) PublishRotation(ctx context.Context, store ArtifactStore, previous map[string][]byte) (ObjectStorageRotationReport, error) {
	pending, err := s.pendingObjects(store)
	if err != nil {
		return ObjectStorageRotationReport{}, err
	}
	if len(pending) != len(s.observed) {
		return ObjectStorageRotationReport{}, fmt.Errorf("object storage: rotation has %d local artifacts but fetched %d", len(pending), len(s.observed))
	}
	for _, object := range pending {
		observed, known := s.observed[object.key]
		if !known {
			return ObjectStorageRotationReport{}, fmt.Errorf("object storage: rotation artifact %s was not fetched", object.relPath)
		}
		if observed.etag == "" {
			return ObjectStorageRotationReport{}, fmt.Errorf("object storage: %s has no entity tag, so it cannot be replaced safely", object.key)
		}
		if len(previous[object.relPath]) == 0 {
			return ObjectStorageRotationReport{}, fmt.Errorf("object storage: rotation is missing the fetched ciphertext of %s", object.relPath)
		}
	}
	if len(pending) == 0 {
		return ObjectStorageRotationReport{}, nil
	}

	var suffix [6]byte
	if _, err := rand.Read(suffix[:]); err != nil {
		return ObjectStorageRotationReport{}, fmt.Errorf("object storage: generate rotation identifier: %w", err)
	}
	rotationID := hex.EncodeToString(suffix[:])
	stagedKey := func(key string) string { return key + objectStorageStagedMarker + rotationID }

	// Phase 1: stage.
	var staged []string
	for _, object := range pending {
		key := stagedKey(object.key)
		if _, err := s.putObject(ctx, key, object.ciphertext, objectWriteCondition{ifNoneMatch: "*"}); err != nil {
			if errors.Is(err, readonly.ErrRefused) {
				return ObjectStorageRotationReport{}, err
			}
			// A failed or canceled write may still have landed, and the
			// staged key is unique to this rotation, so remove it too.
			leftover := s.abortStaged(ctx, append(staged, key))
			return ObjectStorageRotationReport{}, fmt.Errorf("object storage: stage %s: %w; no live artifact was changed%s", key, sanitizedObjectStorageError(err), leftover)
		}
		staged = append(staged, key)
	}

	// Phase 2: verify that no live object changed since the fetch.
	for _, object := range pending {
		etag, err := s.headObject(ctx, object.key)
		if err == nil && etag == s.observed[object.key].etag {
			continue
		}
		leftover := s.abortStaged(ctx, staged)
		if err != nil && !isObjectNotFound(err) {
			return ObjectStorageRotationReport{}, fmt.Errorf("object storage: check %s: %w; no live artifact was changed%s", object.key, sanitizedObjectStorageError(err), leftover)
		}
		return ObjectStorageRotationReport{}, fmt.Errorf("object storage: %s: %w; no live artifact was changed, pull the current artifacts and retry%s", object.key, ErrObjectStorageConflict, leftover)
	}

	// Phase 3: swap.
	var swapped []swappedObject
	for _, object := range pending {
		etag, err := s.putObject(ctx, object.key, object.ciphertext, objectWriteCondition{ifMatch: s.observed[object.key].etag})
		if err != nil {
			return ObjectStorageRotationReport{}, s.rollBackRotation(ctx, s.writeError(object.key, err), swapped, previous, staged, rotationID)
		}
		swapped = append(swapped, swappedObject{object: object, etag: etag})
		if etag == "" {
			// Without the new entity tag, restoring this object could only
			// be an unconditional write, so it is reported, never restored.
			cause := fmt.Errorf("object storage: %s was replaced but the service returned no entity tag", object.key)
			return ObjectStorageRotationReport{}, s.rollBackRotation(ctx, cause, swapped, previous, staged, rotationID)
		}
	}
	for _, entry := range swapped {
		s.observed[entry.object.key] = observedObject{etag: entry.etag, sha256: entry.object.sha256}
	}

	cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), objectStorageRecoveryTimeout)
	defer cancel()
	return ObjectStorageRotationReport{LeftoverStagedKeys: s.deleteStaged(cleanupCtx, staged)}, nil
}

type swappedObject struct {
	object pendingObject
	etag   string
}

// rollBackRotation restores the already swapped artifacts from their previous
// ciphertext. Restoration uses its own bounded context so canceling the
// command does not also cancel recovery.
func (s *ObjectStorageStore) rollBackRotation(ctx context.Context, cause error, swapped []swappedObject, previous map[string][]byte, staged []string, rotationID string) error {
	recoveryCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), objectStorageRecoveryTimeout)
	defer cancel()
	var unrestored []string
	for index := len(swapped) - 1; index >= 0; index-- {
		entry := swapped[index]
		if entry.etag == "" {
			unrestored = append(unrestored, entry.object.key)
			continue
		}
		if _, err := s.putObject(recoveryCtx, entry.object.key, previous[entry.object.relPath], objectWriteCondition{ifMatch: entry.etag}); err != nil {
			unrestored = append(unrestored, entry.object.key)
		}
	}
	if len(unrestored) == 0 {
		leftover := describeLeftoverStaged(s.deleteStaged(recoveryCtx, staged))
		return fmt.Errorf("rotation failed and was rolled back: %w; restored %d already rotated artifacts, so every artifact still uses the current password%s", cause, len(swapped), leftover)
	}
	sort.Strings(unrestored)
	return fmt.Errorf(
		"rotation failed: %w; these artifacts could not be restored and now use the new password while the rest use the current password: %s. "+
			"Every artifact's new-password ciphertext is staged at <key>%s%s; copy each staged object over its live key to finish the rotation, then delete the staged objects",
		cause, strings.Join(unrestored, ", "), objectStorageStagedMarker, rotationID,
	)
}

// abortStaged removes the staged copies of an abandoned rotation with a
// recovery context, because the failure may be the command's own
// cancellation, and describes any copies that could not be deleted.
func (s *ObjectStorageStore) abortStaged(ctx context.Context, staged []string) string {
	recoveryCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), objectStorageRecoveryTimeout)
	defer cancel()
	return describeLeftoverStaged(s.deleteStaged(recoveryCtx, staged))
}

func describeLeftoverStaged(leftover []string) string {
	if len(leftover) == 0 {
		return ""
	}
	return "; could not delete staged objects " + strings.Join(leftover, ", ") + ", which pulls ignore and are safe to delete"
}

// deleteStaged removes staged copies created by this rotation and returns the
// keys it could not delete.
func (s *ObjectStorageStore) deleteStaged(ctx context.Context, keys []string) []string {
	var leftover []string
	for _, key := range keys {
		if err := s.deleteObject(ctx, key); err != nil {
			leftover = append(leftover, key)
		}
	}
	return leftover
}

func (s *ObjectStorageStore) writeError(key string, err error) error {
	if errors.Is(err, readonly.ErrRefused) {
		return err
	}
	if isObjectPreconditionFailed(err) {
		return fmt.Errorf("object storage: %s: %w; another writer published first, pull the current artifacts and retry", key, ErrObjectStorageConflict)
	}
	return fmt.Errorf("object storage: write %s: %w", key, sanitizedObjectStorageError(err))
}

type objectWriteCondition struct {
	ifMatch     string
	ifNoneMatch string
}

// Each request is bounded separately so a stalled request cannot hang a whole
// multi-artifact push or pull.
func (s *ObjectStorageStore) putObject(ctx context.Context, key string, body []byte, condition objectWriteCondition) (string, error) {
	// Every write is conditional; an unconditional write could overwrite a
	// concurrent writer's ciphertext.
	if (condition.ifMatch == "") == (condition.ifNoneMatch == "") {
		return "", fmt.Errorf("object storage: refusing an unconditional write to %s", key)
	}
	if err := readonly.Check(ctx, http.MethodPut, s.objectTarget(key)); err != nil {
		return "", err
	}
	requestCtx, cancel := boundedRequestContext(s.requestContext, ctx)
	defer cancel()
	input := &s3.PutObjectInput{
		Bucket:        aws.String(s.bucket),
		Key:           aws.String(key),
		Body:          bytes.NewReader(body),
		ContentLength: aws.Int64(int64(len(body))),
		ContentType:   aws.String("application/octet-stream"),
	}
	if condition.ifMatch != "" {
		input.IfMatch = aws.String(condition.ifMatch)
	}
	if condition.ifNoneMatch != "" {
		input.IfNoneMatch = aws.String(condition.ifNoneMatch)
	}
	output, err := s.client.PutObject(requestCtx, input)
	if err != nil {
		return "", err
	}
	return aws.ToString(output.ETag), nil
}

func (s *ObjectStorageStore) deleteObject(ctx context.Context, key string) error {
	if err := readonly.Check(ctx, http.MethodDelete, s.objectTarget(key)); err != nil {
		return err
	}
	requestCtx, cancel := boundedRequestContext(s.requestContext, ctx)
	defer cancel()
	_, err := s.client.DeleteObject(requestCtx, &s3.DeleteObjectInput{Bucket: aws.String(s.bucket), Key: aws.String(key)})
	return err
}

func (s *ObjectStorageStore) headObject(ctx context.Context, key string) (string, error) {
	requestCtx, cancel := boundedRequestContext(s.requestContext, ctx)
	defer cancel()
	output, err := s.client.HeadObject(requestCtx, &s3.HeadObjectInput{Bucket: aws.String(s.bucket), Key: aws.String(key)})
	if err != nil {
		return "", err
	}
	return aws.ToString(output.ETag), nil
}

func (s *ObjectStorageStore) getObject(ctx context.Context, key string) ([]byte, string, error) {
	requestCtx, cancel := boundedRequestContext(s.requestContext, ctx)
	defer cancel()
	output, err := s.client.GetObject(requestCtx, &s3.GetObjectInput{Bucket: aws.String(s.bucket), Key: aws.String(key)})
	if err != nil {
		return nil, "", fmt.Errorf("object storage: read %s: %w", key, sanitizedObjectStorageError(err))
	}
	defer output.Body.Close()
	if aws.ToInt64(output.ContentLength) > MaxEncryptedArtifactBytes {
		return nil, "", fmt.Errorf("object storage: %s exceeds the %d-byte size limit", key, MaxEncryptedArtifactBytes)
	}
	ciphertext, err := io.ReadAll(io.LimitReader(output.Body, MaxEncryptedArtifactBytes+1))
	if err != nil {
		return nil, "", fmt.Errorf("object storage: read %s: %w", key, sanitizedObjectStorageError(err))
	}
	if len(ciphertext) > MaxEncryptedArtifactBytes {
		return nil, "", fmt.Errorf("object storage: %s exceeds the %d-byte size limit", key, MaxEncryptedArtifactBytes)
	}
	if len(ciphertext) == 0 {
		return nil, "", fmt.Errorf("object storage: %s is empty", key)
	}
	return ciphertext, aws.ToString(output.ETag), nil
}

// listArtifacts maps each in-scope relative path to its object key. Objects
// that do not end in .enc, including staged rotation copies, are ignored.
func (s *ObjectStorageStore) listArtifacts(ctx context.Context) (map[string]string, error) {
	artifacts := make(map[string]string)
	prefix := s.listPrefix()
	var token *string
	for page := 0; page < objectStorageMaxListPages; page++ {
		requestCtx, cancel := boundedRequestContext(s.requestContext, ctx)
		output, err := s.client.ListObjectsV2(requestCtx, &s3.ListObjectsV2Input{
			Bucket:            aws.String(s.bucket),
			Prefix:            aws.String(prefix),
			MaxKeys:           aws.Int32(objectStorageListPageSize),
			ContinuationToken: token,
		})
		cancel()
		if err != nil {
			return nil, fmt.Errorf("object storage: list %s: %w", s.Locator(), sanitizedObjectStorageError(err))
		}
		for _, object := range output.Contents {
			key := aws.ToString(object.Key)
			if !strings.HasPrefix(key, prefix) || !strings.HasSuffix(key, EncryptedArtifactSuffix) {
				continue
			}
			relPath := strings.TrimSuffix(strings.TrimPrefix(key, prefix), EncryptedArtifactSuffix)
			if relPath == "" {
				continue
			}
			if err := validateRemoteArtifactRelativePath(relPath); err != nil {
				return nil, fmt.Errorf("object storage: %w", err)
			}
			artifacts[relPath] = key
			if len(artifacts) > s.maxArtifacts {
				return nil, fmt.Errorf("object storage: %s holds more than %d encrypted artifacts", s.Locator(), s.maxArtifacts)
			}
		}
		if !aws.ToBool(output.IsTruncated) {
			return artifacts, nil
		}
		if strings.TrimSpace(aws.ToString(output.NextContinuationToken)) == "" {
			return nil, errors.New("object storage: truncated listing has no continuation token")
		}
		token = output.NextContinuationToken
	}
	return nil, fmt.Errorf("object storage: listing exceeded %d pages", objectStorageMaxListPages)
}

func isObjectPreconditionFailed(err error) bool {
	var apiError smithy.APIError
	if errors.As(err, &apiError) {
		switch apiError.ErrorCode() {
		case "PreconditionFailed", "ConditionalRequestConflict":
			return true
		}
	}
	var responseError *transporthttp.ResponseError
	return errors.As(err, &responseError) && responseError.HTTPStatusCode() == http.StatusPreconditionFailed
}

func isObjectNotFound(err error) bool {
	var apiError smithy.APIError
	if errors.As(err, &apiError) {
		switch apiError.ErrorCode() {
		case "NotFound", "NoSuchKey":
			return true
		}
	}
	var responseError *transporthttp.ResponseError
	return errors.As(err, &responseError) && responseError.HTTPStatusCode() == http.StatusNotFound
}

var objectStorageErrorCodes = map[string]struct{}{
	"AccessDenied": {}, "AllAccessDisabled": {}, "AuthorizationHeaderMalformed": {},
	"ConditionalRequestConflict": {}, "EntityTooLarge": {}, "ExpiredToken": {}, "InternalError": {},
	"InvalidAccessKeyId": {}, "InvalidArgument": {}, "InvalidBucketName": {}, "InvalidObjectState": {},
	"InvalidRequest": {}, "InvalidToken": {}, "NoSuchBucket": {}, "NoSuchKey": {}, "NotFound": {},
	"NotImplemented": {}, "PermanentRedirect": {}, "PreconditionFailed": {}, "RequestTimeTooSkewed": {},
	"RequestTimeout": {}, "ServiceUnavailable": {}, "SignatureDoesNotMatch": {}, "SlowDown": {},
	"TokenRefreshRequired": {},
}

// sanitizedObjectStorageError drops provider messages and credential-process
// output while keeping stable error codes and HTTP status.
func sanitizedObjectStorageError(err error) error {
	return sanitizedAWSError(err, objectStorageErrorCodes)
}
