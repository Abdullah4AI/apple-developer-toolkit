package signing

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/readonly"
	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/signing/objectstoretest"
)

const (
	objectTestBucket   = "team-certs"
	objectTestPrefix   = "asc"
	objectTestPassword = "repository-password"
	objectTestNext     = "next-repository-password"
)

func newObjectTestServer(t *testing.T) *objectstoretest.Server {
	t.Helper()
	fake := objectstoretest.New(t, objectTestBucket)
	fake.UseAsAWSEnvironment(t)
	return fake
}

func newObjectTestStore(t *testing.T, fake *objectstoretest.Server, prefix string) *ObjectStorageStore {
	t.Helper()
	store, err := NewObjectStorageStore(context.Background(), ObjectStorageOptions{
		Bucket:   fake.Bucket,
		Prefix:   prefix,
		Region:   "us-east-1",
		Endpoint: fake.HTTP.URL,
	})
	if err != nil {
		t.Fatalf("NewObjectStorageStore() error: %v", err)
	}
	return store
}

func readLocalCiphertext(t *testing.T, store *GitStore, relPath string) []byte {
	t.Helper()
	content, err := os.ReadFile(filepath.Join(store.LocalDir, filepath.FromSlash(relPath)+EncryptedArtifactSuffix))
	if err != nil {
		t.Fatalf("read local ciphertext: %v", err)
	}
	return content
}

func TestObjectStorageRoundTripsCiphertextWithGitLayout(t *testing.T) {
	fake := newObjectTestServer(t)
	remote := newObjectTestStore(t, fake, objectTestPrefix+"/")

	source := newLocalArtifactStore(t)
	files := map[string][]byte{
		"certs/distribution/serial.cer":                     []byte("certificate-plaintext"),
		"profiles/appstore/com.example.app.mobileprovision": []byte("profile-plaintext"),
	}
	for relPath, plaintext := range files {
		if err := source.WriteEncryptedFile(relPath, plaintext, objectTestPassword); err != nil {
			t.Fatal(err)
		}
	}
	if err := remote.Fetch(context.Background(), newLocalArtifactStore(t)); err != nil {
		t.Fatalf("Fetch() of an empty prefix error: %v", err)
	}
	if err := remote.Publish(context.Background(), source); err != nil {
		t.Fatalf("Publish() error: %v", err)
	}

	for relPath := range files {
		key := objectTestPrefix + "/" + relPath + EncryptedArtifactSuffix
		stored, ok := fake.Object(key)
		if !ok {
			t.Fatalf("object %s missing; keys = %v", key, fake.Keys())
		}
		if !bytes.Equal(stored, readLocalCiphertext(t, source, relPath)) {
			t.Fatalf("object %s differs from the git working tree ciphertext", key)
		}
		if bytes.Contains(stored, files[relPath]) {
			t.Fatalf("object %s contains plaintext", key)
		}
	}
	for _, mutation := range fake.Mutations() {
		if mutation.Method != http.MethodPut || mutation.IfNoneMatch != "*" || mutation.IfMatch != "" {
			t.Fatalf("new object write = %+v, want a create-only conditional PUT", mutation)
		}
	}

	destination := newLocalArtifactStore(t)
	if err := newObjectTestStore(t, fake, objectTestPrefix).Fetch(context.Background(), destination); err != nil {
		t.Fatalf("Fetch() error: %v", err)
	}
	for relPath, plaintext := range files {
		decrypted, err := destination.ReadEncryptedFile(relPath, objectTestPassword)
		if err != nil {
			t.Fatalf("decrypt %s: %v", relPath, err)
		}
		if !bytes.Equal(decrypted, plaintext) {
			t.Fatalf("decrypted %s = %q, want %q", relPath, decrypted, plaintext)
		}
		if !bytes.Equal(readLocalCiphertext(t, destination, relPath), readLocalCiphertext(t, source, relPath)) {
			t.Fatalf("fetched ciphertext for %s differs from the published bytes", relPath)
		}
	}
}

