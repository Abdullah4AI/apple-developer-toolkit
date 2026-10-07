package ads

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestReportsPresetDeprecationWarningMatchesSelectedLevel(t *testing.T) {
	tests := map[string]string{
		"campaigns":             "reports apps campaigns",
		"ad-groups":             "reports apps ad-groups",
		"keywords":              "reports apps keywords",
		"search-terms":          "reports apps search-terms",
		"ads":                   "reports apps ads",
		"ad-group-keywords":     "reports apps keywords",
		"ad-group-search-terms": "reports apps search-terms",
	}
	for level, replacement := range tests {
		t.Run(level, func(t *testing.T) {
			command := ReportsPresetCommand()
			if err := command.FlagSet.Parse([]string{"--level", level}); err != nil {
				t.Fatal(err)
			}
			_, stderr := captureAdsDeprecationStreams(t, func() {
				_ = command.Exec(context.Background(), nil)
			})
			want := "Warning: `asc ads v5 reports preset` is deprecated and retires on January 26, 2027. Use `asc ads " + replacement + "`.\n"
			if !strings.HasPrefix(stderr, want) {
				t.Fatalf("stderr = %q, want warning prefix %q", stderr, want)
			}
			if got := strings.Count(stderr, "Warning:"); got != 1 {
				t.Fatalf("stderr contains %d warnings, want 1: %q", got, stderr)
			}
		})
	}
}

func TestReportPresetDateRangeLastDays(t *testing.T) {
	start, end, err := reportPresetDateRange("", "", 7, time.Date(2026, 5, 31, 12, 0, 0, 0, time.UTC), "UTC")
	if err != nil {
		t.Fatalf("reportPresetDateRange() error: %v", err)
	}
	if start != "2026-05-25" || end != "2026-05-31" {
		t.Fatalf("range = %s..%s, want 2026-05-25..2026-05-31", start, end)
	}
}

func TestReportsPresetCommandHelpShowsOperatorGuidance(t *testing.T) {
	cmd := ReportsPresetCommand()
	if !strings.HasPrefix(cmd.ShortHelp, "DEPRECATED:") || !strings.Contains(cmd.LongHelp, "Build and run Apple Ads report presets without JSON payloads.") {
		t.Fatalf("ShortHelp = %q, want preset workflow wording", cmd.ShortHelp)
	}
	for _, want := range []string{
		"Choose the report resource with --level.",
		"Apple Ads accepts UTC and ORTZ",
		"--last-days for an inclusive UTC rolling date range",
		"Report presets default to --sort -impressions",
		"HOURLY granularity is available for",
		"campaign, ad-group, and keyword report levels",
		"Search-term report levels cannot",
		"request row totals",
		"asc ads v5 reports preset --level campaigns --last-days 30 --fields campaignName,impressions,taps,localSpend --sort -impressions",
		"asc ads v5 reports preset --level ads --campaign 12345 --from YYYY-MM-DD --to YYYY-MM-DD --sort -impressions",
	} {
		if !strings.Contains(cmd.LongHelp, want) {
			t.Fatalf("LongHelp missing %q\n%s", want, cmd.LongHelp)
		}
	}
	if got := cmd.FlagSet.Lookup("last-days").Usage; got != "Use an inclusive UTC range ending today" {
		t.Fatalf("--last-days usage = %q", got)
	}
	if got := cmd.FlagSet.Lookup("time-zone").Usage; got != "Apple Ads reporting time zone: UTC or ORTZ" {
		t.Fatalf("--time-zone usage = %q", got)
	}
	if got := cmd.FlagSet.Lookup("granularity").Usage; got != "Report granularity: HOURLY, DAILY, WEEKLY, MONTHLY" {
		t.Fatalf("--granularity usage = %q", got)
	}
}

