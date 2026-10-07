package cmd

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/cli/shared"
	webcli "github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/cli/web"
	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/telemetry"
	webcore "github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/web"
)

type webTwoFactorRoundTripFunc func(*http.Request) (*http.Response, error)

func (fn webTwoFactorRoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func TestRunWebAuthLoginFailuresAreClassified(t *testing.T) {
	tests := []struct {
		name          string
		codeCommand   string
		extraArgs     []string
		authOptions   int
		signedIn      bool
		wantSignIn    bool
		wantSubmit    bool
		wantExit      int
		wantStderr    string
		wantOutcome   telemetry.OutcomeKind
		wantDiagnosis shared.DiagnosticCode
		wantParameter string
	}{
		{
			name:          "no terminal and no code command fails before sign-in",
			wantExit:      ExitError,
			wantStderr:    "Error: 2fa required: run in a terminal for an interactive prompt, pass --two-factor-code-command, or set ASC_WEB_2FA_CODE_COMMAND\n",
			wantOutcome:   telemetry.OutcomeExpectedNegative,
			wantDiagnosis: shared.DiagnosticRequiredInputMissing,
			wantParameter: "--two-factor-code-command",
		},
		{
			name:          "failing code command",
			codeCommand:   "echo 'no code available' >&2; exit 3",
			wantSignIn:    true,
			wantExit:      ExitError,
			wantStderr:    "Error: web auth login failed: 2fa required: two-factor code command failed: no code available\n",
			wantOutcome:   telemetry.OutcomeExpectedNegative,
			wantDiagnosis: shared.DiagnosticDependencyFailed,
			wantParameter: "--two-factor-code-command",
		},
		{
			name:          "rejected code",
			codeCommand:   "echo 000000",
			wantSignIn:    true,
			wantSubmit:    true,
			wantExit:      ExitAuth,
			wantStderr:    "Error: web auth login failed: 2fa verification failed: trusted-device 2fa failed (status 400, codes=[-21669])\n",
			wantOutcome:   telemetry.OutcomeAuthError,
			wantDiagnosis: shared.DiagnosticAuthenticationRejected,
		},
		{
			name:        "2fa challenge setup keeps the HTTP status",
			codeCommand: "echo 000000",
			authOptions: http.StatusServiceUnavailable,
			wantSignIn:  true,
			wantExit:    ExitHTTPServiceUnavailable,
			wantStderr:  "Error: web auth login failed: 2fa challenge setup failed: auth options failed: web api error (status 503)\n",
			wantOutcome: telemetry.OutcomeAPIServerError,
		},
		{
			name:          "unknown provider",
			codeCommand:   "echo 000000",
			extraArgs:     []string{"--public-provider-id", "MISSING"},
			signedIn:      true,
			wantSignIn:    true,
			wantExit:      ExitError,
			wantStderr:    "Error: web provider selection failed: provider selection public-provider-id=MISSING did not match any available providers (available: 7 / TEAM7 / Example Team)\n",
			wantOutcome:   telemetry.OutcomeExpectedNegative,
			wantDiagnosis: shared.DiagnosticInvalidInput,
			wantParameter: "--public-provider-id",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			resetReportFlags(t)
			tempDir := t.TempDir()
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(tempDir, "config.json"))
			t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
			t.Setenv("ASC_WEB_SESSION_CACHE", "1")
			t.Setenv("ASC_WEB_SESSION_CACHE_BACKEND", "file")
			t.Setenv("ASC_WEB_SESSION_CACHE_DIR", filepath.Join(tempDir, "web-sessions"))
			t.Setenv("ASC_WEB_DONT_STORE_PASSWORD", "1")
			t.Setenv(strings.Join([]string{"ASC", "WEB", "APPLE", "ID"}, "_"), "")
			t.Setenv(strings.Join([]string{"ASC", "WEB", "PASSWORD"}, "_"), "secret")
			t.Setenv("ASC_WEB_2FA_CODE_COMMAND", test.codeCommand)

			devNull, err := os.Open(os.DevNull)
			if err != nil {
				t.Fatalf("open %s: %v", os.DevNull, err)
			}
			originalStdin := os.Stdin
			os.Stdin = devNull
			t.Cleanup(func() {
				os.Stdin = originalStdin
				_ = devNull.Close()
			})
			t.Cleanup(webcli.DisableControllingTTYForTesting())

			var appleRequests, submits int
			jar, err := cookiejar.New(nil)
			if err != nil {
				t.Fatalf("cookiejar.New() error: %v", err)
			}
			client := &http.Client{Jar: jar, Transport: webTwoFactorRoundTripFunc(func(req *http.Request) (*http.Response, error) {
				appleRequests++
				status, body := http.StatusOK, `{"trustedDevices":[{}],"securityCode":{"length":6}}`
				switch {
				case req.Method == http.MethodGet && strings.HasSuffix(req.URL.Path, "/appleauth/auth"):
					if test.authOptions != 0 {
						status, body = test.authOptions, `{}`
					}
				case req.Method == http.MethodPost && strings.HasSuffix(req.URL.Path, "/verify/trusteddevice/securitycode"):
					submits++
					status, body = http.StatusBadRequest, `{"serviceErrors":[{"code":"-21669"}]}`
				case req.Method == http.MethodGet && strings.HasSuffix(req.URL.Path, "/olympus/v1/session"):
					body = `{"provider":{"providerId":7,"publicProviderId":"TEAM7","name":"Example Team"},"user":{"emailAddress":"user@example.com"}}`
				default:
					t.Errorf("unexpected Apple request %s %s", req.Method, req.URL)
				}
				return &http.Response{
					StatusCode: status,
					Header:     http.Header{"Content-Type": []string{"application/json"}},
					Body:       io.NopCloser(strings.NewReader(body)),
					Request:    req,
				}, nil
			})}
			signIns := 0
			t.Cleanup(webcli.SetWebLogin(func(_ context.Context, creds webcore.LoginCredentials) (*webcore.AuthSession, error) {
				signIns++
				if test.signedIn {
					return &webcore.AuthSession{Client: client, UserEmail: creds.Username, ProviderID: 7, PublicProviderID: "TEAM7"}, nil
				}
				partial := &webcore.AuthSession{
					Client:           client,
					ServiceKey:       "widget-key",
					AppleIDSessionID: "session-id",
					SCNT:             "scnt",
					UserEmail:        creds.Username,
				}
				return partial, fmt.Errorf("srp login failed: %w", &webcore.TwoFactorRequiredError{})
			}))

			originalEmitTelemetry := emitTelemetry
			t.Cleanup(func() { emitTelemetry = originalEmitTelemetry })
			var gotContext telemetry.EventContext
			emitTelemetry = func(_ string, _ string, _ time.Duration, _ int, eventContext telemetry.EventContext) {
				gotContext = eventContext
			}

			var code int
			stdout, stderr := captureCommandOutput(t, func() {
				code = Run(append([]string{"web", "auth", "login", "--apple-id", "user@example.com"}, test.extraArgs...), "5.10.0")
			})

			if code != test.wantExit {
				t.Fatalf("exit code = %d, want %d; stderr=%q", code, test.wantExit, stderr)
			}
			if stdout != "" {
				t.Fatalf("stdout = %q, want empty", stdout)
			}
			if !strings.HasSuffix(stderr, test.wantStderr) {
				t.Fatalf("stderr = %q, want suffix %q", stderr, test.wantStderr)
			}
			if gotSignIn := signIns > 0; gotSignIn != test.wantSignIn {
				t.Fatalf("sign-in attempted = %t, want %t", gotSignIn, test.wantSignIn)
			}
			if !test.wantSignIn && appleRequests != 0 {
				t.Fatalf("Apple requests = %d, want none before sign-in", appleRequests)
			}
			if gotSubmit := submits > 0; gotSubmit != test.wantSubmit {
				t.Fatalf("code submitted = %t, want %t", gotSubmit, test.wantSubmit)
			}
			if gotContext.OutcomeKind != test.wantOutcome ||
				gotContext.DiagnosticCode != string(test.wantDiagnosis) ||
				gotContext.FailureParameter != test.wantParameter {
				t.Fatalf("telemetry context = %+v, want outcome %q, diagnostic %q, parameter %q", gotContext, test.wantOutcome, test.wantDiagnosis, test.wantParameter)
			}
		})
	}
}
