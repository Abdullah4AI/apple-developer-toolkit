package web

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/handlertest"
)

// Captured from Apple's public production ASC login configuration on 2026-09-11.
const testPublicASCWidgetKey = "e0b80c3bf78523bfe80974d320935bfa30add02e1bff88ec2166c6bd5a706c42"

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
			if requests != 1 {
				t.Errorf("requests = %d, want only the bootstrap request", requests)
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
