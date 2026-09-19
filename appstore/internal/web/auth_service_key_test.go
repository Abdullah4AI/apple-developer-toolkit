package web

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/handlertest"
)

// Captured from Apple's public production ASC login configuration on 2026-09-11.
const testPublicASCWidgetKey = "e0b80c3bf78523bfe80974d320935bfa30add02e1bff88ec2166c6bd5a706c42"

func TestLoginDiscoversServiceKeyWithoutSigningOut(t *testing.T) {
	fixture := handlertest.New(t)
	var discoveryCalls, signinCalls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/logout":
			discoveryCalls++
			if r.Method != http.MethodHead || r.Header.Get("Cookie") != "" || r.Header.Get("Authorization") != "" {
				fixture.Respond(w, "discovery must be a credential-free HEAD request")
				return
			}
			http.SetCookie(w, &http.Cookie{Name: "session", Value: "signed-out", Path: "/"})
			w.Header().Set("Location", "https://idmsa.apple.com/appleauth/signout?widgetKey=rotated-key&asop=destroy-session")
			w.WriteHeader(http.StatusFound)
		case "/appleauth/auth/signin/init":
			signinCalls++
			if r.Method != http.MethodPost || r.Header.Get("X-Apple-Widget-Key") != "rotated-key" {
				fixture.Respond(w, "SRP did not receive the discovered key")
				return
			}
			cookie, err := r.Cookie("session")
			if err != nil || cookie.Value != "authenticated" {
				fixture.Respond(w, "discovery changed the authentication client's cookies")
				return
			}
			w.WriteHeader(http.StatusServiceUnavailable)
		default:
			fixture.Respond(w, "unexpected request: %s %s", r.Method, r.URL)
		}
	}))
	defer server.Close()
	client := newTestServerRoutedClient(t, server)
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, host := range []string{"appstoreconnect.apple.com", "idmsa.apple.com"} {
		jar.SetCookies(&url.URL{Scheme: "https", Host: host}, []*http.Cookie{{Name: "session", Value: "authenticated", Path: "/"}})
	}
	client.Jar = jar
	client.Timeout = 5 * time.Second
	redirectCalls := 0
	client.CheckRedirect = func(_ *http.Request, _ []*http.Request) error {
		redirectCalls++
		return nil
	}
	session, err := loginWithHTTPClient(context.Background(), client, LoginCredentials{
		Username: "fixture@example.invalid", Password: "fixture-password",
	})
	if err == nil || session != nil {
		t.Fatal("expected the downstream SRP failure")
	}
	if discoveryCalls != 1 || signinCalls != 1 || redirectCalls != 0 {
		t.Fatalf("discovery = %d, SRP = %d, redirects = %d; want 1, 1, 0", discoveryCalls, signinCalls, redirectCalls)
	}
	tracked, ok := client.Jar.(*sessionCookieTrackingJar)
	if !ok || tracked.CookieJar != jar || client.Timeout != 5*time.Second || client.CheckRedirect == nil {
		t.Fatal("discovery did not preserve the authentication client and underlying cookie jar")
	}
	for _, cookie := range jar.Cookies(&url.URL{Scheme: "https", Host: "appstoreconnect.apple.com"}) {
		if cookie.Name == "session" && cookie.Value != "authenticated" {
			t.Fatal("discovery response changed the authentication cookie jar")
		}
	}
}

