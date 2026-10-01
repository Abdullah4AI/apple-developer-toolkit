package web

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const (
	staleCookieTestAppleID  = "fixture@example.invalid"
	staleCookieTestPassword = "fixture-password-value"
	staleCookieTestTrust    = "DESfixture0123456789abcdef"
)

// cookieHeaderNames returns every cookie name exactly as it appears on the
// wire, including duplicates, so a test can see what IdMSA would receive.
func cookieHeaderNames(r *http.Request) []string {
	var names []string
	for _, header := range r.Header.Values("Cookie") {
		for _, part := range strings.Split(header, ";") {
			name, _, _ := strings.Cut(strings.TrimSpace(part), "=")
			if name != "" {
				names = append(names, name)
			}
		}
	}
	return names
}

func TestSigninInitSendsEachCookieOnceWithQuotedTrustCookie(t *testing.T) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	idmsa := &url.URL{Scheme: "https", Host: "idmsa.apple.com", Path: "/"}
	jar.SetCookies(idmsa, []*http.Cookie{
		{Name: staleCookieTestTrust, Value: "trust-value", Path: "/"},
		{Name: "aasp", Value: "aasp-value", Path: "/"},
	})
	client := &http.Client{Jar: newSessionCookieTrackingJar(jar), Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		names := cookieHeaderNames(req)
		seen := map[string]int{}
		for _, name := range names {
			seen[name]++
		}
		if len(req.Header.Values("Cookie")) != 1 {
			t.Fatalf("expected one Cookie header, got %d", len(req.Header.Values("Cookie")))
		}
		if seen[staleCookieTestTrust] != 1 || seen["aasp"] != 1 || len(names) != 2 {
			t.Fatalf("expected each cookie exactly once, got names %v", names)
		}
		if header := req.Header.Get("Cookie"); !strings.Contains(header, staleCookieTestTrust+`="trust-value"`) {
			t.Fatal("expected the DES trust cookie value to keep fastlane's quoted form")
		}
		return jsonResponse(http.StatusOK, signinInitFixtureBody(t), nil), nil
	})}

	if _, err := signinInit(context.Background(), client, staleCookieTestAppleID, "A", "service-key"); err != nil {
		t.Fatalf("signinInit: %v", err)
	}
}

// TestExpiredDomainScopedTrustCookieIsNotReplayedToSignin reproduces #2806: Apple scopes its
// sign-in cookies with a Domain attribute, and the cache used to drop their
// deadlines, so a cookie Apple had expired was replayed to IdMSA forever.
func TestExpiredDomainScopedTrustCookieIsNotReplayedToSignin(t *testing.T) {
	t.Setenv(webSessionCacheDirEnv, t.TempDir())
	t.Setenv(webSessionBackendEnv, "file")

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	tracked := newSessionCookieTrackingJar(jar)
	complete := &url.URL{Scheme: "https", Host: "idmsa.apple.com", Path: "/appleauth/auth/2sv/trust"}
	tracked.SetCookies(complete, []*http.Cookie{
		{Name: staleCookieTestTrust, Value: "trust-value", Domain: "idmsa.apple.com", Path: "/", MaxAge: 2592000, Secure: true, HttpOnly: true},
		{Name: "aasp", Value: "aasp-value", Domain: "idmsa.apple.com", Path: "/", MaxAge: 3600, Secure: true, HttpOnly: true},
		{Name: "myacinfo", Value: "session-value", Domain: "apple.com", Path: "/", Secure: true, HttpOnly: true},
	})
	session := &AuthSession{Client: &http.Client{Jar: tracked}, UserEmail: staleCookieTestAppleID}
	if err := PersistSession(session); err != nil {
		t.Fatalf("PersistSession: %v", err)
	}

	// Let 31 days pass: both Apple deadlines are now in the past.
	shiftPersistedSessionClock(t, staleCookieTestAppleID, -31*24*time.Hour)

	loaded, ok, err := LoadCachedSession(staleCookieTestAppleID)
	if err != nil || !ok {
		t.Fatalf("LoadCachedSession ok=%v err=%v", ok, err)
	}
	var sawStaleCookie bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/logout":
			w.Header().Set("Location", "https://idmsa.apple.com/appleauth/signout?widgetKey=fixture-widget-key")
			w.WriteHeader(http.StatusFound)
		case "/appleauth/auth/signin/init":
			for _, name := range cookieHeaderNames(r) {
				if name == staleCookieTestTrust || name == "aasp" {
					sawStaleCookie = true
				}
			}
			if sawStaleCookie {
				w.Header().Set("Retry-After", "60")
				w.WriteHeader(http.StatusServiceUnavailable)
				_, _ = w.Write([]byte(`{"serviceErrors":[{"code":"-20209","message":"Service unavailable."}]}`))
				return
			}
			_, _ = w.Write(signinInitFixtureBody(t))
		case "/appleauth/auth/signin":
			w.WriteHeader(http.StatusOK)
		case "/appleauth/auth/signin/complete":
			w.Header().Set("X-Apple-ID-Session-Id", "fixture-session")
			w.WriteHeader(http.StatusConflict)
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusTeapot)
		}
	}))
	defer server.Close()
	loaded.Client.Transport = newTestServerRoutedClient(t, server).Transport

	_, err = LoginWithClient(context.Background(), loaded.Client, LoginCredentials{Username: staleCookieTestAppleID, Password: staleCookieTestPassword})
	if sawStaleCookie {
		t.Fatalf("expired cached trust cookies were replayed to signin/init: %v", err)
	}
	var tfa *TwoFactorRequiredError
	if !errors.As(err, &tfa) {
		t.Fatalf("expected sign-in to reach two-factor, got %v", err)
	}
}

