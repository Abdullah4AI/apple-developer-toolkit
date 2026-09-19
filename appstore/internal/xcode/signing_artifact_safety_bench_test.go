package xcode

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// BenchmarkValidateSigningArtifactAliasesAuthorizationMembership measures the
// authorization gate in its production context. Protected paths are reversed
// relative to authorized paths so the current linear scan performs a
// last-match search for every entry. The path hooks keep the benchmark focused
// on lexical membership rather than filesystem I/O.
func BenchmarkValidateSigningArtifactAliasesAuthorizationMembership(b *testing.B) {
	previousInfo := signingArtifactPathInfoFn
	previousResolve := signingResolveProspectivePathFn
	previousCaseSemantics := signingCaseInsensitiveVolumeFn
	signingArtifactPathInfoFn = func(string) (os.FileInfo, error) {
		return nil, os.ErrNotExist
	}
	signingResolveProspectivePathFn = func(path string) (string, error) {
		return path, nil
	}
	signingCaseInsensitiveVolumeFn = func(string) (bool, bool) { return true, true }
	b.Cleanup(func() {
		signingArtifactPathInfoFn = previousInfo
		signingResolveProspectivePathFn = previousResolve
		signingCaseInsensitiveVolumeFn = previousCaseSemantics
	})

	for _, count := range []int{512, 1024, 2048, 4096} {
		count := count
		for _, caseVariant := range []bool{false, true} {
			caseVariant := caseVariant
			name := "exact"
			if caseVariant {
				name = "case-variant"
			}
			b.Run(fmt.Sprintf("paths=%d/%s", count, name), func(b *testing.B) {
				root := filepath.Join(os.TempDir(), "asc-signing-authorization-benchmark", fmt.Sprintf("%d", count))
				authorized := make([]string, count)
				protected := make([]string, count)
				for index := range authorized {
					authorized[index] = filepath.Join(root, fmt.Sprintf("Graph-%04d.xcconfig", index))
					protectedPath := authorized[count-1-index]
					if caseVariant {
						protectedPath = strings.ToUpper(protectedPath)
					}
					protected[index] = protectedPath
				}

				b.ReportAllocs()
				b.ResetTimer()
				for range b.N {
					if err := validateSigningArtifactAliasesWithAuthorizedProtectedPaths(
						filepath.Join(root, "plan.json"),
						filepath.Join(root, "receipt.json"),
						nil,
						protected,
						authorized,
					); err != nil {
						b.Fatal(err)
					}
				}
			})
		}
	}
}
