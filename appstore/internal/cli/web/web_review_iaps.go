package web

import (
	"context"
	"flag"
	"fmt"
	"strconv"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/asc"
	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/cli/shared"
	webcore "github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/web"
)

type reviewIAPFinder interface {
	FindReviewIAP(ctx context.Context, appID, iapID string) (webcore.ReviewIAP, bool, error)
}

type reviewIAPMutationOutput struct {
	AppID      string                      `json:"appId"`
	IAPID      string                      `json:"iapId"`
	Operation  string                      `json:"operation"`
	Changed    bool                        `json:"changed"`
	Submission webcore.ReviewIAPSubmission `json:"submission"`
}

func reviewIAPValue(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "n/a"
	}
	return trimmed
}

func buildReviewIAPMutationRows(payload reviewIAPMutationOutput) [][]string {
	return [][]string{
		{"Mutation", "App ID", reviewIAPValue(payload.AppID)},
		{"Mutation", "IAP ID", reviewIAPValue(payload.IAPID)},
		{"Mutation", "Operation", reviewIAPValue(payload.Operation)},
		{"Mutation", "Changed", strconv.FormatBool(payload.Changed)},
		{"Submission", "Submission ID", reviewIAPValue(payload.Submission.ID)},
		{"Submission", "Next Version", strconv.FormatBool(payload.Submission.SubmitWithNextAppStoreVersion)},
	}
}

func renderReviewIAPMutationTable(payload reviewIAPMutationOutput) error {
	headers := []string{"Section", "Field", "Value"}
	asc.RenderTable(headers, buildReviewIAPMutationRows(payload))
	return nil
}

func renderReviewIAPMutationMarkdown(payload reviewIAPMutationOutput) error {
	headers := []string{"Section", "Field", "Value"}
	asc.RenderMarkdown(headers, buildReviewIAPMutationRows(payload))
	return nil
}

func validateReviewIAPAttachInputs(appID, iapID string, confirm bool) error {
	switch {
	case strings.TrimSpace(appID) == "":
		return shared.UsageError("--app is required")
	case shared.SelectorNeedsLookup(appID):
		return shared.UsageError("--app must be a numeric App Store Connect app ID")
	case strings.TrimSpace(iapID) == "":
		return shared.UsageError("--iap-id is required")
	case !confirm:
		return shared.UsageError("--confirm is required")
	default:
		return nil
	}
}

// iapStateIndicatesAlreadyAttached reports whether the IAP's current ASC
// state implies it is already enrolled in the next app version review.
//
// The iris listing returns `state` (per `reviewIAPFields`) but does not
// return the otherwise-authoritative `submitWithNextAppStoreVersion` field,
// so state is the only signal available pre-POST. When the IAP is in one
// of these states, iris POST `/inAppPurchaseSubmissions` returns a 409
// whose error code varies with the IAP-id form used: a re-attach with the
// iris resource UUID surfaces `ENTITY_ERROR.RELATIONSHIP.INVALID` because
// iris's internal UUID→numeric-id translation fails for in-flight IAPs,
// while a re-attach with the public numeric id surfaces
// `ENTITY_ERROR.ATTRIBUTE.INVALID.UNMODIFIABLE` on
// `/data/attributes/submitWithNextAppStoreVersion`. Short-circuiting before
// the POST avoids both error shapes and keeps the CLI's exit semantics
// stable across the two id-form code paths.
func iapStateIndicatesAlreadyAttached(state string) bool {
	switch strings.ToUpper(strings.TrimSpace(state)) {
	case "WAITING_FOR_REVIEW", "IN_REVIEW", "PENDING_BINARY_APPROVAL":
		return true
	}
	return false
}

func verifyReviewIAPBelongsToApp(ctx context.Context, client reviewIAPFinder, appID, iapID string) (webcore.ReviewIAP, error) {
	if client == nil {
		return webcore.ReviewIAP{}, fmt.Errorf("app-scoped IAP verification client is required")
	}

	iap, found, err := client.FindReviewIAP(ctx, appID, iapID)
	if err != nil {
		return webcore.ReviewIAP{}, fmt.Errorf("verify in-app purchase %q under app %q: %w", iapID, appID, err)
	}
	if !found {
		return webcore.ReviewIAP{}, fmt.Errorf("in-app purchase %q was not found under app %q; refusing to attach", iapID, appID)
	}
	if strings.TrimSpace(iap.ID) == "" {
		return webcore.ReviewIAP{}, fmt.Errorf("in-app purchase %q under app %q is missing an iris resource id; refusing to attach", iapID, appID)
	}
	return iap, nil
}