func TestObjectStorageSupportsBucketRootAndPaginates(t *testing.T) {
	fake := newObjectTestServer(t)
	fake.PageSize = 1
	remote := newObjectTestStore(t, fake, "")
	if got := remote.Locator(); got != "s3://"+objectTestBucket {
		t.Fatalf("Locator() = %q", got)
	}

	source := newLocalArtifactStore(t)
	for index := range 3 {
		relPath := fmt.Sprintf("certs/distribution/serial-%d.cer", index)
		if err := source.WriteEncryptedFile(relPath, []byte(relPath), objectTestPassword); err != nil {
			t.Fatal(err)
		}
	}
	if err := remote.Publish(context.Background(), source); err != nil {
		t.Fatalf("Publish() error: %v", err)
	}
	fake.Put("README.md", []byte("unrelated object"))
	fake.Put("certs/distribution/serial-0.cer.enc.asc-rotation-0011223344ff", []byte("staged"))

	destination := newLocalArtifactStore(t)
	if err := newObjectTestStore(t, fake, "").Fetch(context.Background(), destination); err != nil {
		t.Fatalf("Fetch() error: %v", err)
	}
	listed, err := destination.ListEncryptedFiles()
	if err != nil {
		t.Fatal(err)
	}
	if len(listed) != 3 {
		t.Fatalf("fetched %v, want the three artifacts across paginated listings only", listed)
	}
}

func TestObjectStoragePublishSkipsUnchangedAndConditionallyUpdates(t *testing.T) {
	fake := newObjectTestServer(t)
	seed := newLocalArtifactStore(t)
	relPath := "certs/distribution/serial.cer"
	if err := seed.WriteEncryptedFile(relPath, []byte("v1"), objectTestPassword); err != nil {
		t.Fatal(err)
	}
	if err := newObjectTestStore(t, fake, objectTestPrefix).Publish(context.Background(), seed); err != nil {
		t.Fatal(err)
	}
	key := objectTestPrefix + "/" + relPath + EncryptedArtifactSuffix
	seededRequests := len(fake.Mutations())

	remote := newObjectTestStore(t, fake, objectTestPrefix)
	local := newLocalArtifactStore(t)
	if err := remote.Fetch(context.Background(), local); err != nil {
		t.Fatal(err)
	}
	if err := remote.Publish(context.Background(), local); err != nil {
		t.Fatalf("unchanged Publish() error: %v", err)
	}
	if got := len(fake.Mutations()); got != seededRequests {
		t.Fatalf("unchanged publish wrote %d objects", got-seededRequests)
	}

	if err := local.ReplaceEncryptedFile(relPath, []byte("v2"), objectTestPassword); err != nil {
		t.Fatal(err)
	}
	if err := remote.Publish(context.Background(), local); err != nil {
		t.Fatalf("changed Publish() error: %v", err)
	}
	mutations := fake.Mutations()
	last := mutations[len(mutations)-1]
	if last.Key != key || last.IfMatch == "" || last.IfNoneMatch != "" {
		t.Fatalf("update = %+v, want If-Match on the fetched generation", last)
	}
	// A second change from the same instance must use the new generation.
	if err := local.ReplaceEncryptedFile(relPath, []byte("v3"), objectTestPassword); err != nil {
		t.Fatal(err)
	}
	if err := remote.Publish(context.Background(), local); err != nil {
		t.Fatalf("second changed Publish() error: %v", err)
	}
	stored, _ := fake.Object(key)
	if !bytes.Equal(stored, readLocalCiphertext(t, local, relPath)) {
		t.Fatal("stored object is not the latest ciphertext")
	}
}