func TestDomainScopedCookieDeadlineSurvivesRepersist(t *testing.T) {
	t.Setenv(webSessionCacheDirEnv, t.TempDir())
	t.Setenv(webSessionBackendEnv, "file")

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	tracked := newSessionCookieTrackingJar(jar)
	tracked.SetCookies(&url.URL{Scheme: "https", Host: "idmsa.apple.com", Path: "/appleauth/auth/2sv/trust"}, []*http.Cookie{
		{Name: staleCookieTestTrust, Value: "trust-value", Domain: "idmsa.apple.com", Path: "/", MaxAge: 2592000},
		{Name: "myacinfo", Value: "session-value", Domain: "apple.com", Path: "/"},
	})
	if err := PersistSession(&AuthSession{Client: &http.Client{Jar: tracked}, UserEmail: staleCookieTestAppleID}); err != nil {
		t.Fatalf("PersistSession: %v", err)
	}
	first := persistedTrustCookieDeadline(t)
	if first.IsZero() {
		t.Fatal("expected the Domain-scoped trust cookie to keep its Max-Age deadline")
	}

	// A resumed session that sees no new Set-Cookie must keep the deadline.
	loaded, ok, err := LoadCachedSession(staleCookieTestAppleID)
	if err != nil || !ok {
		t.Fatalf("LoadCachedSession ok=%v err=%v", ok, err)
	}
	if err := PersistSession(loaded); err != nil {
		t.Fatalf("re-persist: %v", err)
	}
	if again := persistedTrustCookieDeadline(t); !again.Equal(first) {
		t.Fatalf("trust cookie deadline after re-persist = %v, want %v", again, first)
	}
}

func persistedTrustCookieDeadline(t *testing.T) time.Time {
	t.Helper()
	path := filepath.Join(os.Getenv(webSessionCacheDirEnv), "session-"+webSessionCacheKey(staleCookieTestAppleID)+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read cached session: %v", err)
	}
	var sess persistedSession
	if err := json.Unmarshal(data, &sess); err != nil {
		t.Fatalf("decode cached session: %v", err)
	}
	for _, cookie := range sess.Cookies["https://idmsa.apple.com/"] {
		if cookie.Name == staleCookieTestTrust {
			return cookie.Expires
		}
	}
	t.Fatal("trust cookie missing from the cache")
	return time.Time{}
}

func TestSigninStageServiceErrorIsActionable(t *testing.T) {
	for _, stage := range []string{"signin_init", "signin_complete"} {
		t.Run(stage, func(t *testing.T) {
			client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				return jsonResponse(http.StatusServiceUnavailable, []byte(`{"serviceErrors":[{"code":"-20209","message":"Your request could not be completed. Try again later."}]}`), http.Header{"Retry-After": []string{"120"}}), nil
			})}
			var err error
			if stage == "signin_init" {
				_, err = signinInit(context.Background(), client, staleCookieTestAppleID, "A", "service-key")
			} else {
				err = signinComplete(context.Background(), client, staleCookieTestAppleID, "m1", "m2", json.RawMessage(`{}`), "service-key", "")
			}
			var serviceErr *SigninServiceError
			if !errors.As(err, &serviceErr) {
				t.Fatalf("expected *SigninServiceError, got %T %v", err, err)
			}
			if serviceErr.Stage != stage || serviceErr.HTTPStatusCode() != http.StatusServiceUnavailable || serviceErr.RetryAfter != "120" {
				t.Fatalf("unexpected service error fields: %+v", serviceErr)
			}
			message := err.Error()
			for _, want := range []string{stage, "503", "-20209", "Your request could not be completed. Try again later.", "Retry-After: 120"} {
				if !strings.Contains(message, want) {
					t.Fatalf("expected %q in %q", want, message)
				}
			}
		})
	}
}

