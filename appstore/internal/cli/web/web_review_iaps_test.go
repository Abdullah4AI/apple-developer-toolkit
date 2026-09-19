package web

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"net/http"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/cli/shared"
	webcore "github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/web"
)

func TestWebReviewIAPsAttachRequiresApp(t *testing.T) {
	cmd := WebReviewIAPsAttachCommand()
	if err := cmd.FlagSet.Parse([]string{
		"--iap-id", "9000000001",
		"--confirm",
	}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	_, stderr := captureOutput(t, func() {
		err := cmd.Exec(context.Background(), nil)
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected flag.ErrHelp, got %v", err)
		}
	})
	if !strings.Contains(stderr, "--app is required") {
		t.Fatalf("expected --app guidance in stderr, got %q", stderr)
	}
}

func TestWebReviewIAPsAttachRequiresIAPID(t *testing.T) {
	cmd := WebReviewIAPsAttachCommand()
	if err := cmd.FlagSet.Parse([]string{
		"--app", "123456789",
		"--confirm",
	}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	_, stderr := captureOutput(t, func() {
		err := cmd.Exec(context.Background(), nil)
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected flag.ErrHelp, got %v", err)
		}
	})
	if !strings.Contains(stderr, "--iap-id is required") {
		t.Fatalf("expected --iap-id guidance in stderr, got %q", stderr)
	}
}

func TestWebReviewIAPsAttachRequiresConfirm(t *testing.T) {
	cmd := WebReviewIAPsAttachCommand()
	if err := cmd.FlagSet.Parse([]string{
		"--app", "123456789",
		"--iap-id", "9000000001",
	}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	_, stderr := captureOutput(t, func() {
		err := cmd.Exec(context.Background(), nil)
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected flag.ErrHelp, got %v", err)
		}
	})
	if !strings.Contains(stderr, "--confirm is required") {
		t.Fatalf("expected --confirm guidance in stderr, got %q", stderr)
	}
}

func TestWebReviewIAPsAttachRejectsNonNumericAppID(t *testing.T) {
	cmd := WebReviewIAPsAttachCommand()
	if err := cmd.FlagSet.Parse([]string{
		"--app", "com.example.app",
		"--iap-id", "9000000001",
		"--confirm",
	}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	_, stderr := captureOutput(t, func() {
		err := cmd.Exec(context.Background(), nil)
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected flag.ErrHelp, got %v", err)
		}
	})
	if !strings.Contains(stderr, "--app must be a numeric App Store Connect app ID") {
		t.Fatalf("expected numeric --app guidance in stderr, got %q", stderr)
	}
}

