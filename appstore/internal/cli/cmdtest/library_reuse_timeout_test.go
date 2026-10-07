package cmdtest

import (
	"context"
	"encoding/json"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"testing/synctest"
	"time"
)

func TestLibraryReuseBulkDeleteEachRequestTimeout(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_TIMEOUT", "1s")
	t.Setenv("ASC_TIMEOUT_SECONDS", "")
	t.Setenv("ASC_MAX_RETRIES", "0")
	original := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = original })
	var requests []string
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != "DELETE" || !strings.HasPrefix(req.URL.Path, "/v1/appAssetLibraryPlacements/") {
			t.Fatalf("request=%s %s", req.Method, req.URL)
		}
		requests = append(requests, strings.TrimPrefix(req.URL.Path, "/v1/appAssetLibraryPlacements/"))
		select {
		case <-time.After(750 * time.Millisecond):
			return jsonResponse(http.StatusNoContent, "")
		case <-req.Context().Done():
			return nil, req.Context().Err()
		}
	})
	var err error
	// OS pipe readers must remain outside the simulated-time bubble.
	stdout, stderr := captureOutput(t, func() {
		synctest.Test(t, func(t *testing.T) {
			root := RootCommand("dev")
			if err = root.Parse([]string{"localizations", "placements", "delete", "--placement-ids", "first,second", "--confirm"}); err == nil {
				err = root.Run(context.Background())
			}
		})
	})
	var receipt struct {
		Deleted             bool     `json:"deleted"`
		DeletedPlacementIDs []string `json:"deletedPlacementIds"`
		FailedPlacementID   string   `json:"failedPlacementId"`
	}
	if err != nil || stderr != "" || json.Unmarshal([]byte(stdout), &receipt) != nil || !receipt.Deleted || receipt.FailedPlacementID != "" || !reflect.DeepEqual(receipt.DeletedPlacementIDs, []string{"first", "second"}) || !reflect.DeepEqual(requests, []string{"first", "second"}) {
		t.Fatalf("requests=%v output=%s stderr=%s error=%v", requests, stdout, stderr, err)
	}
}

func TestLibraryReuseBulkDeleteRetainsCallerCancellation(t *testing.T) {
	for _, cancellation := range []string{"deadline", "cancel"} {
		t.Run(cancellation, func(t *testing.T) {
			setupAuth(t)
			t.Setenv("ASC_TIMEOUT", "1s")
			t.Setenv("ASC_TIMEOUT_SECONDS", "")
			t.Setenv("ASC_MAX_RETRIES", "0")
			original := http.DefaultTransport
			t.Cleanup(func() { http.DefaultTransport = original })
			var requests []string
			var cancel context.CancelFunc
			http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
				if req.Method != "DELETE" || !strings.HasPrefix(req.URL.Path, "/v1/appAssetLibraryPlacements/") {
					t.Fatalf("request=%s %s", req.Method, req.URL)
				}
				requests = append(requests, strings.TrimPrefix(req.URL.Path, "/v1/appAssetLibraryPlacements/"))
				if cancellation == "cancel" && len(requests) == 2 {
					cancel()
				}
				select {
				case <-time.After(750 * time.Millisecond):
					return jsonResponse(http.StatusNoContent, "")
				case <-req.Context().Done():
					return nil, req.Context().Err()
				}
			})
			var err, callerErr error
			stdout, _ := captureOutput(t, func() {
				synctest.Test(t, func(t *testing.T) {
					var ctx context.Context
					if cancellation == "deadline" {
						ctx, cancel = context.WithTimeout(context.Background(), 1100*time.Millisecond)
					} else {
						ctx, cancel = context.WithCancel(context.Background())
					}
					defer cancel()
					root := RootCommand("dev")
					if err = root.Parse([]string{"localizations", "placements", "delete", "--placement-ids", "first,second,third", "--confirm"}); err == nil {
						err = root.Run(ctx)
					}
					// Allow deadline callbacks at the same fake instant to finish.
					synctest.Wait()
					callerErr = ctx.Err()
				})
			})
			var receipt struct {
				Deleted             bool     `json:"deleted"`
				DeletedPlacementIDs []string `json:"deletedPlacementIds"`
				FailedPlacementID   string   `json:"failedPlacementId"`
			}
			if err == nil || callerErr == nil || json.Unmarshal([]byte(stdout), &receipt) != nil || receipt.Deleted || receipt.FailedPlacementID != "second" || !reflect.DeepEqual(receipt.DeletedPlacementIDs, []string{"first"}) || !reflect.DeepEqual(requests, []string{"first", "second"}) {
				t.Fatalf("requests=%v output=%s error=%v callerError=%v", requests, stdout, err, callerErr)
			}
		})
	}
}
