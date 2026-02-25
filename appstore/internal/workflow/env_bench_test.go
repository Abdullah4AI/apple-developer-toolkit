package workflow

import (
	"errors"
	"fmt"
	"os/exec"
	"testing"
)

func BenchmarkBuildEnvSlice(b *testing.B) {
	overrides := make(map[string]string, 64)

	for i := 0; i < 32; i++ {
		name := fmt.Sprintf("WORKFLOW_BENCH_EXISTING_%02d", i)
		b.Setenv(name, "old-value")
		overrides[name] = "new-value"
	}
	for i := 0; i < 32; i++ {
		name := fmt.Sprintf("WORKFLOW_BENCH_NEW_%02d", i)
		overrides[name] = "new-value"
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = buildEnvSlice(overrides)
	}
}

func BenchmarkResolveShellCached(b *testing.B) {
	resetShellCacheForTest()
	originalLookPathFn := lookPathFn
	b.Cleanup(func() {
		lookPathFn = originalLookPathFn
		resetShellCacheForTest()
	})

	lookPathFn = func(file string) (string, error) {
		if file == "bash" {
			return "/bin/bash", nil
		}
		if file == "sh" {
			return "", exec.ErrNotFound
		}
		return "", errors.New("unexpected shell lookup")
	}

	if _, _, err := resolveShell(); err != nil {
		b.Fatalf("resolveShell() warmup error: %v", err)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		name, flags, err := resolveShell()
		if err != nil {
			b.Fatalf("resolveShell() error: %v", err)
		}
		if name != "bash" || len(flags) != 3 {
			b.Fatalf("unexpected shell resolution: %q %v", name, flags)
		}
	}
}

func BenchmarkResolveShellUncached(b *testing.B) {
	resetShellCacheForTest()
	originalLookPathFn := lookPathFn
	b.Cleanup(func() {
		lookPathFn = originalLookPathFn
		resetShellCacheForTest()
	})

	lookPathFn = func(file string) (string, error) {
		if file == "bash" {
			return "/bin/bash", nil
		}
		if file == "sh" {
			return "", exec.ErrNotFound
		}
		return "", errors.New("unexpected shell lookup")
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		resetShellCacheForTest()
		name, flags, err := resolveShell()
		if err != nil {
			b.Fatalf("resolveShell() error: %v", err)
		}
		if name != "bash" || len(flags) != 3 {
			b.Fatalf("unexpected shell resolution: %q %v", name, flags)
		}
	}
}
