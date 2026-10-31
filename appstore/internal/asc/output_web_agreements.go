package asc

import (
	"fmt"
	"strings"
)

// WebAgreementContractMessage is an App Store Connect alert banner entry.
type WebAgreementContractMessage struct {
	ID      string `json:"id"`
	Group   string `json:"group"`
	Subject string `json:"subject"`
	Message string `json:"message"`
}

// WebAgreement summarizes one Apple Developer Program agreement record.
type WebAgreement struct {
	AgreementID               string `json:"agreementId"`
	Title                     string `json:"title"`
	Status                    string `json:"status"`
	Version                   string `json:"version"`
	IsProgramLicenseAgreement bool   `json:"isProgramLicenseAgreement"`
	Pending                   bool   `json:"pending"`
	DateEffective             string `json:"dateEffective,omitempty"`
	DateAccepted              string `json:"dateAccepted,omitempty"`
	DateAgreeBy               string `json:"dateAgreeBy,omitempty"`
	DownloadURL               string `json:"downloadUrl,omitempty"`

	ProgramLicenseClassificationUnknown bool `json:"programLicenseClassificationUnknown,omitempty"`
}

// WebAgreementsStatusResult reports agreement state for a web session team.
type WebAgreementsStatusResult struct {
	TeamID              string                        `json:"teamId"`
	Pending             bool                          `json:"pending"`
	PendingAgreementIDs []string                      `json:"pendingAgreementIds"`
	Warnings            []string                      `json:"warnings"`
	ContractMessages    []WebAgreementContractMessage `json:"contractMessages"`
	Agreements          []WebAgreement                `json:"agreements"`
}

// WebAgreementsAcceptResult summarizes an agreement acceptance. When Verified
// is true, Agreements reflects the agreement history re-read after the write
// rather than the acceptAgreements response.
type WebAgreementsAcceptResult struct {
	TeamID       string         `json:"teamId"`
	AgreementIDs []string       `json:"agreementIds"`
	Status       string         `json:"status"`
	Verified     bool           `json:"verified"`
	Agreements   []WebAgreement `json:"agreements"`
}

// WebAgreementDownloadResult is the receipt for a saved agreement download. It
// intentionally omits the (possibly signed) download URL.
type WebAgreementDownloadResult struct {
	AgreementID  string `json:"agreementId"`
	TeamID       string `json:"teamId"`
	Title        string `json:"title,omitempty"`
	Version      string `json:"version,omitempty"`
	Path         string `json:"path"`
	BytesWritten int64  `json:"bytesWritten"`
	ContentType  string `json:"contentType,omitempty"`
}

func webAgreementDownloadRows(result *WebAgreementDownloadResult) ([]string, [][]string) {
	return []string{"Agreement ID", "Team ID", "Title", "Version", "Path", "Bytes", "Content Type"}, [][]string{{
		result.AgreementID,
		result.TeamID,
		result.Title,
		result.Version,
		result.Path,
		fmt.Sprintf("%d", result.BytesWritten),
		result.ContentType,
	}}
}

func webAgreementsStatusTables(result *WebAgreementsStatusResult, render func([]string, [][]string)) error {
	pendingIDs := strings.Join(result.PendingAgreementIDs, ", ")
	if pendingIDs == "" {
		pendingIDs = "none"
	}
	render(
		[]string{"Team ID", "Pending or alert", "Pending Agreement IDs", "Contract Messages", "Agreements"},
		[][]string{{
			result.TeamID,
			fmt.Sprintf("%t", result.Pending),
			pendingIDs,
			fmt.Sprintf("%d", len(result.ContractMessages)),
			fmt.Sprintf("%d", len(result.Agreements)),
		}},
	)

	for _, warning := range result.Warnings {
		render([]string{"Warning"}, [][]string{{warning}})
	}
	if len(result.ContractMessages) > 0 {
		rows := make([][]string, 0, len(result.ContractMessages))
		for _, message := range result.ContractMessages {
			rows = append(rows, []string{message.ID, message.Group, message.Subject, message.Message})
		}
		render([]string{"Message ID", "Group", "Subject", "Message"}, rows)
	}

	if len(result.Agreements) > 0 {
		render(webAgreementsHeaders(), webAgreementsRows(result.Agreements))
	}
	return nil
}

func webAgreementsHeaders() []string {
	return []string{"Agreement ID", "Title", "Version", "Status", "Pending", "Accept By"}
}

func webAgreementsRows(agreements []WebAgreement) [][]string {
	rows := make([][]string, 0, len(agreements))
	for _, agreement := range agreements {
		rows = append(rows, []string{
			agreement.AgreementID,
			agreement.Title,
			agreement.Version,
			agreement.Status,
			fmt.Sprintf("%t", agreement.Pending),
			agreement.DateAgreeBy,
		})
	}
	return rows
}

// webAgreementsAcceptRows renders one row per requested agreement so each
// acceptance timestamp is attributed to its own agreement.
func webAgreementsAcceptRows(result *WebAgreementsAcceptResult) ([]string, [][]string) {
	byID := make(map[string]WebAgreement, len(result.Agreements))
	for _, agreement := range result.Agreements {
		byID[agreement.AgreementID] = agreement
	}
	ids := result.AgreementIDs
	if len(ids) == 0 {
		for _, agreement := range result.Agreements {
			ids = append(ids, agreement.AgreementID)
		}
	}
	rows := make([][]string, 0, len(ids))
	for _, id := range ids {
		rows = append(rows, []string{
			result.TeamID,
			id,
			result.Status,
			fmt.Sprintf("%t", result.Verified),
			byID[id].DateAccepted,
		})
	}
	return []string{"Team ID", "Agreement ID", "Status", "Verified", "Accepted At"}, rows
}
