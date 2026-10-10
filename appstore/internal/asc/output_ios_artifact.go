package asc

import "fmt"

// IOSArtifactResult describes an explicitly selected portable iOS operation.
// Apple acceptance and complete native signature verification are separate gates.
type IOSArtifactResult struct {
	Operation         string `json:"operation"`
	Backend           string `json:"backend"`
	AppPath           string `json:"appPath,omitempty"`
	IPAPath           string `json:"ipaPath,omitempty"`
	BundleID          string `json:"bundleId,omitempty"`
	Platform          string `json:"platform"`
	Configuration     string `json:"configuration,omitempty"`
	SigningType       string `json:"signingType"`
	ProfileValidated  bool   `json:"profileValidated"`
	SignatureVerified bool   `json:"signatureVerified"`
	AppleAcceptance   string `json:"appleAcceptance"`
	SHA256            string `json:"sha256,omitempty"`
	Success           bool   `json:"success"`
	DurationMs        int64  `json:"durationMs"`
}

func iosArtifactResultRows(r *IOSArtifactResult) ([]string, [][]string) {
	return []string{"field", "value"}, [][]string{
		{"operation", r.Operation},
		{"backend", r.Backend},
		{"app_path", r.AppPath},
		{"ipa_path", r.IPAPath},
		{"bundle_id", r.BundleID},
		{"platform", r.Platform},
		{"configuration", r.Configuration},
		{"signing_type", r.SigningType},
		{"profile_validated", fmt.Sprint(r.ProfileValidated)},
		{"signature_verified", fmt.Sprint(r.SignatureVerified)},
		{"apple_acceptance", r.AppleAcceptance},
		{"sha256", r.SHA256},
		{"success", fmt.Sprint(r.Success)},
		{"duration_ms", fmt.Sprint(r.DurationMs)},
	}
}
