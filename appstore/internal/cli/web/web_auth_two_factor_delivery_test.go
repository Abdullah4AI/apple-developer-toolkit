package web

import (
	"context"
	"fmt"
	"io"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/appleauth"
	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/cli/shared"
	webcore "github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/web"
)

// stubTwoFactorDeliveryLogin stubs a sign-in that reaches 2FA and records the
// delivery preference visible when the challenge is prepared.
func stubTwoFactorDeliveryLogin(t *testing.T) (*appleauth.TwoFactorPreference, *bool) {
	t.Helper()
	origLogin := webLoginFn
	origPrepare := prepareTwoFactorChallengeFn
	origEnsure := ensureTwoFactorCodeRequestedFn
	origSubmit := submitTwoFactorCodeFn
	origStatusWriter := twoFactorStatusWriter
	t.Cleanup(func() {
		webLoginFn = origLogin
		prepareTwoFactorChallengeFn = origPrepare
		ensureTwoFactorCodeRequestedFn = origEnsure
		submitTwoFactorCodeFn = origSubmit
		twoFactorStatusWriter = origStatusWriter
	})

	var (
		seen     appleauth.TwoFactorPreference
		loggedIn bool
	)
	webLoginFn = func(ctx context.Context, creds webcore.LoginCredentials) (*webcore.AuthSession, error) {
		loggedIn = true
		return &webcore.AuthSession{}, &webcore.TwoFactorRequiredError{}
	}
	prepareTwoFactorChallengeFn = func(ctx context.Context, session *webcore.AuthSession) (*webcore.TwoFactorChallenge, error) {
		seen = session.TwoFactorPreference()
		return &webcore.TwoFactorChallenge{Method: "phone", Destination: "+1 (•••) •••-••22"}, nil
	}
	ensureTwoFactorCodeRequestedFn = func(ctx context.Context, session *webcore.AuthSession) (*webcore.TwoFactorChallenge, error) {
		return &webcore.TwoFactorChallenge{Method: "phone", Destination: "+1 (•••) •••-••22", Requested: true}, nil
	}
	submitTwoFactorCodeFn = func(ctx context.Context, session *webcore.AuthSession, code string) error {
		return nil
	}
	twoFactorStatusWriter = io.Discard
	return &seen, &loggedIn
}

func readStubTwoFactorCode(context.Context, string) (string, error) {
	return "123456", nil
}

func TestLoginWithOptionalTwoFactorAppliesPreferenceLayeredOverTimeout(t *testing.T) {
	t.Setenv(webTwoFactorMethodEnv, "")
	t.Setenv(webTwoFactorPhoneIDEnv, "")
	seen, _ := stubTwoFactorDeliveryLogin(t)

	// The sign-in flow strips request timeouts, which also drops values that
	// were layered above them; the preference must survive that.
	timedCtx, cancel := shared.ContextWithTimeout(context.Background())
	defer cancel()
	want := appleauth.TwoFactorPreference{Delivery: appleauth.TwoFactorDeliverySMS, PhoneID: 2}
	ctx := contextWithTwoFactorPreference(timedCtx, want)

	if _, err := loginWithOptionalTwoFactor(ctx, "user@example.com", "secret", "", nil, readStubTwoFactorCode, "read-code"); err != nil {
		t.Fatalf("loginWithOptionalTwoFactor returned error: %v", err)
	}
	if *seen != want {
		t.Fatalf("challenge saw preference %+v, want %+v", *seen, want)
	}
}

func TestLoginWithOptionalTwoFactorFallsBackToEnvironmentPreference(t *testing.T) {
	t.Setenv(webTwoFactorMethodEnv, "SMS")
	t.Setenv(webTwoFactorPhoneIDEnv, "7")
	seen, _ := stubTwoFactorDeliveryLogin(t)

	if _, err := loginWithOptionalTwoFactor(context.Background(), "user@example.com", "secret", "", nil, readStubTwoFactorCode, "read-code"); err != nil {
		t.Fatalf("loginWithOptionalTwoFactor returned error: %v", err)
	}
	want := appleauth.TwoFactorPreference{Delivery: appleauth.TwoFactorDeliverySMS, PhoneID: 7}
	if *seen != want {
		t.Fatalf("challenge saw preference %+v, want %+v", *seen, want)
	}
}

