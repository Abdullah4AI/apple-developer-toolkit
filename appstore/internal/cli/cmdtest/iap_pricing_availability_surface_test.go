package cmdtest

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/asc"
)

func TestIAPPricingAvailabilitySetAcceptsSpacedBoolValue(t *testing.T) {
	setupAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", req.Method)
		}
		if req.URL.Path != "/v1/inAppPurchaseAvailabilities" {
			t.Fatalf("expected path /v1/inAppPurchaseAvailabilities, got %s", req.URL.Path)
		}

		var payload asc.InAppPurchaseAvailabilityCreateRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		if payload.Data.Relationships.InAppPurchase.Data.ID != "iap-1" {
			t.Fatalf("expected iap relationship iap-1, got %q", payload.Data.Relationships.InAppPurchase.Data.ID)
		}
		if payload.Data.Attributes.AvailableInNewTerritories {
			t.Fatalf("expected availableInNewTerritories false")
		}
		if len(payload.Data.Relationships.AvailableTerritories.Data) != 2 {
			t.Fatalf("expected two territories, got %+v", payload.Data.Relationships.AvailableTerritories.Data)
		}

		return jsonHTTPResponse(http.StatusCreated, `{"data":{"type":"inAppPurchaseAvailabilities","id":"avail-1","attributes":{"availableInNewTerritories":false}}}`), nil
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"iap", "pricing", "availability", "set", "--iap-id", "iap-1", "--available-in-new-territories", "false", "--territories", "USA,CAN", "--output", "json"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"id":"avail-1"`) {
		t.Fatalf("expected availability response, got %q", stdout)
	}
}
