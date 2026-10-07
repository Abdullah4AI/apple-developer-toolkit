package ads

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/appleads"
	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/cli/shared"
)

type adsReportPresetFlags struct {
	common commonFlags
	output shared.OutputFlags

	level            *string
	campaign         *string
	adGroup          *string
	from             *string
	to               *string
	lastDays         *int
	granularity      *string
	fields           *string
	sort             *string
	limit            *int
	offset           *int
	timeZone         *string
	timeZoneExplicit *bool
	returnRowTotals  *bool
}

type adsReportPresetPayload struct {
	StartTime       string                  `json:"startTime"`
	EndTime         string                  `json:"endTime"`
	Granularity     string                  `json:"granularity,omitempty"`
	ReturnRowTotals bool                    `json:"returnRowTotals,omitempty"`
	Selector        adsReportPresetSelector `json:"selector"`
	TimeZone        string                  `json:"timeZone,omitempty"`
}

type adsReportPresetSelector struct {
	Fields     []string                   `json:"fields,omitempty"`
	OrderBy    []adsReportPresetSort      `json:"orderBy,omitempty"`
	Pagination *adsReportPresetPagination `json:"pagination,omitempty"`
}

type adsReportPresetSort struct {
	Field     string `json:"field"`
	SortOrder string `json:"sortOrder"`
}

type adsReportPresetPagination struct {
	Offset int `json:"offset"`
	Limit  int `json:"limit"`
}

type adsReportLevelSpec struct {
	commandPath []string
	reportPath  []string
}

var adsReportLevels = map[string]adsReportLevelSpec{
	"campaigns":             {commandPath: []string{"reports", "campaigns"}, reportPath: []string{"reports", "apps", "campaigns"}},
	"ad-groups":             {commandPath: []string{"reports", "ad-groups"}, reportPath: []string{"reports", "apps", "ad-groups"}},
	"keywords":              {commandPath: []string{"reports", "keywords"}, reportPath: []string{"reports", "apps", "keywords"}},
	"search-terms":          {commandPath: []string{"reports", "search-terms"}, reportPath: []string{"reports", "apps", "search-terms"}},
	"ads":                   {commandPath: []string{"reports", "ads"}, reportPath: []string{"reports", "apps", "ads"}},
	"ad-group-keywords":     {commandPath: []string{"reports", "ad-group-keywords"}, reportPath: []string{"reports", "apps", "keywords"}},
	"ad-group-search-terms": {commandPath: []string{"reports", "ad-group-search-terms"}, reportPath: []string{"reports", "apps", "search-terms"}},
}

const reportsPresetMigrationHelp = "Use the matching `asc ads reports apps` command for the selected `--level`; ad-group keyword and search-term levels map to the consolidated `keywords` and `search-terms` reports."

