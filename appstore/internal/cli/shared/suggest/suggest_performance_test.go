package suggest

import (
	"math/rand/v2"
	"reflect"
	"sort"
	"strings"
	"testing"
)

// Keep the original ranking as an oracle for the fast-rejection optimization.
func legacyCommands(input string, candidates []string) []string {
	in := strings.ToLower(strings.TrimSpace(input))
	if in == "" {
		return nil
	}

	collected := make([]candidate, 0, len(candidates))
	for _, raw := range candidates {
		name := strings.ToLower(strings.TrimSpace(raw))
		if name == "" || name == in {
			continue
		}

		d := editDistance(in, name)

		// Strongest signal: prefix relationship.
		if strings.HasPrefix(name, in) || strings.HasPrefix(in, name) {
			collected = append(collected, candidate{name: name, score: 0, dist: d})
			continue
		}
		// A remembered component span of a hyphenated name (`phased` for
		// `phased-release` or `list` for `app-list-all`).
		if isSubstringMatch(in, name) {
			collected = append(collected, candidate{name: name, score: 1, dist: d})
			continue
		}
		if !withinThreshold(in, d) {
			continue
		}
		collected = append(collected, candidate{name: name, score: 2, dist: d})
	}

	if len(collected) == 0 {
		return nil
	}

	sort.Slice(collected, func(i, j int) bool {
		if collected[i].score != collected[j].score {
			return collected[i].score < collected[j].score
		}
		if collected[i].dist != collected[j].dist {
			return collected[i].dist < collected[j].dist
		}
		return collected[i].name < collected[j].name
	})

	const max = 3
	out := make([]string, 0, max)
	seen := make(map[string]struct{}, max)
	for _, c := range collected {
		if _, ok := seen[c.name]; ok {
			continue
		}
		seen[c.name] = struct{}{}
		out = append(out, c.name)
		if len(out) >= max {
			break
		}
	}
	return out
}

func TestCommandsFastRejectionPreservesRanking(t *testing.T) {
	candidates := []string{"", "list", "LIST", " list ", "view", "apps", "builds", "build-bundles", "build-localizations", "phased-release", "app-list-all", "release", "releases", "abcd", "abcde", "abcdefg", "é", "éé", "東京", "東京-build", "åpps"}
	inputs := append([]string{"lsit", "buil", "release", "list", "abcd", "abcde", "é", "東京", "", "   ", " LIST "}, candidates...)
	r := rand.New(rand.NewPCG(19, 37))
	alphabet := []rune("ab-listé東京")
	for range 300 {
		var b strings.Builder
		for range r.IntN(16) {
			b.WriteRune(alphabet[r.IntN(len(alphabet))])
		}
		value := b.String()
		candidates = append(candidates, value)
		inputs = append(inputs, value)
	}
	for _, input := range inputs {
		got, want := Commands(input, candidates), legacyCommands(input, candidates)
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("Commands(%q) = %v, original = %v", input, got, want)
		}
	}
	// Oversized weak matches reject cheaply; strong matches still retain distance ranking.
	for _, input := range []string{strings.Repeat("q", 200000), "list" + strings.Repeat("q", 2000), strings.Repeat("q", 2000) + "-release"} {
		got, want := Commands(input, candidates[:21]), legacyCommands(input, candidates[:21])
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("oversized Commands length %d = %v, original = %v", len(input), got, want)
		}
	}
}

func BenchmarkCommandsPerformance(b *testing.B) {
	candidates := []string{"apps", "builds", "build-bundles", "build-localizations", "list", "view", "create", "phased-release", "app-list-all", "subscriptions", "configuration", "delete", "reviews"}
	for _, tc := range []struct{ name, input string }{
		{"Typo", "lsit"}, {"Prefix", "buil"}, {"Component", "release"}, {"Unmatched", "external"}, {"Oversized", strings.Repeat("q", 200000)},
	} {
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				_ = Commands(tc.input, candidates)
			}
		})
	}
}
