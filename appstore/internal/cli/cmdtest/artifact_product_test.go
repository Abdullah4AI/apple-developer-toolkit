package cmdtest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/cmd"
	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/artifacts/artifactstest"
)

func TestRunPKGInfoReadsProductArchive(t *testing.T) {
	isolateArtifactCommandEnv(t)
	plist := []byte(`<plist version="1.0"><dict><key>CFBundleIdentifier</key><string>com.example.demo</string><key>CFBundleName</key><string>Demo</string><key>CFBundleShortVersionString</key><string>2.3.4</string><key>CFBundleVersion</key><string>56</string><key>LSMinimumSystemVersion</key><string>13.0</string><key>CFBundleSupportedPlatforms</key><array><string>MacOSX</string></array></dict></plist>`)
	payload := artifactstest.GzipCPIO(t, []artifactstest.CPIOEntry{
		{Name: ".", Mode: artifactstest.ModeDirectory},
		{Name: "./Demo.app/Contents/Info.plist", Data: plist},
	})
	files := map[string][]byte{
		"Distribution":         []byte(`<installer-gui-script minSpecVersion="2"><product id="com.example.demo" version="2.3.4"/><options hostArchitectures="x86_64,arm64"/><volume-check><allowed-os-versions><os-version min="13.0"/></allowed-os-versions></volume-check><pkg-ref id="com.example.demo" version="2.3.4" installKBytes="2048">#Demo.pkg</pkg-ref></installer-gui-script>`),
		"Demo.pkg/PackageInfo": []byte(`<pkg-info identifier="com.example.demo" version="2.3.4" install-location="/Applications"><bundle path="./Demo.app" id="com.example.demo" CFBundleVersion="56"/></pkg-info>`),
		"Demo.pkg/Payload":     payload,
	}
	dir := t.TempDir()
	product := filepath.Join(dir, "Demo.pkg")
	if err := os.WriteFile(product, artifactstest.XarTree(t, files, ""), 0o600); err != nil {
		t.Fatal(err)
	}
	var exit int
	stdout, stderr := captureOutput(t, func() { exit = cmd.Run([]string{"pkg-info", "--path", product, "--output", "json"}, "test") })
	if exit != cmd.ExitSuccess || stderr != "" {
		t.Fatalf("exit=%d stderr=%q", exit, stderr)
	}
	assertJSONEqual(t, stdout, map[string]any{
		"signatureVerification": "not-verified",
		"path":                  product,
		"productId":             "com.example.demo",
		"version":               "2.3.4",
		"installLocation":       "/Applications",
		"bundleIds":             []any{"com.example.demo"},
		"bundleId":              "com.example.demo",
		"buildNumber":           "56",
		"minimumOSVersion":      "13.0",
		"platforms":             []any{"MacOSX"},
		"hostArchitectures":     []any{"x86_64", "arm64"},
		"status":                "readable",
		"packageSignature":      "unsigned",
		"signer":                nil,
		"components": []any{map[string]any{
			"path":            "Demo.pkg",
			"identifier":      "com.example.demo",
			"version":         "2.3.4",
			"installLocation": "/Applications",
			"installKBytes":   float64(2048),
			"bundleIds":       []any{"com.example.demo"},
			"primary":         true,
			"app": map[string]any{
				"path":             "Demo.app",
				"bundleId":         "com.example.demo",
				"name":             "Demo",
				"version":          "2.3.4",
				"buildNumber":      "56",
				"minimumOSVersion": "13.0",
				"platforms":        []any{"MacOSX"},
			},
		}},
	})

	// An unreadable app payload keeps the receipt readable and warns on stderr.
	files["Demo.pkg/Payload"] = []byte("pbzx")
	if err := os.WriteFile(product, artifactstest.XarTree(t, files, ""), 0o600); err != nil {
		t.Fatal(err)
	}
	stdout, stderr = captureOutput(t, func() { exit = cmd.Run([]string{"pkg-info", "--path", product, "--output", "json"}, "test") })
	if exit != cmd.ExitSuccess || !strings.Contains(stderr, "Warning: pkg-info: component Demo.pkg: app Info.plist is unreadable: pbzx payloads are not supported") {
		t.Fatalf("exit=%d stderr=%q", exit, stderr)
	}
	if !strings.Contains(stdout, `"status":"readable"`) || strings.Contains(stdout, `"app"`) {
		t.Fatalf("stdout=%s", stdout)
	}
}

func TestRunPKGInfoRejectsProductArchiveWithoutPrimaryComponent(t *testing.T) {
	isolateArtifactCommandEnv(t)
	product := filepath.Join(t.TempDir(), "Demo.pkg")
	files := map[string][]byte{"Distribution": []byte(`<installer-gui-script><product id="com.example.demo" version="1"/><pkg-ref id="com.example.demo">#Demo.pkg</pkg-ref></installer-gui-script>`)}
	if err := os.WriteFile(product, artifactstest.XarTree(t, files, ""), 0o600); err != nil {
		t.Fatal(err)
	}
	var exit int
	stdout, stderr := captureOutput(t, func() { exit = cmd.Run([]string{"pkg-info", "--path", product, "--output", "json"}, "test") })
	if exit != cmd.ExitError || !strings.Contains(stderr, "pkg-info: read primary component Demo.pkg: component package is missing from the archive") {
		t.Fatalf("exit=%d stderr=%q", exit, stderr)
	}
	if !strings.Contains(stdout, `"status":"unreadable"`) || strings.Contains(stdout, `"components"`) {
		t.Fatalf("stdout=%s", stdout)
	}
}