func TestAuthServiceKeyDiscoveryFallsBack(t *testing.T) {
	tests := []struct {
		name     string
		status   int
		location string
	}{
		{name: "not a redirect", status: 200, location: "https://idmsa.apple.com/appleauth/signout?widgetKey=ignored"},
		{name: "unavailable", status: 503},
		{name: "missing location", status: 302},
		{name: "malformed location", status: 302, location: "https://[invalid?widgetKey=ignored"},
		{name: "wrong scheme", status: 302, location: "http://idmsa.apple.com/appleauth/signout?widgetKey=ignored"},
		{name: "wrong host", status: 302, location: "https://example.invalid/appleauth/signout?widgetKey=ignored"},
		{name: "unexpected port", status: 302, location: "https://idmsa.apple.com:8443/appleauth/signout?widgetKey=ignored"},
		{name: "userinfo", status: 302, location: "https://user@idmsa.apple.com/appleauth/signout?widgetKey=ignored"},
		{name: "wrong path", status: 302, location: "https://idmsa.apple.com/other?widgetKey=ignored"},
		{name: "missing key", status: 302, location: "https://idmsa.apple.com/appleauth/signout?asop=destroy-session"},
		{name: "empty key", status: 302, location: "https://idmsa.apple.com/appleauth/signout?widgetKey=%20"},
		{name: "duplicate key", status: 302, location: "https://idmsa.apple.com/appleauth/signout?widgetKey=one&widgetKey=two"},
		{name: "malformed query", status: 302, location: "https://idmsa.apple.com/appleauth/signout?widgetKey=%zz"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fixture := handlertest.New(t)
			var discoveryCalls, legacyCalls int
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/logout":
					discoveryCalls++
					w.Header().Set("Location", tt.location)
					w.WriteHeader(tt.status)
				case "/olympus/v1/app/config":
					legacyCalls++
					_, _ = w.Write([]byte(`{"authServiceKey":"legacy-key"}`))
				default:
					fixture.Respond(w, "unexpected request: %s %s", r.Method, r.URL)
				}
			}))
			defer server.Close()
			key, err := getAuthServiceKey(context.Background(), newTestServerRoutedClient(t, server))
			if err != nil || key != "legacy-key" || discoveryCalls != 1 || legacyCalls != 1 {
				t.Fatalf("expected legacy fallback; error = %v, discovery = %d, legacy = %d", err, discoveryCalls, legacyCalls)
			}
		})
	}
}

func TestAuthServiceKeyDiscoveryTransportFailure(t *testing.T) {
	for _, cancelRequest := range []bool{false, true} {
		t.Run(fmt.Sprintf("canceled=%t", cancelRequest), func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			fixture := handlertest.New(t)
			var legacyCalls int
			client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				if req.Method == http.MethodHead && req.URL.Path == "/logout" {
					if cancelRequest {
						cancel()
						return nil, ctx.Err()
					}
					return nil, errors.New("discovery connection failed")
				}
				if req.Method != http.MethodGet || req.URL.Path != "/olympus/v1/app/config" {
					return nil, fixture.Errorf("unexpected request: %s %s", req.Method, req.URL)
				}
				legacyCalls++
				return &http.Response{StatusCode: http.StatusNotFound, Header: make(http.Header), Body: http.NoBody, Request: req}, nil
			})}
			key, err := getAuthServiceKey(ctx, client)
			if cancelRequest {
				if !errors.Is(err, context.Canceled) || key != "" || legacyCalls != 0 {
					t.Fatalf("expected cancellation without fallback; error = %v, legacy calls = %d", err, legacyCalls)
				}
			} else if err != nil || key != testPublicASCWidgetKey || legacyCalls != 1 {
				t.Fatalf("expected bundled fallback; error = %v, legacy calls = %d", err, legacyCalls)
			}
		})
	}
}

func TestLoginAuthServiceKeyFailureGuidance(t *testing.T) {
	transportErr := errors.New("fixture transport failure")
	tests := []struct {
		name   string
		status int
		body   string
		cause  error
	}{
		{name: "forbidden", status: http.StatusForbidden, body: `{}`},
		{name: "server failure", status: http.StatusInternalServerError, body: `{}`},
		{name: "invalid JSON", status: http.StatusOK, body: `<html>Unavailable</html>`},
		{name: "transport failure", cause: transportErr},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fixture := handlertest.New(t)
			var requests int
			client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				requests++
				if req.Method == http.MethodHead && req.URL.Path == "/logout" {
					return &http.Response{StatusCode: http.StatusServiceUnavailable, Header: make(http.Header), Body: http.NoBody, Request: req}, nil
				}
				if req.Method != http.MethodGet || req.URL.Host != "appstoreconnect.apple.com" || req.URL.Path != "/olympus/v1/app/config" {
					return nil, fixture.Errorf("unexpected request after bootstrap failure: %s %s", req.Method, req.URL)
				}
				if tt.cause != nil {
					return nil, tt.cause
				}
				return &http.Response{StatusCode: tt.status, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(tt.body)), Request: req}, nil
			})}
			session, err := loginWithHTTPClient(context.Background(), client, LoginCredentials{
				Username: "fixture@example.invalid", Password: "fixture-password",
			})
			if err == nil || session != nil {
				t.Fatal("expected bootstrap failure without a session")
			}
			const guidance = "could not load Apple login configuration; password authentication has not started. Run with --api-debug for request details: "
			if !strings.HasPrefix(err.Error(), guidance) {
				t.Errorf("missing pre-authentication diagnostic guidance: %v", err)
			}
			cause := errors.Unwrap(err)
			if cause == nil || strings.TrimPrefix(err.Error(), guidance) != cause.Error() {
				t.Errorf("bootstrap error must preserve its wrapped cause: %v", err)
			}
			if tt.cause != nil && !errors.Is(err, tt.cause) {
				t.Errorf("bootstrap error must wrap the transport failure: %v", err)
			}
			if requests != 2 {
				t.Errorf("requests = %d, want discovery and the legacy bootstrap request", requests)
			}
		})
	}
}

