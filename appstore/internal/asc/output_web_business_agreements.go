package asc

import "strconv"

// WebBusinessAgreementAcceptResult reports the configuration and legal entity
// confirmed by rereading Business agreements after an acceptance request.
type WebBusinessAgreementAcceptResult struct {
	PublicProviderID string `json:"publicProviderId"`
	LegalEntityID    string `json:"legalEntityId"`
	ContractID       string `json:"contractId"`
	ContractConfigID string `json:"contractConfigId"`
	ConfigVersion    int    `json:"configVersion"`
	Status           string `json:"status"`
	Verified         bool   `json:"verified"`
}

func webBusinessAgreementAcceptRows(result *WebBusinessAgreementAcceptResult) ([]string, [][]string) {
	return []string{"Public Provider ID", "Legal Entity ID", "Contract ID", "Configuration ID", "Version", "Status", "Verified"}, [][]string{{
		result.PublicProviderID, result.LegalEntityID, result.ContractID, result.ContractConfigID,
		strconv.Itoa(result.ConfigVersion), result.Status, strconv.FormatBool(result.Verified),
	}}
}
