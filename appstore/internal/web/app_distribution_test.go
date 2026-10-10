package web

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestGetAppDistributionBuildsExpectedRequest(t *testing.T) {
	var gotPath string
	var gotQuery string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		gotPath = r.URL.Path
		gotQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"data": {
				"type": "apps",
				"id": "app-123",
				"attributes": {
					"name": "Example",
					"bundleId": "com.example.app",
					"distributionType": "CUSTOM",
					"educationDiscountType": "NOT_APPLICABLE"
				}
			}
		}`))
	}))
	defer server.Close()

	got, err := testWebClient(server).GetAppDistribution(context.Background(), "app-123")
	if err != nil {
		t.Fatalf("GetAppDistribution() error = %v", err)
	}
	if gotPath != "/apps/app-123" {
		t.Fatalf("path = %q, want /apps/app-123", gotPath)
	}
	if gotQuery != "" {
		t.Fatalf("query = %q, want empty", gotQuery)
	}
	if got.AppID != "app-123" {
		t.Fatalf("appID = %q, want app-123", got.AppID)
	}
	if got.DistributionType != "CUSTOM" {
		t.Fatalf("distributionType = %q, want CUSTOM", got.DistributionType)
	}
	if got.EducationDiscountType != "NOT_APPLICABLE" {
		t.Fatalf("educationDiscountType = %q, want NOT_APPLICABLE", got.EducationDiscountType)
	}
	if got.BundleID != "com.example.app" {
		t.Fatalf("bundleId = %q, want com.example.app", got.BundleID)
	}
}

func TestGetAppDistributionOmitsMissingAttributes(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data": {"type": "apps", "id": "app-456", "attributes": {"name": "Example"}}}`))
	}))
	defer server.Close()

	got, err := testWebClient(server).GetAppDistribution(context.Background(), "app-456")
	if err != nil {
		t.Fatalf("GetAppDistribution() error = %v", err)
	}
	if got.DistributionType != "" || got.EducationDiscountType != "" {
		t.Fatalf("expected empty distribution attributes, got %+v", got)
	}
	if got.AppID != "app-456" {
		t.Fatalf("appID = %q, want app-456", got.AppID)
	}
}

func TestGetAppDistributionRequiresAppID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("unexpected request to %s", r.URL.Path)
	}))
	defer server.Close()

	if _, err := testWebClient(server).GetAppDistribution(context.Background(), "  "); err == nil {
		t.Fatal("expected error for empty app id")
	}
}

func TestGetAppDistributionPropagatesAPIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"errors":[{"status":"403","code":"FORBIDDEN_ERROR","title":"Access denied","detail":"no access"}]}`))
	}))
	defer server.Close()

	if _, err := testWebClient(server).GetAppDistribution(context.Background(), "app-123"); err == nil {
		t.Fatal("expected error for 403 response")
	}
}

func TestSetAppDistributionBuildsJSONAPIRequestAndVerifiesWithoutTouchingRecipients(t *testing.T) {
	var requestCount int
	var gotQuery string
	var gotPatch struct {
		Data struct {
			ID         string            `json:"id"`
			Type       string            `json:"type"`
			Attributes map[string]string `json:"attributes"`
		} `json:"data"`
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		if r.URL.Path != "/apps/app-123" {
			t.Fatalf("unexpected path %s; custom recipient endpoints must not be called", r.URL.Path)
		}
		switch r.Method {
		case http.MethodGet:
			gotQuery = r.URL.RawQuery
			w.Header().Set("Content-Type", "application/json")
			if requestCount == 1 {
				_, _ = w.Write([]byte(`{"data":{"type":"apps","id":"app-123","attributes":{"distributionType":"APP_STORE","educationDiscountType":"DISCOUNTED"}}}`))
				return
			}
			_, _ = w.Write([]byte(`{"data":{"type":"apps","id":"app-123","attributes":{"distributionType":"CUSTOM","educationDiscountType":"NOT_APPLICABLE"}}}`))
		case http.MethodPatch:
			if err := json.NewDecoder(r.Body).Decode(&gotPatch); err != nil {
				t.Fatalf("decode PATCH: %v", err)
			}
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Fatalf("unexpected method %s", r.Method)
		}
	}))
	defer server.Close()

	result, err := testWebClient(server).SetAppDistribution(context.Background(), AppDistributionSetRequest{
		AppID:                 "app-123",
		DistributionType:      AppDistributionTypeCustom,
		EducationDiscountType: "",
	})
	if err != nil {
		t.Fatalf("SetAppDistribution() error = %v", err)
	}
	if requestCount != 3 {
		t.Fatalf("request count = %d, want preflight GET, PATCH, and verification GET", requestCount)
	}
	if gotQuery != "fields[apps]=distributionType,educationDiscountType" {
		t.Fatalf("query = %q, want captured Apple fields query", gotQuery)
	}
	if gotPatch.Data.ID != "app-123" || gotPatch.Data.Type != "apps" {
		t.Fatalf("unexpected JSON:API resource: %+v", gotPatch.Data)
	}
	if gotPatch.Data.Attributes["distributionType"] != AppDistributionTypeCustom || gotPatch.Data.Attributes["educationDiscountType"] != AppDistributionEducationNotApplicable {
		t.Fatalf("unexpected PATCH attributes: %+v", gotPatch.Data.Attributes)
	}
	if result == nil || !result.Changed || !result.Verified || result.Status != "verified" {
		t.Fatalf("unexpected verified receipt: %+v", result)
	}
}

func TestSetAppDistributionNoOpSkipsPatch(t *testing.T) {
	var requestCount int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		if r.Method != http.MethodGet {
			t.Fatalf("unexpected %s request for no-op update", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"type":"apps","id":"app-123","attributes":{"distributionType":"APP_STORE","educationDiscountType":"NOT_DISCOUNTED"}}}`))
	}))
	defer server.Close()

	result, err := testWebClient(server).SetAppDistribution(context.Background(), AppDistributionSetRequest{
		AppID:            "app-123",
		DistributionType: AppDistributionTypeAppStore,
	})
	if err != nil {
		t.Fatalf("SetAppDistribution() error = %v", err)
	}
	if requestCount != 1 {
		t.Fatalf("request count = %d, want one preflight GET", requestCount)
	}
	if result == nil || result.Changed || !result.Verified || result.Status != "unchanged" {
		t.Fatalf("unexpected no-op receipt: %+v", result)
	}
}

func TestSetAppDistributionRejectsDirectURLBeforePatch(t *testing.T) {
	var requestCount int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		if r.Method != http.MethodGet {
			t.Fatalf("unexpected %s request after DIRECT_URL preflight", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"type":"apps","id":"app-123","attributes":{"distributionType":"DIRECT_URL","educationDiscountType":"NOT_APPLICABLE"}}}`))
	}))
	defer server.Close()

	_, err := testWebClient(server).SetAppDistribution(context.Background(), AppDistributionSetRequest{
		AppID:            "app-123",
		DistributionType: AppDistributionTypeCustom,
	})
	if err == nil || !strings.Contains(err.Error(), "DIRECT_URL") {
		t.Fatalf("error = %v, want DIRECT_URL refusal", err)
	}
	if requestCount != 1 {
		t.Fatalf("request count = %d, want preflight GET only", requestCount)
	}
}

func TestSetAppDistributionMarksServerErrorUncertain(t *testing.T) {
	var requestCount int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		if r.Method == http.MethodPatch {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		if r.Method != http.MethodGet {
			t.Fatalf("unexpected method %s", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"type":"apps","id":"app-123","attributes":{"distributionType":"APP_STORE","educationDiscountType":"DISCOUNTED"}}}`))
	}))
	defer server.Close()

	result, err := testWebClient(server).SetAppDistribution(context.Background(), AppDistributionSetRequest{
		AppID:                 "app-123",
		DistributionType:      AppDistributionTypeCustom,
		EducationDiscountType: "",
	})
	var uncertainErr *AppDistributionUnverifiedError
	if !errors.As(err, &uncertainErr) {
		t.Fatalf("error = %v, want AppDistributionUnverifiedError", err)
	}
	if requestCount != 3 {
		t.Fatalf("request count = %d, want preflight GET, failed PATCH, and verification GET", requestCount)
	}
	if result == nil || result.Status != "uncertain" || result.Verified {
		t.Fatalf("unexpected uncertain receipt: %+v", result)
	}
}

