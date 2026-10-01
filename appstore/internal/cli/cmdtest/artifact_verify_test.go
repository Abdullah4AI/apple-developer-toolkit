package cmdtest

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/cmd"
	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/artifacts/artifactstest"
)

type verificationReceipt struct {
	SignatureVerification       string `json:"signatureVerification"`
	SignatureVerificationDetail string `json:"signatureVerificationDetail"`
	Status                      string `json:"status"`
}

func runVerification(t *testing.T, args ...string) (int, verificationReceipt, string) {
	t.Helper()
	var exit int
	stdout, stderr := captureOutput(t, func() { exit = cmd.Run(args, "test") })
	var receipt verificationReceipt
	if err := json.Unmarshal([]byte(stdout), &receipt); err != nil {
		t.Fatalf("stdout=%q stderr=%q: %v", stdout, stderr, err)
	}
	return exit, receipt, stderr
}

func TestRunArtifactInfoVerifySignatureFailsClosed(t *testing.T) {
	isolateArtifactCommandEnv(t)
	dir := t.TempDir()
	// Throwaway keys never chain to Apple's roots.
	chain := artifactstest.NewTrustChain(t, time.Now().Add(-time.Hour), time.Now().AddDate(1, 0, 0))
	infoPlist := `<plist version="1.0"><dict><key>CFBundleIdentifier</key><string>com.example.demo</string><key>CFBundleExecutable</key><string>Demo</string></dict></plist>`
	options := artifactstest.CodeSignatureOptions{Chain: &chain, InfoPlist: []byte(infoPlist)}
	signedIPA := filepath.Join(dir, "signed.ipa")
	writeArtifactZip(t, signedIPA, map[string]string{
		"Payload/Demo.app/Info.plist":               infoPlist,
		"Payload/Demo.app/Demo":                     string(artifactstest.SignedMachO(t, options)),
		"Payload/Demo.app/embedded.mobileprovision": `<plist version="1.0"><dict><key>Entitlements</key><dict/></dict></plist>`,
	})
	tamperedIPA := filepath.Join(dir, "tampered.ipa")
	tampered := artifactstest.SignedMachO(t, options)
	tampered[5000] ^= 0x01
	writeArtifactZip(t, tamperedIPA, map[string]string{
		"Payload/Demo.app/Info.plist":               infoPlist,
		"Payload/Demo.app/Demo":                     string(tampered),
		"Payload/Demo.app/embedded.mobileprovision": `<plist version="1.0"><dict><key>Entitlements</key><dict/></dict></plist>`,
	})
	info := map[string][]byte{"PackageInfo": []byte(`<pkg-info version="1.0" identifier="com.example.pkg"/>`)}
	signedPKG := filepath.Join(dir, "signed.pkg")
	unsignedPKG := filepath.Join(dir, "unsigned.pkg")
	tamperedPKG := filepath.Join(dir, "tampered.pkg")
	installerChain := artifactstest.NewTrustChainWithLeaf(t, artifactstest.DeveloperIDInstallerLeaf, time.Now().Add(-time.Hour), time.Now().AddDate(1, 0, 0))
	// Payload sorts after PackageInfo, so its bytes end the heap; changing
	// one leaves the signed table of contents intact.
	tamperedPayload := artifactstest.SignedXar(t, map[string][]byte{"PackageInfo": info["PackageInfo"], "Payload": []byte("payload bytes")}, installerChain, time.Now())
	tamperedPayload[len(tamperedPayload)-1] ^= 0x01
	for path, data := range map[string][]byte{
		signedPKG:   artifactstest.SignedXar(t, info, installerChain, time.Now()),
		unsignedPKG: artifactstest.Xar(t, info, ""),
		tamperedPKG: tamperedPayload,
	} {
		if err := os.WriteFile(path, data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	tests := []struct {
		name   string
		args   []string
		status string
		detail string
	}{
		{"untrusted IPA", []string{"ipa-info", "--path", signedIPA, "--verify-signature", "--output", "json"}, "untrusted-chain", "does not chain to an embedded Apple root"},
		{"tampered IPA", []string{"ipa-info", "--path", tamperedIPA, "--verify-signature", "--output", "json"}, "invalid", "code page 1 does not match"},
		{"missing IPA", []string{"ipa-info", "--path", filepath.Join(dir, "missing.ipa"), "--verify-signature", "--output", "json"}, "unsupported", "artifact could not be read"},
		{"untrusted pkg", []string{"pkg-info", "--path", signedPKG, "--verify-signature", "--output", "json"}, "untrusted-chain", "RSA signature verified"},
		{"unsigned pkg", []string{"pkg-info", "--path", unsignedPKG, "--verify-signature", "--output", "json"}, "invalid", "package is unsigned"},
		{"tampered pkg payload", []string{"pkg-info", "--path", tamperedPKG, "--verify-signature", "--output", "json"}, "invalid", `xar file "Payload" does not match its archived sha1 checksum`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			exit, receipt, stderr := runVerification(t, test.args...)
			if exit != cmd.ExitError {
				t.Fatalf("exit=%d stderr=%q", exit, stderr)
			}
			if receipt.SignatureVerification != test.status || !strings.Contains(receipt.SignatureVerificationDetail, test.detail) {
				t.Fatalf("receipt=%+v", receipt)
			}
			if test.status != "unsupported" && !strings.Contains(stderr, "signature verification "+test.status) {
				t.Fatalf("stderr=%q", stderr)
			}
		})
	}

	// Without the flag the same artifacts keep the unverified receipt and exit 0.
	for _, args := range [][]string{
		{"ipa-info", "--path", tamperedIPA, "--output", "json"},
		{"pkg-info", "--path", unsignedPKG, "--output", "json"},
		{"pkg-info", "--path", tamperedPKG, "--output", "json"},
	} {
		exit, receipt, stderr := runVerification(t, args...)
		if exit != cmd.ExitSuccess || stderr != "" || receipt.SignatureVerification != "not-verified" || receipt.SignatureVerificationDetail != "" {
			t.Fatalf("%v: exit=%d receipt=%+v stderr=%q", args, exit, receipt, stderr)
		}
	}
}

func TestRunPKGInfoVerifiesAppleSignedPackage(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("Apple-signed packages ship with Xcode on macOS")
	}
	matches, _ := filepath.Glob("/Applications/Xcode*.app/Contents/Resources/Packages/MobileDeviceDevelopment.pkg")
	if len(matches) == 0 {
		t.Skip("no Apple-signed flat package is installed")
	}
	// CI runners install /Applications/Xcode.app as a symlink to a versioned
	// Xcode, and pkg-info refuses to follow symlinks in --path.
	pkgPath, err := filepath.EvalSymlinks(matches[0])
	if err != nil {
		t.Fatalf("resolve %s: %v", matches[0], err)
	}
	isolateArtifactCommandEnv(t)
	exit, receipt, stderr := runVerification(t, "pkg-info", "--path", pkgPath, "--verify-signature", "--output", "json")
	if exit != cmd.ExitSuccess || receipt.SignatureVerification != "valid" || !strings.Contains(receipt.SignatureVerificationDetail, "Apple Root CA") {
		t.Fatalf("exit=%d receipt=%+v stderr=%q", exit, receipt, stderr)
	}
}