func TestObjectStoragePublishNeverOverwritesConcurrentWrites(t *testing.T) {
	relPath := "certs/distribution/serial.cer"
	key := objectTestPrefix + "/" + relPath + EncryptedArtifactSuffix
	for _, existing := range []bool{false, true} {
		t.Run(fmt.Sprintf("existing=%v", existing), func(t *testing.T) {
			fake := newObjectTestServer(t)
			if existing {
				fake.Put(key, []byte("fetched ciphertext"))
			}
			remote := newObjectTestStore(t, fake, objectTestPrefix)
			local := newLocalArtifactStore(t)
			if err := remote.Fetch(context.Background(), local); err != nil {
				t.Fatal(err)
			}
			fake.Put(key, []byte("concurrent ciphertext"))
			if err := local.ReplaceEncryptedFile(relPath, []byte("local"), objectTestPassword); err != nil {
				t.Fatal(err)
			}

			err := remote.Publish(context.Background(), local)
			if !errors.Is(err, ErrObjectStorageConflict) {
				t.Fatalf("Publish() error = %v, want ErrObjectStorageConflict", err)
			}
			if !strings.Contains(err.Error(), key) || !strings.Contains(err.Error(), "pull") {
				t.Fatalf("error = %q, want the key and a pull-and-retry instruction", err)
			}
			stored, _ := fake.Object(key)
			if string(stored) != "concurrent ciphertext" {
				t.Fatalf("stored = %q, concurrent write was overwritten", stored)
			}
		})
	}
}

func TestObjectStorageFetchRejectsUnsafeRemoteObjects(t *testing.T) {
	for name, key := range map[string]string{
		"traversal":       objectTestPrefix + "/certs/../../escape.cer.enc",
		"empty component": objectTestPrefix + "/certs//double.cer.enc",
	} {
		t.Run(name, func(t *testing.T) {
			fake := newObjectTestServer(t)
			fake.Put(key, []byte("ciphertext"))
			err := newObjectTestStore(t, fake, objectTestPrefix).Fetch(context.Background(), newLocalArtifactStore(t))
			if err == nil {
				t.Fatal("Fetch() error = nil, want an unsafe path rejection")
			}
		})
	}

	t.Run("oversize", func(t *testing.T) {
		fake := newObjectTestServer(t)
		fake.Put(objectTestPrefix+"/certs/distribution/big.cer.enc", make([]byte, MaxEncryptedArtifactBytes+1))
		err := newObjectTestStore(t, fake, objectTestPrefix).Fetch(context.Background(), newLocalArtifactStore(t))
		if err == nil || !strings.Contains(err.Error(), "size limit") {
			t.Fatalf("Fetch() error = %v, want the size limit", err)
		}
	})

	t.Run("artifact count", func(t *testing.T) {
		fake := newObjectTestServer(t)
		for index := range 3 {
			fake.Put(fmt.Sprintf("%s/certs/distribution/%d.cer.enc", objectTestPrefix, index), []byte("ciphertext"))
		}
		remote, err := NewObjectStorageStore(context.Background(), ObjectStorageOptions{
			Bucket: fake.Bucket, Prefix: objectTestPrefix, Region: "us-east-1", Endpoint: fake.HTTP.URL, MaxArtifacts: 2,
		})
		if err != nil {
			t.Fatal(err)
		}
		if err := remote.Fetch(context.Background(), newLocalArtifactStore(t)); err == nil || !strings.Contains(err.Error(), "more than 2") {
			t.Fatalf("Fetch() error = %v, want the artifact limit", err)
		}
	})
}

func TestObjectStorageErrorsKeepCodesButDropProviderMessages(t *testing.T) {
	fake := newObjectTestServer(t)
	fake.BeforeRequest = func(string, string) int { return http.StatusForbidden }
	err := newObjectTestStore(t, fake, objectTestPrefix).Fetch(context.Background(), newLocalArtifactStore(t))
	if err == nil {
		t.Fatal("Fetch() error = nil")
	}
	if strings.Contains(err.Error(), "secret detail") || strings.Contains(err.Error(), "object-store-test-secret") {
		t.Fatalf("error leaks provider text: %q", err)
	}
	if !strings.Contains(err.Error(), "AccessDenied") {
		t.Fatalf("error = %q, want the provider code or status", err)
	}
}

