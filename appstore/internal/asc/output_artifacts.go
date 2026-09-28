package asc

import "strings"

// ArtifactIPAInfo is the offline IPA inspection receipt.
type ArtifactIPAInfo struct {
	SignatureVerification string                  `json:"signatureVerification"`
	Path                  string                  `json:"path"`
	BundleID              string                  `json:"bundleId,omitempty"`
	Name                  string                  `json:"name,omitempty"`
	Version               string                  `json:"version,omitempty"`
	BuildNumber           string                  `json:"buildNumber,omitempty"`
	MinimumOSVersion      string                  `json:"minimumOSVersion,omitempty"`
	Platforms             []string                `json:"platforms,omitempty"`
	TeamID                string                  `json:"teamId,omitempty"`
	SignerCommonName      string                  `json:"signerCommonName,omitempty"`
	Status                string                  `json:"status"`
	NestedBundles         []ArtifactNestedBundle  `json:"nestedBundles"`
	Entitlements          map[string]any          `json:"entitlements,omitempty"`
	Profile               *ArtifactProfileSummary `json:"profile,omitempty"`
	CodeSignature         string                  `json:"codeSignature,omitempty"`
	Signer                *ArtifactSigner         `json:"signer"`
	Architectures         []ArtifactArchitecture  `json:"architectures,omitempty"`
	SignerConsistent      *bool                   `json:"signerConsistent,omitempty"`
}

// ArtifactArchitecture is the code signature of one main executable slice.
// cpuType and cpuSubtype are the raw Mach-O values; arch is the lipo name.
type ArtifactArchitecture struct {
	CPUType       uint32          `json:"cpuType"`
	CPUSubtype    uint32          `json:"cpuSubtype"`
	Arch          string          `json:"arch"`
	CodeSignature string          `json:"codeSignature"`
	Signer        *ArtifactSigner `json:"signer"`
}

// ArtifactSigner is the leaf certificate read from an artifact signature. It is
// not validated against a trust store.
type ArtifactSigner struct {
	CommonName        string `json:"commonName,omitempty"`
	TeamID            string `json:"teamId,omitempty"`
	Organization      string `json:"organization,omitempty"`
	IssuerCommonName  string `json:"issuerCommonName,omitempty"`
	SerialNumber      string `json:"serialNumber,omitempty"`
	NotBefore         string `json:"notBefore,omitempty"`
	NotAfter          string `json:"notAfter,omitempty"`
	SHA1Fingerprint   string `json:"sha1Fingerprint,omitempty"`
	SHA256Fingerprint string `json:"sha256Fingerprint,omitempty"`
}

// ArtifactNestedBundle is an extension or App Clip found in an IPA.
type ArtifactNestedBundle struct {
	BundleID string `json:"bundleId,omitempty"`
	Name     string `json:"name,omitempty"`
	Path     string `json:"path"`
}

// ArtifactProfileSummary is the embedded provisioning profile summary.
type ArtifactProfileSummary struct {
	Name           string `json:"name,omitempty"`
	UUID           string `json:"uuid,omitempty"`
	ExpirationDate string `json:"expirationDate,omitempty"`
	ProfileType    string `json:"profileType,omitempty"`
}

// ArtifactPKGInfo is the offline flat package inspection receipt.
type ArtifactPKGInfo struct {
	SignatureVerification string          `json:"signatureVerification"`
	Path                  string          `json:"path"`
	ProductID             string          `json:"productId,omitempty"`
	Version               string          `json:"version,omitempty"`
	InstallLocation       string          `json:"installLocation,omitempty"`
	BundleIDs             []string        `json:"bundleIds,omitempty"`
	SignerCommonName      string          `json:"signerCommonName,omitempty"`
	TeamID                string          `json:"teamId,omitempty"`
	Status                string          `json:"status"`
	PackageSignature      string          `json:"packageSignature,omitempty"`
	Signer                *ArtifactSigner `json:"signer"`
}

func artifactIPAInfoRows(result *ArtifactIPAInfo) ([]string, [][]string) {
	headers := []string{"Bundle ID", "Version", "Build", "Status", "Signer", "Team ID", "Signature", "Architectures"}
	if result == nil {
		return headers, nil
	}
	architectures := make([]string, 0, len(result.Architectures))
	for _, architecture := range result.Architectures {
		architectures = append(architectures, architecture.Arch)
	}
	return headers, [][]string{{result.BundleID, result.Version, result.BuildNumber, result.Status, artifactSignerLabel(result.SignerCommonName, result.CodeSignature), result.TeamID, result.SignatureVerification, strings.Join(architectures, ", ")}}
}

func artifactPKGInfoRows(result *ArtifactPKGInfo) ([]string, [][]string) {
	headers := []string{"Product ID", "Version", "Install Location", "Status", "Signer", "Team ID", "Signature"}
	if result == nil {
		return headers, nil
	}
	return headers, [][]string{{result.ProductID, result.Version, result.InstallLocation, result.Status, artifactSignerLabel(result.SignerCommonName, result.PackageSignature), result.TeamID, result.SignatureVerification}}
}

// artifactSignerLabel shows the signer, or the signature classification when
// no certificate identifies one.
func artifactSignerLabel(commonName, signature string) string {
	if commonName != "" || signature == "" {
		return commonName
	}
	return "(" + signature + ")"
}
