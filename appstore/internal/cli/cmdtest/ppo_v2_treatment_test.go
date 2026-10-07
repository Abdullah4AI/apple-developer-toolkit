package cmdtest

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"
)

func TestPPOTreatmentCreateRelationshipVersion(t *testing.T) {
	for _, v2 := range []bool{false, true} {
		name := "v1"
		if v2 {
			name = "v2"
		}
		t.Run(name, func(t *testing.T) {
			setupAuth(t)
			original := http.DefaultTransport
			t.Cleanup(func() { http.DefaultTransport = original })
			called := false
			http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
				called = true
				if req.Method != "POST" || req.URL.Path != "/v1/appStoreVersionExperimentTreatments" {
					t.Fatalf("unexpected request: %s %s", req.Method, req.URL)
				}
				var body struct {
					Data struct {
						Relationships map[string]struct {
							Data struct {
								Type string `json:"type"`
								ID   string `json:"id"`
							} `json:"data"`
						} `json:"relationships"`
					} `json:"data"`
				}
				if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
					t.Fatal(err)
				}
				key := "appStoreVersionExperiment"
				if v2 {
					key += "V2"
				}
				relationship, ok := body.Data.Relationships[key]
				if !ok || len(body.Data.Relationships) != 1 || relationship.Data.ID != "exp-1" || relationship.Data.Type != "appStoreVersionExperiments" {
					t.Fatalf("wrong relationship: %+v", body.Data.Relationships)
				}
				return jsonResponse(201, `{"data":{"type":"appStoreVersionExperimentTreatments","id":"treatment-1","attributes":{"name":"Header"}}}`)
			})
			args := []string{"product-pages", "experiments", "treatments", "create", "--experiment-id", "exp-1", "--name", "Header"}
			if v2 {
				args = append(args, "--v2")
			}
			root := RootCommand("dev")
			root.FlagSet.SetOutput(io.Discard)
			var err error
			captureOutput(t, func() {
				if err = root.Parse(args); err == nil {
					err = root.Run(context.Background())
				}
			})
			if err != nil || !called {
				t.Fatalf("create failed: %v called=%v", err, called)
			}
		})
	}
}
