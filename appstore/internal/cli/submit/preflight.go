package submit

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/cli/shared"
	validatecli "github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/cli/validate"
	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/validation"
)

// checkResult represents the outcome of a single preflight check.
type checkResult struct {
	Name     string `json:"name"`
	Passed   bool   `json:"passed"`
	Advisory bool   `json:"advisory,omitempty"`
	Message  string `json:"message,omitempty"`
	Hint     string `json:"hint,omitempty"`
}

// preflightResult aggregates all preflight check outcomes.
type preflightResult struct {
	AppID     string        `json:"app_id"`
	Version   string        `json:"version"`
	Platform  string        `json:"platform"`
	Checks    []checkResult `json:"checks"`
	PassCount int           `json:"pass_count"`
	FailCount int           `json:"fail_count"`
}

func defaultSubmitPreflightOutputFormat() string {
	if shared.DefaultOutputFormat() == "json" {
		return "json"
	}
	return "text"
}

const submitPreflightDeprecationWarning = "Warning: `asc submit preflight` is deprecated. Use `asc validate`."

// SubmitPreflightCommand returns the "submit preflight" subcommand.
func SubmitPreflightCommand() *ffcli.Command {
	fs := flag.NewFlagSet("submit preflight", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID)")
	version := fs.String("version", "", "App Store version string")
	platform := fs.String("platform", "IOS", "Platform: IOS, MAC_OS, TV_OS, VISION_OS")
	output := shared.BindOutputFlagsWithAllowed(fs, "output", defaultSubmitPreflightOutputFormat(), "Output format: text, json", "text", "json")

	return &ffcli.Command{
		Name:       "preflight",
		ShortUsage: "asc submit preflight [flags]",
		ShortHelp:  "DEPRECATED: use `asc validate` for App Store submission readiness.",
		LongHelp: `Deprecated compatibility command for ` + "`asc validate`" + `.

Use ` + "`asc validate`" + ` for the canonical, more comprehensive App Store
submission readiness report. This compatibility command keeps the older
preflight-style text/json output for existing scripts while delegating to the
same shared readiness engine.

Examples:
  asc validate --app "123456789" --version "1.0"
  asc validate --app "123456789" --version "1.0" --platform TV_OS
  asc submit preflight --app "123456789" --version "2.0" --output json`,
		FlagSet:   fs,
		UsageFunc: shared.DeprecatedUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return shared.UsageErrorf("unexpected argument(s): %s", strings.Join(args, " "))
			}

			resolvedAppID := shared.ResolveAppID(*appID)
			if resolvedAppID == "" {
				fmt.Fprintln(os.Stderr, "Error: --app is required (or set ASC_APP_ID)")
				return flag.ErrHelp
			}
			if strings.TrimSpace(*version) == "" {
				fmt.Fprintln(os.Stderr, "Error: --version is required")
				return flag.ErrHelp
			}

			normalizedPlatform, err := shared.NormalizeAppStoreVersionPlatform(*platform)
			if err != nil {
				return shared.UsageError(err.Error())
			}
			normalizedOutput, err := shared.ValidateOutputFormatAllowed(*output.Output, *output.Pretty, "text", "json")
			if err != nil {
				return shared.UsageError(err.Error())
			}

			fmt.Fprintln(os.Stderr, submitPreflightDeprecationWarning)

			result, err := runSubmitPreflightCompatibility(ctx, resolvedAppID, strings.TrimSpace(*version), normalizedPlatform)
			if err != nil {
				return fmt.Errorf("submit preflight: %w", err)
			}

			if normalizedOutput == "text" {
				printPreflightText(os.Stdout, result)
				if result.FailCount > 0 {
					return fmt.Errorf("submit preflight: %d issue(s) found", result.FailCount)
				}
				return nil
			}

			if err := shared.PrintOutput(result, normalizedOutput, *output.Pretty); err != nil {
				return err
			}
			if result.FailCount > 0 {
				return fmt.Errorf("submit preflight: %d issue(s) found", result.FailCount)
			}
			return nil
		},
	}
}

func runSubmitPreflightCompatibility(ctx context.Context, appID, version, platform string) (*preflightResult, error) {
	report, err := validatecli.BuildReadinessReport(ctx, validatecli.ReadinessOptions{
		AppID:    appID,
		Version:  version,
		Platform: platform,
	})
	if err != nil {
		return nil, err
	}
	return preflightResultFromReport(appID, version, report), nil
}

