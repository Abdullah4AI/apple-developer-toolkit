package subscriptions

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/asc"
	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/cli/shared"
)

// SubscriptionsIntroductoryOffersImportCommand returns the introductory offers import subcommand.
func SubscriptionsIntroductoryOffersImportCommand() *ffcli.Command {
	fs := flag.NewFlagSet("introductory-offers import", flag.ExitOnError)

	subscriptionID := fs.String("subscription-id", "", "Subscription ID")
	inputPath := fs.String("input", "", "Input CSV file path (required)")
	offerDuration := fs.String("offer-duration", "", "Default offer duration")
	offerMode := fs.String("offer-mode", "", "Default offer mode")
	numberOfPeriods := fs.Int("number-of-periods", 0, "Default number of periods")
	startDate := fs.String("start-date", "", "Default start date (YYYY-MM-DD)")
	endDate := fs.String("end-date", "", "Default end date (YYYY-MM-DD)")
	dryRun := fs.Bool("dry-run", false, "Validate input and print summary without creating offers")
	continueOnError := fs.Bool("continue-on-error", true, "Continue processing rows after runtime failures (default true)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "import",
		ShortUsage: "asc subscriptions introductory-offers import --subscription-id \"SUB_ID\" --input \"./offers.csv\" [flags]",
		ShortHelp:  "Import introductory offers from a CSV file.",
		LongHelp: `Import introductory offers from a CSV file.

CSV is UTF-8 with a required header row.

Required column:
  territory

Optional columns:
  offer_mode, offer_duration, number_of_periods, start_date, end_date, price_point_id

Header aliases:
  price_point -> price_point_id

Territory values:
  3-letter ASC territory IDs, 2-letter country codes, and English territory names

Precedence:
  Row values override command-level defaults.

Examples:
  asc subscriptions introductory-offers import --subscription-id "SUB_ID" --input "./offers.csv"
  asc subscriptions introductory-offers import --subscription-id "SUB_ID" --input "./offers.csv" --offer-duration ONE_WEEK --offer-mode FREE_TRIAL --number-of-periods 1
  asc subscriptions introductory-offers import --subscription-id "SUB_ID" --input "./offers.csv" --dry-run --offer-duration ONE_WEEK --offer-mode FREE_TRIAL --number-of-periods 1`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			_ = ctx
			_ = output

			if strings.TrimSpace(*subscriptionID) == "" {
				fmt.Fprintln(os.Stderr, "Error: --subscription-id is required")
				return flag.ErrHelp
			}
			if strings.TrimSpace(*inputPath) == "" {
				fmt.Fprintln(os.Stderr, "Error: --input is required")
				return flag.ErrHelp
			}
			normalizedOfferDuration := ""
			if strings.TrimSpace(*offerDuration) != "" {
				duration, err := normalizeSubscriptionOfferDuration(*offerDuration)
				if err != nil {
					fmt.Fprintln(os.Stderr, "Error:", err.Error())
					return flag.ErrHelp
				}
				normalizedOfferDuration = string(duration)
			}
			normalizedOfferMode := ""
			if strings.TrimSpace(*offerMode) != "" {
				mode, err := normalizeSubscriptionOfferMode(*offerMode)
				if err != nil {
					fmt.Fprintln(os.Stderr, "Error:", err.Error())
					return flag.ErrHelp
				}
				normalizedOfferMode = string(mode)
			}
			if *numberOfPeriods < 0 {
				fmt.Fprintln(os.Stderr, "Error: --number-of-periods must be greater than or equal to 0")
				return flag.ErrHelp
			}
			normalizedStartDate := ""
			if strings.TrimSpace(*startDate) != "" {
				date, err := shared.NormalizeDate(*startDate, "--start-date")
				if err != nil {
					fmt.Fprintln(os.Stderr, "Error:", err.Error())
					return flag.ErrHelp
				}
				normalizedStartDate = date
			}
			normalizedEndDate := ""
			if strings.TrimSpace(*endDate) != "" {
				date, err := shared.NormalizeDate(*endDate, "--end-date")
				if err != nil {
					fmt.Fprintln(os.Stderr, "Error:", err.Error())
					return flag.ErrHelp
				}
				normalizedEndDate = date
			}
			rows, err := readSubscriptionIntroductoryOffersImportCSV(*inputPath)
			if err != nil {
				return fmt.Errorf("subscriptions introductory-offers import: %w", err)
			}
			defaults := buildSubscriptionIntroductoryOfferImportDefaults(
				normalizedOfferDuration,
				normalizedOfferMode,
				*numberOfPeriods,
				normalizedStartDate,
				normalizedEndDate,
			)
			resolvedRows, err := resolveSubscriptionIntroductoryOfferImportRows(rows, defaults)
			if err != nil {
				return fmt.Errorf("subscriptions introductory-offers import: %w", err)
			}
			summary := &subscriptionIntroductoryOfferImportSummary{
				SubscriptionID:  strings.TrimSpace(*subscriptionID),
				InputFile:       filepath.Clean(strings.TrimSpace(*inputPath)),
				DryRun:          *dryRun,
				ContinueOnError: *continueOnError,
				Total:           len(resolvedRows),
			}

			if *dryRun {
				summary.Created = len(resolvedRows)
				return shared.PrintOutputWithRenderers(
					summary,
					*output.Output,
					*output.Pretty,
					func() error { return renderSubscriptionIntroductoryOfferImportSummary(summary, false) },
					func() error { return renderSubscriptionIntroductoryOfferImportSummary(summary, true) },
				)
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("subscriptions introductory-offers import: %w", err)
			}

			for _, row := range resolvedRows {
				attrs := asc.SubscriptionIntroductoryOfferCreateAttributes{
					Duration:        asc.SubscriptionOfferDuration(row.offerDuration),
					OfferMode:       asc.SubscriptionOfferMode(row.offerMode),
					NumberOfPeriods: row.numberOfPeriods,
				}
				if row.startDate != "" {
					attrs.StartDate = row.startDate
				}
				if row.endDate != "" {
					attrs.EndDate = row.endDate
				}

				createCtx, createCancel := shared.ContextWithTimeout(ctx)
				_, err := client.CreateSubscriptionIntroductoryOffer(createCtx, summary.SubscriptionID, attrs, row.territory, row.pricePointID)
				createCancel()
				if err != nil {
					appendSubscriptionIntroductoryOfferImportFailure(summary, row, err)
					if !*continueOnError {
						break
					}
					continue
				}

				summary.Created++
			}

			if err := shared.PrintOutputWithRenderers(
				summary,
				*output.Output,
				*output.Pretty,
				func() error { return renderSubscriptionIntroductoryOfferImportSummary(summary, false) },
				func() error { return renderSubscriptionIntroductoryOfferImportSummary(summary, true) },
			); err != nil {
				return err
			}
			if summary.Failed > 0 {
				return shared.NewReportedError(fmt.Errorf("subscriptions introductory-offers import: %d row(s) failed", summary.Failed))
			}
			return nil
		},
	}
}

