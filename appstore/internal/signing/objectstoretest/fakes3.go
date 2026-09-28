// Package objectstoretest provides an in-process, path-style S3-compatible
// server for tests that exercise the signing sync object storage backend.
// It implements only the operations that backend uses and honors the
// If-Match and If-None-Match conditions, so tests never dial a real bucket.
package objectstoretest

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/pem"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
)

// Request records one request the server received.
type Request struct {
	Method      string
	Key         string
	IfMatch     string
	IfNoneMatch string
	Status      int
}

// Server is a fake S3 bucket served over TLS.
type Server struct {
	// Bucket is the only bucket the server accepts.
	Bucket string
	// HTTP is the underlying TLS test server.
	HTTP *httptest.Server
	// PageSize bounds ListObjectsV2 pages so tests can cover pagination.
	PageSize int
	// BeforeRequest may return a non-zero HTTP status to fail a request
	// before it is applied, simulating a provider or network failure.
	BeforeRequest func(method, key string) int
	// OmitPutETag reports whether a successful PUT response omits its ETag,
	// as some S3-compatible services do.
	OmitPutETag func(key string) bool
	// AfterRequest runs after a request was applied, while no lock is held,
	// so a test can simulate a concurrent writer between two requests.
	AfterRequest func(method, key string)

	mu       sync.Mutex
	objects  map[string][]byte
	versions map[string]int
	requests []Request
}

// New starts a fake bucket that is closed when the test ends.
func New(t testing.TB, bucket string) *Server {
	t.Helper()
	server := &Server{
		Bucket:   bucket,
		PageSize: 1000,
		objects:  make(map[string][]byte),
		versions: make(map[string]int),
	}
	server.HTTP = httptest.NewTLSServer(http.HandlerFunc(server.serve))
	t.Cleanup(server.HTTP.Close)
	return server
}

// UseAsAWSEnvironment points the standard AWS SDK environment at this server:
// it trusts the server certificate, supplies static test credentials, and
// isolates shared configuration so no test reads the operator's profiles.
func (s *Server) UseAsAWSEnvironment(t testing.TB) {
	t.Helper()
	dir := t.TempDir()
	caPath := filepath.Join(dir, "ca.pem")
	certificate := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: s.HTTP.Certificate().Raw})
	if err := os.WriteFile(caPath, certificate, 0o600); err != nil {
		t.Fatalf("write CA bundle: %v", err)
	}
	t.Setenv("AWS_CA_BUNDLE", caPath)
	t.Setenv("AWS_CONFIG_FILE", filepath.Join(dir, "config"))
	t.Setenv("AWS_SHARED_CREDENTIALS_FILE", filepath.Join(dir, "credentials"))
	t.Setenv("AWS_PROFILE", "")
	t.Setenv("AWS_EC2_METADATA_DISABLED", "true")
	t.Setenv("AWS_ACCESS_KEY_ID", "AKIAOBJECTSTORETEST")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "object-store-test-secret")
	t.Setenv("AWS_SESSION_TOKEN", "")
	t.Setenv("AWS_REGION", "")
	t.Setenv("AWS_DEFAULT_REGION", "")
	t.Setenv("AWS_ENDPOINT_URL", "")
	t.Setenv("AWS_ENDPOINT_URL_S3", "")
}

// Put stores an object directly, as another writer would.
func (s *Server) Put(key string, content []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.objects[key] = append([]byte(nil), content...)
	s.versions[key]++
}

// Object returns a stored object's content.
func (s *Server) Object(key string) ([]byte, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	content, ok := s.objects[key]
	return append([]byte(nil), content...), ok
}

// Keys returns every stored key in lexical order.
func (s *Server) Keys() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	keys := make([]string, 0, len(s.objects))
	for key := range s.objects {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// Requests returns every request the server received.
func (s *Server) Requests() []Request {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]Request(nil), s.requests...)
}

// Mutations returns the PUT and DELETE requests the server received.
func (s *Server) Mutations() []Request {
	var mutations []Request
	for _, request := range s.Requests() {
		if request.Method == http.MethodPut || request.Method == http.MethodDelete {
			mutations = append(mutations, request)
		}
	}
	return mutations
}

func (s *Server) etag(key string) string {
	sum := md5.Sum(append([]byte(strconv.Itoa(s.versions[key])+":"), s.objects[key]...))
	return `"` + hex.EncodeToString(sum[:]) + `"`
}

func (s *Server) serve(w http.ResponseWriter, r *http.Request) {
	status := s.handle(w, r)
	key := s.keyFor(r)
	s.mu.Lock()
	s.requests = append(s.requests, Request{
		Method:      r.Method,
		Key:         key,
		IfMatch:     r.Header.Get("If-Match"),
		IfNoneMatch: r.Header.Get("If-None-Match"),
		Status:      status,
	})
	s.mu.Unlock()
	if s.AfterRequest != nil && status < 300 {
		s.AfterRequest(r.Method, key)
	}
}

func (s *Server) keyFor(r *http.Request) string {
	path := strings.TrimPrefix(r.URL.Path, "/")
	_, key, _ := strings.Cut(path, "/")
	return key
}