// TestWebReviewIAPsAttachPostsIrisResourceIDWhenSelectorWasProductID guards
// the subtle correctness property requested in PR review: when --iap-id is a
// bundle-style productId (matched against attributes.productId), the
// /iris/v1/inAppPurchaseSubmissions POST must still carry the iris resource
// id in the relationship, not the productId selector.
func TestWebReviewIAPsAttachPostsIrisResourceIDWhenSelectorWasProductID(t *testing.T) {
	_ = stubWebProgressLabels(t)

	origResolveSession := resolveSessionFn
	t.Cleanup(func() {
		resolveSessionFn = origResolveSession
	})

	const (
		irisResourceID = "ae6d89d7-15c5-4a3d-9041-663a4d40638e"
		productID      = "com.example.lifetime"
	)

	resolveSessionFn = func(ctx context.Context, appleID, password, twoFactorCode string) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{
			Client: &http.Client{
				Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
					switch {
					case req.Method == http.MethodGet && req.URL.Path == "/iris/v1/apps/123456789/inAppPurchases":
						return &http.Response{
							StatusCode: http.StatusOK,
							Header:     http.Header{"Content-Type": []string{"application/json"}},
							Body: io.NopCloser(strings.NewReader(`{
								"data": [{
									"type": "inAppPurchases",
									"id": "` + irisResourceID + `",
									"attributes": {
										"productId": "` + productID + `",
										"referenceName": "Lifetime",
										"state": "READY_TO_SUBMIT"
									}
								}]
							}`)),
							Request: req,
						}, nil
					case req.Method == http.MethodPost && req.URL.Path == "/iris/v1/inAppPurchaseSubmissions":
						body, err := io.ReadAll(req.Body)
						if err != nil {
							t.Fatalf("read POST body: %v", err)
						}
						var post struct {
							Data struct {
								Relationships struct {
									InAppPurchaseV2 struct {
										Data struct {
											ID string `json:"id"`
										} `json:"data"`
									} `json:"inAppPurchaseV2"`
								} `json:"relationships"`
							} `json:"data"`
						}
						if err := json.Unmarshal(body, &post); err != nil {
							t.Fatalf("decode POST body: %v; body=%s", err, body)
						}
						gotID := post.Data.Relationships.InAppPurchaseV2.Data.ID
						if gotID != irisResourceID {
							t.Fatalf("expected attach POST to carry iris resource id %q, got %q", irisResourceID, gotID)
						}
						if gotID == productID {
							t.Fatalf("attach POST must not carry productId as relationship id; body=%s", body)
						}
						return &http.Response{
							StatusCode: http.StatusCreated,
							Header:     http.Header{"Content-Type": []string{"application/json"}},
							Body: io.NopCloser(strings.NewReader(`{
								"data": {
									"type": "inAppPurchaseSubmissions",
									"id": "submission-1",
									"attributes": {"submitWithNextAppStoreVersion": true},
									"relationships": {
										"inAppPurchaseV2": {
											"data": {"type": "inAppPurchases", "id": "` + irisResourceID + `"}
										}
									}
								}
							}`)),
							Request: req,
						}, nil
					default:
						t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
						return nil, nil
					}
				}),
			},
		}, "cache", nil
	}

	cmd := WebReviewIAPsAttachCommand()
	if err := cmd.FlagSet.Parse([]string{
		"--app", "123456789",
		"--iap-id", productID,
		"--confirm",
		"--output", "json",
	}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	stdout, _ := captureOutput(t, func() {
		if err := cmd.Exec(context.Background(), nil); err != nil {
			t.Fatalf("exec error: %v", err)
		}
	})

	var payload reviewIAPMutationOutput
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("failed to parse stdout JSON: %v\nstdout=%s", err, stdout)
	}
	// IAPID in the output preserves what the caller passed (the selector).
	if payload.IAPID != productID {
		t.Fatalf("expected IAPID to echo caller selector %q, got %q", productID, payload.IAPID)
	}
	// Submission.InAppPurchaseID is the iris-resolved id (from the POST response relationship).
	if payload.Submission.InAppPurchaseID != irisResourceID {
		t.Fatalf("expected Submission.InAppPurchaseID = %q, got %q", irisResourceID, payload.Submission.InAppPurchaseID)
	}
}

func TestWebReviewIAPsAttachTreatsAlreadyAttachedConflictAsNoChange(t *testing.T) {
	_ = stubWebProgressLabels(t)

	origResolveSession := resolveSessionFn
	t.Cleanup(func() {
		resolveSessionFn = origResolveSession
	})

	const (
		irisResourceID = "ae6d89d7-15c5-4a3d-9041-663a4d40638e"
		productID      = "com.example.lifetime"
	)

	resolveSessionFn = func(ctx context.Context, appleID, password, twoFactorCode string) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{
			Client: &http.Client{
				Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
					switch {
					case req.Method == http.MethodGet && req.URL.Path == "/iris/v1/apps/123456789/inAppPurchases":
						return &http.Response{
							StatusCode: http.StatusOK,
							Header:     http.Header{"Content-Type": []string{"application/json"}},
							Body: io.NopCloser(strings.NewReader(`{
								"data": [{
									"type": "inAppPurchases",
									"id": "` + irisResourceID + `",
									"attributes": {
										"productId": "` + productID + `",
										"referenceName": "Lifetime",
										"state": "READY_TO_SUBMIT"
									}
								}]
							}`)),
							Request: req,
						}, nil
					case req.Method == http.MethodPost && req.URL.Path == "/iris/v1/inAppPurchaseSubmissions":
						return &http.Response{
							StatusCode: http.StatusConflict,
							Header:     http.Header{"Content-Type": []string{"application/json"}},
							Body: io.NopCloser(strings.NewReader(`{
								"errors": [{
									"status": "409",
									"code": "ENTITY_ERROR.ATTRIBUTE.INVALID.ALREADY_EXISTS",
									"title": "The request entity conflicts with the current state."
								}]
							}`)),
							Request: req,
						}, nil
					default:
						t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
						return nil, nil
					}
				}),
			},
		}, "cache", nil
	}

	cmd := WebReviewIAPsAttachCommand()
	if err := cmd.FlagSet.Parse([]string{
		"--app", "123456789",
		"--iap-id", productID,
		"--confirm",
		"--output", "json",
	}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	stdout, _ := captureOutput(t, func() {
		if err := cmd.Exec(context.Background(), nil); err != nil {
			t.Fatalf("exec error: %v", err)
		}
	})

	var payload reviewIAPMutationOutput
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("failed to parse stdout JSON: %v\nstdout=%s", err, stdout)
	}
	if payload.IAPID != productID {
		t.Fatalf("expected IAPID to echo caller selector %q, got %q", productID, payload.IAPID)
	}
	if payload.Changed {
		t.Fatalf("expected already-attached conflict to report changed=false, got %#v", payload)
	}
	if payload.Submission.ID != "" {
		t.Fatalf("expected no submission id for idempotent conflict, got %#v", payload.Submission)
	}
	if payload.Submission.InAppPurchaseID != irisResourceID || !payload.Submission.SubmitWithNextAppStoreVersion {
		t.Fatalf("unexpected idempotent submission output: %#v", payload.Submission)
	}
}

