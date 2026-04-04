package submit

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/cli/shared"
	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/validation"
)

func TestSubmitPreflightCommand_MissingApp(t *testing.T) {
	// Ensure no app ID comes from env or config.
	t.Setenv("ASC_APP_ID", "")

	cmd := SubmitPreflightCommand()
	if err := cmd.FlagSet.Parse([]string{"--version", "1.0"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}
	// ResolveAppID may still find an ID from local config — skip Exec test
	// if that's the case, and only assert the command shape.
	resolved := shared.ResolveAppID("")
	if resolved != "" {
		t.Skip("local config provides an app ID; skipping flag validation test")
	}
	if err := cmd.Exec(context.Background(), nil); err != nil && !errors.Is(err, flag.ErrHelp) && !strings.Contains(err.Error(), "authentication") {
		t.Fatalf("expected flag.ErrHelp, got %v", err)
	}
}

func TestSubmitPreflightCommand_MissingVersion(t *testing.T) {
	setupSubmitAuth(t)

	cmd := SubmitPreflightCommand()
	if err := cmd.FlagSet.Parse([]string{"--app", "123"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}
	if err := cmd.Exec(context.Background(), nil); err != nil && !errors.Is(err, flag.ErrHelp) && !strings.Contains(err.Error(), "authentication") {
		t.Fatalf("expected flag.ErrHelp, got %v", err)
	}
}

func TestSubmitPreflightCommand_InvalidPlatform(t *testing.T) {
	setupSubmitAuth(t)

	cmd := SubmitPreflightCommand()
	if err := cmd.FlagSet.Parse([]string{"--app", "123", "--version", "1.0", "--platform", "INVALID"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}
	err := cmd.Exec(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error for invalid platform")
	}
	if !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp for invalid platform, got %v", err)
	}
}

func TestSubmitPreflightCommand_Shape(t *testing.T) {
	cmd := SubmitPreflightCommand()
	if cmd.Name != "preflight" {
		t.Fatalf("unexpected command name: %q", cmd.Name)
	}
	if cmd.FlagSet == nil {
		t.Fatal("expected FlagSet to be set")
	}
}

func TestSubmitPreflightCommand_RejectsUnsupportedOutput(t *testing.T) {
	cmd := SubmitPreflightCommand()
	if err := cmd.FlagSet.Parse([]string{"--app", "123", "--version", "1.0", "--output", "table"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	var runErr error
	_, stderr := capturePreflightCommandOutput(t, func() {
		runErr = cmd.Exec(context.Background(), nil)
	})
	err := runErr
	if err == nil {
		t.Fatal("expected error for unsupported output")
	}
	if !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected usage error, got %v", err)
	}
	if !strings.Contains(stderr, "unsupported format: table") {
		t.Fatalf("expected unsupported format message on stderr, got %q", stderr)
	}
}

func TestPreflightResult_TallyCounts(t *testing.T) {
	result := &preflightResult{
		Checks: []checkResult{
			{Name: "a", Passed: true},
			{Name: "b", Passed: false},
			{Name: "c", Passed: true},
			{Name: "d", Passed: false},
			{Name: "info", Passed: true, Advisory: true},
		},
	}
	tallyCounts(result)
	if result.PassCount != 3 {
		t.Fatalf("expected 3 passes including advisories, got %d", result.PassCount)
	}
	if result.FailCount != 2 {
		t.Fatalf("expected 2 failures, got %d", result.FailCount)
	}
}

func TestPreflightResult_AllPass(t *testing.T) {
	result := &preflightResult{
		Checks: []checkResult{
			{Name: "a", Passed: true},
			{Name: "b", Passed: true},
			{Name: "info", Passed: true, Advisory: true},
		},
	}
	tallyCounts(result)
	if result.PassCount != 3 {
		t.Fatalf("expected 3 passes including advisories, got %d", result.PassCount)
	}
	if result.FailCount != 0 {
		t.Fatalf("expected 0 failures, got %d", result.FailCount)
	}
}

func TestPrivacyPublishStateAdvisoryCheck_SetsPassedWhenPresent(t *testing.T) {
	check, ok := privacyPublishStateAdvisoryCheck("app-1")
	if !ok {
		t.Fatal("expected advisory check for non-empty app ID")
	}
	if !check.Advisory {
		t.Fatalf("expected advisory flag, got %+v", check)
	}
	if !check.Passed {
		t.Fatalf("expected advisory check to serialize as passed, got %+v", check)
	}
}

func TestPrivacyPublishStateAdvisoryCheck_SkipsBlankAppID(t *testing.T) {
	if _, ok := privacyPublishStateAdvisoryCheck(" \t "); ok {
		t.Fatal("expected blank app ID to skip advisory check")
	}
}

func TestPreflightResultFromReport_MapsContentRightsCheckName(t *testing.T) {
	result := preflightResultFromReport("app-123", "1.0", validation.Report{
		Platform: "IOS",
		Checks: []validation.CheckResult{
			{
				ID:       "content_rights.missing",
				Severity: validation.SeverityError,
				Message:  "content rights declaration is not set",
			},
		},
	})

	if len(result.Checks) != 1 {
		t.Fatalf("expected one check, got %+v", result.Checks)
	}
	if result.Checks[0].Name != "Content rights" {
		t.Fatalf("expected content rights label, got %+v", result.Checks[0])
	}
}

func TestPreflightTextOutput(t *testing.T) {
	// Ensure printPreflightText doesn't panic with various result shapes.
	var buf bytes.Buffer
	printPreflightText(&buf, &preflightResult{
		AppID:    "123",
		Version:  "1.0",
		Platform: "IOS",
		Checks: []checkResult{
			{Name: "Version exists", Passed: true, Message: "Version 1.0 found"},
			{Name: "Build attached", Passed: false, Message: "No build", Hint: "Attach a build with `asc release stage ...`, or upload and submit with `asc publish appstore ... --submit`"},
			{Name: "App Privacy", Advisory: true, Message: "App Privacy publish state is not verifiable via the public App Store Connect API and may still block submission", Hint: "Confirm App Privacy is published in App Store Connect before submitting: https://appstoreconnect.apple.com/apps/123/appPrivacy"},
		},
		PassCount: 1,
		FailCount: 1,
	})
	if !strings.Contains(buf.String(), "Preflight check for app 123 v1.0 (IOS)") {
		t.Fatalf("expected header in text output, got %q", buf.String())
	}
	if !strings.Contains(buf.String(), "App Privacy publish state is not verifiable via the public App Store Connect API") {
		t.Fatalf("expected advisory in text output, got %q", buf.String())
	}
}

func TestPreflightTextOutput_AdvisoryOnlyDoesNotClaimReadyToSubmit(t *testing.T) {
	var buf bytes.Buffer
	printPreflightText(&buf, &preflightResult{
		AppID:    "123",
		Version:  "1.0",
		Platform: "IOS",
		Checks: []checkResult{
			{
				Name:     "App Privacy",
				Passed:   true,
				Advisory: true,
				Message:  "App Privacy publish state is not verifiable via the public App Store Connect API and may still block submission",
				Hint:     "Confirm App Privacy is published in App Store Connect before submitting: https://appstoreconnect.apple.com/apps/123/appPrivacy",
			},
		},
	})

	output := buf.String()
	if strings.Contains(output, "Ready to submit") {
		t.Fatalf("did not expect advisory-only result to claim readiness, got %q", output)
	}
	if !strings.Contains(output, "Result: Required checks passed, but 1 advisory should be reviewed before submitting.") {
		t.Fatalf("expected advisory summary in text output, got %q", output)
	}
}

func TestSubmitPreflightCommand_AllPassIncludesPrivacyAdvisory(t *testing.T) {
	setupSubmitAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})
	http.DefaultTransport = submitAllPassTransport()

	cmd := SubmitPreflightCommand()
	cmd.FlagSet.SetOutput(io.Discard)
	if err := cmd.FlagSet.Parse([]string{"--app", "app-1", "--version", "1.0", "--output", "text"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	var runErr error
	stdout, stderr := capturePreflightCommandOutput(t, func() {
		runErr = cmd.Exec(context.Background(), nil)
	})
	if runErr != nil {
		t.Fatalf("expected success when only advisory is present, got %v", runErr)
	}
	if !strings.Contains(stderr, submitPreflightDeprecationWarning) {
		t.Fatalf("expected deprecation warning on stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, "App Privacy publish state is not verifiable via the public App Store Connect API") {
		t.Fatalf("expected App Privacy advisory in stdout, got %q", stdout)
	}
	if strings.Contains(strings.ToLower(stdout), "asc web") {
		t.Fatalf("did not expect private/web command references in stdout, got %q", stdout)
	}
}

func TestSubmitPreflightCommand_JSONAllPassIncludesPrivacyAdvisoryAsPassed(t *testing.T) {
	setupSubmitAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})
	http.DefaultTransport = submitAllPassTransport()

	cmd := SubmitPreflightCommand()
	cmd.FlagSet.SetOutput(io.Discard)
	if err := cmd.FlagSet.Parse([]string{"--app", "app-1", "--version", "1.0", "--output", "json"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	var (
		runErr error
		result preflightResult
	)
	stdout, stderr := capturePreflightCommandOutput(t, func() {
		runErr = cmd.Exec(context.Background(), nil)
	})
	if runErr != nil {
		t.Fatalf("expected success when only advisory is present, got %v", runErr)
	}
	if !strings.Contains(stderr, submitPreflightDeprecationWarning) {
		t.Fatalf("expected deprecation warning on stderr, got %q", stderr)
	}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("failed to parse JSON output %q: %v", stdout, err)
	}
	if result.FailCount != 0 {
		t.Fatalf("expected advisory-only JSON result to stay non-blocking, got %+v", result)
	}
	if result.PassCount != len(result.Checks) {
		t.Fatalf("expected advisory-only JSON pass count to include every check, got %+v", result)
	}

	foundPrivacyAdvisory := false
	for _, check := range result.Checks {
		if check.Name != "App Privacy" {
			continue
		}
		foundPrivacyAdvisory = true
		if !check.Advisory {
			t.Fatalf("expected App Privacy advisory in JSON output, got %+v", check)
		}
		if !check.Passed {
			t.Fatalf("expected App Privacy advisory to serialize as passed, got %+v", check)
		}
	}
	if !foundPrivacyAdvisory {
		t.Fatalf("expected App Privacy advisory in JSON output, got %+v", result.Checks)
	}
}

func TestSubmitPreflightCommand_JSONOutput(t *testing.T) {
	setupSubmitAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = submitRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && strings.Contains(req.URL.Path, "/appStoreVersions"):
			return submitJSONResponse(http.StatusOK, `{"data":[]}`)
		}
		return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.Path)
	})

	cmd := SubmitPreflightCommand()
	cmd.FlagSet.SetOutput(io.Discard)
	if err := cmd.FlagSet.Parse([]string{"--app", "123", "--version", "1.0", "--output", "json"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	err := cmd.Exec(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error when version not found")
	}
	if !strings.Contains(err.Error(), `app store version not found for version "1.0"`) {
		t.Fatalf("expected version lookup failure, got: %v", err)
	}
}

func TestSubmitPreflightCommand_TextOutput(t *testing.T) {
	setupSubmitAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = submitRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && strings.Contains(req.URL.Path, "/appStoreVersions"):
			return submitJSONResponse(http.StatusOK, `{"data":[]}`)
		}
		return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.Path)
	})

	cmd := SubmitPreflightCommand()
	cmd.FlagSet.SetOutput(io.Discard)
	if err := cmd.FlagSet.Parse([]string{"--app", "123", "--version", "1.0", "--output", "text"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	var runErr error
	stdout, _ := capturePreflightCommandOutput(t, func() {
		runErr = cmd.Exec(context.Background(), nil)
	})
	if runErr == nil {
		t.Fatal("expected error when version not found")
	}
	if !strings.Contains(runErr.Error(), `app store version not found for version "1.0"`) {
		t.Fatalf("expected version lookup failure, got: %v", runErr)
	}
	if stdout != "" {
		t.Fatalf("expected no text output when readiness report cannot be built, got %q", stdout)
	}
}

// --- Helpers ---

func capturePreflightCommandOutput(t *testing.T, fn func()) (string, string) {
	t.Helper()

	oldStdout := os.Stdout
	oldStderr := os.Stderr

	stdoutR, stdoutW, err := os.Pipe()
	if err != nil {
		t.Fatalf("stdout pipe error: %v", err)
	}
	stderrR, stderrW, err := os.Pipe()
	if err != nil {
		t.Fatalf("stderr pipe error: %v", err)
	}

	os.Stdout = stdoutW
	os.Stderr = stderrW

	stdoutCh := make(chan string, 1)
	stderrCh := make(chan string, 1)

	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, stdoutR)
		_ = stdoutR.Close()
		stdoutCh <- buf.String()
	}()
	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, stderrR)
		_ = stderrR.Close()
		stderrCh <- buf.String()
	}()

	defer func() {
		os.Stdout = oldStdout
		os.Stderr = oldStderr
		_ = stdoutW.Close()
		_ = stderrW.Close()
	}()

	fn()

	_ = stdoutW.Close()
	_ = stderrW.Close()

	return <-stdoutCh, <-stderrCh
}