func TestObjectStorageReadOnlyAllowsFetchButRefusesWrites(t *testing.T) {
	fake := newObjectTestServer(t)
	relPath := "certs/distribution/serial.cer"
	fake.Put(objectTestPrefix+"/"+relPath+EncryptedArtifactSuffix, []byte("old ciphertext"))
	enableRemoteStoreReadOnly(t, "env")

	remote := newObjectTestStore(t, fake, objectTestPrefix)
	local := newLocalArtifactStore(t)
	if err := remote.Fetch(context.Background(), local); err != nil {
		t.Fatalf("read-only Fetch() error: %v", err)
	}
	if err := local.ReplaceEncryptedFile(relPath, []byte("certificate"), objectTestPassword); err != nil {
		t.Fatal(err)
	}
	if err := remote.Publish(context.Background(), local); !errors.Is(err, readonly.ErrRefused) {
		t.Fatalf("Publish() error = %v, want readonly.ErrRefused", err)
	}
	previous := map[string][]byte{relPath: []byte("old ciphertext")}
	if _, err := remote.PublishRotation(context.Background(), local, previous); !errors.Is(err, readonly.ErrRefused) {
		t.Fatalf("PublishRotation() error = %v, want readonly.ErrRefused", err)
	}
	if mutations := fake.Mutations(); len(mutations) != 0 {
		t.Fatalf("mutations reached the bucket in read-only mode: %+v", mutations)
	}
}

func TestNewObjectStorageStoreValidatesLocator(t *testing.T) {
	fake := newObjectTestServer(t)
	valid := ObjectStorageOptions{Bucket: objectTestBucket, Region: "us-east-1", Endpoint: fake.HTTP.URL}
	tests := []struct {
		name   string
		mutate func(*ObjectStorageOptions)
		want   string
	}{
		{"empty bucket", func(o *ObjectStorageOptions) { o.Bucket = "" }, "bucket"},
		{"uppercase bucket", func(o *ObjectStorageOptions) { o.Bucket = "Team_Certs" }, "bucket"},
		{"ip address bucket", func(o *ObjectStorageOptions) { o.Bucket = "192.168.1.1" }, "IP address"},
		{"reserved prefix bucket", func(o *ObjectStorageOptions) { o.Bucket = "xn--certs" }, "reserved prefix"},
		{"traversal prefix", func(o *ObjectStorageOptions) { o.Prefix = "../escape" }, "prefix"},
		{"leading slash prefix", func(o *ObjectStorageOptions) { o.Prefix = "/asc" }, "prefix"},
		{"bad region", func(o *ObjectStorageOptions) { o.Region = "US East" }, "region"},
		{"http endpoint", func(o *ObjectStorageOptions) { o.Endpoint = "http://127.0.0.1:9000" }, "HTTPS"},
		{"endpoint path", func(o *ObjectStorageOptions) { o.Endpoint = fake.HTTP.URL + "/bucket" }, "path"},
		{"endpoint credentials", func(o *ObjectStorageOptions) { o.Endpoint = "https://user:pass@example.com" }, "credentials"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			options := valid
			tt.mutate(&options)
			if err := ValidateObjectStorageOptions(options); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("ValidateObjectStorageOptions() error = %v, want mention of %q", err, tt.want)
			}
			if _, err := NewObjectStorageStore(context.Background(), options); err == nil {
				t.Fatal("NewObjectStorageStore() error = nil")
			}
		})
	}
	for _, bucket := range []string{"team-certs", "certs.example.com", "certs-ab12cd34-s3alias", "certs--use1-az4--x-s3"} {
		options := valid
		options.Bucket = bucket
		if err := ValidateObjectStorageOptions(options); err != nil {
			t.Fatalf("ValidateObjectStorageOptions(%q) error = %v", bucket, err)
		}
	}
	if len(fake.Requests()) != 0 {
		t.Fatal("locator validation contacted the bucket")
	}
}

func TestNewObjectStorageStoreRequiresARegion(t *testing.T) {
	fake := newObjectTestServer(t)
	_, err := NewObjectStorageStore(context.Background(), ObjectStorageOptions{Bucket: objectTestBucket, Endpoint: fake.HTTP.URL})
	if err == nil || !strings.Contains(err.Error(), "region") {
		t.Fatalf("error = %v, want a missing region failure", err)
	}
	t.Setenv("AWS_REGION", "eu-west-1")
	if _, err := NewObjectStorageStore(context.Background(), ObjectStorageOptions{Bucket: objectTestBucket, Endpoint: fake.HTTP.URL}); err != nil {
		t.Fatalf("region from the AWS environment rejected: %v", err)
	}
}