func TestSetAppDistributionReconcilesRequestTimeoutWithoutRetry(t *testing.T) {
	for _, observed := range []string{"APP_STORE", "CUSTOM"} {
		t.Run(observed, func(t *testing.T) {
			var reads, writes int
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.Method {
				case http.MethodPatch:
					writes++
					w.WriteHeader(http.StatusRequestTimeout)
				case http.MethodGet:
					reads++
					state, education := "APP_STORE", "DISCOUNTED"
					if reads > 1 && observed == "CUSTOM" {
						state, education = "CUSTOM", "NOT_APPLICABLE"
					}
					w.Header().Set("Content-Type", "application/json")
					_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"type": "apps", "id": "app-123", "attributes": map[string]string{"distributionType": state, "educationDiscountType": education}}})
				default:
					t.Errorf("unexpected method %s", r.Method)
					http.Error(w, "unexpected method", http.StatusMethodNotAllowed)
				}
			}))
			defer server.Close()
			result, err := testWebClient(server).SetAppDistribution(context.Background(), AppDistributionSetRequest{AppID: "app-123", DistributionType: AppDistributionTypeCustom})
			var uncertain *AppDistributionUnverifiedError
			var apiErr *APIError
			if !errors.As(err, &uncertain) || !errors.As(err, &apiErr) || apiErr.Status != http.StatusRequestTimeout {
				t.Fatalf("error=%v, want uncertain error preserving HTTP408", err)
			}
			if writes != 1 || reads != 2 {
				t.Fatalf("writes=%d reads=%d, want one PATCH and preflight/reconciliation GETs", writes, reads)
			}
			if result == nil || result.Status != "uncertain" || result.Verified != (observed == "CUSTOM") {
				t.Fatalf("receipt=%+v, want uncertain status reflecting readback %s", result, observed)
			}
		})
	}
}

func TestSetAppDistributionMarksTransportFailureUncertain(t *testing.T) {
	var requestCount int
	client := &Client{
		httpClient: &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			requestCount++
			if r.Method == http.MethodPatch {
				return nil, errors.New("connection reset by peer")
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(strings.NewReader(`{"data":{"type":"apps","id":"app-123","attributes":{"distributionType":"APP_STORE","educationDiscountType":"DISCOUNTED"}}}`)),
				Request:    r,
			}, nil
		})},
		baseURL: "https://example.test",
	}

	result, err := client.SetAppDistribution(context.Background(), AppDistributionSetRequest{
		AppID:                 "app-123",
		DistributionType:      AppDistributionTypeCustom,
		EducationDiscountType: "",
	})
	var uncertainErr *AppDistributionUnverifiedError
	if !errors.As(err, &uncertainErr) {
		t.Fatalf("error = %v, want AppDistributionUnverifiedError", err)
	}
	if requestCount != 3 {
		t.Fatalf("request count = %d, want preflight GET, failed PATCH, and verification GET", requestCount)
	}
	if result == nil || result.Status != "uncertain" || result.Verified {
		t.Fatalf("unexpected uncertain receipt: %+v", result)
	}
}

func TestSetAppDistributionDoesNotRetryAfterContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var requestCount int
	client := &Client{
		httpClient: &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			requestCount++
			if requestCount > 2 {
				t.Fatalf("unexpected request %d after context cancellation", requestCount)
			}
			if r.Method == http.MethodPatch {
				cancel()
				return nil, context.Canceled
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(strings.NewReader(`{"data":{"type":"apps","id":"app-123","attributes":{"distributionType":"APP_STORE","educationDiscountType":"DISCOUNTED"}}}`)),
				Request:    r,
			}, nil
		})},
		baseURL: "https://example.test",
	}

	result, err := client.SetAppDistribution(ctx, AppDistributionSetRequest{
		AppID:                 "app-123",
		DistributionType:      AppDistributionTypeCustom,
		EducationDiscountType: "",
	})
	var uncertainErr *AppDistributionUnverifiedError
	if !errors.As(err, &uncertainErr) {
		t.Fatalf("error = %v, want AppDistributionUnverifiedError", err)
	}
	if !strings.Contains(err.Error(), "verification unavailable because command context expired/canceled; inspect state before retry") {
		t.Fatalf("error = %v, want expired-context diagnostic", err)
	}
	if result == nil || result.Status != "uncertain" || result.Verified {
		t.Fatalf("unexpected uncertain receipt: %+v", result)
	}
	if requestCount != 2 {
		t.Fatalf("request count = %d, want preflight GET and PATCH only", requestCount)
	}
}