func TestGetAuthServiceKey(t *testing.T) {
	tests := []struct {
		name    string
		status  int
		body    string
		want    string
		wantErr bool
	}{
		{name: "discovered key wins", status: 200, body: `{"authServiceKey":" discovered ","serviceKey":"legacy"}`, want: "discovered"},
		{name: "legacy field", status: 200, body: `{"serviceKey":" legacy "}`, want: "legacy"},
		{name: "missing endpoint", status: 404, body: `<html>Not Found</html>`, want: testPublicASCWidgetKey},
		{name: "unauthorized", status: 401, body: `{}`, wantErr: true},
		{name: "forbidden", status: 403, body: `{}`, wantErr: true},
		{name: "server failure", status: 500, body: `{}`, wantErr: true},
		{name: "invalid JSON", status: 200, body: `<html>Unavailable</html>`, wantErr: true},
		{name: "empty key", status: 200, body: `{"authServiceKey":" "}`, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fixture := handlertest.New(t)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodHead && r.URL.Path == "/logout" {
					w.WriteHeader(http.StatusServiceUnavailable)
					return
				}
				if r.Method != http.MethodGet || r.URL.Path != "/olympus/v1/app/config" || r.URL.Query().Get("hostname") != "itunesconnect.apple.com" {
					fixture.Respond(w, "unexpected bootstrap request: %s %s", r.Method, r.URL)
					return
				}
				w.WriteHeader(tt.status)
				_, _ = w.Write([]byte(tt.body))
			}))
			defer server.Close()
			got, err := getAuthServiceKey(context.Background(), newTestServerRoutedClient(t, server))
			if (err != nil) != tt.wantErr {
				t.Fatalf("getAuthServiceKey error = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Fatal("getAuthServiceKey returned an unexpected key")
			}
		})
	}
}

func TestLoginContinuesToSRPAfterAuthServiceKey404(t *testing.T) {
	fixture := handlertest.New(t)
	var bootstrapCalls, signinCalls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/logout":
			w.WriteHeader(http.StatusServiceUnavailable)
		case "/olympus/v1/app/config":
			bootstrapCalls++
			w.WriteHeader(http.StatusNotFound)
		case "/appleauth/auth/signin/init":
			signinCalls++
			if r.Method != http.MethodPost || r.Header.Get("X-Apple-Widget-Key") != testPublicASCWidgetKey {
				fixture.Respond(w, "SRP did not receive the public ASC widget key in a POST")
				return
			}
			var payload struct {
				AccountName string `json:"accountName"`
				PublicKey   string `json:"a"`
			}
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil || payload.AccountName != "fixture@example.invalid" || payload.PublicKey == "" {
				fixture.Respond(w, "invalid SRP init payload")
				return
			}
			// Stop at a deliberate downstream failure: bootstrap must have succeeded.
			w.WriteHeader(http.StatusServiceUnavailable)
		default:
			fixture.Respond(w, "unexpected request: %s %s", r.Method, r.URL)
		}
	}))
	defer server.Close()
	session, err := loginWithHTTPClient(context.Background(), newTestServerRoutedClient(t, server), LoginCredentials{
		Username: "fixture@example.invalid", Password: "fixture-password",
	})
	if err == nil || session != nil {
		t.Fatal("expected the downstream SRP failure")
	}
	if bootstrapCalls != 1 || signinCalls != 1 {
		t.Fatalf("bootstrap requests = %d, SRP requests = %d; want one each", bootstrapCalls, signinCalls)
	}
}
