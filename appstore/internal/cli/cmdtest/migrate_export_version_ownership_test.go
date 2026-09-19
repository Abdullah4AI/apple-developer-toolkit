package cmdtest

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestMigrateExportRejectsVersionOwnedByAnotherAppBeforeOutput proves that an
// explicit --version-id is checked against --app before migrate export creates
// its output directory or fetches any metadata to write.
func TestMigrateExportRejectsVersionOwnedByAnotherAppBeforeOutput(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	outputDir := filepath.Join(t.TempDir(), "fastlane")
	var localizationRequests int
	installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/VERSION_B":
			body := `{"data":{"type":"appStoreVersions","id":"VERSION_B","attributes":{"versionString":"1.0","platform":"IOS"},"relationships":{"app":{"data":{"type":"apps","id":"APP_B"}}}}}`
			return migrateJSONResponse(http.StatusOK, body), nil
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/VERSION_B/appStoreVersionLocalizations":
			localizationRequests++
			return migrateJSONResponse(http.StatusOK, `{"data":[],"links":{"next":""}}`), nil
		default:
			return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.String())
		}
	}))

	rootCmd := RootCommand("1.2.3")
	rootCmd.FlagSet.SetOutput(io.Discard)

	var runErr error
	captureOutput(t, func() {
		if err := rootCmd.Parse([]string{
			"migrate", "export",
			"--app", "APP_A",
			"--version-id", "VERSION_B",
			"--output-dir", outputDir,
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = rootCmd.Run(context.Background())
	})

	if runErr == nil {
		t.Fatal("expected migrate export to fail for a version owned by another app")
	}
	if !strings.Contains(runErr.Error(), "belongs to app") {
		t.Fatalf("expected ownership error, got %v", runErr)
	}
	if localizationRequests != 0 {
		t.Fatalf("expected ownership rejection before localization fetch, got %d requests", localizationRequests)
	}
	if _, err := os.Stat(outputDir); !os.IsNotExist(err) {
		t.Fatalf("expected no output directory side effect, stat error = %v", err)
	}
}
