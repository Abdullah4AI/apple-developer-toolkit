package assetlibrary

import (
	"flag"
	"net/url"
	"regexp"
	"strings"

	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/cli/shared"
)

var libraryStateToken = regexp.MustCompile(`^[A-Z][A-Z0-9_]*$`)

var libraryListFilterFlags = []string{"category", "state", "spec-id", "sort", "reference-name", "id"}

type libraryListFilters struct{ category, state, specID, sort, referenceName, id string }

func bindLibraryListFilters(fs *flag.FlagSet) *libraryListFilters {
	filters := &libraryListFilters{}
	fs.StringVar(&filters.category, "category", "", "Filter categories (CSV): CREATIVE_ASSETS, APP_SCREENSHOTS_AND_PREVIEWS")
	fs.StringVar(&filters.state, "state", "", "Filter asset states (CSV; provider validates values)")
	fs.StringVar(&filters.specID, "spec-id", "", "Filter asset specification IDs (comma-separated)")
	fs.StringVar(&filters.sort, "sort", "", "Sort by referenceName, createdDate or lastModifiedDate (CSV; prefix - for descending)")
	fs.StringVar(&filters.referenceName, "reference-name", "", "Filter internal reference names (CSV)")
	fs.StringVar(&filters.id, "id", "", "Filter asset IDs (comma-separated)")
	return filters
}

func (filters *libraryListFilters) query(fs *flag.FlagSet) (url.Values, error) {
	values := url.Values{}
	provided := map[string]bool{}
	fs.Visit(func(f *flag.Flag) { provided[f.Name] = true })
	for _, field := range []struct {
		flag, key, raw string
		valid          func(string) bool
	}{
		{"category", "filter[category]", filters.category, func(token string) bool { return token == "CREATIVE_ASSETS" || token == "APP_SCREENSHOTS_AND_PREVIEWS" }},
		{"state", "filter[state]", filters.state, libraryStateToken.MatchString},
		{"spec-id", "filter[specId]", filters.specID, resourceIDPattern.MatchString},
		{"reference-name", "filter[referenceName]", filters.referenceName, func(string) bool { return true }},
		{"id", "filter[id]", filters.id, resourceIDPattern.MatchString},
		{"sort", "sort", filters.sort, func(token string) bool {
			value := strings.TrimPrefix(token, "-")
			return value == "referenceName" || value == "lastModifiedDate" || value == "createdDate"
		}},
	} {
		if !provided[field.flag] {
			continue
		}
		tokens := strings.Split(field.raw, ",")
		for i, token := range tokens {
			tokens[i] = strings.TrimSpace(token)
			if tokens[i] == "" || !field.valid(tokens[i]) {
				return nil, shared.UsageErrorf("asset-library: --%s contains an empty or unsupported token %q", field.flag, tokens[i])
			}
		}
		values.Set(field.key, strings.Join(tokens, ","))
	}
	return values, nil
}
