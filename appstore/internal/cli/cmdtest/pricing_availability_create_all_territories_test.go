package cmdtest

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/cmd"
	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/asc"
)

// Apple's territory catalog split across two pages, as GET /v1/territories
// returns it when the catalog exceeds the page size.
const (
	territoryCatalogFirstPage  = `{"data":[{"type":"territories","id":"USA"},{"type":"territories","id":"FRA"}],"links":{"next":"https://api.appstoreconnect.apple.com/v1/territories?cursor=next"}}`
	territoryCatalogSecondPage = `{"data":[{"type":"territories","id":"CAN"}],"links":{}}`
)

func TestPricingAvailabilityCreateAllTerritoriesIncludesEveryCatalogTerritory(t *testing.T) {
	for _, available := range []bool{true, false} {
		t.Run(strconv.FormatBool(available), func(t *testing.T) {
			var createBody string
			stdout, stderr, seen, runErr := runIfExistsCommand(t, []string{
				"pricing", "availability", "create",
				"--app", "app-1",
				"--all-territories",
				"--available", strconv.FormatBool(available),
				"--available-in-new-territories", "true",
				"--output", "json",
			}, func(req ifExistsRequest) (*http.Response, error) {
				switch {
				case req.Method == http.MethodGet && req.Path == "/v1/territories" && strings.Contains(req.Query, "cursor=next"):
					return jsonResponse(http.StatusOK, territoryCatalogSecondPage)
				case req.Method == http.MethodGet && req.Path == "/v1/territories":
					return jsonResponse(http.StatusOK, territoryCatalogFirstPage)
				case req.Method == http.MethodPost && req.Path == "/v2/appAvailabilities":
					createBody = req.Body
					return jsonResponse(http.StatusCreated, `{"data":{"type":"appAvailabilities","id":"availability-1","attributes":{"availableInNewTerritories":true}}}`)
				default:
					t.Fatalf("unexpected request %s %s", req.Method, req.Path)
					return nil, nil
				}
			})
			if runErr != nil {
				t.Fatalf("run error: %v", runErr)
			}
			if stderr != "" {
				t.Fatalf("stderr = %q, want empty", stderr)
			}
			if len(seen) != 3 {
				t.Fatalf("requests = %+v, want two catalog pages and one create", seen)
			}

			var payload asc.AppAvailabilityV2CreateRequest
			if err := json.Unmarshal([]byte(createBody), &payload); err != nil {
				t.Fatalf("decode create body: %v; body=%q", err, createBody)
			}
			gotTerritories := make([]string, 0, len(payload.Included))
			for _, included := range payload.Included {
				if included.Attributes == nil || included.Relationships == nil {
					t.Fatalf("included territory availability is incomplete: %#v", included)
				}
				if included.Attributes.Available != available {
					t.Fatalf("territory %s available = %t, want %t", included.Relationships.Territory.Data.ID, included.Attributes.Available, available)
				}
				gotTerritories = append(gotTerritories, included.Relationships.Territory.Data.ID)
			}
			if got, want := strings.Join(gotTerritories, ","), "USA,FRA,CAN"; got != want {
				t.Fatalf("included territories = %s, want every catalog territory %s", got, want)
			}
			if !strings.Contains(stdout, `"id":"availability-1"`) {
				t.Fatalf("stdout = %q, want Apple's created availability", stdout)
			}
		})
	}
}

