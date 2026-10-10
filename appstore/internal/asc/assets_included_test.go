package asc

import (
	"context"
	"net/http"
	"reflect"
	"testing"
)

func TestAppScreenshotSetsWithScreenshotsUsesIncludedScreenshots(t *testing.T) {
	setsBody := `{
		"data": [
			{"type":"appScreenshotSets","id":"set-a","attributes":{"screenshotDisplayType":"APP_IPHONE_65"},
			 "relationships":{"appScreenshots":{"links":{"related":"r-a"},"meta":{"paging":{"total":2,"limit":50}},"data":[{"type":"appScreenshots","id":"s2"},{"type":"appScreenshots","id":"s1"}]}}},
			{"type":"appScreenshotSets","id":"set-b","attributes":{"screenshotDisplayType":"APP_IPAD_PRO_3GEN_129"},
			 "relationships":{"appScreenshots":{"links":{"related":"r-b"},"meta":{"paging":{"total":0,"limit":50}},"data":[]}}},
			{"type":"appScreenshotSets","id":"set-c","attributes":{"screenshotDisplayType":"APP_IPHONE_67"},
			 "relationships":{"appScreenshots":{"links":{"related":"r-c"},"meta":{"paging":{"total":2,"limit":1}},"data":[{"type":"appScreenshots","id":"s3"}]}}}
		],
		"included": [
			{"type":"appScreenshots","id":"s1","attributes":{"fileName":"one.png"}},
			{"type":"appScreenshots","id":"s2","attributes":{"fileName":"two.png"}},
			{"type":"appScreenshots","id":"s3","attributes":{"fileName":"three.png"}}
		],
		"links": {}
	}`
	fallbackBody := `{"data":[{"type":"appScreenshots","id":"s3","attributes":{"fileName":"three.png"}},{"type":"appScreenshots","id":"s4","attributes":{"fileName":"four.png"}}],"links":{}}`
	orderBody := `{"data":[{"type":"appScreenshots","id":"s4"},{"type":"appScreenshots","id":"s3"}],"links":{}}`

	var requests []string
	client := newTestClient(t, func(req *http.Request) {
		requests = append(requests, req.URL.Path+"?"+req.URL.RawQuery)
	}, jsonResponse(http.StatusOK, setsBody), jsonResponse(http.StatusOK, fallbackBody), jsonResponse(http.StatusOK, orderBody))

	ctx := context.Background()
	response, err := client.GetAllAppScreenshotSets(ctx, "loc-1", WithAppScreenshotSetsIncludeScreenshots())
	if err != nil {
		t.Fatalf("GetAllAppScreenshotSets() error: %v", err)
	}
	sets, err := client.AppScreenshotSetsWithScreenshots(ctx, response, nil)
	if err != nil {
		t.Fatalf("AppScreenshotSetsWithScreenshots() error: %v", err)
	}

	wantRequests := []string{
		"/v1/appStoreVersionLocalizations/loc-1/appScreenshotSets?include=appScreenshots&limit%5BappScreenshots%5D=50",
		"/v1/appScreenshotSets/set-c/appScreenshots?",
		"/v1/appScreenshotSets/set-c/relationships/appScreenshots?limit=200",
	}
	if !reflect.DeepEqual(requests, wantRequests) {
		t.Fatalf("requests = %q, want %q", requests, wantRequests)
	}

	got := map[string][]string{}
	for _, set := range sets {
		ids := []string{}
		for _, shot := range set.Screenshots {
			ids = append(ids, shot.ID)
		}
		got[set.Set.ID] = ids
	}
	want := map[string][]string{"set-a": {"s2", "s1"}, "set-b": {}, "set-c": {"s4", "s3"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("screenshots by set = %v, want %v", got, want)
	}
	if relationships := string(sets[0].Set.Relationships); relationships != `{"appScreenshots":{"links":{"related":"r-a"}}}` {
		t.Fatalf("set relationships = %s, want linkage removed", relationships)
	}
}