func (s *Server) handle(w http.ResponseWriter, r *http.Request) int {
	if !strings.HasPrefix(r.Header.Get("Authorization"), "AWS4-HMAC-SHA256 ") {
		return writeError(w, r, http.StatusForbidden, "AccessDenied")
	}
	path := strings.TrimPrefix(r.URL.Path, "/")
	bucket, key, _ := strings.Cut(path, "/")
	if bucket != s.Bucket {
		return writeError(w, r, http.StatusNotFound, "NoSuchBucket")
	}
	if s.BeforeRequest != nil {
		if status := s.BeforeRequest(r.Method, key); status != 0 {
			return writeError(w, r, status, injectedErrorCode(status))
		}
	}
	if key == "" && r.Method == http.MethodGet && r.URL.Query().Get("list-type") == "2" {
		return s.list(w, r.URL.Query())
	}
	if key == "" {
		return writeError(w, r, http.StatusBadRequest, "InvalidRequest")
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	content, exists := s.objects[key]
	switch r.Method {
	case http.MethodGet, http.MethodHead:
		if !exists {
			return writeError(w, r, http.StatusNotFound, "NoSuchKey")
		}
		w.Header().Set("ETag", s.etag(key))
		w.Header().Set("Content-Length", strconv.Itoa(len(content)))
		w.Header().Set("Content-Type", "application/octet-stream")
		w.WriteHeader(http.StatusOK)
		if r.Method == http.MethodGet {
			_, _ = w.Write(content)
		}
		return http.StatusOK
	case http.MethodPut:
		if ifNoneMatch := r.Header.Get("If-None-Match"); ifNoneMatch != "" && (ifNoneMatch != "*" || exists) {
			return writeError(w, r, http.StatusPreconditionFailed, "PreconditionFailed")
		}
		if ifMatch := r.Header.Get("If-Match"); ifMatch != "" && (!exists || ifMatch != s.etag(key)) {
			if !exists {
				return writeError(w, r, http.StatusNotFound, "NoSuchKey")
			}
			return writeError(w, r, http.StatusPreconditionFailed, "PreconditionFailed")
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			return writeError(w, r, http.StatusBadRequest, "IncompleteBody")
		}
		s.objects[key] = body
		s.versions[key]++
		if s.OmitPutETag == nil || !s.OmitPutETag(key) {
			w.Header().Set("ETag", s.etag(key))
		}
		w.WriteHeader(http.StatusOK)
		return http.StatusOK
	case http.MethodDelete:
		delete(s.objects, key)
		delete(s.versions, key)
		w.WriteHeader(http.StatusNoContent)
		return http.StatusNoContent
	default:
		return writeError(w, r, http.StatusMethodNotAllowed, "MethodNotAllowed")
	}
}

type listContents struct {
	Key  string `xml:"Key"`
	ETag string `xml:"ETag"`
	Size int    `xml:"Size"`
}

type listBucketResult struct {
	XMLName               xml.Name       `xml:"http://s3.amazonaws.com/doc/2006-03-01/ ListBucketResult"`
	Name                  string         `xml:"Name"`
	Prefix                string         `xml:"Prefix"`
	KeyCount              int            `xml:"KeyCount"`
	MaxKeys               int            `xml:"MaxKeys"`
	IsTruncated           bool           `xml:"IsTruncated"`
	Contents              []listContents `xml:"Contents"`
	ContinuationToken     string         `xml:"ContinuationToken,omitempty"`
	NextContinuationToken string         `xml:"NextContinuationToken,omitempty"`
}

func (s *Server) list(w http.ResponseWriter, query url.Values) int {
	s.mu.Lock()
	prefix := query.Get("prefix")
	keys := make([]string, 0, len(s.objects))
	for key := range s.objects {
		if strings.HasPrefix(key, prefix) {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	start := 0
	if token := query.Get("continuation-token"); token != "" {
		parsed, err := strconv.Atoi(token)
		if err != nil || parsed < 0 || parsed > len(keys) {
			s.mu.Unlock()
			w.WriteHeader(http.StatusBadRequest)
			return http.StatusBadRequest
		}
		start = parsed
	}
	pageSize := s.PageSize
	if pageSize <= 0 {
		pageSize = 1000
	}
	end := min(start+pageSize, len(keys))
	result := listBucketResult{
		Name:              s.Bucket,
		Prefix:            prefix,
		MaxKeys:           pageSize,
		ContinuationToken: query.Get("continuation-token"),
	}
	for _, key := range keys[start:end] {
		result.Contents = append(result.Contents, listContents{Key: key, ETag: s.etag(key), Size: len(s.objects[key])})
	}
	result.KeyCount = len(result.Contents)
	if end < len(keys) {
		result.IsTruncated = true
		result.NextContinuationToken = strconv.Itoa(end)
	}
	s.mu.Unlock()

	w.Header().Set("Content-Type", "application/xml")
	w.WriteHeader(http.StatusOK)
	_, _ = io.WriteString(w, xml.Header)
	_ = xml.NewEncoder(w).Encode(result)
	return http.StatusOK
}

// injectedErrorCode keeps 4xx failures non-retryable so tests stay fast.
func injectedErrorCode(status int) string {
	switch {
	case status == http.StatusForbidden:
		return "AccessDenied"
	case status < 500:
		return "InvalidRequest"
	default:
		return "InternalError"
	}
}

func writeError(w http.ResponseWriter, r *http.Request, status int, code string) int {
	w.Header().Set("Content-Type", "application/xml")
	w.WriteHeader(status)
	if r.Method != http.MethodHead {
		_, _ = fmt.Fprintf(w, "%s<Error><Code>%s</Code><Message>provider message with secret detail</Message></Error>", xml.Header, code)
	}
	return status
}