// WebReviewIAPsCommand groups iris-API operations that attach a non-renewing
// in-app purchase to the next app version review. Mirrors
// `asc web review subscriptions` but routes to the IAP iris endpoint.
func WebReviewIAPsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("web review iaps", flag.ExitOnError)
	return &ffcli.Command{
		Name:       "iaps",
		ShortUsage: "asc web review iaps <subcommand> [flags]",
		ShortHelp:  "Attach non-renewing IAPs to the next app version review.",
		LongHelp: `WEB SESSION WORKFLOWS

Attach a non-renewing in-app purchase to the next app version review.

Subcommands:
  attach  Attach one non-renewing IAP to the next app version review

Apple's public REST API does not expose this flow for non-renewing purchases:
POST /v1/reviewSubmissionItems rejects all IAP relationship variants, and a
standalone POST /v1/inAppPurchaseSubmissions returns
FIRST_IAP_MUST_BE_SUBMITTED_ON_VERSION. The web UI's "Add App In-App Purchase
or Subscription" dialog posts to /iris/v1/inAppPurchaseSubmissions with
submitWithNextAppStoreVersion=true; this command exposes that same call.

`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			WebReviewIAPsAttachCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// WebReviewIAPsAttachCommand attaches a non-renewing IAP to the next app
// version review via the iris endpoint.
func WebReviewIAPsAttachCommand() *ffcli.Command {
	fs := flag.NewFlagSet("web review iaps attach", flag.ExitOnError)

	appID := fs.String("app", "", "App ID")
	iapID := fs.String("iap-id", "", "Iris IAP resource ID, product ID, or exact reference name")
	confirm := fs.Bool("confirm", false, "Confirm the attach operation")
	authFlags := bindWebSessionFlags(fs)
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "attach",
		ShortUsage: "asc web review iaps attach --app APP_ID --iap-id IAP_SELECTOR --confirm [flags]",
		ShortHelp:  "Attach a non-renewing IAP to the next app version review.",
		LongHelp: `WEB SESSION WORKFLOWS

Attach a non-renewing in-app purchase to the next app version review.

The --iap-id selector accepts the Iris resource id, the bundle-style productId,
or the exact current referenceName from ` + "`asc iap list`" + `. Apple's Iris listing does not
expose the public numeric ASC IAP id, so that numeric id is not resolved by this command.

`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			trimmedAppID := strings.TrimSpace(*appID)
			trimmedIAPID := strings.TrimSpace(*iapID)
			if err := validateReviewIAPAttachInputs(trimmedAppID, trimmedIAPID, *confirm); err != nil {
				return err
			}

			session, requestCtx, cancel, err := resolveWebSessionForCommand(ctx, authFlags)
			defer cancel()
			if err != nil {
				return err
			}
			client := newWebClientFn(session)

			reviewIAP, err := verifyReviewIAPBelongsToApp(requestCtx, client, trimmedAppID, trimmedIAPID)
			if err != nil {
				return fmt.Errorf("web review iaps attach: %w", withReviewSelectorDiagnostic(err, "--iap-id"))
			}
			// The iris attach POST takes the iris resource id, not the
			// caller's selector — `--iap-id` may have been a bundle-style
			// productId that matched via `attributes.productId`, in which
			// case posting the selector as the relationship id would be
			// wrong. Always use the resolved iris UUID.
			resolvedIrisID := strings.TrimSpace(reviewIAP.ID)
			if reviewIAP.SubmitWithNextAppStoreVersion || iapStateIndicatesAlreadyAttached(reviewIAP.State) {
				payload := reviewIAPMutationOutput{
					AppID:     trimmedAppID,
					IAPID:     trimmedIAPID,
					Operation: "attach",
					Changed:   false,
					Submission: webcore.ReviewIAPSubmission{
						InAppPurchaseID:               resolvedIrisID,
						SubmitWithNextAppStoreVersion: true,
					},
				}
				return shared.PrintOutputWithRenderers(
					payload,
					*output.Output,
					*output.Pretty,
					func() error { return renderReviewIAPMutationTable(payload) },
					func() error { return renderReviewIAPMutationMarkdown(payload) },
				)
			}

			submission, err := withWebSpinnerValue("Attaching IAP to next app version", func() (webcore.ReviewIAPSubmission, error) {
				return client.CreateInAppPurchaseSubmission(requestCtx, resolvedIrisID)
			})
			if err != nil {
				if webcore.IsAlreadyExistsConflict(err) {
					payload := reviewIAPMutationOutput{
						AppID:     trimmedAppID,
						IAPID:     trimmedIAPID,
						Operation: "attach",
						Changed:   false,
						Submission: webcore.ReviewIAPSubmission{
							InAppPurchaseID:               resolvedIrisID,
							SubmitWithNextAppStoreVersion: true,
						},
					}
					return shared.PrintOutputWithRenderers(
						payload,
						*output.Output,
						*output.Pretty,
						func() error { return renderReviewIAPMutationTable(payload) },
						func() error { return renderReviewIAPMutationMarkdown(payload) },
					)
				}
				return withWebAuthHint(
					fmt.Errorf("web review iaps attach for app %q, iap %q: %w", trimmedAppID, trimmedIAPID, err),
					"web review iaps attach",
				)
			}

			payload := reviewIAPMutationOutput{
				AppID:      trimmedAppID,
				IAPID:      trimmedIAPID,
				Operation:  "attach",
				Changed:    submission.SubmitWithNextAppStoreVersion,
				Submission: submission,
			}
			return shared.PrintOutputWithRenderers(
				payload,
				*output.Output,
				*output.Pretty,
				func() error { return renderReviewIAPMutationTable(payload) },
				func() error { return renderReviewIAPMutationMarkdown(payload) },
			)
		},
	}
}
