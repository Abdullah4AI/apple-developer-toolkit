package cmdtest

import (
	"path/filepath"
	"testing"

	rootcmd "github.com/Abdullah4AI/apple-developer-toolkit/appstore/cmd"
)

// TestPricingInputValidationReturnsUsageExitCode locks the usage-error contract
// for pricing pre-request flag validation: every check must print
// "Error: <message>" to stderr, naming the command path the operator invoked,
// and exit with code 2 rather than the generic runtime failure code.
//
// Part of #518.
func TestPricingInputValidationReturnsUsageExitCode(t *testing.T) {
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "territories list limit 201",
			args:    []string{"pricing", "territories", "list", "--limit", "201"},
			wantErr: "pricing territories list: --limit must be between 1 and 200",
		},
		{
			name:    "territories list next non apple url",
			args:    []string{"pricing", "territories", "list", "--next", "http://example.com/x"},
			wantErr: "pricing territories list: --next must be an App Store Connect URL",
		},
		{
			name:    "price-points app 123 limit 201",
			args:    []string{"pricing", "price-points", "--app", "123", "--limit", "201"},
			wantErr: "pricing price-points: --limit must be between 1 and 200",
		},
		{
			name:    "price-points app 123 next non apple url",
			args:    []string{"pricing", "price-points", "--app", "123", "--next", "http://example.com/x"},
			wantErr: "pricing price-points: --next must be an App Store Connect URL",
		},
		{
			name:    "schedule manual-prices schedule s next non apple url",
			args:    []string{"pricing", "schedule", "manual-prices", "--schedule", "s", "--next", "http://example.com/x"},
			wantErr: "pricing schedule manual-prices: --next must be an App Store Connect URL",
		},
		{
			name:    "schedule automatic-prices schedule s next non apple url",
			args:    []string{"pricing", "schedule", "automatic-prices", "--schedule", "s", "--next", "http://example.com/x"},
			wantErr: "pricing schedule automatic-prices: --next must be an App Store Connect URL",
		},
		{
			name:    "schedule automatic-prices next without schedule",
			args:    []string{"pricing", "schedule", "automatic-prices", "--next", "http://api.appstoreconnect.apple.com/v1/x"},
			wantErr: "pricing schedule automatic-prices: --next must be an App Store Connect URL",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			stdout, stderr, runErr := runCommand(t, test.args)

			if runErr == nil {
				t.Fatal("expected error, got nil")
			}
			if got := rootcmd.ExitCodeFromError(runErr); got != rootcmd.ExitUsage {
				t.Fatalf("exit code = %d, want %d (err=%v)", got, rootcmd.ExitUsage, runErr)
			}
			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			assertUsageErrorStderr(t, stderr, test.wantErr)
		})
	}
}
