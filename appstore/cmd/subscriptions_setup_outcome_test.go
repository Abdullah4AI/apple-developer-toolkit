package cmd

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/asc"
	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/cli/shared"
	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/cli/validate"
	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/telemetry"
	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/validation"
)

func TestRunSubscriptionsSetupMissingMetadataReportsBlockersAsExpectedNegative(t *testing.T) {
	resetReportFlags(t)

	const subscription = `{"data":{"type":"subscriptions","id":"sub-1","attributes":{"name":"Pro Monthly","productId":"com.example.pro.monthly","subscriptionPeriod":"ONE_MONTH","state":"MISSING_METADATA"}}}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptionGroups/group-1/subscriptions":
			fmt.Fprint(w, `{"data":[],"links":{"next":""}}`)
		case req.Method == http.MethodPost && req.URL.Path == "/v1/subscriptions":
			w.WriteHeader(http.StatusCreated)
			fmt.Fprint(w, subscription)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptionGroups/group-1":
			fmt.Fprint(w, `{"data":{"type":"subscriptionGroups","id":"group-1","attributes":{"referenceName":"Pro"}}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptions/sub-1":
			fmt.Fprint(w, subscription)
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
		}
	}))
	defer server.Close()

	client := newHTTPStatusTestClient(t, server.URL)
	t.Cleanup(shared.SetASCClientFactoryForTesting(func() (*asc.Client, error) { return client, nil }))
	t.Cleanup(validate.SetFetchSubscriptionsFunc(func(context.Context, *asc.Client, string) ([]validation.Subscription, error) {
		return []validation.Subscription{{ID: "sub-1", Name: "Pro Monthly", State: "MISSING_METADATA", GroupID: "group-1"}}, nil
	}))
	t.Cleanup(validate.SetFetchAvailableTerritoriesFunc(func(context.Context, *asc.Client, string) (string, int, error) {
		return "app-availability-1", 1, nil
	}))
	t.Cleanup(validate.SetFetchPricingTerritoriesFunc(func(context.Context, *asc.Client) ([]string, error) {
		return []string{"USA"}, nil
	}))
	t.Cleanup(validate.SetFetchAppBuildCountFunc(func(context.Context, *asc.Client, string) (int, bool, string, error) {
		return 1, true, "", nil
	}))

	originalEmitTelemetry := emitTelemetry
	t.Cleanup(func() { emitTelemetry = originalEmitTelemetry })
	var gotContext telemetry.EventContext
	emitTelemetry = func(_ string, _ string, _ time.Duration, _ int, eventContext telemetry.EventContext) {
		gotContext = eventContext
	}

	stdout, stderr := captureCommandOutput(t, func() {
		if code := Run([]string{
			"subscriptions", "setup",
			"--app", "app-1",
			"--group-id", "group-1",
			"--reference-name", "Pro Monthly",
			"--product-id", "com.example.pro.monthly",
			"--subscription-period", "ONE_MONTH",
			"--output", "json",
		}, "4.0.0"); code != ExitError {
			t.Fatalf("Run() exit code = %d, want %d", code, ExitError)
		}
	})

	if !strings.Contains(stdout, `"failedStep":"verify_state"`) || !strings.Contains(stdout, `"diagnostics":[`) {
		t.Fatalf("stdout = %q, want the structured result with diagnostics", stdout)
	}
	if !strings.HasPrefix(stderr, "Error: subscriptions setup: verify_state: apple reports subscription state MISSING_METADATA after setup\n") || strings.Count(stderr, "Error:") != 1 {
		t.Fatalf("stderr = %q, want one error line with the failed step first", stderr)
	}
	for _, want := range []string{
		"\n- Subscription localizations: ",
		"\n- Review screenshot delivery: Upload an App Review screenshot with `asc subscriptions review screenshots create --subscription-id \"sub-1\"",
	} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("stderr = %q, want blocking diagnostic %q", stderr, want)
		}
	}
	if strings.Contains(stderr, "Promotional image") {
		t.Fatalf("stderr = %q, want only blocking diagnostics", stderr)
	}
	if gotContext.OutcomeKind != telemetry.OutcomeExpectedNegative ||
		gotContext.FailureStage != telemetry.FailureStageValidation ||
		gotContext.DiagnosticCode != string(shared.DiagnosticStateNotReady) {
		t.Fatalf("unexpected telemetry context: %+v", gotContext)
	}
}
