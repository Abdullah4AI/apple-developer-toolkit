package cmdtest

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"howett.net/plist"

	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/cli/shared"
	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/validation"
)

const validateIPABundleID = "com.example.demo"

// validateIPAInfo is an Info.plist for the fixture's attached build 1.0 (1.0)
// declaring the given UIDeviceFamily values; nil omits the key.
func validateIPAInfo(deviceFamilies []int) map[string]any {
	info := map[string]any{
		"CFBundleIdentifier":         validateIPABundleID,
		"CFBundleShortVersionString": "1.0",
		"CFBundleVersion":            "1.0",
	}
	if deviceFamilies != nil {
		info["UIDeviceFamily"] = deviceFamilies
	}
	return info
}

// writeValidateIPA writes a minimal IPA with the given top-level app Info.plist.
func writeValidateIPA(t *testing.T, info map[string]any) string {
	t.Helper()

	data, err := plist.Marshal(info, plist.XMLFormat)
	if err != nil {
		t.Fatalf("marshal Info.plist: %v", err)
	}

	var buffer bytes.Buffer
	archive := zip.NewWriter(&buffer)
	entry, err := archive.Create("Payload/Demo.app/Info.plist")
	if err != nil {
		t.Fatalf("create Info.plist entry: %v", err)
	}
	if _, err := entry.Write(data); err != nil {
		t.Fatalf("write Info.plist entry: %v", err)
	}
	if err := archive.Close(); err != nil {
		t.Fatalf("close IPA: %v", err)
	}

	path := filepath.Join(t.TempDir(), "App.ipa")
	if err := os.WriteFile(path, buffer.Bytes(), 0o600); err != nil {
		t.Fatalf("write IPA: %v", err)
	}
	return path
}

func validateIPadFixture() validateFixture {
	fixture := validValidateFixture()
	fixture.app = `{"data":{"type":"apps","id":"app-1","attributes":{"bundleId":"` + validateIPABundleID + `","primaryLocale":"en-US","contentRightsDeclaration":"DOES_NOT_USE_THIRD_PARTY_CONTENT"}}}`
	return fixture
}

func withIPadScreenshots(fixture validateFixture) validateFixture {
	fixture.screenshotSets = map[string]string{
		"ver-loc-1": `{"data":[` +
			`{"type":"appScreenshotSets","id":"set-1","attributes":{"screenshotDisplayType":"APP_IPHONE_65"}},` +
			`{"type":"appScreenshotSets","id":"set-2","attributes":{"screenshotDisplayType":"APP_IPAD_PRO_3GEN_129"}}]}`,
	}
	fixture.screenshotsBySet = map[string]string{
		"set-1": `{"data":[{"type":"appScreenshots","id":"shot-1","attributes":{"fileName":"shot.png","fileSize":1024,"imageAsset":{"width":1242,"height":2688}}}]}`,
		"set-2": `{"data":[{"type":"appScreenshots","id":"shot-2","attributes":{"fileName":"ipad.png","fileSize":1024,"imageAsset":{"width":2048,"height":2732}}}]}`,
	}
	return fixture
}

// runValidateIPadReport runs validate for ver-1 and decodes any JSON report.
func runValidateIPadReport(t *testing.T, fixture validateFixture, args ...string) (validation.Report, error) {
	t.Helper()

	stdout, _, runErr := runValidateWithFixture(t, fixture, append([]string{"validate", "--app", "app-1", "--version-id", "ver-1", "--output", "json"}, args...)...)

	var report validation.Report
	if strings.TrimSpace(stdout) != "" {
		if err := json.Unmarshal([]byte(stdout), &report); err != nil {
			t.Fatalf("failed to parse JSON output %q: %v", stdout, err)
		}
	}
	return report, runErr
}

func findValidateCheck(checks []validation.CheckResult, id string) (validation.CheckResult, bool) {
	for _, check := range checks {
		if check.ID == id {
			return check, true
		}
	}
	return validation.CheckResult{}, false
}

