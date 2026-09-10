package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/handlertest"
)

func TestListResolutionCenterThreadsByAppUsesAppScopedEndpoint(t *testing.T) {
	fixture := handlertest.New(t)
	var gotQuery url.Values
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/apps/app-123/resolutionCenterThreads" {
			fixture.Respond(w, "unexpected path: %s", r.URL.Path)
			return
		}
		gotQuery = r.URL.Query()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"data": [
				{
					"id": "thread-sub",
					"type": "resolutionCenterThreads",
					"attributes": {
						"threadType": "REJECTION_BINARY",
						"state": "OPEN",
						"createdDate": "2026-02-25T00:00:00Z",
						"lastMessageResponseDate": "2026-02-26T00:00:00Z",
						"canDeveloperAddNote": true
					},
					"relationships": {
						"reviewSubmission": {"data": {"type": "reviewSubmissions", "id": "sub-1"}},
						"appStoreVersions": {"data": [{"type": "appStoreVersions", "id": "ver-1"}]}
					}
				},
				{
					"id": "thread-app",
					"type": "resolutionCenterThreads",
					"attributes": {
						"threadType": "APP_MESSAGE_INFORMATIONAL",
						"state": "OPEN",
						"createdDate": "2026-02-20T00:00:00Z"
					},
					"relationships": {
						"reviewSubmission": {"data": null}
					}
				}
			]
		}`))
	}))
	defer server.Close()

	threads, err := testWebClient(server).ListResolutionCenterThreadsByApp(context.Background(), "app-123")
	if err != nil {
		t.Fatalf("ListResolutionCenterThreadsByApp error = %v", err)
	}

	if got := gotQuery.Get("include"); got != "appStoreVersions,app,appMessageThreadDetail,build,betaBackgroundAssetReviewSubmission" {
		t.Fatalf("unexpected include: %q", got)
	}
	wantThreadTypes := "REJECTION_BINARY,REJECTION_METADATA,REJECTION_REVIEW_SUBMISSION,APP_MESSAGE_ARC,APP_MESSAGE_ARB,APP_MESSAGE_COMM,APP_MESSAGE_INFORMATIONAL"
	if got := gotQuery.Get("filter[threadType]"); got != wantThreadTypes {
		t.Fatalf("unexpected filter[threadType]: %q", got)
	}
	if got := gotQuery.Get("limit[appStoreVersions]"); got != "2000" {
		t.Fatalf("unexpected limit[appStoreVersions]: %q", got)
	}
	if _, exists := gotQuery["filter[reviewSubmission]"]; exists {
		t.Fatalf("app-scoped listing must not send filter[reviewSubmission]: %v", gotQuery)
	}

	if len(threads) != 2 {
		t.Fatalf("expected 2 threads, got %d (%#v)", len(threads), threads)
	}
	if threads[0].ID != "thread-sub" || threads[0].ReviewSubmissionID != "sub-1" {
		t.Fatalf("unexpected submission-attached thread: %#v", threads[0])
	}
	if threads[0].ThreadType != "REJECTION_BINARY" || !threads[0].CanDeveloperAddNote {
		t.Fatalf("unexpected thread attributes: %#v", threads[0])
	}
	if len(threads[0].AppStoreVersionIDs) != 1 || threads[0].AppStoreVersionIDs[0] != "ver-1" {
		t.Fatalf("unexpected app store version ids: %#v", threads[0].AppStoreVersionIDs)
	}
	// The reopened gap: a thread with no reviewSubmission relationship is
	// invisible to the submission-filtered reader but blocks the app.
	if threads[1].ID != "thread-app" || threads[1].ReviewSubmissionID != "" {
		t.Fatalf("expected an app-only thread, got %#v", threads[1])
	}
}

func TestListResolutionCenterThreadsByAppFindsThreadsSubmissionFilterMisses(t *testing.T) {
	fixture := handlertest.New(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/resolutionCenterThreads":
			if got := r.URL.Query().Get("filter[reviewSubmission]"); got != "sub-1" {
				fixture.Respond(w, "unexpected submission filter: %q", got)
				return
			}
			_, _ = w.Write([]byte(`{"data": [{"id": "thread-sub", "type": "resolutionCenterThreads", "attributes": {"state": "OPEN"}}]}`))
		case "/apps/app-123/resolutionCenterThreads":
			_, _ = w.Write([]byte(`{"data": [
				{"id": "thread-sub", "type": "resolutionCenterThreads", "attributes": {"state": "OPEN"}},
				{"id": "thread-metadata", "type": "resolutionCenterThreads", "attributes": {"state": "OPEN", "threadType": "REJECTION_METADATA"}}
			]}`))
		default:
			fixture.Respond(w, "unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()
	client := testWebClient(server)

	bySubmission, err := client.ListResolutionCenterThreadsBySubmission(context.Background(), "sub-1")
	if err != nil {
		t.Fatalf("ListResolutionCenterThreadsBySubmission error = %v", err)
	}
	if len(bySubmission) != 1 || bySubmission[0].ID != "thread-sub" {
		t.Fatalf("unexpected submission-scoped threads: %#v", bySubmission)
	}

	byApp, err := client.ListResolutionCenterThreadsByApp(context.Background(), "app-123")
	if err != nil {
		t.Fatalf("ListResolutionCenterThreadsByApp error = %v", err)
	}
	if len(byApp) != 2 {
		t.Fatalf("expected app scope to return the thread the submission filter missed, got %#v", byApp)
	}
	if byApp[1].ID != "thread-metadata" {
		t.Fatalf("expected thread-metadata from the app scope, got %#v", byApp[1])
	}
}

func TestListResolutionCenterThreadsByAppFollowsNextLinks(t *testing.T) {
	fixture := handlertest.New(t)
	nextLink := ""
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/apps/app-123/resolutionCenterThreads" {
			fixture.Respond(w, "unexpected path: %s", r.URL.Path)
			return
		}
		requests++
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Query().Get("cursor") == "page-2" {
			_, _ = w.Write([]byte(`{"data": [{"id": "thread-2", "type": "resolutionCenterThreads", "attributes": {"state": "CLOSED"}}]}`))
			return
		}
		_, _ = w.Write([]byte(`{
			"data": [{"id": "thread-1", "type": "resolutionCenterThreads", "attributes": {"state": "OPEN"}}],
			"links": {"next": "` + nextLink + `"}
		}`))
	}))
	defer server.Close()
	nextLink = server.URL + "/apps/app-123/resolutionCenterThreads?cursor=page-2"

	threads, err := testWebClient(server).ListResolutionCenterThreadsByApp(context.Background(), "app-123")
	if err != nil {
		t.Fatalf("ListResolutionCenterThreadsByApp error = %v", err)
	}
	if requests != 2 {
		t.Fatalf("expected 2 page requests, got %d", requests)
	}
	if len(threads) != 2 || threads[0].ID != "thread-1" || threads[1].ID != "thread-2" {
		t.Fatalf("unexpected paginated threads: %#v", threads)
	}
}

func TestListResolutionCenterThreadsByAppRequiresAppID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("unexpected request: %s", r.URL.Path)
	}))
	defer server.Close()

	if _, err := testWebClient(server).ListResolutionCenterThreadsByApp(context.Background(), "  "); err == nil {
		t.Fatal("expected an error for a blank app id")
	} else if !strings.Contains(err.Error(), "app id is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGetResolutionCenterDraftMessageDecodesDraft(t *testing.T) {
	fixture := handlertest.New(t)
	var gotQuery url.Values
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/resolutionCenterThreads/thread-1/resolutionCenterDraftMessage" {
			fixture.Respond(w, "unexpected path: %s", r.URL.Path)
			return
		}
		gotQuery = r.URL.Query()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"data": {
				"id": "draft-1",
				"type": "resolutionCenterDraftMessages",
				"attributes": {
					"createdDate": "2026-03-01T09:00:00Z",
					"messageBody": "<p>Draft reply &amp; notes</p>"
				},
				"relationships": {
					"fromActor": {"data": {"type": "actors", "id": "actor-1"}},
					"resolutionCenterMessageAttachments": {"data": [{"type": "resolutionCenterMessageAttachments", "id": "att-1"}]}
				}
			},
			"included": [
				{"id": "actor-1", "type": "actors", "attributes": {"actorType": "APP_STORE_CONNECT_USER", "name": "Rudrank Riyam"}},
				{
					"id": "att-1",
					"type": "resolutionCenterMessageAttachments",
					"attributes": {
						"fileName": "notes.txt",
						"fileSize": 42,
						"assetDeliveryState": "AVAILABLE",
						"downloadUrl": "https://iosapps.itunes.apple.com/signed?token=secret"
					}
				}
			]
		}`))
	}))
	defer server.Close()

	draft, err := testWebClient(server).GetResolutionCenterDraftMessage(context.Background(), "thread-1", true)
	if err != nil {
		t.Fatalf("GetResolutionCenterDraftMessage error = %v", err)
	}
	if draft == nil {
		t.Fatal("expected a draft message")
	}

	if got := gotQuery.Get("include"); got != "resolutionCenterMessageAttachments,fromActor" {
		t.Fatalf("unexpected include: %q", got)
	}
	if got := gotQuery.Get("limit[resolutionCenterMessageAttachments]"); got != "1000" {
		t.Fatalf("unexpected attachment limit: %q", got)
	}

	if draft.ID != "draft-1" || draft.ThreadID != "thread-1" {
		t.Fatalf("unexpected draft identity: %#v", draft)
	}
	if draft.CreatedDate != "2026-03-01T09:00:00Z" {
		t.Fatalf("unexpected created date: %q", draft.CreatedDate)
	}
	if draft.MessageBody != "<p>Draft reply &amp; notes</p>" {
		t.Fatalf("expected raw HTML preserved verbatim, got %q", draft.MessageBody)
	}
	if draft.MessageBodyPlain != "Draft reply & notes" {
		t.Fatalf("unexpected plain text projection: %q", draft.MessageBodyPlain)
	}
	if draft.FromActor == nil || draft.FromActor.ID != "actor-1" {
		t.Fatalf("unexpected from actor: %#v", draft.FromActor)
	}
	if len(draft.Attachments) != 1 {
		t.Fatalf("expected one attachment, got %#v", draft.Attachments)
	}
	attachment := draft.Attachments[0]
	if attachment.AttachmentID != "att-1" || attachment.FileName != "notes.txt" || attachment.FileSize != 42 {
		t.Fatalf("unexpected attachment metadata: %#v", attachment)
	}
	if !attachment.Downloadable {
		t.Fatalf("expected the attachment to report as downloadable: %#v", attachment)
	}
	if attachment.DownloadURL != "" {
		t.Fatalf("expected the signed url to be redacted, got %q", attachment.DownloadURL)
	}
	if attachment.ThreadID != "thread-1" || attachment.MessageID != "draft-1" {
		t.Fatalf("unexpected attachment linkage: %#v", attachment)
	}
}