func TestBuildReportPresetPayloadLastDaysUsesUTC(t *testing.T) {
	payload, err := buildReportPresetPayload(reportPresetTestFlags(
		"campaigns",
		"",
		"",
		"",
		1,
		"utc",
	), time.Date(2026, 6, 1, 1, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("buildReportPresetPayload() error: %v", err)
	}
	if payload.StartTime != "2026-06-01" || payload.EndTime != "2026-06-01" {
		t.Fatalf("range = %s..%s, want 2026-06-01..2026-06-01", payload.StartTime, payload.EndTime)
	}
	if payload.TimeZone != "UTC" {
		t.Fatalf("timeZone = %q, want UTC", payload.TimeZone)
	}
}

func TestBuildReportPresetPayloadValidatesTimeZone(t *testing.T) {
	_, err := buildReportPresetPayload(reportPresetTestFlags(
		"campaigns",
		"",
		"",
		"",
		1,
		"America/Los_Angeles",
	), time.Date(2026, 6, 1, 1, 0, 0, 0, time.UTC))
	if err == nil || !strings.Contains(err.Error(), "--time-zone must be UTC or ORTZ") {
		t.Fatalf("error = %v, want time-zone validation", err)
	}
}

func TestBuildReportPresetPayloadRejectsLastDaysWithORTZ(t *testing.T) {
	_, err := buildReportPresetPayload(reportPresetTestFlags(
		"campaigns",
		"",
		"",
		"",
		1,
		"ORTZ",
	), time.Date(2026, 6, 1, 1, 0, 0, 0, time.UTC))
	if err == nil || !strings.Contains(err.Error(), "--last-days is not supported for ORTZ reports; use --from and --to") {
		t.Fatalf("error = %v, want last-days ORTZ validation", err)
	}
}

func TestBuildReportPresetPayloadDefaultsSearchTermsToORTZ(t *testing.T) {
	payload, err := buildReportPresetPayload(reportPresetTestFlags(
		"search-terms",
		"12345",
		"2026-05-01",
		"2026-05-31",
		0,
		"UTC",
	), time.Date(2026, 6, 1, 1, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("buildReportPresetPayload() error: %v", err)
	}
	if payload.TimeZone != "ORTZ" {
		t.Fatalf("timeZone = %q, want ORTZ", payload.TimeZone)
	}
}

func TestBuildReportPresetPayloadRejectsExplicitUTCForSearchTerms(t *testing.T) {
	flags := reportPresetTestFlags(
		"search-terms",
		"12345",
		"2026-05-01",
		"2026-05-31",
		0,
		"UTC",
	)
	timeZoneExplicit := true
	flags.timeZoneExplicit = &timeZoneExplicit

	_, err := buildReportPresetPayload(flags, time.Date(2026, 6, 1, 1, 0, 0, 0, time.UTC))
	if err == nil || !strings.Contains(err.Error(), "--time-zone must be ORTZ for search-term report levels") {
		t.Fatalf("error = %v, want explicit search-term time-zone validation", err)
	}
}

func TestBuildReportPresetPayloadDefaultsSort(t *testing.T) {
	payload, err := buildReportPresetPayload(reportPresetTestFlags(
		"ads",
		"12345",
		"2026-05-01",
		"2026-05-31",
		0,
		"UTC",
	), time.Date(2026, 6, 1, 1, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("buildReportPresetPayload() error: %v", err)
	}
	if len(payload.Selector.OrderBy) != 1 || payload.Selector.OrderBy[0].Field != "impressions" || payload.Selector.OrderBy[0].SortOrder != "DESCENDING" {
		t.Fatalf("orderBy = %+v, want default impressions descending", payload.Selector.OrderBy)
	}
}

func TestBuildReportPresetPayloadAllowsHourlyWhereSupported(t *testing.T) {
	for _, level := range []string{"campaigns", "ad-groups", "keywords", "ad-group-keywords"} {
		t.Run(level, func(t *testing.T) {
			flags := reportPresetTestFlags(
				level,
				"12345",
				"2026-05-25",
				"2026-06-01",
				0,
				"UTC",
			)
			granularity := "hourly"
			flags.granularity = &granularity

			payload, err := buildReportPresetPayload(flags, time.Date(2026, 6, 1, 1, 0, 0, 0, time.UTC))
			if err != nil {
				t.Fatalf("buildReportPresetPayload() error: %v", err)
			}
			if payload.Granularity != "HOURLY" {
				t.Fatalf("granularity = %q, want HOURLY", payload.Granularity)
			}
		})
	}
}

func TestBuildReportPresetPayloadAllowsDateWindowBoundaries(t *testing.T) {
	for _, tt := range []struct {
		name        string
		granularity string
		from        string
		to          string
	}{
		{name: "hourly exactly seven days apart", granularity: "HOURLY", from: "2026-05-25", to: "2026-06-01"},
		{name: "daily exactly ninety days apart", granularity: "DAILY", from: "2026-03-03", to: "2026-06-01"},
		{name: "weekly more than fourteen days apart", granularity: "WEEKLY", from: "2026-05-17", to: "2026-06-01"},
		{name: "weekly exactly three hundred sixty five days apart", granularity: "WEEKLY", from: "2025-06-01", to: "2026-06-01"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			flags := reportPresetTestFlags(
				"campaigns",
				"",
				tt.from,
				tt.to,
				0,
				"UTC",
			)
			granularity := tt.granularity
			flags.granularity = &granularity

			payload, err := buildReportPresetPayload(flags, time.Date(2026, 6, 1, 1, 0, 0, 0, time.UTC))
			if err != nil {
				t.Fatalf("buildReportPresetPayload() error: %v", err)
			}
			if payload.StartTime != tt.from || payload.EndTime != tt.to {
				t.Fatalf("range = %s..%s, want %s..%s", payload.StartTime, payload.EndTime, tt.from, tt.to)
			}
		})
	}
}

func TestBuildReportPresetPayloadRejectsDateWindowsAppleWillRefuse(t *testing.T) {
	for _, tt := range []struct {
		name        string
		granularity string
		from        string
		to          string
		wantErr     string
	}{
		{name: "hourly more than seven days", granularity: "HOURLY", from: "2026-05-24", to: "2026-06-01", wantErr: "--granularity HOURLY supports a maximum 7-day date range"},
		{name: "hourly start too old", granularity: "HOURLY", from: "2026-05-01", to: "2026-05-07", wantErr: "--granularity HOURLY start date must be within the last 30 days"},
		{name: "daily more than ninety days", granularity: "DAILY", from: "2026-03-02", to: "2026-06-01", wantErr: "--granularity DAILY supports a maximum 90-day date range"},
		{name: "daily start too old", granularity: "DAILY", from: "2026-02-01", to: "2026-02-07", wantErr: "--granularity DAILY start date must be within the last 90 days"},
		{name: "weekly too short", granularity: "WEEKLY", from: "2026-05-18", to: "2026-06-01", wantErr: "--granularity WEEKLY requires a date range more than 14 days and at most 365 days"},
		{name: "weekly too long", granularity: "WEEKLY", from: "2025-05-31", to: "2026-06-01", wantErr: "--granularity WEEKLY requires a date range more than 14 days and at most 365 days"},
		{name: "weekly start too old", granularity: "WEEKLY", from: "2024-05-01", to: "2024-05-20", wantErr: "--granularity WEEKLY start date must be within the last 24 months"},
		{name: "monthly too short", granularity: "MONTHLY", from: "2026-01-01", to: "2026-04-01", wantErr: "--granularity MONTHLY requires a date range more than 3 months"},
		{name: "monthly start too old", granularity: "MONTHLY", from: "2024-05-01", to: "2024-09-01", wantErr: "--granularity MONTHLY start date must be within the last 24 months"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			flags := reportPresetTestFlags(
				"campaigns",
				"",
				tt.from,
				tt.to,
				0,
				"UTC",
			)
			granularity := tt.granularity
			flags.granularity = &granularity

			_, err := buildReportPresetPayload(flags, time.Date(2026, 6, 1, 1, 0, 0, 0, time.UTC))
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("error = %v, want %q", err, tt.wantErr)
			}
		})
	}
}

