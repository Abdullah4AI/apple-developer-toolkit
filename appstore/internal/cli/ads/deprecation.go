package ads

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/appleads"
)

// adsLegacyMigrationKind describes how much compatibility work is required
// when moving a Campaign Management API v5 command to Platform API v1.
//
// The distinction is intentionally kept in the CLI package. It is migration
// guidance, not an API-client concern, and the v5 EndpointSpec table remains
// the source of truth for the request that is sent during the compatibility
// window.
type adsLegacyMigrationKind string

const (
	adsLegacyDirect   adsLegacyMigrationKind = "direct"
	adsLegacyBreaking adsLegacyMigrationKind = "breaking"
	adsLegacyNone     adsLegacyMigrationKind = "none"
)

type adsLegacyMigration struct {
	kind        adsLegacyMigrationKind
	replacement []string
	guidance    string
}

const adsLegacyRetirementNotice = "Apple Ads Campaign Management API v5 is deprecated and retires on January 26, 2027."

// adsLegacyMigrations is the complete v5-to-v1 migration ledger. Keep one
// entry for every v5 EndpointSpec, including operations for which no single
// Platform API command exists. This makes omissions fail loudly in tests and
// keeps help and runtime warnings on the same contract.
var adsLegacyMigrations = map[string]adsLegacyMigration{
	"get-user-acl":                    {kind: adsLegacyDirect, replacement: []string{"acls", "list"}},
	"get-me-details":                  {kind: adsLegacyDirect, replacement: []string{"me", "view"}},
	"search-for-ios-apps":             {kind: adsLegacyDirect, replacement: []string{"apps", "search"}},
	"get-app-details":                 {kind: adsLegacyDirect, replacement: []string{"apps", "view"}},
	"get-localized-app-details":       {kind: adsLegacyBreaking, replacement: []string{"apps", "locales", "find"}},
	"find-app-eligibility-records":    {kind: adsLegacyBreaking, replacement: []string{"apps", "eligibility", "find"}},
	"find-app-assets":                 {kind: adsLegacyBreaking, replacement: []string{"assets", "find"}},
	"get-product-pages":               {kind: adsLegacyBreaking, replacement: []string{"product-pages", "find"}},
	"get-product-pages-by-identifier": {kind: adsLegacyDirect, replacement: []string{"product-pages", "view"}},
	"get-product-page-locales":        {kind: adsLegacyBreaking, replacement: []string{"product-pages", "locales", "find"}},
	"get-supported-countries-or-regions": {
		kind:     adsLegacyNone,
		guidance: "No one-command replacement exists. Maintain the supported country or region catalog used by your workflow.",
	},
	"get-app-preview-device-sizes": {
		kind:     adsLegacyNone,
		guidance: "No one-command replacement exists. Use the Platform API creative requirements for the selected supply source.",
	},

	"get-all-budget-orders": {kind: adsLegacyDirect, replacement: []string{"budget-orders", "find"}},
	"create-a-budget-order": {kind: adsLegacyDirect, replacement: []string{"budget-orders", "create"}},
	"get-a-budget-order":    {kind: adsLegacyDirect, replacement: []string{"budget-orders", "view"}},
	"update-a-budget-order": {kind: adsLegacyDirect, replacement: []string{"budget-orders", "update"}},

	"get-all-campaigns": {kind: adsLegacyBreaking, replacement: []string{"campaigns", "find"}},
	"create-a-campaign": {kind: adsLegacyDirect, replacement: []string{"campaigns", "create"}},
	"find-campaigns":    {kind: adsLegacyDirect, replacement: []string{"campaigns", "find"}},
	"delete-a-campaign": {kind: adsLegacyDirect, replacement: []string{"campaigns", "delete"}},
	"get-a-campaign":    {kind: adsLegacyDirect, replacement: []string{"campaigns", "view"}},
	"update-a-campaign": {kind: adsLegacyDirect, replacement: []string{"campaigns", "update"}},

	"get-all-ad-groups":                  {kind: adsLegacyBreaking, replacement: []string{"ad-groups", "find"}},
	"create-an-ad-group":                 {kind: adsLegacyDirect, replacement: []string{"ad-groups", "create"}},
	"find-ad-groups":                     {kind: adsLegacyDirect, replacement: []string{"ad-groups", "find"}},
	"find-ad-groups-across-organization": {kind: adsLegacyBreaking, replacement: []string{"ad-groups", "find"}},
	"delete-an-ad-group":                 {kind: adsLegacyDirect, replacement: []string{"ad-groups", "delete"}},
	"get-an-ad-group":                    {kind: adsLegacyDirect, replacement: []string{"ad-groups", "view"}},
	"update-an-ad-group":                 {kind: adsLegacyDirect, replacement: []string{"ad-groups", "update"}},

	"get-all-ads":                  {kind: adsLegacyBreaking, replacement: []string{"ads", "find"}},
	"create-an-ad":                 {kind: adsLegacyDirect, replacement: []string{"ads", "create"}},
	"delete-an-ad":                 {kind: adsLegacyDirect, replacement: []string{"ads", "delete"}},
	"get-an-ad":                    {kind: adsLegacyDirect, replacement: []string{"ads", "view"}},
	"update-an-ad":                 {kind: adsLegacyDirect, replacement: []string{"ads", "update"}},
	"find-ads":                     {kind: adsLegacyDirect, replacement: []string{"ads", "find"}},
	"find-ads-across-organization": {kind: adsLegacyBreaking, replacement: []string{"ads", "find"}},

	"get-all-targeting-keywords-in-an-ad-group": {kind: adsLegacyBreaking, replacement: []string{"targeting-keywords", "find"}},
	"find-targeting-keywords-in-a-campaign":     {kind: adsLegacyDirect, replacement: []string{"targeting-keywords", "find"}},
	"get-a-targeting-keyword-in-an-ad-group":    {kind: adsLegacyDirect, replacement: []string{"targeting-keywords", "view"}},
	"create-targeting-keywords":                 {kind: adsLegacyDirect, replacement: []string{"targeting-keywords", "create-bulk"}},
	"update-targeting-keywords":                 {kind: adsLegacyDirect, replacement: []string{"targeting-keywords", "update-bulk"}},
	"delete-a-targeting-keyword":                {kind: adsLegacyDirect, replacement: []string{"targeting-keywords", "delete"}},
	"delete-targeting-keywords": {
		kind:     adsLegacyNone,
		guidance: "No one-command replacement exists. Query matching keywords with `asc ads targeting-keywords find`, then delete each ID with `asc ads targeting-keywords delete --confirm`.",
	},

	"get-all-campaign-negative-keywords": {kind: adsLegacyBreaking, replacement: []string{"negative-keywords", "find"}},
	"find-campaign-negative-keywords":    {kind: adsLegacyBreaking, replacement: []string{"negative-keywords", "find"}},
	"get-a-campaign-negative-keyword":    {kind: adsLegacyBreaking, replacement: []string{"negative-keywords", "view"}},
	"create-campaign-negative-keywords":  {kind: adsLegacyBreaking, replacement: []string{"negative-keywords", "create-bulk"}},
	"update-campaign-negative-keywords":  {kind: adsLegacyBreaking, replacement: []string{"negative-keywords", "update-bulk"}},
	"delete-campaign-negative-keywords": {
		kind:     adsLegacyNone,
		guidance: "No one-command replacement exists. Query matching negative keywords with `asc ads negative-keywords find`, then delete each ID with `asc ads negative-keywords delete --confirm`.",
	},

	"get-all-ad-group-negative-keywords": {kind: adsLegacyBreaking, replacement: []string{"negative-keywords", "find"}},
	"find-ad-group-negative-keywords":    {kind: adsLegacyBreaking, replacement: []string{"negative-keywords", "find"}},
	"get-an-ad-group-negative-keyword":   {kind: adsLegacyBreaking, replacement: []string{"negative-keywords", "view"}},
	"create-ad-group-negative-keywords":  {kind: adsLegacyBreaking, replacement: []string{"negative-keywords", "create-bulk"}},
	"update-ad-group-negative-keywords":  {kind: adsLegacyBreaking, replacement: []string{"negative-keywords", "update-bulk"}},
	"delete-ad-group-negative-keywords": {
		kind:     adsLegacyNone,
		guidance: "No one-command replacement exists. Query matching negative keywords with `asc ads negative-keywords find`, then delete each ID with `asc ads negative-keywords delete --confirm`.",
	},

	"search-for-geolocations":     {kind: adsLegacyBreaking, replacement: []string{"geo", "search"}},
	"get-a-list-of-geo-locations": {kind: adsLegacyBreaking, replacement: []string{"geo", "resolve"}},

	"get-all-creatives": {kind: adsLegacyBreaking, replacement: []string{"creatives", "find"}},
	"create-a-creative": {kind: adsLegacyBreaking, replacement: []string{"creatives", "create"}},
	"find-creatives":    {kind: adsLegacyBreaking, replacement: []string{"creatives", "find"}},
	"get-a-creative":    {kind: adsLegacyBreaking, replacement: []string{"creatives", "view"}},

	"find-ad-creative-rejection-reasons": {kind: adsLegacyBreaking, replacement: []string{"rejection-reasons", "apps", "find"}},
	"gets-a-product-page-reason":         {kind: adsLegacyBreaking, replacement: []string{"rejection-reasons", "apps", "view"}},

	"get-campaign-level-reports":                    {kind: adsLegacyBreaking, replacement: []string{"reports", "apps", "campaigns"}},
	"get-ad-group-level-reports":                    {kind: adsLegacyBreaking, replacement: []string{"reports", "apps", "ad-groups"}},
	"get-keyword-level-reports":                     {kind: adsLegacyBreaking, replacement: []string{"reports", "apps", "keywords"}},
	"get-search-term-level-reports":                 {kind: adsLegacyBreaking, replacement: []string{"reports", "apps", "search-terms"}},
	"get-ad-level-reports":                          {kind: adsLegacyBreaking, replacement: []string{"reports", "apps", "ads"}},
	"get-keyword-level-within-ad-group-reports":     {kind: adsLegacyBreaking, replacement: []string{"reports", "apps", "keywords"}},
	"get-search-term-level-within-ad-group-reports": {kind: adsLegacyBreaking, replacement: []string{"reports", "apps", "search-terms"}},

	"get-all-impression-share-reports": {
		kind:     adsLegacyNone,
		guidance: "No one-command replacement exists. Use `asc ads insights impression-share find` with an equivalent query payload.",
	},
	"impression-share-report": {kind: adsLegacyBreaking, replacement: []string{"insights", "impression-share", "find"}},
	"get-a-single-impression-share-report": {
		kind:     adsLegacyNone,
		guidance: "No one-command replacement exists. Use `asc ads insights impression-share find` with an equivalent query payload.",
	},
}