func TestLoginWithOptionalTwoFactorRejectsInvalidEnvironmentBeforeSignIn(t *testing.T) {
	t.Setenv(webTwoFactorMethodEnv, "push")
	t.Setenv(webTwoFactorPhoneIDEnv, "")
	_, loggedIn := stubTwoFactorDeliveryLogin(t)

	_, err := loginWithOptionalTwoFactor(context.Background(), "user@example.com", "secret", "", nil, readStubTwoFactorCode, "read-code")
	if err == nil || !strings.Contains(err.Error(), webTwoFactorMethodEnv+" must be one of: auto, sms") {
		t.Fatalf("expected invalid environment error, got %v", err)
	}
	if *loggedIn {
		t.Fatal("expected validation to fail before sending credentials")
	}
}

func TestLoginWithOptionalTwoFactorHintsPhoneIDForAmbiguousSMSPhone(t *testing.T) {
	t.Setenv(webTwoFactorMethodEnv, "")
	t.Setenv(webTwoFactorPhoneIDEnv, "")
	stubTwoFactorDeliveryLogin(t)
	prepareTwoFactorChallengeFn = func(ctx context.Context, session *webcore.AuthSession) (*webcore.TwoFactorChallenge, error) {
		return nil, &appleauth.TrustedPhoneSelectionError{Phones: []appleauth.TrustedPhoneNumber{
			{ID: 1, NumberWithDialCode: "+1 (•••) •••-••11"},
			{ID: 2, NumberWithDialCode: "+1 (•••) •••-••22"},
		}}
	}
	ctx := contextWithTwoFactorPreference(context.Background(), appleauth.TwoFactorPreference{Delivery: appleauth.TwoFactorDeliverySMS})

	_, err := loginWithOptionalTwoFactor(ctx, "user@example.com", "secret", "", nil, readStubTwoFactorCode, "read-code")
	want := "2fa challenge setup failed: multiple trusted phone numbers are available; select one by ID: 1 (+1 (•••) •••-••11), 2 (+1 (•••) •••-••22); pass --two-factor-phone-id or set " + webTwoFactorPhoneIDEnv
	if err == nil || err.Error() != want {
		t.Fatalf("unexpected error:\n got: %v\nwant: %s", err, want)
	}
}

func TestLoginWithOptionalTwoFactorRejectedPhoneCodeGuidanceFollowsDeliveryMode(t *testing.T) {
	tests := []struct {
		mode     string
		wantHint bool
	}{
		{mode: "sms", wantHint: true},
		{mode: "voice", wantHint: false},
	}
	for _, tt := range tests {
		t.Run(tt.mode, func(t *testing.T) {
			t.Setenv(webTwoFactorMethodEnv, "")
			t.Setenv(webTwoFactorPhoneIDEnv, "")
			stubTwoFactorDeliveryLogin(t)
			prepareTwoFactorChallengeFn = func(ctx context.Context, session *webcore.AuthSession) (*webcore.TwoFactorChallenge, error) {
				session.SetPreparedTwoFactorState("phone", 1, tt.mode, "+1 (•••) •••-••11", false)
				return &webcore.TwoFactorChallenge{Method: "phone", Destination: "+1 (•••) •••-••11"}, nil
			}
			submitTwoFactorCodeFn = func(ctx context.Context, session *webcore.AuthSession, code string) error {
				return fmt.Errorf("phone 2fa failed (status 400, codes=[-21669]): %w", webcore.ErrTwoFactorCodeRejected)
			}

			_, err := loginWithOptionalTwoFactor(context.Background(), "user@example.com", "secret", "", nil, readStubTwoFactorCode, "read-code")
			if err == nil {
				t.Fatal("expected rejected code error")
			}
			if !strings.Contains(err.Error(), "2fa verification failed: Apple rejected the verification code") {
				t.Fatalf("expected readable rejection, got %v", err)
			}
			if got := strings.Contains(err.Error(), "latest SMS"); got != tt.wantHint {
				t.Fatalf("SMS hint present = %v, want %v; error = %v", got, tt.wantHint, err)
			}
		})
	}
}

