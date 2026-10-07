package cmdtest

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	rootcmd "github.com/Abdullah4AI/apple-developer-toolkit/appstore/cmd"
)

type localizationImportReceipt struct {
	Type      string `json:"type"`
	VersionID string `json:"versionId"`
	DryRun    bool   `json:"dryRun"`
	Total     int    `json:"total"`
	Planned   int    `json:"planned"`
	Succeeded int    `json:"succeeded"`
	Skipped   int    `json:"skipped"`
	Failed    int    `json:"failed"`
	Results   []struct {
		Locale         string   `json:"locale"`
		Action         string   `json:"action"`
		Status         string   `json:"status"`
		Fields         []string `json:"fields"`
		LocalizationID string   `json:"localizationId"`
		Error          string   `json:"error"`
	} `json:"results"`
}

func writeLocalizationImportFile(t *testing.T, contents string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "localizations.json")
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatalf("write import file: %v", err)
	}
	return path
}

func decodeLocalizationImportReceipt(t *testing.T, stdout string) localizationImportReceipt {
	t.Helper()
	var receipt localizationImportReceipt
	if err := json.Unmarshal([]byte(stdout), &receipt); err != nil {
		t.Fatalf("unmarshal receipt: %v\nstdout=%s", err, stdout)
	}
	return receipt
}

func localizationImportSummary(receipt localizationImportReceipt) string {
	lines := make([]string, 0, len(receipt.Results))
	for _, item := range receipt.Results {
		lines = append(lines, fmt.Sprintf("%s %s %s [%s] %s", item.Locale, item.Action, item.Status, strings.Join(item.Fields, ","), item.LocalizationID))
	}
	return strings.Join(lines, "\n")
}

func TestSubscriptionVersionLocalizationsImportCreatesUpdatesAndSkips(t *testing.T) {
	setupAuth(t)
	filePath := writeLocalizationImportFile(t, `{
  "en-US": {"name": "Premium", "description": "All features"},
  "de_DE": {"name": "Premium", "description": "Alle Funktionen"},
  "fr-FR": {"name": "Premium FR", "description": "Toutes les fonctions"}
}`)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })

	var calls []string
	var bodies []string
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		calls = append(calls, req.Method+" "+req.URL.Path)
		if req.Body != nil {
			raw, _ := io.ReadAll(req.Body)
			bodies = append(bodies, strings.TrimSpace(string(raw)))
		}
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptionVersions/ver-1/localizations":
			if req.URL.Query().Get("limit") != "200" {
				t.Fatalf("expected limit=200, got %q", req.URL.RawQuery)
			}
			return jsonResponse(http.StatusOK, `{"data":[
				{"type":"subscriptionLocalizations","id":"loc-en","attributes":{"locale":"en-US","name":"Premium","description":"All features"}},
				{"type":"subscriptionLocalizations","id":"loc-de","attributes":{"locale":"de-DE","name":"Premium","description":"Alt"}}
			]}`)
		case req.Method == http.MethodPatch && req.URL.Path == "/v2/subscriptionLocalizations/loc-de":
			return jsonResponse(http.StatusOK, `{"data":{"type":"subscriptionLocalizations","id":"loc-de","attributes":{}}}`)
		case req.Method == http.MethodPost && req.URL.Path == "/v2/subscriptionLocalizations":
			return jsonResponse(http.StatusCreated, `{"data":{"type":"subscriptionLocalizations","id":"loc-fr","attributes":{}}}`)
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})

	stdout, stderr, runErr := runIAPImport(t, []string{
		"subscriptions", "versions", "localizations", "import",
		"--version-id", "ver-1",
		"--file", filePath,
		"--confirm",
		"--output", "json",
	})
	if runErr != nil {
		t.Fatalf("expected success, got %v (stderr=%q)", runErr, stderr)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	wantCalls := "GET /v1/subscriptionVersions/ver-1/localizations\nPATCH /v2/subscriptionLocalizations/loc-de\nPOST /v2/subscriptionLocalizations"
	if got := strings.Join(calls, "\n"); got != wantCalls {
		t.Fatalf("unexpected calls:\ngot:\n%s\nwant:\n%s", got, wantCalls)
	}
	wantPatch := `{"data":{"type":"subscriptionLocalizations","id":"loc-de","attributes":{"description":"Alle Funktionen"}}}`
	if bodies[0] != wantPatch {
		t.Fatalf("expected update to send only the changed field\ngot:  %s\nwant: %s", bodies[0], wantPatch)
	}
	wantCreate := `{"data":{"type":"subscriptionLocalizations","attributes":{"name":"Premium FR","locale":"fr-FR","description":"Toutes les fonctions"},"relationships":{"version":{"data":{"type":"subscriptionVersions","id":"ver-1"}}}}}`
	if bodies[1] != wantCreate {
		t.Fatalf("unexpected create body\ngot:  %s\nwant: %s", bodies[1], wantCreate)
	}

	receipt := decodeLocalizationImportReceipt(t, stdout)
	if receipt.Type != "subscriptionLocalizations" || receipt.VersionID != "ver-1" || receipt.DryRun {
		t.Fatalf("unexpected receipt header: %+v", receipt)
	}
	if receipt.Total != 3 || receipt.Succeeded != 2 || receipt.Skipped != 1 || receipt.Planned != 0 || receipt.Failed != 0 {
		t.Fatalf("unexpected receipt counts: %+v", receipt)
	}
	wantResults := "en-US skip skipped [] loc-en\nde-DE update succeeded [description] loc-de\nfr-FR create succeeded [name,description] loc-fr"
	if got := localizationImportSummary(receipt); got != wantResults {
		t.Fatalf("unexpected results:\ngot:\n%s\nwant:\n%s", got, wantResults)
	}
}

