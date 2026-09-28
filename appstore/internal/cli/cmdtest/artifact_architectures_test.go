package cmdtest

import (
	"crypto/sha256"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/cmd"
	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/artifacts/artifactstest"
)

func writeUniversalIPA(t *testing.T, executable []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "universal.ipa")
	writeArtifactZip(t, path, map[string]string{
		"Payload/Demo.app/Info.plist":               `<plist version="1.0"><dict><key>CFBundleIdentifier</key><string>com.example.demo</string><key>CFBundleExecutable</key><string>Demo</string><key>CFBundleShortVersionString</key><string>1.2.3</string><key>CFBundleVersion</key><string>9</string></dict></plist>`,
		"Payload/Demo.app/Demo":                     string(executable),
		"Payload/Demo.app/embedded.mobileprovision": `<plist version="1.0"><dict><key>Entitlements</key><dict/></dict></plist>`,
	})
	return path
}

func TestRunIPAInfoReportsEveryUniversalArchitecture(t *testing.T) {
	isolateArtifactCommandEnv(t)
	chain := artifactstest.NewChain(t)
	path := writeUniversalIPA(t, artifactstest.UniversalMachO(
		false,
		artifactstest.FatSlice{CPUType: artifactstest.CPUTypeARM64, Data: chain.SignedExecutable(t)},
		artifactstest.FatSlice{CPUType: artifactstest.CPUTypeX86_64, CPUSubtype: 3, Data: chain.SignedExecutable(t)},
	))
	var exit int
	stdout, stderr := captureOutput(t, func() { exit = cmd.Run([]string{"ipa-info", "--path", path, "--output", "json"}, "test") })
	if exit != cmd.ExitSuccess || stderr != "" {
		t.Fatalf("exit=%d stderr=%q", exit, stderr)
	}
	assertJSONEqual(t, stdout, map[string]any{
		"signatureVerification": "not-verified",
		"path":                  path,
		"bundleId":              "com.example.demo",
		"version":               "1.2.3",
		"buildNumber":           "9",
		"teamId":                artifactstest.TeamID,
		"signerCommonName":      artifactstest.SignerCommonName,
		"status":                "readable",
		"nestedBundles":         []any{},
		"codeSignature":         "signed",
		"signer":                signerJSON(chain),
		"architectures": []any{
			map[string]any{"cpuType": artifactstest.CPUTypeARM64, "cpuSubtype": 0, "arch": "arm64", "codeSignature": "signed", "signer": signerJSON(chain)},
			map[string]any{"cpuType": artifactstest.CPUTypeX86_64, "cpuSubtype": 3, "arch": "x86_64", "codeSignature": "signed", "signer": signerJSON(chain)},
		},
		"signerConsistent": true,
	})

	stdout, _ = captureOutput(t, func() { exit = cmd.Run([]string{"ipa-info", "--path", path, "--output", "table"}, "test") })
	if exit != cmd.ExitSuccess || !strings.Contains(stdout, "Architectures") || !strings.Contains(stdout, "arm64, x86_64") {
		t.Fatalf("exit=%d table=%q", exit, stdout)
	}
}

func TestRunIPAInfoWarnsWhenUniversalSlicesAreSignedDifferently(t *testing.T) {
	isolateArtifactCommandEnv(t)
	first, second := artifactstest.NewChain(t), artifactstest.NewChain(t)
	tests := []struct {
		name    string
		second  []byte
		detail  string
		status2 string
	}{
		{"different signer", second.SignedExecutable(t), fmt.Sprintf("x86_64 signed by %q (SHA-256 %X)", artifactstest.SignerCommonName, sha256.Sum256(second.Leaf.Raw)), "signed"},
		{"unsigned slice", artifactstest.MachO(nil), "x86_64 unsigned", "unsigned"},
		{"unreadable slice", []byte("not a Mach-O slice"), "x86_64 unreadable (main executable is not a Mach-O file)", "unreadable"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := writeUniversalIPA(t, artifactstest.UniversalMachO(
				true,
				artifactstest.FatSlice{CPUType: artifactstest.CPUTypeARM64, Data: first.SignedExecutable(t)},
				artifactstest.FatSlice{CPUType: artifactstest.CPUTypeX86_64, CPUSubtype: 3, Data: test.second},
			))
			var exit int
			stdout, stderr := captureOutput(t, func() { exit = cmd.Run([]string{"ipa-info", "--path", path, "--output", "json"}, "test") })
			want := fmt.Sprintf("Warning: ipa-info: architecture slices are not signed consistently: arm64 signed by %q (SHA-256 %X), %s", artifactstest.SignerCommonName, sha256.Sum256(first.Leaf.Raw), test.detail)
			if exit != cmd.ExitSuccess || strings.TrimSpace(stderr) != want {
				t.Fatalf("exit=%d stderr=%q\nwant=%q", exit, stderr, want)
			}
			if !strings.Contains(stdout, `"signerConsistent":false`) || !strings.Contains(stdout, `"codeSignature":"`+test.status2+`"`) {
				t.Fatalf("stdout=%s", stdout)
			}
		})
	}
}