func TestWebReviewIAPsAttachRejectsMatchedIAPWithoutIrisResourceID(t *testing.T) {
	_ = stubWebProgressLabels(t)

	origResolveSession := resolveSessionFn
	t.Cleanup(func() {
		resolveSessionFn = origResolveSession
	})

	resolveSessionFn = func(ctx context.Context, appleID, password, twoFactorCode string) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{
			Client: &http.Client{
				Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
					switch {
					case req.Method == http.MethodGet && req.URL.Path == "/iris/v1/apps/123456789/inAppPurchases":
						return &http.Response{
							StatusCode: http.StatusOK,
							Header:     http.Header{"Content-Type": []string{"application/json"}},
							Body: io.NopCloser(strings.NewReader(`{
								"data": [{
									"type": "inAppPurchases",
									"id": "   ",
									"attributes": {
										"productId": "com.example.lifetime",
										"referenceName": "Lifetime",
										"state": "READY_TO_SUBMIT"
									}
								}]
							}`)),
							Request: req,
						}, nil
					case req.Method == http.MethodPost && req.URL.Path == "/iris/v1/inAppPurchaseSubmissions":
						t.Fatal("attach POST must not run when the matched IAP has no iris resource id")
						return nil, nil
					default:
						t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
						return nil, nil
					}
				}),
			},
		}, "cache", nil
	}

	cmd := WebReviewIAPsAttachCommand()
	if err := cmd.FlagSet.Parse([]string{
		"--app", "123456789",
		"--iap-id", "com.example.lifetime",
		"--confirm",
	}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	_, stderr := captureOutput(t, func() {
		err := cmd.Exec(context.Background(), nil)
		if err == nil {
			t.Fatal("expected missing iris id error, got nil")
		}
		if !strings.Contains(err.Error(), "missing an iris resource id") {
			t.Fatalf("expected missing iris id error, got %v", err)
		}
	})
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
}

