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

func TestSubscriptionsPricingAvailabilityEditAcceptsSpacedTrueBoolValue(t *testing.T) {
	setupAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", req.Method)
		}
		if req.URL.Path != "/v1/subscriptionAvailabilities" {
			t.Fatalf("expected path /v1/subscriptionAvailabilities, got %s", req.URL.Path)
		}

		var payload asc.SubscriptionAvailabilityCreateRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		if !payload.Data.Attributes.AvailableInNewTerritories {
			t.Fatalf("expected availableInNewTerritories true")
		}
		if payload.Data.Relationships.Subscription.Data.ID != "8000000001" {
			t.Fatalf("expected subscription relationship 8000000001, got %q", payload.Data.Relationships.Subscription.Data.ID)
		}

		return jsonHTTPResponse(http.StatusCreated, `{"data":{"type":"subscriptionAvailabilities","id":"avail-1","attributes":{"availableInNewTerritories":true}}}`), nil
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"subscriptions", "pricing", "availability", "edit", "--subscription-id", "8000000001", "--available-in-new-territories", "true", "--territories", "USA", "--output", "json"}); err != nil {
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