func TestBuildReportPresetPayloadRejectsHourlyWhereUnsupported(t *testing.T) {
	for _, tt := range []struct {
		level    string
		timeZone string
		sort     string
	}{
		{level: "search-terms", timeZone: "UTC"},
		{level: "ad-group-search-terms", timeZone: "UTC"},
		{level: "ads", timeZone: "UTC", sort: "-impressions"},
	} {
		t.Run(tt.level, func(t *testing.T) {
			flags := reportPresetTestFlags(
				tt.level,
				"12345",
				"2026-05-26",
				"2026-06-01",
				0,
				tt.timeZone,
			)
			granularity := "HOURLY"
			flags.granularity = &granularity
			flags.sort = &tt.sort

			_, err := buildReportPresetPayload(flags, time.Date(2026, 6, 1, 1, 0, 0, 0, time.UTC))
			if err == nil || !strings.Contains(err.Error(), "--granularity HOURLY is only supported") {
				t.Fatalf("error = %v, want hourly level validation", err)
			}
		})
	}
}

func TestBuildReportPresetPayloadRejectsRowTotalsForSearchTerms(t *testing.T) {
	flags := reportPresetTestFlags(
		"search-terms",
		"12345",
		"2026-05-01",
		"2026-05-31",
		0,
		"UTC",
	)
	returnRowTotals := true
	flags.returnRowTotals = &returnRowTotals

	_, err := buildReportPresetPayload(flags, time.Date(2026, 6, 1, 1, 0, 0, 0, time.UTC))
	if err == nil || !strings.Contains(err.Error(), "--return-row-totals cannot be used with search-term report levels") {
		t.Fatalf("error = %v, want row totals search-term validation", err)
	}
}