func TestWebReviewIAPsAttachVerifiesIAPBelongsToAppBeforeMutating(t *testing.T) {
	_ = stubWebProgressLabels(t)

	origResolveSession := resolveSessionFn
	t.Cleanup(func() {
		resolveSessionFn = origResolveSession
	})

	verified := false
	resolveSessionFn = func(ctx context.Context, appleID, password, twoFactorCode string) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{
			Client: &http.Client{
				Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
					switch {
					case req.Method == http.MethodGet && req.URL.Path == "/iris/v1/apps/123456789/inAppPurchases":
						verified = true
						return &http.Response{
							StatusCode: http.StatusOK,
							Header:     http.Header{"Content-Type": []string{"application/json"}},
							Body: io.NopCloser(strings.NewReader(`{
								"data": [{
									"type": "inAppPurchases",
									"id": "9000000001",
									"attributes": {
										"name": "Remove Ads",
										"productId": "com.example.removeads",
										"inAppPurchaseType": "NON_CONSUMABLE",
										"state": "READY_TO_SUBMIT",
										"submitWithNextAppStoreVersion": false
									}
								}]
							}`)),
							Request: req,
						}, nil
					case req.Method == http.MethodPost && req.URL.Path == "/iris/v1/inAppPurchaseSubmissions":
						if !verified {
							t.Fatal("expected app-scoped IAP verification before web mutation")
						}
					default:
						t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
					}
					requestBody, err := io.ReadAll(req.Body)
					if err != nil {
						t.Fatalf("read request body: %v", err)
					}
					body := string(requestBody)
					for _, want := range []string{
						`"id":"9000000001"`,
						`"submitWithNextAppStoreVersion":true`,
						`"inAppPurchaseV2"`,
					} {
						if !strings.Contains(body, want) {
							t.Fatalf("expected request body to contain %q, got %s", want, body)
						}
					}
					return &http.Response{
						StatusCode: http.StatusCreated,
						Header:     http.Header{"Content-Type": []string{"application/json"}},
						Body: io.NopCloser(strings.NewReader(`{
							"data": {
								"type": "inAppPurchaseSubmissions",
								"id": "submission-1",
								"attributes": {
									"submitWithNextAppStoreVersion": true
								},
								"relationships": {
									"inAppPurchaseV2": {
										"data": {"type": "inAppPurchases", "id": "9000000001"}
									}
								}
							}
						}`)),
						Request: req,
					}, nil
				}),
			},
		}, "cache", nil
	}

	cmd := WebReviewIAPsAttachCommand()
	if err := cmd.FlagSet.Parse([]string{
		"--app", "123456789",
		"--iap-id", "9000000001",
		"--confirm",
		"--output", "json",
	}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	stdout, _ := captureOutput(t, func() {
		if err := cmd.Exec(context.Background(), nil); err != nil {
			t.Fatalf("exec error: %v", err)
		}
	})

	var payload reviewIAPMutationOutput
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("failed to parse stdout JSON: %v\nstdout=%s", err, stdout)
	}
	if payload.AppID != "123456789" || payload.IAPID != "9000000001" || payload.Operation != "attach" || !payload.Changed {
		t.Fatalf("unexpected mutation output: %#v", payload)
	}
	if payload.Submission.ID != "submission-1" || payload.Submission.InAppPurchaseID != "9000000001" {
		t.Fatalf("unexpected submission output: %#v", payload.Submission)
	}
}

func TestWebReviewIAPsAttachSkipsAlreadyAttachedIAP(t *testing.T) {
	_ = stubWebProgressLabels(t)

	origResolveSession := resolveSessionFn
	t.Cleanup(func() {
		resolveSessionFn = origResolveSession
	})

	resolveSessionFn = func(ctx context.Context, appleID, password, twoFactorCode string) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{
			Client: &http.Client{
				Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
					if req.Method != http.MethodGet || req.URL.Path != "/iris/v1/apps/123456789/inAppPurchases" {
						t.Fatalf("unexpected request for already-attached IAP: %s %s", req.Method, req.URL.Path)
					}
					return &http.Response{
						StatusCode: http.StatusOK,
						Header:     http.Header{"Content-Type": []string{"application/json"}},
						Body: io.NopCloser(strings.NewReader(`{
							"data": [{
								"type": "inAppPurchases",
								"id": "9000000001",
								"attributes": {
									"name": "Remove Ads",
									"productId": "com.example.removeads",
									"inAppPurchaseType": "NON_CONSUMABLE",
									"state": "READY_TO_SUBMIT",
									"submitWithNextAppStoreVersion": true
								}
							}]
						}`)),
						Request: req,
					}, nil
				}),
			},
		}, "cache", nil
	}

	cmd := WebReviewIAPsAttachCommand()
	if err := cmd.FlagSet.Parse([]string{
		"--app", "123456789",
		"--iap-id", "9000000001",
		"--confirm",
		"--output", "json",
	}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	stdout, _ := captureOutput(t, func() {
		if err := cmd.Exec(context.Background(), nil); err != nil {
			t.Fatalf("exec error: %v", err)
		}
	})

	var payload reviewIAPMutationOutput
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("failed to parse stdout JSON: %v\nstdout=%s", err, stdout)
	}
	if payload.AppID != "123456789" || payload.IAPID != "9000000001" || payload.Operation != "attach" {
		t.Fatalf("unexpected mutation output: %#v", payload)
	}
	if payload.Changed {
		t.Fatalf("expected already-attached IAP to report changed=false, got %#v", payload)
	}
	if payload.Submission.ID != "" || payload.Submission.InAppPurchaseID != "9000000001" || !payload.Submission.SubmitWithNextAppStoreVersion {
		t.Fatalf("unexpected idempotent submission output: %#v", payload.Submission)
	}
}