func TestPricingAvailabilityCreateTerritorySelectorValidation(t *testing.T) {
	tests := []struct {
		name     string
		selector []string
		wantErr  string
	}{
		{
			name:     "missing selector",
			selector: nil,
			wantErr:  "Error: --territory or --all-territories is required",
		},
		{
			name:     "territory and all territories",
			selector: []string{"--territory", "USA", "--all-territories"},
			wantErr:  "Error: --territory and --all-territories are mutually exclusive",
		},
		{
			name:     "blank territory and all territories",
			selector: []string{"--territory", "", "--all-territories"},
			wantErr:  "Error: --territory and --all-territories are mutually exclusive",
		},
		{
			name:     "territory list without values",
			selector: []string{"--territory", ","},
			wantErr:  "Error: --territory must include at least one value",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			args := append([]string{"pricing", "availability", "create", "--app", "app-1"}, test.selector...)
			args = append(args, "--available", "true", "--available-in-new-territories", "true")
			stdout, stderr, seen, runErr := runIfExistsCommand(t, args, func(req ifExistsRequest) (*http.Response, error) {
				t.Fatalf("unexpected request %s %s; selector validation must precede HTTP", req.Method, req.Path)
				return nil, nil
			})
			if runErr == nil {
				t.Fatal("run error = nil, want a usage error")
			}
			if got := cmd.ExitCodeFromError(runErr); got != cmd.ExitUsage {
				t.Fatalf("exit code = %d, want %d", got, cmd.ExitUsage)
			}
			if stdout != "" {
				t.Fatalf("stdout = %q, want empty", stdout)
			}
			if !strings.Contains(stderr, test.wantErr) {
				t.Fatalf("stderr = %q, want %q", stderr, test.wantErr)
			}
			if len(seen) != 0 {
				t.Fatalf("requests = %+v, want none", seen)
			}
		})
	}
}

func TestPricingAvailabilityCreateAllTerritoriesIfExistsUpdatesEveryExistingTerritory(t *testing.T) {
	// The existing record has USA unavailable and GBR available, so
	// --all-territories --available true must PATCH only USA through the edit
	// path, without the caller naming either territory.
	territoriesBefore := `{"data":[{"type":"territoryAvailabilities","id":"ta-usa","attributes":{"available":false},"relationships":{"territory":{"data":{"type":"territories","id":"USA"}}}},{"type":"territoryAvailabilities","id":"ta-gbr","attributes":{"available":true},"relationships":{"territory":{"data":{"type":"territories","id":"GBR"}}}}],"links":{}}`
	patched := []string{}
	_, stderr, _, runErr := runIfExistsCommand(t, []string{
		"pricing", "availability", "create",
		"--app", "app-1",
		"--all-territories",
		"--available", "true",
		"--available-in-new-territories", "true",
		"--if-exists", "update",
		"--output", "json",
	}, func(req ifExistsRequest) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.Path == "/v1/territories":
			return jsonResponse(http.StatusOK, territoryCatalog)
		case req.Method == http.MethodPost && req.Path == "/v2/appAvailabilities":
			return jsonResponse(http.StatusConflict, availabilityExists409)
		case req.Method == http.MethodGet && req.Path == "/v1/apps/app-1/appAvailabilityV2":
			return jsonResponse(http.StatusOK, existingAvailability)
		case req.Method == http.MethodGet && req.Path == "/v2/appAvailabilities/availability-1/territoryAvailabilities":
			if len(patched) > 0 {
				return jsonResponse(http.StatusOK, existingTerritoryAvailabilities)
			}
			return jsonResponse(http.StatusOK, territoriesBefore)
		case req.Method == http.MethodPatch && strings.HasPrefix(req.Path, "/v1/territoryAvailabilities/"):
			patched = append(patched, strings.TrimPrefix(req.Path, "/v1/territoryAvailabilities/"))
			return jsonResponse(http.StatusOK, `{"data":{"type":"territoryAvailabilities","id":"ta-usa","attributes":{"available":true}}}`)
		default:
			t.Fatalf("unexpected request %s %s", req.Method, req.Path)
			return nil, nil
		}
	})
	if runErr != nil {
		t.Fatalf("run error: %v", runErr)
	}
	if got := strings.Join(patched, ","); got != "ta-usa" {
		t.Fatalf("patched = %q, want only ta-usa", got)
	}
	if !strings.Contains(stderr, "Updated 1 territories; 1 already matched") {
		t.Fatalf("stderr = %q, want the edit path's all-territories summary", stderr)
	}
	if !strings.Contains(stderr, "updated it in place (--if-exists update)") {
		t.Fatalf("stderr = %q, want the update outcome", stderr)
	}
}