func TestSetAppDistributionMarksVerificationMismatchUncertain(t *testing.T) {
	var requestCount int
	setAppDistributionVerificationWait(t, func(context.Context, time.Duration) error {
		return context.Canceled
	})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		switch r.Method {
		case http.MethodPatch:
			w.WriteHeader(http.StatusNoContent)
		case http.MethodGet:
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"data":{"type":"apps","id":"app-123","attributes":{"distributionType":"APP_STORE","educationDiscountType":"DISCOUNTED"}}}`))
		default:
			t.Fatalf("unexpected method %s", r.Method)
		}
	}))
	defer server.Close()

	result, err := testWebClient(server).SetAppDistribution(context.Background(), AppDistributionSetRequest{
		AppID:                 "app-123",
		DistributionType:      AppDistributionTypeCustom,
		EducationDiscountType: "",
	})
	var uncertainErr *AppDistributionUnverifiedError
	if !errors.As(err, &uncertainErr) {
		t.Fatalf("error = %v, want AppDistributionUnverifiedError", err)
	}
	if !strings.Contains(err.Error(), `app distribution update was accepted by Apple but app "app-123" does not report the requested distribution state`) {
		t.Fatalf("error = %v, want accepted-update mismatch diagnostic", err)
	}
	if requestCount != 3 {
		t.Fatalf("request count = %d, want preflight GET, PATCH, and verification GET", requestCount)
	}
	if result == nil || result.Status != "uncertain" || result.Verified {
		t.Fatalf("unexpected mismatch receipt: %+v", result)
	}
}

func TestSetAppDistributionEventuallyVerifiesAfterStaleRead(t *testing.T) {
	var requestCount int
	var getCount int
	var patchCount int
	var patchAttributes map[string]string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		if r.URL.Path != "/apps/app-123" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		switch r.Method {
		case http.MethodGet:
			getCount++
			w.Header().Set("Content-Type", "application/json")
			var body string
			switch getCount {
			case 1:
				body = `{"data":{"type":"apps","id":"app-123","attributes":{"distributionType":"APP_STORE","educationDiscountType":"DISCOUNTED"}}}`
			case 2:
				// Model Apple's eventually consistent read immediately after the
				// accepted PATCH: the old education value is still visible.
				body = `{"data":{"type":"apps","id":"app-123","attributes":{"distributionType":"APP_STORE","educationDiscountType":"DISCOUNTED"}}}`
			case 3:
				body = `{"data":{"type":"apps","id":"app-123","attributes":{"distributionType":"APP_STORE","educationDiscountType":"NOT_DISCOUNTED"}}}`
			default:
				t.Fatalf("unexpected GET request %d", requestCount)
			}
			_, _ = w.Write([]byte(body))
		case http.MethodPatch:
			patchCount++
			var payload struct {
				Data struct {
					Attributes map[string]string `json:"attributes"`
				} `json:"data"`
			}
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatalf("decode PATCH: %v", err)
			}
			patchAttributes = payload.Data.Attributes
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Fatalf("unexpected method %s", r.Method)
		}
	}))
	defer server.Close()

	result, err := testWebClient(server).SetAppDistribution(context.Background(), AppDistributionSetRequest{
		AppID:                 "app-123",
		DistributionType:      AppDistributionTypeAppStore,
		EducationDiscountType: AppDistributionEducationNotDiscounted,
	})
	if err != nil {
		t.Fatalf("SetAppDistribution() error = %v", err)
	}
	if requestCount != 4 {
		t.Fatalf("request count = %d, want preflight GET, PATCH, stale GET, and matching GET", requestCount)
	}
	if patchCount != 1 {
		t.Fatalf("PATCH count = %d, want exactly one mutation", patchCount)
	}
	if _, ok := patchAttributes["distributionType"]; ok {
		t.Fatalf("education-only PATCH must not resend distributionType: %+v", patchAttributes)
	}
	if patchAttributes["educationDiscountType"] != AppDistributionEducationNotDiscounted {
		t.Fatalf("unexpected PATCH attributes: %+v", patchAttributes)
	}
	if result == nil || !result.Changed || !result.Verified || result.Status != "verified" {
		t.Fatalf("unexpected verified receipt: %+v", result)
	}
}

func TestSetAppDistributionVerificationWindowIsBounded(t *testing.T) {
	var requestCount int
	var patchCount int
	var waits []time.Duration
	setAppDistributionVerificationWait(t, func(_ context.Context, delay time.Duration) error {
		waits = append(waits, delay)
		return nil
	})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		switch r.Method {
		case http.MethodGet:
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"data":{"type":"apps","id":"app-123","attributes":{"distributionType":"APP_STORE","educationDiscountType":"DISCOUNTED"}}}`))
		case http.MethodPatch:
			patchCount++
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Fatalf("unexpected method %s", r.Method)
		}
	}))
	defer server.Close()

	result, err := testWebClient(server).SetAppDistribution(context.Background(), AppDistributionSetRequest{
		AppID:                 "app-123",
		DistributionType:      AppDistributionTypeAppStore,
		EducationDiscountType: AppDistributionEducationNotDiscounted,
	})
	var uncertainErr *AppDistributionUnverifiedError
	if !errors.As(err, &uncertainErr) {
		t.Fatalf("error = %v, want AppDistributionUnverifiedError", err)
	}
	if !strings.Contains(err.Error(), "verification window expired") {
		t.Fatalf("error = %v, want bounded-window diagnostic", err)
	}
	if requestCount != 7 {
		t.Fatalf("request count = %d, want preflight GET, PATCH, and five bounded verification GETs", requestCount)
	}
	if patchCount != 1 {
		t.Fatalf("PATCH count = %d, want exactly one mutation", patchCount)
	}
	wantWaits := []time.Duration{time.Second, 2 * time.Second, 4 * time.Second, 8 * time.Second}
	if len(waits) != len(wantWaits) {
		t.Fatalf("wait count = %d, want %d (%v)", len(waits), len(wantWaits), wantWaits)
	}
	for i := range wantWaits {
		if waits[i] != wantWaits[i] {
			t.Fatalf("waits = %v, want %v", waits, wantWaits)
		}
	}
	if result == nil || result.Status != "uncertain" || result.Verified {
		t.Fatalf("unexpected uncertain receipt: %+v", result)
	}
}

func TestSetAppDistributionVerificationStopsOnContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	setAppDistributionVerificationWait(t, func(_ context.Context, _ time.Duration) error {
		cancel()
		return context.Canceled
	})
	var requestCount int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		switch r.Method {
		case http.MethodGet:
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"data":{"type":"apps","id":"app-123","attributes":{"distributionType":"APP_STORE","educationDiscountType":"DISCOUNTED"}}}`))
		case http.MethodPatch:
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Fatalf("unexpected method %s", r.Method)
		}
	}))
	defer server.Close()

	result, err := testWebClient(server).SetAppDistribution(ctx, AppDistributionSetRequest{
		AppID:                 "app-123",
		DistributionType:      AppDistributionTypeAppStore,
		EducationDiscountType: AppDistributionEducationNotDiscounted,
	})
	var uncertainErr *AppDistributionUnverifiedError
	if !errors.As(err, &uncertainErr) {
		t.Fatalf("error = %v, want AppDistributionUnverifiedError", err)
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context cancellation", err)
	}
	if requestCount != 3 {
		t.Fatalf("request count = %d, want preflight GET, PATCH, and immediate verification GET", requestCount)
	}
	if result == nil || result.Status != "uncertain" || result.Verified {
		t.Fatalf("unexpected uncertain receipt: %+v", result)
	}
}

func TestSetAppDistributionVerificationReadErrorStopsPolling(t *testing.T) {
	var requestCount int
	var getCount int
	var patchCount int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		switch r.Method {
		case http.MethodGet:
			getCount++
			if getCount == 1 {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"data":{"type":"apps","id":"app-123","attributes":{"distributionType":"APP_STORE","educationDiscountType":"DISCOUNTED"}}}`))
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"errors":[{"status":"503","code":"SERVICE_UNAVAILABLE","title":"temporary"}]}`))
		case http.MethodPatch:
			patchCount++
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Fatalf("unexpected method %s", r.Method)
		}
	}))
	defer server.Close()

	result, err := testWebClient(server).SetAppDistribution(context.Background(), AppDistributionSetRequest{
		AppID:                 "app-123",
		DistributionType:      AppDistributionTypeAppStore,
		EducationDiscountType: AppDistributionEducationNotDiscounted,
	})
	var uncertainErr *AppDistributionUnverifiedError
	if !errors.As(err, &uncertainErr) {
		t.Fatalf("error = %v, want AppDistributionUnverifiedError", err)
	}
	if !strings.Contains(err.Error(), "verification failed") {
		t.Fatalf("error = %v, want verification failure diagnostic", err)
	}
	if requestCount != 3 || getCount != 2 {
		t.Fatalf("requests = %d total, %d GET, want preflight GET, PATCH, and one immediate read error", requestCount, getCount)
	}
	if patchCount != 1 {
		t.Fatalf("PATCH count = %d, want exactly one mutation", patchCount)
	}
	if result == nil || !result.Changed || result.Verified || result.Status != "uncertain" {
		t.Fatalf("unexpected uncertain receipt: %+v", result)
	}
}

func setAppDistributionVerificationWait(t *testing.T, wait func(context.Context, time.Duration) error) {
	t.Helper()
	previous := appDistributionVerificationWaitFn
	appDistributionVerificationWaitFn = wait
	t.Cleanup(func() {
		appDistributionVerificationWaitFn = previous
	})
}

func TestSetAppDistributionRejectsUnexpectedResourceIdentity(t *testing.T) {
	for _, phase := range []string{"preflight change", "preflight no-op", "verification"} {
		for _, identity := range []struct{ name, id, kind string }{{"wrong ID", "app-999", "apps"}, {"missing ID", "", "apps"}, {"wrong type", "app-123", "builds"}} {
			t.Run(phase+"/"+identity.name, func(t *testing.T) {
				requests, patches := 0, 0
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					requests++
					if r.Method == http.MethodPatch {
						patches++
						w.WriteHeader(http.StatusNoContent)
						return
					}
					id, kind, distribution, education := identity.id, identity.kind, "CUSTOM", "NOT_APPLICABLE"
					if phase == "preflight change" || (phase == "verification" && requests == 1) {
						distribution, education = "APP_STORE", "DISCOUNTED"
					}
					if phase == "verification" && requests == 1 {
						id, kind = "app-123", "apps"
					}
					w.Header().Set("Content-Type", "application/json")
					if err := json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"id": id, "type": kind, "attributes": map[string]any{"distributionType": distribution, "educationDiscountType": education}}}); err != nil {
						t.Error(err)
					}
				}))
				defer server.Close()
				result, err := testWebClient(server).SetAppDistribution(context.Background(), AppDistributionSetRequest{AppID: "app-123", DistributionType: AppDistributionTypeCustom})
				if err == nil {
					t.Fatalf("accepted unexpected resource identity: %+v", result)
				}
				if phase == "verification" {
					var uncertain *AppDistributionUnverifiedError
					if !errors.As(err, &uncertain) || result == nil || result.Verified || requests != 3 || patches != 1 {
						t.Fatalf("result=%+v error=%v requests=%d patches=%d", result, err, requests, patches)
					}
				} else if result != nil || requests != 1 || patches != 0 {
					t.Fatalf("preflight wrote or returned success: result=%+v error=%v requests=%d patches=%d", result, err, requests, patches)
				}
			})
		}
	}
}