// seedObjectRotation publishes artifacts encrypted with the current password
// and returns a fetched store ready for a rewrite plus the fetched ciphertext.
func seedObjectRotation(t *testing.T, fake *objectstoretest.Server, relPaths []string) (*ObjectStorageStore, *GitStore, map[string][]byte) {
	t.Helper()
	seed := newLocalArtifactStore(t)
	for _, relPath := range relPaths {
		if err := seed.WriteEncryptedFile(relPath, []byte("plaintext "+relPath), objectTestPassword); err != nil {
			t.Fatal(err)
		}
	}
	if err := newObjectTestStore(t, fake, objectTestPrefix).Publish(context.Background(), seed); err != nil {
		t.Fatal(err)
	}
	remote := newObjectTestStore(t, fake, objectTestPrefix)
	local := newLocalArtifactStore(t)
	if err := remote.Fetch(context.Background(), local); err != nil {
		t.Fatal(err)
	}
	previous := make(map[string][]byte, len(relPaths))
	for _, relPath := range relPaths {
		previous[relPath] = readLocalCiphertext(t, local, relPath)
		if err := local.ReplaceEncryptedFile(relPath, []byte("plaintext "+relPath), objectTestNext); err != nil {
			t.Fatal(err)
		}
	}
	return remote, local, previous
}

func objectKey(relPath string) string {
	return objectTestPrefix + "/" + relPath + EncryptedArtifactSuffix
}

var objectRotationPaths = []string{
	"certs/distribution/a.cer",
	"certs/distribution/b.cer",
	"profiles/appstore/c.mobileprovision",
}

func TestObjectStorageRotationStagesThenSwapsThenCleansUp(t *testing.T) {
	fake := newObjectTestServer(t)
	remote, local, previous := seedObjectRotation(t, fake, objectRotationPaths)
	before := len(fake.Mutations())

	report, err := remote.PublishRotation(context.Background(), local, previous)
	if err != nil {
		t.Fatalf("PublishRotation() error: %v", err)
	}
	if len(report.LeftoverStagedKeys) != 0 {
		t.Fatalf("leftover staged keys = %v", report.LeftoverStagedKeys)
	}

	mutations := fake.Mutations()[before:]
	count := len(objectRotationPaths)
	if len(mutations) != 3*count {
		t.Fatalf("mutations = %+v, want stage, swap, and cleanup per artifact", mutations)
	}
	for index, mutation := range mutations {
		switch {
		case index < count:
			if mutation.Method != http.MethodPut || !strings.Contains(mutation.Key, ".enc.asc-rotation-") || mutation.IfNoneMatch != "*" {
				t.Fatalf("stage %d = %+v, want a create-only staged PUT", index, mutation)
			}
		case index < 2*count:
			if mutation.Method != http.MethodPut || mutation.Key != objectKey(objectRotationPaths[index-count]) || mutation.IfMatch == "" {
				t.Fatalf("swap %d = %+v, want a conditional live PUT", index, mutation)
			}
		default:
			if mutation.Method != http.MethodDelete || !strings.Contains(mutation.Key, ".enc.asc-rotation-") {
				t.Fatalf("cleanup %d = %+v, want a staged DELETE", index, mutation)
			}
		}
	}
	if keys := fake.Keys(); len(keys) != count {
		t.Fatalf("keys = %v, want only the live artifacts", keys)
	}

	verify := newLocalArtifactStore(t)
	if err := newObjectTestStore(t, fake, objectTestPrefix).Fetch(context.Background(), verify); err != nil {
		t.Fatal(err)
	}
	for _, relPath := range objectRotationPaths {
		if _, err := verify.ReadEncryptedFile(relPath, objectTestNext); err != nil {
			t.Fatalf("new password rejected for %s: %v", relPath, err)
		}
		if _, err := verify.ReadEncryptedFile(relPath, objectTestPassword); err == nil {
			t.Fatalf("old password still decrypts %s", relPath)
		}
	}
}