func TestIAPVersionLocalizationsImportDryRunMakesNoWrites(t *testing.T) {
	setupAuth(t)
	filePath := writeLocalizationImportFile(t, `{
  "en-US": {"name": "Coins"},
  "ja": {"name": "コイン", "description": "コインの山"}
}`)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method == http.MethodGet && req.URL.Path == "/v1/inAppPurchaseVersions/iap-ver/localizations" {
			return jsonResponse(http.StatusOK, `{"data":[
				{"type":"inAppPurchaseLocalizations","id":"loc-en","attributes":{"locale":"en-US","name":"Gold","description":"A pile of coins"}}
			]}`)
		}
		t.Fatalf("unexpected request during dry run: %s %s", req.Method, req.URL.String())
		return nil, nil
	})

	stdout, stderr, runErr := runIAPImport(t, []string{
		"iap", "versions", "localizations", "import",
		"--version-id", "iap-ver",
		"--file", filePath,
		"--dry-run",
		"--output", "json",
	})
	if runErr != nil {
		t.Fatalf("expected success, got %v (stderr=%q)", runErr, stderr)
	}

	receipt := decodeLocalizationImportReceipt(t, stdout)
	if receipt.Type != "inAppPurchaseLocalizations" || !receipt.DryRun || receipt.Total != 2 || receipt.Planned != 2 || receipt.Succeeded != 0 {
		t.Fatalf("unexpected receipt: %+v", receipt)
	}
	wantResults := "en-US update planned [name] loc-en\nja create planned [name,description] "
	if got := localizationImportSummary(receipt); got != wantResults {
		t.Fatalf("unexpected plan:\ngot:\n%s\nwant:\n%s", got, wantResults)
	}
}

func TestSubscriptionGroupVersionLocalizationsImportReportsPartialFailure(t *testing.T) {
	setupAuth(t)
	filePath := writeLocalizationImportFile(t, `{
  "en-US": {"customAppName": "Premium App"},
  "it": {"name": "Premium"},
  "es-MX": {"name": "Premium MX"}
}`)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })

	var calls []string
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		calls = append(calls, req.Method+" "+req.URL.Path)
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptionGroupVersions/group-ver/localizations":
			return jsonResponse(http.StatusOK, `{"data":[
				{"type":"subscriptionGroupLocalizations","id":"loc-en","attributes":{"locale":"en-US","name":"Premium"}}
			]}`)
		case req.Method == http.MethodPatch && req.URL.Path == "/v2/subscriptionGroupLocalizations/loc-en":
			raw, _ := io.ReadAll(req.Body)
			if !strings.Contains(string(raw), `"attributes":{"customAppName":"Premium App"}`) {
				t.Fatalf("expected only customAppName in the update, got %s", raw)
			}
			return jsonResponse(http.StatusOK, `{"data":{"type":"subscriptionGroupLocalizations","id":"loc-en","attributes":{}}}`)
		case req.Method == http.MethodPost && req.URL.Path == "/v2/subscriptionGroupLocalizations":
			raw, _ := io.ReadAll(req.Body)
			if strings.Contains(string(raw), `"locale":"it"`) {
				return jsonResponse(http.StatusConflict, `{"errors":[{"status":"409","code":"ENTITY_ERROR","title":"Conflict","detail":"locale rejected"}]}`)
			}
			return jsonResponse(http.StatusCreated, `{"data":{"type":"subscriptionGroupLocalizations","id":"loc-mx","attributes":{}}}`)
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})

	stdout, stderr, runErr := runIAPImport(t, []string{
		"subscriptions", "groups", "versions", "localizations", "import",
		"--version-id", "group-ver",
		"--file", filePath,
		"--confirm",
		"--output", "json",
	})
	if code := rootcmd.ExitCodeFromError(runErr); code != rootcmd.ExitError {
		t.Fatalf("expected exit %d for a partial failure, got %d: %v", rootcmd.ExitError, code, runErr)
	}
	if !strings.Contains(stderr, "1 of 3 locales failed") {
		t.Fatalf("expected the failure count on stderr, got %q", stderr)
	}
	if len(calls) != 4 {
		t.Fatalf("expected every locale to be attempted after a failure, got calls %v", calls)
	}

	receipt := decodeLocalizationImportReceipt(t, stdout)
	if receipt.Type != "subscriptionGroupLocalizations" || receipt.Total != 3 || receipt.Succeeded != 2 || receipt.Failed != 1 {
		t.Fatalf("unexpected receipt counts: %+v", receipt)
	}
	wantResults := "en-US update succeeded [customAppName] loc-en\nit create failed [name] \nes-MX create succeeded [name] loc-mx"
	if got := localizationImportSummary(receipt); got != wantResults {
		t.Fatalf("unexpected results:\ngot:\n%s\nwant:\n%s", got, wantResults)
	}
	if !strings.Contains(receipt.Results[1].Error, "locale rejected") {
		t.Fatalf("expected the API error on the failed locale, got %q", receipt.Results[1].Error)
	}
}

