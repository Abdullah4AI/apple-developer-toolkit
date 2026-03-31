package web

import (
	"context"
	"errors"
	"flag"
	"strings"
	"testing"

	webcore "github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/web"
)

func TestWebAppsMedicalDeviceSetValidationErrors(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{name: "missing app", args: []string{"--declared", "false"}, wantErr: "--app is required"},
		{name: "missing declared", args: []string{"--app", "app-1"}, wantErr: "--declared is required"},
		{name: "true unsupported", args: []string{"--app", "app-1", "--declared", "true"}, wantErr: "--declared true is not yet supported"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("ASC_APP_ID", "")
			cmd := WebAppsMedicalDeviceSetCommand()
			if err := cmd.FlagSet.Parse(tc.args); err != nil {
				t.Fatalf("parse error: %v", err)
			}
			stdout, stderr := captureWebCommandOutput(t, func() {
				err := cmd.Exec(context.Background(), nil)
				if !errors.Is(err, flag.ErrHelp) {
					t.Fatalf("expected flag.ErrHelp, got %v", err)
				}
			})
			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if !strings.Contains(stderr, tc.wantErr) {
				t.Fatalf("expected stderr to contain %q, got %q", tc.wantErr, stderr)
			}
		})
	}
}

func TestWebAppsMedicalDeviceSetResolvesSessionBeforeTimeoutContext(t *testing.T) {
	origResolveSession := resolveSessionFn
	t.Cleanup(func() {
		resolveSessionFn = origResolveSession
	})

	resolveErr := errors.New("stop before network call")
	hadDeadline := false
	resolveSessionFn = func(ctx context.Context, appleID, password, twoFactorCode string) (*webcore.AuthSession, string, error) {
		_, hadDeadline = ctx.Deadline()
		return nil, "", resolveErr
	}

	cmd := WebAppsMedicalDeviceSetCommand()
	if err := cmd.FlagSet.Parse([]string{
		"--app", "app-1",
		"--declared", "false",
		"--apple-id", "user@example.com",
	}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	err := cmd.Exec(context.Background(), nil)
	if !errors.Is(err, resolveErr) {
		t.Fatalf("expected resolveSession error %v, got %v", resolveErr, err)
	}
	if hadDeadline {
		t.Fatal("expected resolveSession to run before timeout context creation")
	}
}

func TestWebAppsMedicalDeviceSetUpdatesDeclaration(t *testing.T) {
	origResolveSession := resolveSessionFn
	origNewWebClient := newWebClientFn
	origSet := setWebMedicalDeviceDeclarationFn
	t.Cleanup(func() {
		resolveSessionFn = origResolveSession
		newWebClientFn = origNewWebClient
		setWebMedicalDeviceDeclarationFn = origSet
	})

	resolveSessionFn = func(ctx context.Context, appleID, password, twoFactorCode string) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{PublicProviderID: "account-123"}, "cache", nil
	}
	newWebClientFn = func(session *webcore.AuthSession) *webcore.Client {
		return &webcore.Client{}
	}

	var gotAccountID string
	var gotAppID string
	var gotDeclared bool
	setWebMedicalDeviceDeclarationFn = func(ctx context.Context, client *webcore.Client, accountID, appID string, declared bool) (*webcore.MedicalDeviceDeclarationResult, error) {
		gotAccountID = accountID
		gotAppID = appID
		gotDeclared = declared
		return &webcore.MedicalDeviceDeclarationResult{
			AppID:              appID,
			RequirementID:      "req-123",
			RequirementName:    "MEDICAL_DEVICE",
			Status:             "COLLECTED",
			Declared:           false,
			CountriesOrRegions: []string{"EEA", "GBR", "USA"},
		}, nil
	}

	cmd := WebAppsMedicalDeviceSetCommand()
	if err := cmd.FlagSet.Parse([]string{
		"--app", "app-1",
		"--declared", "false",
		"--output", "json",
	}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	stdout, stderr := captureWebCommandOutput(t, func() {
		if err := cmd.Exec(context.Background(), nil); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"appId":"app-1"`) {
		t.Fatalf("expected app id in stdout, got %q", stdout)
	}
	if !strings.Contains(stdout, `"declared":false`) {
		t.Fatalf("expected declared=false in stdout, got %q", stdout)
	}
	if gotAccountID != "account-123" {
		t.Fatalf("expected account-123, got %q", gotAccountID)
	}
	if gotAppID != "app-1" {
		t.Fatalf("expected app-1, got %q", gotAppID)
	}
	if gotDeclared {
		t.Fatal("expected declared=false")
	}
}

func TestWebAppsMedicalDeviceSetRequiresPublicProviderID(t *testing.T) {
	origResolveSession := resolveSessionFn
	t.Cleanup(func() {
		resolveSessionFn = origResolveSession
	})

	resolveSessionFn = func(ctx context.Context, appleID, password, twoFactorCode string) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{}, "cache", nil
	}

	cmd := WebAppsMedicalDeviceSetCommand()
	if err := cmd.FlagSet.Parse([]string{
		"--app", "app-1",
		"--declared", "false",
	}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	err := cmd.Exec(context.Background(), nil)
	if err == nil || !strings.Contains(err.Error(), "public provider/account id") {
		t.Fatalf("expected missing account id error, got %v", err)
	}
}