func TestObjectStorageRotationAbortsBeforeSwapWhenAnObjectChanged(t *testing.T) {
	fake := newObjectTestServer(t)
	remote, local, previous := seedObjectRotation(t, fake, objectRotationPaths)
	concurrentKey := objectKey(objectRotationPaths[1])
	var staged atomic.Int32
	fake.AfterRequest = func(method, key string) {
		if method == http.MethodPut && strings.Contains(key, ".asc-rotation-") && staged.Add(1) == int32(len(objectRotationPaths)) {
			fake.Put(concurrentKey, []byte("concurrent push"))
		}
	}

	_, err := remote.PublishRotation(context.Background(), local, previous)
	if !errors.Is(err, ErrObjectStorageConflict) {
		t.Fatalf("PublishRotation() error = %v, want ErrObjectStorageConflict", err)
	}
	if !strings.Contains(err.Error(), "no live artifact was changed") {
		t.Fatalf("error = %q, want it to state that live artifacts are untouched", err)
	}
	for _, relPath := range objectRotationPaths {
		key := objectKey(relPath)
		stored, _ := fake.Object(key)
		if key == concurrentKey {
			if string(stored) != "concurrent push" {
				t.Fatal("concurrent push was overwritten")
			}
			continue
		}
		if !bytes.Equal(stored, previous[relPath]) {
			t.Fatalf("%s was changed before the conflict check", key)
		}
	}
	if keys := fake.Keys(); len(keys) != len(objectRotationPaths) {
		t.Fatalf("keys = %v, want staged objects removed", keys)
	}
}

func TestObjectStorageRotationRollsBackAPartialSwap(t *testing.T) {
	fake := newObjectTestServer(t)
	remote, local, previous := seedObjectRotation(t, fake, objectRotationPaths)
	failingKey := objectKey(objectRotationPaths[1])
	fake.BeforeRequest = func(method, key string) int {
		if method == http.MethodPut && key == failingKey {
			return http.StatusForbidden
		}
		return 0
	}

	_, err := remote.PublishRotation(context.Background(), local, previous)
	if err == nil || !strings.Contains(err.Error(), "restored") || !strings.Contains(err.Error(), "current password") {
		t.Fatalf("PublishRotation() error = %v, want a restored-to-current-password failure", err)
	}
	for _, relPath := range objectRotationPaths {
		stored, _ := fake.Object(objectKey(relPath))
		if !bytes.Equal(stored, previous[relPath]) {
			t.Fatalf("%s was not restored", relPath)
		}
	}
	if keys := fake.Keys(); len(keys) != len(objectRotationPaths) {
		t.Fatalf("keys = %v, want staged objects removed after a complete rollback", keys)
	}
}

func TestObjectStorageRotationReportsMixedStateWhenRollbackFails(t *testing.T) {
	fake := newObjectTestServer(t)
	remote, local, previous := seedObjectRotation(t, fake, objectRotationPaths)
	firstKey := objectKey(objectRotationPaths[0])
	failingKey := objectKey(objectRotationPaths[1])
	var firstWrites atomic.Int32
	fake.BeforeRequest = func(method, key string) int {
		if method != http.MethodPut {
			return 0
		}
		if key == failingKey {
			return http.StatusForbidden
		}
		// The first live write succeeds; its rollback is refused.
		if key == firstKey && firstWrites.Add(1) > 1 {
			return http.StatusForbidden
		}
		return 0
	}

	_, err := remote.PublishRotation(context.Background(), local, previous)
	if err == nil {
		t.Fatal("PublishRotation() error = nil")
	}
	for _, want := range []string{firstKey, ".asc-rotation-", "new password", "copy"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error = %q, want it to mention %q", err, want)
		}
	}
	staged := 0
	for _, key := range fake.Keys() {
		if strings.Contains(key, ".asc-rotation-") {
			staged++
		}
	}
	if staged != len(objectRotationPaths) {
		t.Fatalf("staged objects = %d, want every staged copy kept for recovery", staged)
	}
}