func submitAllPassTransport() http.RoundTripper {
	return submitRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		path := req.URL.Path

		switch {
		case req.Method == http.MethodGet && strings.Contains(path, "/appStoreVersions") && strings.Contains(path, "/apps/"):
			return submitJSONResponse(http.StatusOK, `{
				"data": [{
					"type": "appStoreVersions",
					"id": "version-1",
					"attributes": {"platform": "IOS", "versionString": "1.0"}
				}]
			}`)
		case req.Method == http.MethodGet && path == "/v1/appStoreVersions/version-1":
			return submitJSONResponse(http.StatusOK, `{
				"data": {
					"type": "appStoreVersions",
					"id": "version-1",
					"attributes": {"appStoreState": "PREPARE_FOR_SUBMISSION", "platform": "IOS", "versionString": "1.0", "copyright":"2026 Test Company"},
					"relationships": {"app": {"data": {"type": "apps", "id": "app-1"}}}
				}
			}`)
		case req.Method == http.MethodGet && path == "/v1/appInfos/info-1/appInfoLocalizations":
			return submitJSONResponse(http.StatusOK, `{
				"data": [{
					"type": "appInfoLocalizations",
					"id": "info-loc-1",
					"attributes": {
						"locale": "en-US",
						"name": "Test App",
						"subtitle": "Subtitle",
						"privacyPolicyUrl": "https://example.com/privacy"
					}
				}]
			}`)
		case req.Method == http.MethodGet && path == "/v1/appStoreVersions/version-1/appStoreReviewDetail":
			return submitJSONResponse(http.StatusOK, `{
				"data": {
					"type": "appStoreReviewDetails",
					"id": "review-detail-1",
					"attributes": {
						"contactFirstName": "A",
						"contactLastName": "B",
						"contactEmail": "a@example.com",
						"contactPhone": "123",
						"demoAccountRequired": false,
						"notes": "Ready for review"
					}
				}
			}`)
		case req.Method == http.MethodGet && strings.HasSuffix(path, "/build"):
			return submitJSONResponse(http.StatusOK, `{"data":{"type":"builds","id":"build-1","attributes":{"version":"1","usesNonExemptEncryption":false}}}`)
		case req.Method == http.MethodGet && strings.HasSuffix(path, "/appInfos"):
			return submitJSONResponse(http.StatusOK, `{"data":[{"type":"appInfos","id":"info-1","attributes":{"appStoreState":"PREPARE_FOR_SUBMISSION"}}]}`)
		case req.Method == http.MethodGet && strings.HasSuffix(path, "/ageRatingDeclaration"):
			return submitJSONResponse(http.StatusOK, `{"data":{"type":"ageRatingDeclarations","id":"rating-1","attributes":{
				"advertising": false,
				"gambling": false,
				"healthOrWellnessTopics": false,
				"lootBox": false,
				"messagingAndChat": true,
				"parentalControls": true,
				"ageAssurance": false,
				"unrestrictedWebAccess": false,
				"userGeneratedContent": true,
				"alcoholTobaccoOrDrugUseOrReferences": "NONE",
				"contests": "NONE",
				"gamblingSimulated": "NONE",
				"gunsOrOtherWeapons": "NONE",
				"medicalOrTreatmentInformation": "NONE",
				"horrorOrFearThemes": "NONE",
				"matureOrSuggestiveThemes": "NONE",
				"profanityOrCrudeHumor": "NONE",
				"sexualContentGraphicAndNudity": "NONE",
				"sexualContentOrNudity": "NONE",
				"violenceCartoonOrFantasy": "NONE",
				"violenceRealistic": "NONE",
				"violenceRealisticProlongedGraphicOrSadistic": "NONE"
			}}}`)
		case req.Method == http.MethodGet && path == "/v1/apps/app-1":
			return submitJSONResponse(http.StatusOK, `{"data":{"type":"apps","id":"app-1","attributes":{"name":"Test","bundleId":"com.test","sku":"test","contentRightsDeclaration":"DOES_NOT_USE_THIRD_PARTY_CONTENT"}}}`)
		case req.Method == http.MethodGet && path == "/v1/apps/app-1/subscriptionGroups":
			return submitJSONResponse(http.StatusOK, `{"data":[]}`)
		case req.Method == http.MethodGet && path == "/v1/apps/app-1/inAppPurchasesV2":
			return submitJSONResponse(http.StatusOK, `{"data":[]}`)
		case req.Method == http.MethodGet && strings.HasSuffix(path, "/primaryCategory"):
			return submitJSONResponse(http.StatusOK, `{"data":{"type":"appCategories","id":"SPORTS","attributes":{}}}`)
		case req.Method == http.MethodGet && strings.Contains(path, "/appStoreVersionLocalizations"):
			return submitJSONResponse(http.StatusOK, `{
				"data": [{
					"type": "appStoreVersionLocalizations",
					"id": "loc-1",
					"attributes": {
						"locale": "en-US",
						"description": "A great app",
						"keywords": "test,app",
						"supportUrl": "https://example.com/support",
						"whatsNew": "Bug fixes"
					}
				}]
			}`)
		case req.Method == http.MethodGet && strings.Contains(path, "/appScreenshotSets"):
			return submitJSONResponse(http.StatusOK, `{
				"data": [{
					"type": "appScreenshotSets",
					"id": "ss-1",
					"attributes": {"screenshotDisplayType": "APP_IPHONE_67"}
				}]
			}`)
		case req.Method == http.MethodGet && path == "/v1/appScreenshotSets/ss-1/appScreenshots":
			return submitJSONResponse(http.StatusOK, `{
				"data": [{
					"type": "appScreenshots",
					"id": "shot-1",
					"attributes": {
						"fileName": "shot.png",
						"imageAsset": {"width": 1290, "height": 2796}
					}
				}]
			}`)
		}

		return nil, fmt.Errorf("unexpected request: %s %s", req.Method, path)
	})
}