// adsLegacyMigrationForSpec returns the migration entry and reports whether
// the spec is a v5 command that should be marked deprecated.
func adsLegacyMigrationForSpec(spec appleads.EndpointSpec) (adsLegacyMigration, bool) {
	if spec.Version == appleads.APIVersionPlatformV1 {
		return adsLegacyMigration{}, false
	}
	migration, ok := adsLegacyMigrations[spec.Name]
	return migration, ok
}

func adsLegacyGuidance(migration adsLegacyMigration) string {
	if migration.kind == adsLegacyNone {
		return migration.guidance
	}
	return "Use `asc ads " + strings.Join(migration.replacement, " ") + "`."
}

func markAdsLegacyCommandDeprecated(cmd *ffcli.Command, oldPath []string, migration adsLegacyMigration) *ffcli.Command {
	guidance := adsLegacyGuidance(migration)
	return markAdsLegacyCommandDeprecatedWithGuidance(cmd, oldPath, guidance, nil)
}

func markAdsLegacyCommandDeprecatedWithGuidance(cmd *ffcli.Command, oldPath []string, helpGuidance string, runtimeGuidance func() string) *ffcli.Command {
	if cmd == nil {
		return nil
	}
	oldCommand := "asc ads " + strings.Join(oldPath, " ")
	help := adsLegacyRetirementNotice + " " + helpGuidance
	cmd.ShortHelp = "DEPRECATED: " + help
	if longHelp := strings.TrimSpace(cmd.LongHelp); longHelp != "" {
		cmd.LongHelp = "DEPRECATED: " + help + "\n\n" + longHelp
	} else {
		cmd.LongHelp = "DEPRECATED: " + help
	}

	originalExec := cmd.Exec
	cmd.Exec = func(ctx context.Context, args []string) error {
		guidance := helpGuidance
		if runtimeGuidance != nil {
			if selected := strings.TrimSpace(runtimeGuidance()); selected != "" {
				guidance = selected
			}
		}
		fmt.Fprintf(os.Stderr, "Warning: `%s` is deprecated and retires on January 26, 2027. %s\n", oldCommand, guidance)
		if originalExec == nil {
			return nil
		}
		return originalExec(ctx, args)
	}
	return cmd
}

func adsLegacyReplacementRegistered(path []string) bool {
	if len(path) == 0 {
		return false
	}
	if _, ok := appleads.PlatformEndpointByCommandPath(path...); ok {
		return true
	}
	// These are hand-written v1 workflow replacements, not EndpointSpecs.
	return strings.Join(path, " ") == "campaigns pause" || strings.Join(path, " ") == "campaigns resume"
}
