package cmdtest

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/asc"
)

func TestSubscriptionsPricingAvailabilityGetWarnsAndMatchesCanonicalViewOutput(t *testing.T) {
	setupAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/subscriptions/sub-1/subscriptionAvailability" {
			t.Fatalf("expected path /v1/subscriptions/sub-1/subscriptionAvailability, got %s", req.URL.Path)
		}
		return jsonHTTPResponse(http.StatusOK, `{"data":{"type":"subscriptionAvailabilities","id":"avail-1","attributes":{"availableInNewTerritories":false}}}`), nil
	})

	run := func(args []string) (string, string) {
		root := RootCommand("1.2.3")
		root.FlagSet.SetOutput(io.Discard)

		return captureOutput(t, func() {
			if err := root.Parse(args); err != nil {
				t.Fatalf("parse error: %v", err)
			}
			if err := root.Run(context.Background()); err != nil {
				t.Fatalf("run error: %v", err)
			}
		})
	}

	canonicalStdout, canonicalStderr := run([]string{"subscriptions", "pricing", "availability", "view", "--subscription-id", "sub-1", "--output", "json"})
	aliasStdout, aliasStderr := run([]string{"subscriptions", "pricing", "availability", "get", "--subscription-id", "sub-1", "--output", "json"})

	if canonicalStderr != "" {
		t.Fatalf("expected canonical command to avoid warnings, got %q", canonicalStderr)
	}
	if !strings.Contains(aliasStderr, "Warning: `asc subscriptions pricing availability get` is deprecated. Use `asc subscriptions pricing availability view`.") {
		t.Fatalf("expected deprecation warning, got %q", aliasStderr)
	}
	assertOnlyDeprecatedCommandWarnings(t, aliasStderr)

	var canonicalPayload map[string]any
	if err := json.Unmarshal([]byte(canonicalStdout), &canonicalPayload); err != nil {
		t.Fatalf("parse canonical stdout: %v", err)
	}
	var aliasPayload map[string]any
	if err := json.Unmarshal([]byte(aliasStdout), &aliasPayload); err != nil {
		t.Fatalf("parse alias stdout: %v", err)
	}
	if canonicalStdout != aliasStdout {
		t.Fatalf("expected canonical and alias output to match, canonical=%q alias=%q", canonicalStdout, aliasStdout)
	}
}

func TestSubscriptionsPricingAvailabilitySetWarnsAndMatchesCanonicalEditOutput(t *testing.T) {
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
		if payload.Data.Relationships.Subscription.Data.ID != "sub-1" {
			t.Fatalf("expected subscription relationship sub-1, got %q", payload.Data.Relationships.Subscription.Data.ID)
		}
		if payload.Data.Attributes.AvailableInNewTerritories {
			t.Fatalf("expected availableInNewTerritories false")
		}
		if len(payload.Data.Relationships.AvailableTerritories.Data) != 2 {
			t.Fatalf("expected two territories, got %+v", payload.Data.Relationships.AvailableTerritories.Data)
		}

		return jsonHTTPResponse(http.StatusCreated, `{"data":{"type":"subscriptionAvailabilities","id":"avail-1","attributes":{"availableInNewTerritories":false}}}`), nil
	})

	run := func(args []string) (string, string) {
		root := RootCommand("1.2.3")
		root.FlagSet.SetOutput(io.Discard)

		return captureOutput(t, func() {
			if err := root.Parse(args); err != nil {
				t.Fatalf("parse error: %v", err)
			}
			if err := root.Run(context.Background()); err != nil {
				t.Fatalf("run error: %v", err)
			}
		})
	}

	canonicalStdout, canonicalStderr := run([]string{"subscriptions", "pricing", "availability", "edit", "--subscription-id", "sub-1", "--available-in-new-territories", "false", "--territories", "USA,CAN", "--output", "json"})
	aliasStdout, aliasStderr := run([]string{"subscriptions", "pricing", "availability", "set", "--subscription-id", "sub-1", "--available-in-new-territories", "false", "--territories", "USA,CAN", "--output", "json"})

	if canonicalStderr != "" {
		t.Fatalf("expected canonical command to avoid warnings, got %q", canonicalStderr)
	}
	if !strings.Contains(aliasStderr, "Warning: `asc subscriptions pricing availability set` is deprecated. Use `asc subscriptions pricing availability edit`.") {
		t.Fatalf("expected deprecation warning, got %q", aliasStderr)
	}
	assertOnlyDeprecatedCommandWarnings(t, aliasStderr)

	var canonicalPayload map[string]any
	if err := json.Unmarshal([]byte(canonicalStdout), &canonicalPayload); err != nil {
		t.Fatalf("parse canonical stdout: %v", err)
	}
	var aliasPayload map[string]any
	if err := json.Unmarshal([]byte(aliasStdout), &aliasPayload); err != nil {
		t.Fatalf("parse alias stdout: %v", err)
	}
	if canonicalStdout != aliasStdout {
		t.Fatalf("expected canonical and alias output to match, canonical=%q alias=%q", canonicalStdout, aliasStdout)
	}
}

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
		if payload.Data.Relationships.Subscription.Data.ID != "sub-1" {
			t.Fatalf("expected subscription relationship sub-1, got %q", payload.Data.Relationships.Subscription.Data.ID)
		}

		return jsonHTTPResponse(http.StatusCreated, `{"data":{"type":"subscriptionAvailabilities","id":"avail-1","attributes":{"availableInNewTerritories":true}}}`), nil
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"subscriptions", "pricing", "availability", "edit", "--subscription-id", "sub-1", "--available-in-new-territories", "true", "--territories", "USA", "--output", "json"}); err != nil {
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

func TestSubscriptionsPricingAvailabilitySetAliasUsesPricingErrorPrefix(t *testing.T) {
	setupAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return nil, errors.New("boom")
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	_, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"subscriptions", "pricing", "availability", "set", "--subscription-id", "sub-1", "--available-in-new-territories", "false", "--territories", "USA"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if runErr == nil {
		t.Fatal("expected runtime error")
	}
	if !strings.Contains(runErr.Error(), "subscriptions pricing availability edit: failed to set:") || !strings.Contains(runErr.Error(), "boom") {
		t.Fatalf("expected pricing error prefix, got %q", runErr.Error())
	}
	if !strings.Contains(stderr, "Warning: `asc subscriptions pricing availability set` is deprecated. Use `asc subscriptions pricing availability edit`.") {
		t.Fatalf("expected deprecation warning, got %q", stderr)
	}
	assertOnlyDeprecatedCommandWarnings(t, stderr)
}