func TestVersionLocalizationsImportRequiresConfirm(t *testing.T) {
	setupAuth(t)
	filePath := writeLocalizationImportFile(t, `{"en-US": {"name": "Premium"}}`)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		t.Fatalf("unexpected HTTP request without --confirm: %s %s", req.Method, req.URL.String())
		return nil, nil
	})

	stdout, stderr, runErr := runIAPImport(t, []string{
		"subscriptions", "versions", "localizations", "import",
		"--version-id", "ver-1",
		"--file", filePath,
	})
	if !errors.Is(runErr, flag.ErrHelp) {
		t.Fatalf("expected a usage error, got %v", runErr)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "--confirm is required unless --dry-run is set") {
		t.Fatalf("expected the confirm gate on stderr, got %q", stderr)
	}
}

func TestVersionLocalizationsImportRejectsInvalidFilesBeforeAnyRequest(t *testing.T) {
	tests := []struct {
		name        string
		contents    string
		wantMessage string
	}{
		{name: "not an object", contents: `[{"locale":"en-US"}]`, wantMessage: "expected a JSON object mapping locales to fields"},
		{name: "unknown field", contents: `{"en-US": {"name": "Pro", "customAppName": "Pro App"}}`, wantMessage: `unknown field "customAppName" (allowed: name, description)`},
		{name: "invalid locale", contents: `{"english": {"name": "Pro"}}`, wantMessage: `invalid locale "english"`},
		{name: "duplicate locale", contents: `{"en-US": {"name": "Pro"}, "en_US": {"name": "Pro"}}`, wantMessage: `locale "en_US" duplicates "en-US"`},
		{name: "empty value", contents: `{"en-US": {"name": "  "}}`, wantMessage: `field "name" must not be empty`},
		{name: "trailing document", contents: `{"en-US": {"name": "Pro"}} {}`, wantMessage: "expected a single JSON object"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setupAuth(t)
			filePath := writeLocalizationImportFile(t, test.contents)

			originalTransport := http.DefaultTransport
			t.Cleanup(func() { http.DefaultTransport = originalTransport })
			http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
				t.Fatalf("unexpected HTTP request for an invalid file: %s %s", req.Method, req.URL.String())
				return nil, nil
			})

			stdout, stderr, runErr := runIAPImport(t, []string{
				"iap", "versions", "localizations", "import",
				"--version-id", "iap-ver",
				"--file", filePath,
				"--confirm",
				"--output", "json",
			})
			if code := rootcmd.ExitCodeFromError(runErr); code != rootcmd.ExitUsage {
				t.Fatalf("expected a usage error (exit %d), got exit %d: %v", rootcmd.ExitUsage, code, runErr)
			}
			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if !strings.Contains(stderr, test.wantMessage) {
				t.Fatalf("expected %q on stderr, got %q", test.wantMessage, stderr)
			}
		})
	}
}

func TestVersionLocalizationsImportRejectsCreateWithoutNameBeforeAnyWrite(t *testing.T) {
	setupAuth(t)
	filePath := writeLocalizationImportFile(t, `{
  "en-US": {"name": "Premium Plus"},
  "de-DE": {"description": "Alle Funktionen"}
}`)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptionVersions/ver-1/localizations" {
			return jsonResponse(http.StatusOK, `{"data":[
				{"type":"subscriptionLocalizations","id":"loc-en","attributes":{"locale":"en-US","name":"Premium"}}
			]}`)
		}
		t.Fatalf("unexpected write for an invalid plan: %s %s", req.Method, req.URL.String())
		return nil, nil
	})

	stdout, stderr, runErr := runIAPImport(t, []string{
		"subscriptions", "versions", "localizations", "import",
		"--version-id", "ver-1",
		"--file", filePath,
		"--confirm",
		"--output", "json",
	})
	if code := rootcmd.ExitCodeFromError(runErr); code != rootcmd.ExitUsage {
		t.Fatalf("expected a usage error (exit %d), got exit %d: %v", rootcmd.ExitUsage, code, runErr)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, `locale "de-DE" does not exist on the version yet; creating it requires "name"`) {
		t.Fatalf("expected the missing-name plan error, got %q", stderr)
	}
}