// ReportsPresetCommand returns an operator-friendly Apple Ads reporting helper.
func ReportsPresetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("preset", flag.ExitOnError)
	flags := adsReportPresetFlags{
		common: commonFlags{
			AdsProfile: fs.String("ads-profile", "", "Use named Apple Ads authentication profile"),
			Org:        fs.String("org", "", "Apple Ads organization ID (or ASC_ADS_ORG_ID env)"),
		},
		output:          bindAdsRawOutputFlags(fs),
		level:           fs.String("level", "campaigns", "Report level: campaigns, ad-groups, keywords, search-terms, ads, ad-group-keywords, ad-group-search-terms"),
		campaign:        fs.String("campaign", "", "Campaign ID for campaign-scoped report levels"),
		adGroup:         fs.String("ad-group", "", "Ad group ID for ad-group-scoped report levels"),
		from:            fs.String("from", "", "Start date in YYYY-MM-DD"),
		to:              fs.String("to", "", "End date in YYYY-MM-DD"),
		lastDays:        fs.Int("last-days", 0, "Use an inclusive UTC range ending today"),
		granularity:     fs.String("granularity", "DAILY", "Report granularity: HOURLY, DAILY, WEEKLY, MONTHLY"),
		fields:          fs.String("fields", "", "Comma-separated selector fields to request"),
		sort:            fs.String("sort", "-impressions", "Sort field; omit direction or prefix with - for descending, use field:asc for ascending"),
		limit:           fs.Int("limit", 1000, "Report row limit (1..1000)"),
		offset:          fs.Int("offset", 0, "Report row offset (>=0)"),
		timeZone:        fs.String("time-zone", "UTC", "Apple Ads reporting time zone: UTC or ORTZ"),
		returnRowTotals: fs.Bool("return-row-totals", false, "Request row totals in the report response"),
	}

	command := &ffcli.Command{
		Name:       "preset",
		ShortUsage: "asc ads v5 reports preset --level campaigns --from YYYY-MM-DD --to YYYY-MM-DD [flags]",
		ShortHelp:  "Build and run Apple Ads report presets without JSON payloads.",
		LongHelp: `Build and run Apple Ads report presets without JSON payloads.

This helper builds a documented ReportingRequest for the existing Apple Ads
report endpoints. Choose the report resource with --level. Campaign-scoped and
ad-group-scoped report levels require --campaign and/or --ad-group.

Apple Ads accepts UTC and ORTZ (organization time zone) for --time-zone. Use
--last-days for an inclusive UTC rolling date range; use --from and --to when
requesting ORTZ because Apple Ads resolves the organization time zone. Use the
raw report commands with --file when you need custom conditions or advanced
selector JSON. Report presets default to --sort -impressions because Apple Ads
requires selector.orderBy. HOURLY granularity is available for
	campaign, ad-group, and keyword report levels. Search-term report levels cannot
	request row totals.

Examples:
  asc ads v5 reports preset --level campaigns --last-days 30 --fields campaignName,impressions,taps,localSpend --sort -impressions --org "123456"
  asc ads v5 reports preset --level keywords --campaign 12345 --last-days 7 --fields keyword,impressions,taps --org "123456"
  asc ads v5 reports preset --level ads --campaign 12345 --from YYYY-MM-DD --to YYYY-MM-DD --sort -impressions --org "123456"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if err := rejectUnexpectedArgs(args); err != nil {
				return err
			}
			timeZoneExplicit := false
			fs.Visit(func(f *flag.Flag) {
				if f.Name == "time-zone" {
					timeZoneExplicit = true
				}
			})
			flags.timeZoneExplicit = &timeZoneExplicit
			return executeReportsPreset(ctx, flags)
		},
	}
	return markAdsLegacyCommandDeprecatedWithGuidance(command, []string{"v5", "reports", "preset"}, reportsPresetMigrationHelp, func() string {
		return reportsPresetMigrationGuidance(strings.TrimSpace(*flags.level))
	})
}

func reportsPresetMigrationGuidance(level string) string {
	levelSpec, ok := adsReportLevels[level]
	if !ok {
		return reportsPresetMigrationHelp
	}
	return "Use `asc ads " + strings.Join(levelSpec.reportPath, " ") + "`."
}

func executeReportsPreset(ctx context.Context, flags adsReportPresetFlags) error {
	outputFormat, err := validateAdsRawOutput(flags.output)
	if err != nil {
		return shared.UsageError(err.Error())
	}
	level := strings.TrimSpace(*flags.level)
	levelSpec, ok := adsReportLevels[level]
	if !ok {
		return shared.UsageError("--level must be one of: " + strings.Join(sortedReportPresetLevels(), ", "))
	}
	spec, ok := appleads.EndpointByCommandPath(levelSpec.commandPath...)
	if !ok {
		return fmt.Errorf("ads v5 reports preset: endpoint for level %q is not registered", level)
	}
	pathParams, err := reportPresetPathParams(spec, flags)
	if err != nil {
		return shared.UsageError(err.Error())
	}
	payload, err := buildReportPresetPayload(flags, time.Now().UTC())
	if err != nil {
		return shared.UsageError(err.Error())
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("ads v5 reports preset: marshal request: %w", err)
	}

	client, err := resolveClient(ctx, flags.common, spec.RequiresOrg)
	if err != nil {
		return fmt.Errorf("ads v5 reports preset: %w", err)
	}

	requestCtx, cancel := requestContext(ctx)
	defer cancel()

	result, err := client.Do(requestCtx, spec, pathParams, url.Values{}, body)
	if err != nil {
		return fmt.Errorf("ads v5 reports preset: %w", err)
	}
	return shared.PrintOutput(result, outputFormat, *flags.output.Pretty)
}

func reportPresetPathParams(spec appleads.EndpointSpec, flags adsReportPresetFlags) (map[string]string, error) {
	params := map[string]string{}
	usedFlags := map[string]bool{}
	for _, param := range spec.PathParams {
		usedFlags[param.Flag] = true
		var raw string
		switch param.Name {
		case "campaignId":
			raw = strings.TrimSpace(*flags.campaign)
		case "adgroupId":
			raw = strings.TrimSpace(*flags.adGroup)
		default:
			return nil, fmt.Errorf("unsupported report path parameter %q", param.Name)
		}
		if raw == "" {
			return nil, fmt.Errorf("--%s is required for --level %s", param.Flag, strings.TrimSpace(*flags.level))
		}
		parsed, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("--%s must be an integer", param.Flag)
		}
		if parsed < 0 {
			return nil, fmt.Errorf("--%s must be >= 0", param.Flag)
		}
		params[param.Name] = raw
	}
	level := strings.TrimSpace(*flags.level)
	if !usedFlags["campaign"] && strings.TrimSpace(*flags.campaign) != "" {
		return nil, fmt.Errorf("--campaign is not supported for --level %s", level)
	}
	if !usedFlags["ad-group"] && strings.TrimSpace(*flags.adGroup) != "" {
		return nil, fmt.Errorf("--ad-group is not supported for --level %s", level)
	}
	return params, nil
}

func buildReportPresetPayload(flags adsReportPresetFlags, now time.Time) (adsReportPresetPayload, error) {
	level := strings.TrimSpace(*flags.level)
	reportingTimeZone, err := normalizeReportPresetTimeZone(*flags.timeZone, level, flags.timeZoneExplicit != nil && *flags.timeZoneExplicit)
	if err != nil {
		return adsReportPresetPayload{}, err
	}
	start, end, err := reportPresetDateRange(*flags.from, *flags.to, *flags.lastDays, now, reportingTimeZone)
	if err != nil {
		return adsReportPresetPayload{}, err
	}
	granularity := strings.ToUpper(strings.TrimSpace(*flags.granularity))
	if err := validateReportPresetGranularity(level, granularity); err != nil {
		return adsReportPresetPayload{}, err
	}
	if err := validateReportPresetDateWindow(granularity, start, end, now); err != nil {
		return adsReportPresetPayload{}, err
	}
	if *flags.limit < 1 || *flags.limit > appleads.MaxPageLimit(appleads.EndpointSpec{}) {
		return adsReportPresetPayload{}, fmt.Errorf("--limit must be between 1 and 1000")
	}
	if *flags.offset < 0 {
		return adsReportPresetPayload{}, fmt.Errorf("--offset must be >= 0")
	}
	if isSearchTermReportLevel(level) && *flags.returnRowTotals {
		return adsReportPresetPayload{}, fmt.Errorf("--return-row-totals cannot be used with search-term report levels")
	}

	selector := adsReportPresetSelector{
		Fields: normalizeReportPresetFields(*flags.fields),
		Pagination: &adsReportPresetPagination{
			Offset: *flags.offset,
			Limit:  *flags.limit,
		},
	}
	sortValue := strings.TrimSpace(*flags.sort)
	if sortValue == "" {
		sortValue = "-impressions"
	}
	if sortValue != "" {
		sortSpec, err := parseReportPresetSort(sortValue)
		if err != nil {
			return adsReportPresetPayload{}, err
		}
		selector.OrderBy = []adsReportPresetSort{sortSpec}
	}
	return adsReportPresetPayload{
		StartTime:       start,
		EndTime:         end,
		Granularity:     granularity,
		ReturnRowTotals: *flags.returnRowTotals,
		Selector:        selector,
		TimeZone:        reportingTimeZone,
	}, nil
}

func validateReportPresetGranularity(level string, granularity string) error {
	if granularity == "" {
		return fmt.Errorf("--granularity is required")
	}
	if !slices.Contains([]string{"HOURLY", "DAILY", "WEEKLY", "MONTHLY"}, granularity) {
		return fmt.Errorf("--granularity must be one of: HOURLY, DAILY, WEEKLY, MONTHLY")
	}
	if granularity == "HOURLY" && !slices.Contains([]string{"campaigns", "ad-groups", "keywords", "ad-group-keywords"}, level) {
		return fmt.Errorf("--granularity HOURLY is only supported for campaign, ad-group, and keyword report levels")
	}
	return nil
}

func validateReportPresetDateWindow(granularity string, start string, end string, now time.Time) error {
	startDate, err := parseReportPresetDate("--from", start)
	if err != nil {
		return err
	}
	endDate, err := parseReportPresetDate("--to", end)
	if err != nil {
		return err
	}
	span := endDate.Sub(startDate)
	today := reportPresetToday(now)

	switch granularity {
	case "HOURLY":
		if span > 7*24*time.Hour {
			return fmt.Errorf("--granularity HOURLY supports a maximum 7-day date range")
		}
		if startDate.Before(today.AddDate(0, 0, -30)) {
			return fmt.Errorf("--granularity HOURLY start date must be within the last 30 days")
		}
	case "DAILY":
		if span > 90*24*time.Hour {
			return fmt.Errorf("--granularity DAILY supports a maximum 90-day date range")
		}
		if startDate.Before(today.AddDate(0, 0, -90)) {
			return fmt.Errorf("--granularity DAILY start date must be within the last 90 days")
		}
	case "WEEKLY":
		if span <= 14*24*time.Hour || span > 365*24*time.Hour {
			return fmt.Errorf("--granularity WEEKLY requires a date range more than 14 days and at most 365 days")
		}
		if startDate.Before(today.AddDate(-2, 0, 0)) {
			return fmt.Errorf("--granularity WEEKLY start date must be within the last 24 months")
		}
	case "MONTHLY":
		if !endDate.After(startDate.AddDate(0, 3, 0)) {
			return fmt.Errorf("--granularity MONTHLY requires a date range more than 3 months")
		}
		if startDate.Before(today.AddDate(-2, 0, 0)) {
			return fmt.Errorf("--granularity MONTHLY start date must be within the last 24 months")
		}
	}
	return nil
}

func reportPresetToday(now time.Time) time.Time {
	now = now.UTC()
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
}

func reportPresetDateRange(from, to string, lastDays int, now time.Time, reportingTimeZone string) (string, string, error) {
	from = strings.TrimSpace(from)
	to = strings.TrimSpace(to)
	if lastDays < 0 {
		return "", "", fmt.Errorf("--last-days must be >= 0")
	}
	if lastDays > 0 {
		if from != "" || to != "" {
			return "", "", fmt.Errorf("--last-days cannot be combined with --from or --to")
		}
		if reportingTimeZone != "UTC" {
			return "", "", fmt.Errorf("--last-days is not supported for ORTZ reports; use --from and --to")
		}
		reportingNow := now.UTC()
		end := reportingNow.Format("2006-01-02")
		start := reportingNow.AddDate(0, 0, -(lastDays - 1)).Format("2006-01-02")
		return start, end, nil
	}
	if from == "" || to == "" {
		return "", "", fmt.Errorf("either --last-days or both --from and --to are required")
	}
	startDate, err := parseReportPresetDate("--from", from)
	if err != nil {
		return "", "", err
	}
	endDate, err := parseReportPresetDate("--to", to)
	if err != nil {
		return "", "", err
	}
	if endDate.Before(startDate) {
		return "", "", fmt.Errorf("--to must be on or after --from")
	}
	return from, to, nil
}

func normalizeReportPresetTimeZone(value string, level string, explicit bool) (string, error) {
	normalized := strings.ToUpper(strings.TrimSpace(value))
	if normalized == "" {
		normalized = "UTC"
	}
	if !slices.Contains([]string{"UTC", "ORTZ"}, normalized) {
		return "", fmt.Errorf("--time-zone must be UTC or ORTZ")
	}
	if isSearchTermReportLevel(level) {
		if !explicit && normalized == "UTC" {
			return "ORTZ", nil
		}
		if normalized != "ORTZ" {
			return "", fmt.Errorf("--time-zone must be ORTZ for search-term report levels")
		}
	}
	return normalized, nil
}

func isSearchTermReportLevel(level string) bool {
	return slices.Contains([]string{"search-terms", "ad-group-search-terms"}, level)
}

func parseReportPresetDate(flagName string, value string) (time.Time, error) {
	parsed, err := time.Parse("2006-01-02", value)
	if err != nil {
		return time.Time{}, fmt.Errorf("%s must be in YYYY-MM-DD format", flagName)
	}
	return parsed, nil
}

func parseReportPresetSort(value string) (adsReportPresetSort, error) {
	value = strings.TrimSpace(value)
	field, direction, ok := strings.Cut(value, ":")
	field = strings.TrimSpace(field)
	prefixedDescending := strings.HasPrefix(field, "-")
	if prefixedDescending {
		field = strings.TrimSpace(strings.TrimPrefix(field, "-"))
	}
	if field == "" {
		return adsReportPresetSort{}, fmt.Errorf("--sort field is required")
	}
	field = normalizeReportPresetField(field)
	sortOrder := "DESCENDING"
	if ok {
		switch strings.ToLower(strings.TrimSpace(direction)) {
		case "asc", "ascending":
			sortOrder = "ASCENDING"
		case "desc", "descending":
			sortOrder = "DESCENDING"
		default:
			return adsReportPresetSort{}, fmt.Errorf("--sort direction must be asc or desc")
		}
	}
	return adsReportPresetSort{Field: field, SortOrder: sortOrder}, nil
}

func normalizeReportPresetFields(value string) []string {
	fields := shared.SplitCSV(value)
	for index, field := range fields {
		fields[index] = normalizeReportPresetField(field)
	}
	return fields
}

func normalizeReportPresetField(field string) string {
	if field == "spend" {
		return "localSpend"
	}
	return field
}

func sortedReportPresetLevels() []string {
	levels := make([]string, 0, len(adsReportLevels))
	for level := range adsReportLevels {
		levels = append(levels, level)
	}
	slices.Sort(levels)
	return levels
}
