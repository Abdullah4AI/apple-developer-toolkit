package xcode

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestSigningAuthorizedPathIndexPreservesNormalizedAndCaseAwareMembership(t *testing.T) {
	previousCaseSemantics := signingCaseInsensitiveVolumeFn
	t.Cleanup(func() { signingCaseInsensitiveVolumeFn = previousCaseSemantics })

	root := t.TempDir()
	exact := filepath.Join(root, "Config", "..", "Protected.xcconfig")
	signingCaseInsensitiveVolumeFn = func(string) (bool, bool) {
		t.Fatal("exact normalized authorization unexpectedly probed filesystem case semantics")
		return false, false
	}
	index := newSigningAuthorizedPathIndex([]string{exact})
	if !index.contains(filepath.Join(root, "Protected.xcconfig")) {
		t.Fatal("normalized exact authorization was rejected")
	}

	authorized := filepath.Join(root, "Protected.xcconfig")
	variant := filepath.Join(root, "PROTECTED.XCCONFIG")
	index = newSigningAuthorizedPathIndex([]string{authorized})
	signingCaseInsensitiveVolumeFn = func(string) (bool, bool) { return true, true }
	if !index.contains(variant) {
		t.Fatal("case variant on a known case-insensitive directory was rejected")
	}

	signingCaseInsensitiveVolumeFn = func(string) (bool, bool) { return false, true }
	if index.contains(variant) {
		t.Fatal("case variant on a known case-sensitive directory was authorized")
	}

	signingCaseInsensitiveVolumeFn = func(string) (bool, bool) { return false, false }
	if index.contains(variant) {
		t.Fatal("case variant with unknown directory semantics was authorized")
	}
	if index.contains(filepath.Join(root, "Other.xcconfig")) {
		t.Fatal("unrelated path was authorized")
	}
}

func TestValidateSigningArtifactAliasesSkipsUnauthorizedProtectedPathBeforeInspection(t *testing.T) {
	root := t.TempDir()
	planPath := filepath.Join(root, "plan.json")
	receiptPath := filepath.Join(root, "receipt.json")
	protectedPath := filepath.Join(root, "protected.xcconfig")

	previousInfo := signingArtifactPathInfoFn
	previousResolve := signingResolveProspectivePathFn
	signingArtifactPathInfoFn = func(path string) (os.FileInfo, error) {
		if filepath.Clean(path) == protectedPath {
			t.Fatalf("unauthorized protected path was inspected: %s", path)
		}
		return nil, os.ErrNotExist
	}
	signingResolveProspectivePathFn = func(path string) (string, error) {
		if filepath.Clean(path) == protectedPath {
			t.Fatalf("unauthorized protected path was resolved: %s", path)
		}
		return path, nil
	}
	t.Cleanup(func() {
		signingArtifactPathInfoFn = previousInfo
		signingResolveProspectivePathFn = previousResolve
	})

	if err := validateSigningArtifactAliasesWithAuthorizedProtectedPaths(
		planPath,
		receiptPath,
		nil,
		[]string{protectedPath},
		nil,
	); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			t.Fatalf("unauthorized protected path validation returned a path error: %v", err)
		}
		t.Fatalf("validateSigningArtifactAliasesWithAuthorizedProtectedPaths() error = %v", err)
	}
}