func TestSigninStageDebugLogShowsRedactedBodyExcerpt(t *testing.T) {
	origLogger := webDebugLogger
	origDebugEnabled := webDebugEnabledFn
	t.Cleanup(func() {
		webDebugLogger = origLogger
		webDebugEnabledFn = origDebugEnabled
	})
	var logs bytes.Buffer
	webDebugLogger = slog.New(slog.NewTextHandler(&logs, nil))
	webDebugEnabledFn = func() bool { return true }

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	jar.SetCookies(&url.URL{Scheme: "https", Host: "idmsa.apple.com", Path: "/"}, []*http.Cookie{
		{Name: "myacinfo", Value: "cookie-secret-value", Path: "/"},
	})
	body := `{"serviceErrors":[{"code":"-20209","message":"Try again later."}],` +
		`"accountName":"` + staleCookieTestAppleID + `","m1":"m1-proof-secret","m2":"m2-proof-secret",` +
		`"echo":"myacinfo=cookie-secret-value ` + staleCookieTestPassword + `"}`
	client := &http.Client{Jar: jar, Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return jsonResponse(http.StatusServiceUnavailable, []byte(body), nil), nil
	})}
	_ = signinComplete(context.Background(), client, staleCookieTestAppleID, "m1-proof-secret", "m2-proof-secret", json.RawMessage(`{}`), "service-key", "")

	output := logs.String()
	if !strings.Contains(output, "body=") || !strings.Contains(output, "-20209") || !strings.Contains(output, "Try again later.") {
		t.Fatalf("expected a service error body excerpt in debug output, got %q", output)
	}
	for _, secret := range []string{"cookie-secret-value", "m1-proof-secret", "m2-proof-secret", staleCookieTestPassword, staleCookieTestAppleID} {
		if strings.Contains(output, secret) {
			t.Fatalf("debug output leaked %q: %q", secret, output)
		}
	}

	logs.Reset()
	htmlClient := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusServiceUnavailable,
			Header:     http.Header{"Content-Type": []string{"text/html"}},
			Body:       io.NopCloser(strings.NewReader("<html><head><title>Service Unavailable</title></head><body>" + staleCookieTestAppleID + "</body></html>")),
		}, nil
	})}
	_, _ = signinInit(context.Background(), htmlClient, staleCookieTestAppleID, "A", "service-key")
	output = logs.String()
	if !strings.Contains(output, "Service Unavailable") {
		t.Fatalf("expected HTML title in debug output, got %q", output)
	}
	if strings.Contains(output, staleCookieTestAppleID) {
		t.Fatalf("debug output leaked the non-JSON body: %q", output)
	}
}

func signinInitFixtureBody(t *testing.T) []byte {
	t.Helper()
	salt := make([]byte, 16)
	serverB := make([]byte, 256)
	if _, err := rand.Read(salt); err != nil {
		t.Fatal(err)
	}
	if _, err := rand.Read(serverB); err != nil {
		t.Fatal(err)
	}
	body, err := json.Marshal(map[string]any{
		"iteration": 1000,
		"salt":      base64.StdEncoding.EncodeToString(salt),
		"protocol":  "s2k",
		"b":         base64.StdEncoding.EncodeToString(serverB),
		"c":         "fixture-challenge",
	})
	if err != nil {
		t.Fatal(err)
	}
	return body
}

func jsonResponse(status int, body []byte, header http.Header) *http.Response {
	if header == nil {
		header = make(http.Header)
	}
	header.Set("Content-Type", "application/json")
	return &http.Response{StatusCode: status, Header: header, Body: io.NopCloser(bytes.NewReader(body))}
}

// shiftPersistedSessionClock moves every absolute timestamp in a cached
// session by delta, which is equivalent to the wall clock moving by -delta.
func shiftPersistedSessionClock(t *testing.T, appleID string, delta time.Duration) {
	t.Helper()
	path := filepath.Join(os.Getenv(webSessionCacheDirEnv), "session-"+webSessionCacheKey(appleID)+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read cached session: %v", err)
	}
	var sess persistedSession
	if err := json.Unmarshal(data, &sess); err != nil {
		t.Fatalf("decode cached session: %v", err)
	}
	sess.UpdatedAt = sess.UpdatedAt.Add(delta)
	for origin, cookies := range sess.Cookies {
		for i := range cookies {
			if !cookies[i].Expires.IsZero() {
				cookies[i].Expires = cookies[i].Expires.Add(delta)
			}
		}
		sess.Cookies[origin] = cookies
	}
	data, err = json.Marshal(sess)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
}