func TestRunIPAInfoVerifySignatureChecksPrimaryUniversalSlice(t *testing.T) {
	isolateArtifactCommandEnv(t)
	chain := artifactstest.NewTrustChain(t, time.Now().Add(-time.Hour), time.Now().AddDate(1, 0, 0))
	infoPlist := `<plist version="1.0"><dict><key>CFBundleIdentifier</key><string>com.example.demo</string><key>CFBundleExecutable</key><string>Demo</string></dict></plist>`
	options := artifactstest.CodeSignatureOptions{Chain: &chain, InfoPlist: []byte(infoPlist)}
	signed := artifactstest.SignedMachO(t, options)
	tampered := artifactstest.SignedMachO(t, options)
	tampered[5000] ^= 0x01
	writeIPA := func(name string, executable []byte) string {
		path := filepath.Join(t.TempDir(), name)
		writeArtifactZip(t, path, map[string]string{
			"Payload/Demo.app/Info.plist":               infoPlist,
			"Payload/Demo.app/Demo":                     string(executable),
			"Payload/Demo.app/embedded.mobileprovision": `<plist version="1.0"><dict><key>Entitlements</key><dict/></dict></plist>`,
		})
		return path
	}
	type receipt struct {
		SignatureVerification       string `json:"signatureVerification"`
		SignatureVerificationDetail string `json:"signatureVerificationDetail"`
		CodeSignature               string `json:"codeSignature"`
		Architectures               []struct {
			Arch          string `json:"arch"`
			CodeSignature string `json:"codeSignature"`
		} `json:"architectures"`
		SignerConsistent *bool `json:"signerConsistent"`
	}
	tests := []struct {
		name   string
		first  []byte
		second []byte
		status string
		detail string
	}{
		// The tampered slice is not verified, so the primary slice reaches the chain check.
		{"signed primary", signed, tampered, "untrusted-chain", "does not chain to an embedded Apple root"},
		{"tampered primary", tampered, signed, "invalid", "code page 1 does not match"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := writeIPA("universal.ipa", artifactstest.UniversalMachO(
				false,
				artifactstest.FatSlice{CPUType: artifactstest.CPUTypeARM64, Data: test.first},
				artifactstest.FatSlice{CPUType: artifactstest.CPUTypeX86_64, CPUSubtype: 3, Data: test.second},
			))
			var exit int
			stdout, stderr := captureOutput(t, func() {
				exit = cmd.Run([]string{"ipa-info", "--path", path, "--verify-signature", "--output", "json"}, "test")
			})
			var got receipt
			if err := json.Unmarshal([]byte(stdout), &got); err != nil {
				t.Fatalf("stdout=%q stderr=%q: %v", stdout, stderr, err)
			}
			if exit != cmd.ExitError || got.SignatureVerification != test.status || !strings.Contains(got.SignatureVerificationDetail, test.detail) {
				t.Fatalf("exit=%d receipt=%+v stderr=%q", exit, got, stderr)
			}
			if got.CodeSignature != "signed" || len(got.Architectures) != 2 || got.Architectures[0].Arch != "arm64" || got.Architectures[1].Arch != "x86_64" ||
				got.Architectures[0].CodeSignature != "signed" || got.Architectures[1].CodeSignature != "signed" || got.SignerConsistent == nil || !*got.SignerConsistent {
				t.Fatalf("receipt=%+v", got)
			}
		})
	}
}

