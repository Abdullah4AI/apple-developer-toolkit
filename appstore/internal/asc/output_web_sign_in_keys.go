package asc

// WebSignInKeyReceipt contains only non-secret key metadata and a local P8 path.
type WebSignInKeyReceipt struct {
	KeyID    string `json:"keyId"`
	Name     string `json:"name,omitempty"`
	BundleID string `json:"bundleId,omitempty"`
	P8Path   string `json:"p8Path"`
}

func webSignInKeyReceiptRows(r *WebSignInKeyReceipt) ([]string, [][]string) {
	headers := []string{"Key ID", "Name", "Bundle ID", "P8 Path"}
	if r == nil {
		return headers, nil
	}
	return headers, [][]string{{r.KeyID, r.Name, r.BundleID, r.P8Path}}
}