func TestGetResolutionCenterDraftMessageKeepsRawHTMLWithoutProjection(t *testing.T) {
	fixture := handlertest.New(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/resolutionCenterThreads/thread-1/resolutionCenterDraftMessage" {
			fixture.Respond(w, "unexpected path: %s", r.URL.Path)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data": {"id": "draft-1", "type": "resolutionCenterDraftMessages", "attributes": {"messageBody": "<p>Body</p>"}}}`))
	}))
	defer server.Close()

	draft, err := testWebClient(server).GetResolutionCenterDraftMessage(context.Background(), "thread-1", false)
	if err != nil {
		t.Fatalf("GetResolutionCenterDraftMessage error = %v", err)
	}
	if draft == nil {
		t.Fatal("expected a draft message")
	}
	if draft.MessageBody != "<p>Body</p>" {
		t.Fatalf("unexpected message body: %q", draft.MessageBody)
	}
	if draft.MessageBodyPlain != "" {
		t.Fatalf("expected no plain text projection, got %q", draft.MessageBodyPlain)
	}
}

func TestGetResolutionCenterDraftMessageReportsAbsentDraft(t *testing.T) {
	tests := []struct {
		name   string
		status int
		body   string
	}{
		{name: "null data", status: http.StatusOK, body: `{"data": null}`},
		{name: "not found", status: http.StatusNotFound, body: `{"errors": [{"code": "NOT_FOUND"}]}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tt.status)
				_, _ = w.Write([]byte(tt.body))
			}))
			defer server.Close()

			draft, err := testWebClient(server).GetResolutionCenterDraftMessage(context.Background(), "thread-1", false)
			if err != nil {
				t.Fatalf("GetResolutionCenterDraftMessage error = %v", err)
			}
			if draft != nil {
				t.Fatalf("expected no draft message, got %#v", draft)
			}
		})
	}
}

func TestGetResolutionCenterDraftMessageRejectsMissingData(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{}`))
	}))
	defer server.Close()

	draft, err := testWebClient(server).GetResolutionCenterDraftMessage(context.Background(), "thread-1", false)
	if err == nil {
		t.Fatalf("expected missing data error, got draft=%#v err=%v", draft, err)
	}
}

func TestGetResolutionCenterDraftMessageSurfacesServerErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"errors": [{"code": "PARAMETER_ERROR.INVALID"}]}`))
	}))
	defer server.Close()

	if _, err := testWebClient(server).GetResolutionCenterDraftMessage(context.Background(), "thread-1", false); err == nil {
		t.Fatal("expected a 400 to surface as an error")
	}
}

func TestGetResolutionCenterDraftMessageRequiresThreadID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("unexpected request: %s", r.URL.Path)
	}))
	defer server.Close()

	if _, err := testWebClient(server).GetResolutionCenterDraftMessage(context.Background(), " ", false); err == nil {
		t.Fatal("expected an error for a blank thread id")
	} else if !strings.Contains(err.Error(), "thread id is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}