type subscriptionIntroductoryOfferImportSummary struct {
	SubscriptionID  string                                              `json:"subscriptionId"`
	InputFile       string                                              `json:"inputFile"`
	DryRun          bool                                                `json:"dryRun"`
	ContinueOnError bool                                                `json:"continueOnError"`
	Total           int                                                 `json:"total"`
	Created         int                                                 `json:"created"`
	Failed          int                                                 `json:"failed"`
	Failures        []subscriptionIntroductoryOfferImportSummaryFailure `json:"failures,omitempty"`
}

type subscriptionIntroductoryOfferImportSummaryFailure struct {
	Row       int    `json:"row"`
	Territory string `json:"territory,omitempty"`
	Error     string `json:"error"`
}

func renderSubscriptionIntroductoryOfferImportSummary(summary *subscriptionIntroductoryOfferImportSummary, markdown bool) error {
	if summary == nil {
		return fmt.Errorf("summary is nil")
	}

	render := asc.RenderTable
	if markdown {
		render = asc.RenderMarkdown
	}

	render(
		[]string{"Subscription ID", "Input File", "Dry Run", "Total", "Created", "Failed"},
		[][]string{{
			summary.SubscriptionID,
			summary.InputFile,
			fmt.Sprintf("%t", summary.DryRun),
			fmt.Sprintf("%d", summary.Total),
			fmt.Sprintf("%d", summary.Created),
			fmt.Sprintf("%d", summary.Failed),
		}},
	)

	if len(summary.Failures) > 0 {
		rows := make([][]string, 0, len(summary.Failures))
		for _, failure := range summary.Failures {
			rows = append(rows, []string{
				fmt.Sprintf("%d", failure.Row),
				failure.Territory,
				failure.Error,
			})
		}
		render([]string{"Row", "Territory", "Error"}, rows)
	}

	return nil
}

func appendSubscriptionIntroductoryOfferImportFailure(summary *subscriptionIntroductoryOfferImportSummary, row subscriptionIntroductoryOfferImportResolvedRow, err error) {
	if summary == nil || err == nil {
		return
	}
	summary.Failed++
	summary.Failures = append(summary.Failures, subscriptionIntroductoryOfferImportSummaryFailure{
		Row:       row.row,
		Territory: row.territory,
		Error:     err.Error(),
	})
}