// In production, iris never returns `submitWithNextAppStoreVersion` in the
// listing (it is filtered out via `reviewIAPFields`), so the existing
// short-circuit above never fires. The fallback signal is `state`, which iris
// does return. When state is one of the in-flight values, the IAP is already
// enrolled in the next app version review and the CLI must short-circuit
// before POSTing — otherwise iris answers a re-attach with
// `ENTITY_ERROR.RELATIONSHIP.INVALID` (UUID form) or
// `ENTITY_ERROR.ATTRIBUTE.INVALID.UNMODIFIABLE` (numeric form). This test
// asserts no POST is made.
func TestWebReviewIAPsAttachShortCircuitsWhenStateIsWaitingForReview(t *testing.T) {
	_ = stubWebProgressLabels(t)

	origResolveSession := resolveSessionFn
	t.Cleanup(func() {
		resolveSessionFn = origResolveSession
	})

	const (
		irisResourceID = "ae6d89d7-15c5-4a3d-9041-663a4d40638e"
		productID      = "com.example.lifetime"
	)

	resolveSessionFn = func(ctx context.Context, appleID, password, twoFactorCode string) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{
			Client: &http.Client{
				Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
					if req.Method == http.MethodPost {
						t.Fatalf("unexpected POST while IAP is already in flight: %s %s", req.Method, req.URL.Path)
					}
					if req.Method != http.MethodGet || req.URL.Path != "/iris/v1/apps/123456789/inAppPurchases" {
						t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
					}
					return &http.Response{
						StatusCode: http.StatusOK,
						Header:     http.Header{"Content-Type": []string{"application/json"}},
						Body: io.NopCloser(strings.NewReader(`{
							"data": [{
								"type": "inAppPurchases",
								"id": "` + irisResourceID + `",
								"attributes": {
									"productId": "` + productID + `",
									"referenceName": "Lifetime",
									"state": "WAITING_FOR_REVIEW"
								}
							}]
						}`)),
						Request: req,
					}, nil
				}),
			},
		}, "cache", nil
	}

	cmd := WebReviewIAPsAttachCommand()
	if err := cmd.FlagSet.Parse([]string{
		"--app", "123456789",
		"--iap-id", productID,
		"--confirm",
		"--output", "json",
	}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	stdout, _ := captureOutput(t, func() {
		if err := cmd.Exec(context.Background(), nil); err != nil {
			t.Fatalf("exec error: %v", err)
		}
	})

	var payload reviewIAPMutationOutput
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("failed to parse stdout JSON: %v\nstdout=%s", err, stdout)
	}
	if payload.IAPID != productID {
		t.Fatalf("expected IAPID to echo caller selector %q, got %q", productID, payload.IAPID)
	}
	if payload.Changed {
		t.Fatalf("expected in-flight IAP to report changed=false, got %#v", payload)
	}
	if payload.Submission.ID != "" {
		t.Fatalf("expected no submission id when short-circuiting, got %#v", payload.Submission)
	}
	if payload.Submission.InAppPurchaseID != irisResourceID || !payload.Submission.SubmitWithNextAppStoreVersion {
		t.Fatalf("unexpected idempotent submission output: %#v", payload.Submission)
	}
}

func TestIAPStateIndicatesAlreadyAttached(t *testing.T) {
	cases := []struct {
		state string
		want  bool
	}{
		{"WAITING_FOR_REVIEW", true},
		{"IN_REVIEW", true},
		{"PENDING_BINARY_APPROVAL", true},
		{"waiting_for_review", true},     // case-insensitive
		{"  WAITING_FOR_REVIEW  ", true}, // trimmed
		{"READY_TO_SUBMIT", false},
		{"MISSING_METADATA", false},
		{"REJECTED", false},
		{"DEVELOPER_ACTION_NEEDED", false},
		{"APPROVED", false},
		{"", false},
	}
	for _, tc := range cases {
		t.Run(tc.state, func(t *testing.T) {
			if got := iapStateIndicatesAlreadyAttached(tc.state); got != tc.want {
				t.Fatalf("iapStateIndicatesAlreadyAttached(%q)=%v want %v", tc.state, got, tc.want)
			}
		})
	}
}