func TestRunPKGInfoVerifySignatureCoversProductArchiveTOC(t *testing.T) {
	isolateArtifactCommandEnv(t)
	chain := artifactstest.NewTrustChainWithLeaf(t, artifactstest.DeveloperIDInstallerLeaf, time.Now().Add(-time.Hour), time.Now().AddDate(1, 0, 0))
	files := map[string][]byte{
		"Distribution":         []byte(`<installer-gui-script minSpecVersion="2"><product id="com.example.demo" version="2.3.4"/><pkg-ref id="com.example.demo" version="2.3.4">#Demo.pkg</pkg-ref></installer-gui-script>`),
		"Demo.pkg/PackageInfo": []byte(`<pkg-info identifier="com.example.demo" version="2.3.4" install-location="/Applications"/>`),
	}
	path := filepath.Join(t.TempDir(), "Demo.pkg")
	if err := os.WriteFile(path, artifactstest.SignedXar(t, files, chain, time.Now()), 0o600); err != nil {
		t.Fatal(err)
	}
	var exit int
	stdout, stderr := captureOutput(t, func() {
		exit = cmd.Run([]string{"pkg-info", "--path", path, "--verify-signature", "--output", "json"}, "test")
	})
	var got struct {
		SignatureVerification       string `json:"signatureVerification"`
		SignatureVerificationDetail string `json:"signatureVerificationDetail"`
		Status                      string `json:"status"`
		ProductID                   string `json:"productId"`
		PackageSignature            string `json:"packageSignature"`
		Components                  []struct {
			Path    string `json:"path"`
			Primary bool   `json:"primary"`
		} `json:"components"`
	}
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("stdout=%q stderr=%q: %v", stdout, stderr, err)
	}
	if exit != cmd.ExitError || got.Status != "readable" || got.ProductID != "com.example.demo" || got.PackageSignature != "signed" ||
		len(got.Components) != 1 || got.Components[0].Path != "Demo.pkg" || !got.Components[0].Primary {
		t.Fatalf("exit=%d receipt=%+v stderr=%q", exit, got, stderr)
	}
	if got.SignatureVerification != "untrusted-chain" || !strings.Contains(got.SignatureVerificationDetail, "RSA signature verified") {
		t.Fatalf("receipt=%+v", got)
	}
}
