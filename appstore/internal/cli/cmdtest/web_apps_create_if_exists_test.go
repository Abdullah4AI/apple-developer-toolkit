package cmdtest

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWebAppsCreateIfExistsRejectsUnsupportedModesBeforeHTTP(t *testing.T) {
	var requests int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		requests++
		http.Error(w, "no HTTP expected", http.StatusInternalServerError)
	}))
	defer server.Close()
	setWebAppsCreateASCClient(t, server)

	for _, value := range []string{"update", "overwrite", ""} {
		t.Run("value="+value, func(t *testing.T) {
			assertUsageExit(t, webAppsCreateAccessArgs("--if-exists", value), `--if-exists must be one of fail, skip (got "`+value+`")`)
		})
	}
	if requests != 0 {
		t.Fatalf("expected no HTTP, got %d requests", requests)
	}
}

func TestWebAppsCreateIfExistsSkipRejectsAccessBeforeHTTP(t *testing.T) {
	var requests int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		requests++
		http.Error(w, "no HTTP expected", http.StatusInternalServerError)
	}))
	defer server.Close()
	setWebAppsCreateASCClient(t, server)

	assertUsageExit(t, webAppsCreateAccessArgs("--if-exists", "skip", "--access", "full"), "--if-exists skip cannot be combined with --access")
	if requests != 0 {
		t.Fatalf("expected no HTTP, got %d requests", requests)
	}
}

func TestWebAppsCreateHelpDocumentsIfExists(t *testing.T) {
	root := RootCommand("1.2.3")
	cmd := findSubcommand(root, "web", "apps", "create")
	if cmd == nil {
		t.Fatal("expected web apps create command")
	}
	flag := cmd.FlagSet.Lookup("if-exists")
	if flag == nil {
		t.Fatal("expected --if-exists flag on web apps create")
	}
	if flag.DefValue != "fail" {
		t.Fatalf("--if-exists default = %q, want fail", flag.DefValue)
	}
	if !strings.Contains(flag.Usage, "fail, skip") || strings.Contains(flag.Usage, "update") {
		t.Fatalf("--if-exists usage = %q, want fail, skip only", flag.Usage)
	}
	usage := cmd.UsageFunc(cmd)
	if !strings.Contains(usage, "--if-exists skip") || !strings.Contains(usage, "runs before") {
		t.Fatalf("expected --if-exists precedence in help, got %q", usage)
	}
}
