package registry

import (
	"testing"

	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/cli/search"
)

// Include document collection and ranking over the real catalog, while excluding
// command factory construction shared by both versions of the ranker.
func BenchmarkSearchCommandsPerformance(b *testing.B) {
	commands := NewCatalog("benchmark").All()
	for _, query := range []string{"upload build", "external testers", "cert profiles"} {
		b.Run(query, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				_ = search.SearchCommands(commands, query, 10)
			}
		})
	}
}
