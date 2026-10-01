package artifacts

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func appleTrustPolicyForTest(t *testing.T) *trustPolicy {
	t.Helper()
	policy, err := appleTrustPolicy()
	if err != nil {
		t.Fatal(err)
	}
	return policy
}

// codesignAccepts reports whether codesign --verify accepts path.
func codesignAccepts(t *testing.T, path string) bool {
	t.Helper()
	codesign, err := exec.LookPath("codesign")
	if err != nil {
		t.Skip("codesign is not installed")
	}
	return exec.Command(codesign, "--verify", "--strict", path).Run() == nil
}

// Real Apple platform binaries exercise BER CMS, multiple code directories,
// and CDHashes attributes that the synthetic fixtures only approximate.
func TestVerifyMachOAgreesWithCodesignOnSystemBinary(t *testing.T) {
	data, err := os.ReadFile("/bin/ls")
	if err != nil {
		t.Skipf("system binary unavailable: %v", err)
	}
	copyPath := filepath.Join(t.TempDir(), "ls")
	if err := os.WriteFile(copyPath, data, 0o755); err != nil {
		t.Fatal(err)
	}
	policy := appleTrustPolicyForTest(t)
	result := verifyMachOBytes(data, policy)
	if !codesignAccepts(t, copyPath) {
		t.Skip("codesign does not accept the copied system binary")
	}
	if result.Status != VerificationValid || !strings.Contains(result.Detail, "leaf is an Apple software signing certificate") {
		t.Fatalf("codesign accepts /bin/ls but verification=%+v", result)
	}

	// Flip one byte of code inside the first slice's signed pages.
	tampered := append([]byte(nil), data...)
	capture := &signatureCapture{}
	if _, err := readMachOSignatureCapture(bytes.NewReader(data), int64(len(data)), capture); err != nil {
		t.Fatal(err)
	}
	offset := capture.base + 0x2000
	tampered[offset] ^= 0xff
	if err := os.WriteFile(copyPath, tampered, 0o755); err != nil {
		t.Fatal(err)
	}
	if codesignAccepts(t, copyPath) {
		t.Fatal("codesign accepted a tampered binary")
	}
	if result := verifyMachOBytes(tampered, policy); result.Status != VerificationInvalid || !strings.Contains(result.Detail, "code page") {
		t.Fatalf("tampered verification=%+v", result)
	}
}

func TestVerifyXarAgreesWithPkgutil(t *testing.T) {
	pkgutil, err := exec.LookPath("pkgutil")
	if err != nil {
		t.Skip("pkgutil is not installed")
	}
	matches, _ := filepath.Glob("/Applications/Xcode*.app/Contents/Resources/Packages/MobileDeviceDevelopment.pkg")
	if len(matches) == 0 {
		t.Skip("no Apple-signed flat package is installed")
	}
	data, err := os.ReadFile(matches[0])
	if err != nil {
		t.Skipf("package unavailable: %v", err)
	}
	checkSignature := func(data []byte) bool {
		path := filepath.Join(t.TempDir(), "check.pkg")
		if err := os.WriteFile(path, data, 0o644); err != nil {
			t.Fatal(err)
		}
		return exec.Command(pkgutil, "--check-signature", path).Run() == nil
	}
	policy := appleTrustPolicyForTest(t)
	verify := func(data []byte) SignatureVerification {
		manifest, _ := inspectPKGVerifying(bytes.NewReader(data), int64(len(data)), policy)
		return *manifest.SignatureVerification
	}
	if !checkSignature(data) {
		t.Skip("pkgutil does not accept the installed package")
	}
	if result := verify(data); result.Status != VerificationValid || !strings.Contains(result.Detail, "leaf is an Apple Software Update signing certificate") || !strings.Contains(result.Detail, "file payload checksums recomputed") {
		t.Fatalf("pkgutil accepts the package but verification=%+v", result)
	}

	document, heap, _, err := readXarFiles(bytes.NewReader(data), int64(len(data)))
	if err != nil || document.Signature == nil {
		t.Fatalf("document=%+v err=%v", document, err)
	}
	checksum, err := verifiedXarChecksum(bytes.NewReader(data), int64(len(data)), document)
	if err != nil {
		t.Fatal(err)
	}
	tampered := append([]byte(nil), data...)
	tampered[checksum.heap+document.Signature.Offset+10] ^= 0xff
	if checkSignature(tampered) {
		t.Fatal("pkgutil accepted a tampered signature")
	}
	if result := verify(tampered); result.Status != VerificationInvalid {
		t.Fatalf("tampered verification=%+v", result)
	}

	// A payload byte changed after signing leaves the signed table of
	// contents intact; pkgutil rejects it through the member checksum.
	payload := xarChild(document.Files, "Payload", "file")
	if payload == nil || payload.Data.Length < 2 {
		t.Fatalf("package has no payload: %+v", document.Files)
	}
	tampered = append([]byte(nil), data...)
	tampered[int64(len(data))-heap.Size()+payload.Data.Offset+payload.Data.Length/2] ^= 0xff
	if checkSignature(tampered) {
		t.Fatal("pkgutil accepted a tampered payload")
	}
	if result := verify(tampered); result.Status != VerificationInvalid || !strings.Contains(result.Detail, `xar file "Payload" does not match its archived sha1 checksum`) {
		t.Fatalf("tampered payload verification=%+v", result)
	}
}

// Every package Xcode bundles is Apple-signed; each must verify, payload
// checksums included, wherever pkgutil accepts it. Packages are read through
// the file so their payloads stream rather than load into memory.
func TestVerifyXarAcceptsEveryXcodePackage(t *testing.T) {
	pkgutil, err := exec.LookPath("pkgutil")
	if err != nil {
		t.Skip("pkgutil is not installed")
	}
	matches, _ := filepath.Glob("/Applications/Xcode*.app/Contents/Resources/Packages/*.pkg")
	if len(matches) == 0 {
		t.Skip("no Xcode packages are installed")
	}
	policy := appleTrustPolicyForTest(t)
	for _, path := range matches {
		t.Run(filepath.Base(filepath.Dir(filepath.Dir(filepath.Dir(filepath.Dir(path)))))+"/"+filepath.Base(path), func(t *testing.T) {
			if exec.Command(pkgutil, "--check-signature", path).Run() != nil {
				t.Skip("pkgutil does not accept the package")
			}
			file, err := os.Open(path)
			if err != nil {
				t.Skipf("package unavailable: %v", err)
			}
			defer file.Close()
			info, err := file.Stat()
			if err != nil {
				t.Fatal(err)
			}
			manifest, _ := inspectPKGVerifying(file, info.Size(), policy)
			if result := manifest.SignatureVerification; result == nil || result.Status != VerificationValid || !strings.Contains(result.Detail, "file payload checksums recomputed") {
				t.Fatalf("pkgutil accepts %s but verification=%+v", path, result)
			}
		})
	}
}

// Platform binaries such as xpcproxy seal embedded launch-constraint blobs in
// special slot 8, which the presence check must accept.
func TestVerifyMachOAgreesWithCodesignOnLaunchConstrainedBinary(t *testing.T) {
	const path = "/usr/libexec/xpcproxy"
	data, err := os.ReadFile(path)
	if err != nil {
		t.Skipf("system binary unavailable: %v", err)
	}
	if !codesignAccepts(t, path) {
		t.Skip("codesign does not accept the system binary")
	}
	if result := verifyMachOBytes(data, appleTrustPolicyForTest(t)); result.Status != VerificationValid {
		t.Fatalf("codesign accepts %s but verification=%+v", path, result)
	}
}
