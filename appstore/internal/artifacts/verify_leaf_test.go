package artifacts

import (
	"archive/zip"
	"bytes"
	"crypto/x509"
	"encoding/asn1"
	"testing"
	"time"

	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/artifacts/artifactstest"
)

var (
	codeSigningLeafTypes = []artifactstest.LeafType{
		artifactstest.AppleDevelopmentLeaf,
		artifactstest.AppleDistributionLeaf,
		artifactstest.IPhoneDeveloperLeaf,
		artifactstest.IPhoneDistributionLeaf,
		artifactstest.MacAppDistributionLeaf,
		artifactstest.MacDevelopmentLeaf,
		artifactstest.DeveloperIDApplicationLeaf,
		artifactstest.MacAppStoreSigningLeaf,
		artifactstest.AppleSoftwareSigningLeaf,
	}
	installerLeafTypes = []artifactstest.LeafType{
		artifactstest.DeveloperIDInstallerLeaf,
		artifactstest.MacInstallerDistributionLeaf,
		artifactstest.AppleSoftwareUpdateSigningLeaf,
	}
	untypedLeaf = artifactstest.LeafType{Name: "untyped", ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageCodeSigning}}
)

func verifyTypedMachO(t *testing.T, leafType artifactstest.LeafType) SignatureVerification {
	t.Helper()
	chain := artifactstest.NewTrustChainWithLeaf(t, leafType, validFrom, validUntil)
	ipa := verificationIPA(t, artifactstest.SignedMachO(t, sealedOptions(&chain)), zip.Deflate, nil)
	return verifyIPA(t, ipa, testPolicy(chain))
}

func verifyTypedPKG(t *testing.T, leafType artifactstest.LeafType) SignatureVerification {
	t.Helper()
	chain := artifactstest.NewTrustChainWithLeaf(t, leafType, validFrom, validUntil)
	pkg := artifactstest.SignedXar(t, map[string][]byte{"PackageInfo": []byte(`<pkg-info identifier="com.example.pkg"/>`)}, chain, time.Now())
	manifest, _ := inspectPKGVerifying(bytes.NewReader(pkg), int64(len(pkg)), testPolicy(chain))
	if manifest.SignatureVerification == nil {
		t.Fatal("verification result is missing")
	}
	return *manifest.SignatureVerification
}

func TestVerifyMachOEnforcesCodeSigningLeafType(t *testing.T) {
	for _, leafType := range codeSigningLeafTypes {
		t.Run(leafType.Name, func(t *testing.T) {
			expectVerification(t, verifyTypedMachO(t, leafType), VerificationValid, "leaf is "+withArticle(leafType.Name)+" certificate; chain to Test Root CA verified")
		})
	}
	for _, leafType := range installerLeafTypes {
		t.Run(leafType.Name, func(t *testing.T) {
			expectVerification(t, verifyTypedMachO(t, leafType), VerificationUntrustedChain, "leaf is "+withArticle(leafType.Name)+" certificate; not valid for code signing")
		})
	}
	t.Run("untyped", func(t *testing.T) {
		expectVerification(t, verifyTypedMachO(t, untypedLeaf), VerificationUntrustedChain, "leaf is not a recognized Apple certificate type; not valid for code signing")
	})
}

func TestVerifyPKGEnforcesInstallerLeafType(t *testing.T) {
	for _, leafType := range installerLeafTypes {
		t.Run(leafType.Name, func(t *testing.T) {
			expectVerification(t, verifyTypedPKG(t, leafType), VerificationValid, "leaf is "+withArticle(leafType.Name)+" certificate; chain to Test Root CA verified")
		})
	}
	for _, leafType := range codeSigningLeafTypes {
		t.Run(leafType.Name, func(t *testing.T) {
			expectVerification(t, verifyTypedPKG(t, leafType), VerificationUntrustedChain, "leaf is "+withArticle(leafType.Name)+" certificate; not valid for signing installer packages")
		})
	}
	t.Run("untyped", func(t *testing.T) {
		expectVerification(t, verifyTypedPKG(t, untypedLeaf), VerificationUntrustedChain, "leaf is not a recognized Apple certificate type; not valid for signing installer packages")
	})
}

func TestVerifyLeafTypeRequiresMatchingExtendedKeyUsage(t *testing.T) {
	withoutCodeSigning := artifactstest.DeveloperIDApplicationLeaf
	withoutCodeSigning.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageEmailProtection}
	expectVerification(t, verifyTypedMachO(t, withoutCodeSigning), VerificationUntrustedChain, "leaf is a Developer ID Application certificate without its extended key usage")

	// An installer marker paired with the code-signing EKU is not an installer leaf.
	wrongEKU := artifactstest.DeveloperIDInstallerLeaf
	wrongEKU.AppleEKU = nil
	wrongEKU.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageCodeSigning}
	expectVerification(t, verifyTypedPKG(t, wrongEKU), VerificationUntrustedChain, "leaf is a Developer ID Installer certificate without its extended key usage")

	// Go treats an absent EKU extension as any usage; Apple always sets one.
	noEKU := artifactstest.DeveloperIDApplicationLeaf
	noEKU.ExtKeyUsage = nil
	expectVerification(t, verifyTypedMachO(t, noEKU), VerificationUntrustedChain, "without its extended key usage")
}

func TestVerifyLeafTypeRejectsMixedPurposeMarkers(t *testing.T) {
	mixed := artifactstest.LeafType{
		Name:        "mixed",
		Markers:     append(append([]asn1.ObjectIdentifier(nil), artifactstest.DeveloperIDApplicationLeaf.Markers...), artifactstest.DeveloperIDInstallerLeaf.Markers...),
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageCodeSigning},
		AppleEKU:    artifactstest.DeveloperIDInstallerLeaf.AppleEKU,
	}
	expectVerification(t, verifyTypedMachO(t, mixed), VerificationUntrustedChain, "leaf carries markers for both code signing and installer packages")
	expectVerification(t, verifyTypedPKG(t, mixed), VerificationUntrustedChain, "leaf carries markers for both code signing and installer packages")
}

func TestVerifyLeafTypeOutranksExpiry(t *testing.T) {
	chain := artifactstest.NewTrustChainWithLeaf(t, artifactstest.DeveloperIDInstallerLeaf, time.Now().AddDate(-2, 0, 0), time.Now().AddDate(-1, 0, 0))
	ipa := verificationIPA(t, artifactstest.SignedMachO(t, sealedOptions(&chain)), zip.Deflate, nil)
	expectVerification(t, verifyIPA(t, ipa, testPolicy(chain)), VerificationUntrustedChain, "not valid for code signing")
}