func TestResolveTwoFactorPreference(t *testing.T) {
	tests := []struct {
		name    string
		method  string
		phoneID int
		env     map[string]string
		want    appleauth.TwoFactorPreference
	}{
		{name: "default"},
		{name: "explicit auto", method: "auto"},
		{name: "sms flag", method: "sms", want: appleauth.TwoFactorPreference{Delivery: "sms"}},
		{name: "sms flag with phone", method: " SMS ", phoneID: 3, want: appleauth.TwoFactorPreference{Delivery: "sms", PhoneID: 3}},
		{name: "sms env", env: map[string]string{webTwoFactorMethodEnv: "sms", webTwoFactorPhoneIDEnv: "4"}, want: appleauth.TwoFactorPreference{Delivery: "sms", PhoneID: 4}},
		{name: "flag phone overrides env phone", method: "sms", phoneID: 5, env: map[string]string{webTwoFactorPhoneIDEnv: "4"}, want: appleauth.TwoFactorPreference{Delivery: "sms", PhoneID: 5}},
		{name: "auto flag overrides env sms", method: "auto", env: map[string]string{webTwoFactorMethodEnv: "sms"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(webTwoFactorMethodEnv, "")
			t.Setenv(webTwoFactorPhoneIDEnv, "")
			for key, value := range tt.env {
				t.Setenv(key, value)
			}
			got, err := resolveTwoFactorPreference(tt.method, tt.phoneID, tt.phoneID != 0)
			if err != nil {
				t.Fatalf("resolveTwoFactorPreference() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("resolveTwoFactorPreference() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestReadTwoFactorCodeFromCommandExportsCodeRequestTime(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses a POSIX shell command")
	}
	requestedAt := time.Date(2026, 10, 17, 21, 15, 43, 123000000, time.UTC)
	ctx := contextWithTwoFactorCodeRequestedAt(context.Background(), requestedAt)

	got, err := readTwoFactorCodeFromCommand(ctx, `printf %s "$`+webTwoFactorCodeRequestedAtEnv+`"`)
	if err != nil {
		t.Fatalf("readTwoFactorCodeFromCommand() error = %v", err)
	}
	if got != "2026-10-17T21:15:43.123Z" {
		t.Fatalf("command saw %s=%q", webTwoFactorCodeRequestedAtEnv, got)
	}
}

func TestLoginWithOptionalTwoFactorPassesCodeRequestTimeToCommand(t *testing.T) {
	t.Setenv(webTwoFactorMethodEnv, "")
	t.Setenv(webTwoFactorPhoneIDEnv, "")
	stubTwoFactorDeliveryLogin(t)
	var requestedSMSAt time.Time
	ensureTwoFactorCodeRequestedFn = func(ctx context.Context, session *webcore.AuthSession) (*webcore.TwoFactorChallenge, error) {
		requestedSMSAt = time.Now()
		return &webcore.TwoFactorChallenge{Method: "phone", Requested: true}, nil
	}
	var (
		seen   time.Time
		seenOK bool
	)
	readCode := func(ctx context.Context, command string) (string, error) {
		seen, seenOK = twoFactorCodeRequestedAt(ctx)
		return "123456", nil
	}

	if _, err := loginWithOptionalTwoFactor(context.Background(), "user@example.com", "secret", "", nil, readCode, "read-code"); err != nil {
		t.Fatalf("loginWithOptionalTwoFactor returned error: %v", err)
	}
	if !seenOK {
		t.Fatal("expected the code command to receive the code request time")
	}
	if seen.After(requestedSMSAt) {
		t.Fatalf("code request time %v is after the SMS request at %v", seen, requestedSMSAt)
	}
}