func TestObjectStorageRotationRequiresTheCompleteFetchedSet(t *testing.T) {
	fake := newObjectTestServer(t)
	remote, local, previous := seedObjectRotation(t, fake, objectRotationPaths)
	delete(previous, objectRotationPaths[0])
	if _, err := remote.PublishRotation(context.Background(), local, previous); err == nil {
		t.Fatal("PublishRotation() accepted an incomplete previous ciphertext set")
	}
	if err := local.WriteEncryptedFile("certs/distribution/extra.cer", []byte("extra"), objectTestNext); err != nil {
		t.Fatal(err)
	}
	previous[objectRotationPaths[0]] = []byte("x")
	if _, err := remote.PublishRotation(context.Background(), local, previous); err == nil {
		t.Fatal("PublishRotation() accepted an artifact that was never fetched")
	}
	for _, mutation := range fake.Mutations() {
		if strings.Contains(mutation.Key, ".asc-rotation-") {
			t.Fatalf("rotation wrote %+v before validating its inputs", mutation)
		}
	}
}

func TestObjectStorageRotationNeverRestoresWithoutAnEntityTag(t *testing.T) {
	fake := newObjectTestServer(t)
	remote, local, previous := seedObjectRotation(t, fake, objectRotationPaths)
	firstKey := objectKey(objectRotationPaths[0])
	before := len(fake.Mutations())
	fake.OmitPutETag = func(key string) bool { return key == firstKey }

	_, err := remote.PublishRotation(context.Background(), local, previous)
	if err == nil || !strings.Contains(err.Error(), "no entity tag") || !strings.Contains(err.Error(), firstKey) {
		t.Fatalf("PublishRotation() error = %v, want the missing entity tag reported for %s", err, firstKey)
	}
	for _, mutation := range fake.Mutations()[before:] {
		if mutation.Method == http.MethodPut && mutation.IfMatch == "" && mutation.IfNoneMatch == "" {
			t.Fatalf("unconditional write %+v", mutation)
		}
	}
	for _, relPath := range objectRotationPaths[1:] {
		stored, _ := fake.Object(objectKey(relPath))
		if !bytes.Equal(stored, previous[relPath]) {
			t.Fatalf("%s was swapped after the missing entity tag", relPath)
		}
	}
}

func TestObjectStorageRotationCleansUpStagingAfterCancellation(t *testing.T) {
	fake := newObjectTestServer(t)
	remote, local, previous := seedObjectRotation(t, fake, objectRotationPaths)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	fake.AfterRequest = func(method, key string) {
		if method == http.MethodPut && strings.Contains(key, ".asc-rotation-") {
			cancel()
		}
	}

	if _, err := remote.PublishRotation(ctx, local, previous); err == nil || !strings.Contains(err.Error(), "no live artifact was changed") {
		t.Fatalf("PublishRotation() error = %v, want a canceled stage", err)
	}
	for _, key := range fake.Keys() {
		if strings.Contains(key, ".asc-rotation-") {
			t.Fatalf("staged object %s survived a canceled rotation", key)
		}
	}
}

func TestObjectStorageRotationReportsStagedObjectsItCannotDelete(t *testing.T) {
	fake := newObjectTestServer(t)
	remote, local, previous := seedObjectRotation(t, fake, objectRotationPaths)
	fake.BeforeRequest = func(method, key string) int {
		if method == http.MethodHead && key == objectKey(objectRotationPaths[0]) {
			return http.StatusForbidden
		}
		if method == http.MethodDelete {
			return http.StatusForbidden
		}
		return 0
	}

	_, err := remote.PublishRotation(context.Background(), local, previous)
	if err == nil || !strings.Contains(err.Error(), "could not delete staged objects") || !strings.Contains(err.Error(), objectKey(objectRotationPaths[0])+".asc-rotation-") {
		t.Fatalf("PublishRotation() error = %v, want the leftover staged keys", err)
	}
}