func preflightResultFromReport(appID, version string, report validation.Report) *preflightResult {
	result := &preflightResult{
		AppID:    appID,
		Version:  version,
		Platform: report.Platform,
		Checks:   make([]checkResult, 0, len(report.Checks)),
	}

	for _, check := range report.Checks {
		name := preflightCheckName(check)
		passed := check.Severity != validation.SeverityError && check.Severity != validation.SeverityWarning
		advisory := check.Severity == validation.SeverityInfo
		if check.Severity == validation.SeverityWarning {
			passed = true
		}
		result.Checks = append(result.Checks, checkResult{
			Name:     name,
			Passed:   passed,
			Advisory: advisory,
			Message:  check.Message,
			Hint:     check.Remediation,
		})
	}

	tallyCounts(result)
	return result
}

func preflightCheckName(check validation.CheckResult) string {
	switch {
	case strings.HasPrefix(check.ID, "version."):
		return "Version state"
	case strings.HasPrefix(check.ID, "review_details."):
		return "App Store review details"
	case strings.HasPrefix(check.ID, "categories."):
		return "Primary category"
	case strings.HasPrefix(check.ID, "build.encryption."):
		return "Encryption compliance"
	case strings.HasPrefix(check.ID, "build."):
		return "Build"
	case strings.HasPrefix(check.ID, "pricing."):
		return "Pricing"
	case strings.HasPrefix(check.ID, "availability."):
		return "Availability"
	case strings.HasPrefix(check.ID, "screenshots."):
		return "Screenshots"
	case strings.HasPrefix(check.ID, "age_rating."):
		return "Age rating"
	case strings.HasPrefix(check.ID, "content_rights."):
		return "Content rights"
	case strings.HasPrefix(check.ID, "privacy."):
		return "App Privacy"
	case strings.HasPrefix(check.ID, "metadata."), strings.HasPrefix(check.ID, "required_fields."):
		return "Metadata"
	default:
		return "Readiness"
	}
}

func tallyCounts(result *preflightResult) {
	result.PassCount = 0
	result.FailCount = 0
	for _, c := range result.Checks {
		if c.Passed {
			result.PassCount++
			continue
		}
		if c.Advisory {
			continue
		}
		result.FailCount++
	}
}

func countAdvisories(checks []checkResult) int {
	count := 0
	for _, check := range checks {
		if check.Advisory {
			count++
		}
	}
	return count
}

func privacyPublishStateAdvisoryCheck(appID string) (checkResult, bool) {
	advisory := validation.PrivacyPublishStateAdvisory(appID)
	if advisory.ID == "" {
		return checkResult{}, false
	}
	return checkResult{
		Name:     "App Privacy",
		Passed:   true,
		Advisory: true,
		Message:  advisory.Message,
		Hint:     advisory.Remediation,
	}, true
}

// --- Text output ---

func printPreflightText(w io.Writer, result *preflightResult) {
	header := fmt.Sprintf("Preflight check for app %s v%s (%s)", result.AppID, result.Version, result.Platform)
	fmt.Fprintln(w, header)
	fmt.Fprintln(w, strings.Repeat("\u2500", len(header)))

	for _, c := range result.Checks {
		if c.Advisory {
			fmt.Fprintf(w, "! %s\n", c.Message)
			if c.Hint != "" {
				fmt.Fprintf(w, "  Hint: %s\n", c.Hint)
			}
		} else if c.Passed {
			fmt.Fprintf(w, "\u2713 %s\n", c.Message)
		} else {
			fmt.Fprintf(w, "\u2717 %s\n", c.Message)
			if c.Hint != "" {
				fmt.Fprintf(w, "  Hint: %s\n", c.Hint)
			}
		}
	}

	fmt.Fprintln(w)
	advisoryCount := countAdvisories(result.Checks)
	if result.FailCount == 0 && advisoryCount == 0 {
		fmt.Fprintln(w, "Result: All checks passed. Ready to submit.")
	} else if result.FailCount == 0 {
		label := "advisories"
		if advisoryCount == 1 {
			label = "advisory"
		}
		fmt.Fprintf(w, "Result: Required checks passed, but %d %s should be reviewed before submitting.\n", advisoryCount, label)
	} else {
		fmt.Fprintf(w, "Result: %d issue(s) found. Fix them before submitting.\n", result.FailCount)
	}
}