func TestValidateIPABlocksIPadBuildWithoutIPadScreenshots(t *testing.T) {
	ipa := writeValidateIPA(t, validateIPAInfo([]int{1, 2}))

	report, err := runValidateIPadReport(t, validateIPadFixture(), "--ipa", ipa)

	if _, ok := errors.AsType[ReportedError](err); !ok {
		t.Fatalf("expected ReportedError, got %v", err)
	}
	check, ok := findValidateCheck(report.Checks, "screenshots.required.ipad")
	if !ok {
		t.Fatalf("expected screenshots.required.ipad, got %+v", report.Checks)
	}
	if check.Severity != validation.SeverityError || !strings.Contains(check.Message, "APP_IPAD_PRO_3GEN_129") {
		t.Fatalf("unexpected check %+v", check)
	}
	if hasCheckWithID(report.Checks, "screenshots.required.ipad_unverified") {
		t.Fatalf("did not expect the unverified info check with --ipa, got %+v", report.Checks)
	}
}

func TestValidateIPAPassesWhenIPadScreenshotsExist(t *testing.T) {
	ipa := writeValidateIPA(t, validateIPAInfo([]int{1, 2}))

	report, err := runValidateIPadReport(t, withIPadScreenshots(validateIPadFixture()), "--ipa", ipa)
	if err != nil {
		t.Fatalf("run error: %v", err)
	}
	if report.Summary.Blocking != 0 || hasCheckWithID(report.Checks, "screenshots.required.ipad") {
		t.Fatalf("expected no blocking iPad check, got %+v", report.Checks)
	}
}

func TestValidateIPAPassesIPhoneOnlyBuild(t *testing.T) {
	tests := []struct {
		name     string
		families []int
	}{
		{name: "explicit iPhone", families: []int{1}},
		{name: "absent UIDeviceFamily defaults to iPhone", families: nil},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ipa := writeValidateIPA(t, validateIPAInfo(test.families))

			report, err := runValidateIPadReport(t, validateIPadFixture(), "--ipa", ipa)
			if err != nil {
				t.Fatalf("run error: %v", err)
			}
			for _, id := range []string{"screenshots.required.ipad", "screenshots.required.ipad_unverified"} {
				if hasCheckWithID(report.Checks, id) {
					t.Fatalf("did not expect %s for an iPhone-only build, got %+v", id, report.Checks)
				}
			}
		})
	}
}

func TestValidateIPAWithoutAttachedBuildMatchesVersionString(t *testing.T) {
	fixture := validateIPadFixture()
	fixture.build = ""

	report, err := runValidateIPadReport(t, fixture, "--ipa", writeValidateIPA(t, validateIPAInfo([]int{1, 2})))
	if _, ok := errors.AsType[ReportedError](err); !ok {
		t.Fatalf("expected ReportedError, got %v", err)
	}
	if !hasCheckWithID(report.Checks, "screenshots.required.ipad") {
		t.Fatalf("expected screenshots.required.ipad before the build is attached, got %+v", report.Checks)
	}

	info := validateIPAInfo([]int{1, 2})
	info["CFBundleShortVersionString"] = "2.0"
	_, err = runValidateIPadReport(t, fixture, "--ipa", writeValidateIPA(t, info))
	if err == nil || !strings.Contains(err.Error(), `--ipa version (CFBundleShortVersionString) "2.0" does not match App Store version "1.0"`) {
		t.Fatalf("expected version string mismatch error, got %v", err)
	}
}