func TestReportPresetDateRangeValidation(t *testing.T) {
	tests := []struct {
		name    string
		from    string
		to      string
		days    int
		wantErr string
	}{
		{name: "last days conflicts with explicit range", from: "2026-05-01", days: 7, wantErr: "--last-days cannot be combined"},
		{name: "negative last days", days: -1, wantErr: "--last-days must be >= 0"},
		{name: "missing range", wantErr: "either --last-days or both --from and --to are required"},
		{name: "bad from", from: "2026/05/01", to: "2026-05-31", wantErr: "--from must be in YYYY-MM-DD format"},
		{name: "bad to", from: "2026-05-01", to: "2026/05/31", wantErr: "--to must be in YYYY-MM-DD format"},
		{name: "reversed range", from: "2026-06-01", to: "2026-05-31", wantErr: "--to must be on or after --from"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := reportPresetDateRange(tt.from, tt.to, tt.days, time.Date(2026, 5, 31, 12, 0, 0, 0, time.UTC), "UTC")
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("error = %v, want contains %q", err, tt.wantErr)
			}
		})
	}
}

func reportPresetTestFlags(level, campaign, from, to string, lastDays int, timeZone string) adsReportPresetFlags {
	adGroup := ""
	granularity := "DAILY"
	fields := ""
	sort := ""
	limit := 1000
	offset := 0
	timeZoneExplicit := false
	returnRowTotals := false
	return adsReportPresetFlags{
		level:            &level,
		campaign:         &campaign,
		adGroup:          &adGroup,
		from:             &from,
		to:               &to,
		lastDays:         &lastDays,
		granularity:      &granularity,
		fields:           &fields,
		sort:             &sort,
		limit:            &limit,
		offset:           &offset,
		timeZone:         &timeZone,
		timeZoneExplicit: &timeZoneExplicit,
		returnRowTotals:  &returnRowTotals,
	}
}

func TestParseReportPresetSort(t *testing.T) {
	sortSpec, err := parseReportPresetSort("impressions")
	if err != nil {
		t.Fatalf("parseReportPresetSort() error: %v", err)
	}
	if sortSpec.Field != "impressions" || sortSpec.SortOrder != "DESCENDING" {
		t.Fatalf("sort = %+v, want impressions DESCENDING", sortSpec)
	}

	sortSpec, err = parseReportPresetSort("-impressions")
	if err != nil {
		t.Fatalf("parseReportPresetSort() error: %v", err)
	}
	if sortSpec.Field != "impressions" || sortSpec.SortOrder != "DESCENDING" {
		t.Fatalf("sort = %+v, want impressions DESCENDING", sortSpec)
	}

	sortSpec, err = parseReportPresetSort("-spend")
	if err != nil {
		t.Fatalf("parseReportPresetSort() error: %v", err)
	}
	if sortSpec.Field != "localSpend" || sortSpec.SortOrder != "DESCENDING" {
		t.Fatalf("sort = %+v, want localSpend DESCENDING", sortSpec)
	}

	sortSpec, err = parseReportPresetSort("impressions:desc")
	if err != nil {
		t.Fatalf("parseReportPresetSort() error: %v", err)
	}
	if sortSpec.Field != "impressions" || sortSpec.SortOrder != "DESCENDING" {
		t.Fatalf("sort = %+v, want compatibility descending sort", sortSpec)
	}

	_, err = parseReportPresetSort("impressions:sideways")
	if err == nil || !strings.Contains(err.Error(), "--sort direction must be asc or desc") {
		t.Fatalf("error = %v, want sort direction validation", err)
	}
}

func TestNormalizeReportPresetFields(t *testing.T) {
	fields := normalizeReportPresetFields("campaignName,spend,localSpend,taps")
	if strings.Join(fields, ",") != "campaignName,localSpend,localSpend,taps" {
		t.Fatalf("fields = %v, want spend alias normalized to localSpend", fields)
	}
}