func TestWebReviewIAPsAttachRefusesIAPOutsideApp(t *testing.T) {
	origResolveSession := resolveSessionFn
	t.Cleanup(func() {
		resolveSessionFn = origResolveSession
	})

	resolveSessionFn = func(ctx context.Context, appleID, password, twoFactorCode string) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{
			Client: &http.Client{
				Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
					if req.Method != http.MethodGet || req.URL.Path != "/iris/v1/apps/123456789/inAppPurchases" {
						t.Fatalf("unexpected request before app-scoping refusal: %s %s", req.Method, req.URL.Path)
					}
					return &http.Response{
						StatusCode: http.StatusOK,
						Header:     http.Header{"Content-Type": []string{"application/json"}},
						Body:       io.NopCloser(strings.NewReader(`{"data":[]}`)),
						Request:    req,
					}, nil
				}),
			},
		}, "cache", nil
	}

	cmd := WebReviewIAPsAttachCommand()
	if err := cmd.FlagSet.Parse([]string{
		"--app", "123456789",
		"--iap-id", "9000000001",
		"--confirm",
	}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	err := cmd.Exec(context.Background(), nil)
	if err == nil {
		t.Fatal("expected app-scoping error")
	}
	if !strings.Contains(err.Error(), `in-app purchase "9000000001" was not found under app "123456789"`) {
		t.Fatalf("expected app-scoping error, got %v", err)
	}
}

func TestWebReviewIAPsAttachRejectsAmbiguousSelectorBeforeMutation(t *testing.T) {
	_ = stubWebProgressLabels(t)

	origResolveSession := resolveSessionFn
	t.Cleanup(func() { resolveSessionFn = origResolveSession })

	postCalls := 0
	resolveSessionFn = func(ctx context.Context, appleID, password, twoFactorCode string) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{
			Client: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				switch {
				case req.Method == http.MethodGet && req.URL.Path == "/iris/v1/apps/123456789/inAppPurchases":
					body := `{"data":[{"id":"iap-1","type":"inAppPurchases","attributes":{"productId":"com.example.duplicate","referenceName":"First"}},{"id":"iap-2","type":"inAppPurchases","attributes":{"productId":"com.example.duplicate","referenceName":"Second"}}],"links":{"next":""}}`
					return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(body)), Request: req}, nil
				case req.Method == http.MethodPost:
					postCalls++
					t.Fatalf("ambiguous selector must not attach: %s", req.URL.Path)
					return nil, nil
				default:
					t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
					return nil, nil
				}
			})},
		}, "cache", nil
	}

	cmd := WebReviewIAPsAttachCommand()
	if err := cmd.FlagSet.Parse([]string{"--app", "123456789", "--iap-id", "com.example.duplicate", "--confirm"}); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	err := cmd.Exec(context.Background(), nil)
	if err == nil {
		t.Fatal("expected ambiguous selector error")
	}
	if !strings.Contains(err.Error(), `2 in-app purchases match "com.example.duplicate" by product ID; pass --iap-id with one of:`) {
		t.Fatalf("expected ambiguity diagnostic, got %v", err)
	}
	for _, want := range []string{"iap-1", "com.example.duplicate", "First", "iap-2", "Second", "Use the Iris resource ID to disambiguate"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("expected %q in bounded ambiguity diagnostic, got %v", want, err)
		}
	}
	diagnostic, ok := shared.DiagnosticFromError(err)
	if !ok || diagnostic.Code != shared.DiagnosticInvalidInput || diagnostic.Parameter != "--iap-id" {
		t.Fatalf("diagnostic = %+v, found=%t, want invalid_input for --iap-id", diagnostic, ok)
	}
	if postCalls != 0 {
		t.Fatalf("expected no attach request, got %d", postCalls)
	}
}