func TestValidateIPARejectsMismatchedBinary(t *testing.T) {
	otherBundle := validateIPAInfo([]int{1, 2})
	otherBundle["CFBundleIdentifier"] = "com.example.other"
	otherBuild := validateIPAInfo([]int{1})
	otherBuild["CFBundleVersion"] = "0.9"
	otherTrain := validateIPAInfo([]int{1})
	otherTrain["CFBundleShortVersionString"] = "2.0"
	nonIOSFixture := validateIPadFixture()
	nonIOSFixture.version = `{"data":{"type":"appStoreVersions","id":"ver-1","attributes":{"platform":"MAC_OS","versionString":"1.0","appVersionState":"PREPARE_FOR_SUBMISSION","copyright":"2026 Test Company"},"relationships":{"app":{"data":{"type":"apps","id":"app-1"}}}}}`

	tests := []struct {
		name    string
		fixture validateFixture
		info    map[string]any
		wantErr string
	}{
		{
			name:    "different app",
			fixture: validateIPadFixture(),
			info:    otherBundle,
			wantErr: `--ipa bundle ID "com.example.other" does not match the app's bundle ID "com.example.demo"`,
		},
		{
			name:    "different build of the same app",
			fixture: validateIPadFixture(),
			info:    otherBuild,
			wantErr: `--ipa build number (CFBundleVersion) "0.9" does not match the attached build "1.0"`,
		},
		{
			name:    "same build number on another version train",
			fixture: validateIPadFixture(),
			info:    otherTrain,
			wantErr: `--ipa version (CFBundleShortVersionString) "2.0" does not match App Store version "1.0"`,
		},
		{
			name:    "non-IOS version",
			fixture: nonIOSFixture,
			info:    validateIPAInfo([]int{1, 2}),
			wantErr: "--ipa applies only to IOS App Store versions",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := runValidateIPadReport(t, test.fixture, "--ipa", writeValidateIPA(t, test.info))
			if err == nil || !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("expected %q, got %v", test.wantErr, err)
			}
			diagnostic, _ := shared.DiagnosticFromError(err)
			if !shared.IsValidationError(err) || diagnostic.Code != shared.DiagnosticInvalidInput || diagnostic.Parameter != "--ipa" {
				t.Fatalf("expected validation error with invalid_input on --ipa, got validation=%t diagnostic=%+v", shared.IsValidationError(err), diagnostic)
			}
		})
	}
}

func TestValidateWithoutIPAReportsIPadSupportAsUnverifiedInfo(t *testing.T) {
	report, err := runValidateIPadReport(t, validateIPadFixture(), "--strict")
	if err != nil {
		t.Fatalf("an unknown device family must not block, even with --strict: %v", err)
	}
	check, ok := findValidateCheck(report.Checks, "screenshots.required.ipad_unverified")
	if !ok {
		t.Fatalf("expected screenshots.required.ipad_unverified, got %+v", report.Checks)
	}
	if check.Severity != validation.SeverityInfo || !strings.Contains(check.Remediation, "--ipa") {
		t.Fatalf("unexpected check %+v", check)
	}
	if hasCheckWithID(report.Checks, "screenshots.required.ipad") {
		t.Fatalf("did not expect a blocking iPad check without --ipa, got %+v", report.Checks)
	}
}

func TestValidateIPARejectsUnreadableIPABeforeNetwork(t *testing.T) {
	fixture := validateIPadFixture()
	fixture.requestObserver = func(req *http.Request) {
		t.Errorf("unexpected request %s %s", req.Method, req.URL.Path)
	}

	notIPA := filepath.Join(t.TempDir(), "App.ipa")
	if err := os.WriteFile(notIPA, []byte("not a zip"), 0o600); err != nil {
		t.Fatal(err)
	}
	noBundleID := validateIPAInfo([]int{1, 2})
	delete(noBundleID, "CFBundleIdentifier")
	emptyFamilies := validateIPAInfo(nil)
	emptyFamilies["UIDeviceFamily"] = []any{}

	tests := []struct {
		name    string
		path    string
		wantErr string
	}{
		{name: "missing file", path: filepath.Join(t.TempDir(), "missing.ipa"), wantErr: "validate: --ipa:"},
		{name: "not a zip", path: notIPA, wantErr: "validate: --ipa:"},
		{name: "no bundle identifier", path: writeValidateIPA(t, noBundleID), wantErr: "validate: --ipa: IPA app Info.plist has no CFBundleIdentifier"},
		{name: "empty UIDeviceFamily", path: writeValidateIPA(t, emptyFamilies), wantErr: "UIDeviceFamily"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := runValidateIPadReport(t, fixture, "--ipa", test.path)
			if err == nil || !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("expected %q, got %v", test.wantErr, err)
			}
		})
	}
}