func TestReviewIAPAmbiguousSelectionBoundsProviderText(t *testing.T) {
	recoveryID := "iap-recovery-id-" + strings.Repeat("9", shared.AmbiguousDiagnosticTextLimit+32)
	providerText := strings.Repeat("界", shared.AmbiguousDiagnosticTextLimit)
	providerInvalidUTF8 := string([]byte{'r', 'e', 'f', 0xff, 'e', 'r', 'e', 'n', 'c', 'e'})
	err := reviewIAPAmbiguousSelectionError(&webcore.ReviewIAPAmbiguousError{
		Selector: "com.example.duplicate",
		Field:    "product ID",
		Matches: []webcore.ReviewIAP{
			{ID: recoveryID, ProductID: providerText + "\nproduct-tail", ReferenceName: providerInvalidUTF8},
			{ID: "iap-2", ProductID: "com.example.duplicate", ReferenceName: "Second"},
		},
	})
	if err == nil {
		t.Fatal("expected an ambiguity error")
	}
	message := err.Error()
	if !utf8.ValidString(message) {
		t.Fatalf("ambiguity diagnostic must remain valid UTF-8: %q", message)
	}
	if strings.Contains(message, recoveryID) {
		t.Fatalf("displayed recovery ID should be bounded: %q", message)
	}
	if !strings.Contains(message, recoveryID[:len("iap-recovery-id-")]) {
		t.Fatalf("bounded recovery ID should retain useful context: %q", message)
	}
	if strings.Contains(message, "product-tail") || strings.Contains(message, "\x1b") || strings.Contains(message, "\nref") {
		t.Fatalf("provider text must be bounded and terminal-safe: %q", message)
	}
	if !strings.Contains(message, "ref�erence") {
		t.Fatalf("invalid provider UTF-8 should be replaced while retaining context: %q", message)
	}
	var structured *shared.AmbiguousSelectionError
	if !errors.As(err, &structured) || len(structured.Candidates) != 2 || structured.Candidates[0].ID != recoveryID {
		t.Fatalf("structured ambiguity must retain exact recovery candidates: %#v", structured)
	}
	if structured.Candidates[0].Label != providerText+"\nproduct-tail" || structured.Candidates[0].Extra != providerInvalidUTF8 {
		t.Fatalf("structured ambiguity must retain provider fields for machine consumers: %#v", structured.Candidates[0])
	}
	if structured.Description != `"com.example.duplicate" by product ID` || structured.Flag != "--iap-id" {
		t.Fatalf("structured ambiguity lost selector semantics: %#v", structured)
	}
}

func TestWebReviewIAPsAttachRejectsMissingOrWrongTypeBeforeMutation(t *testing.T) {
	_ = stubWebProgressLabels(t)

	cases := []struct {
		name string
		body string
		want string
	}{
		{
			name: "missing",
			body: `{"data":[],"links":{"next":""}}`,
			want: `was not found under app`,
		},
		{
			name: "wrong type",
			body: `{"data":[{"id":"sub-1","type":"subscriptions","attributes":{"productId":"com.example.wrong"}}],"links":{"next":""}}`,
			want: `unexpected resource type`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			origResolveSession := resolveSessionFn
			t.Cleanup(func() { resolveSessionFn = origResolveSession })

			postCalls := 0
			resolveSessionFn = func(ctx context.Context, appleID, password, twoFactorCode string) (*webcore.AuthSession, string, error) {
				return &webcore.AuthSession{
					Client: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
						if req.Method == http.MethodPost {
							postCalls++
							t.Fatalf("%s selector must not attach: %s", tc.name, req.URL.Path)
						}
						if req.Method != http.MethodGet || req.URL.Path != "/iris/v1/apps/123456789/inAppPurchases" {
							t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
						}
						return &http.Response{
							StatusCode: http.StatusOK,
							Header:     http.Header{"Content-Type": []string{"application/json"}},
							Body:       io.NopCloser(strings.NewReader(tc.body)),
							Request:    req,
						}, nil
					})},
				}, "cache", nil
			}

			cmd := WebReviewIAPsAttachCommand()
			if err := cmd.FlagSet.Parse([]string{"--app", "123456789", "--iap-id", tc.name, "--confirm"}); err != nil {
				t.Fatalf("parse error: %v", err)
			}
			err := cmd.Exec(context.Background(), nil)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("expected %q error, got %v", tc.want, err)
			}
			if postCalls != 0 {
				t.Fatalf("expected no attach request, got %d", postCalls)
			}
		})
	}
}

func TestWebReviewIAPsGroupCommandReturnsHelpWhenNoSubcommand(t *testing.T) {
	cmd := WebReviewIAPsCommand()
	if cmd.UsageFunc == nil {
		t.Fatal("WebReviewIAPsCommand should set UsageFunc for consistent rendering")
	}
	err := cmd.Exec(context.Background(), nil)
	if !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp from group Exec with no subcommand, got %v", err)
	}
}
