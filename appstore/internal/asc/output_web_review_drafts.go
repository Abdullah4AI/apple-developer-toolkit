package asc

import "fmt"

// WebReviewDraftResult is the redacted receipt for an unsent
// Resolution Center draft mutation.
type WebReviewDraftResult struct {
	AppID    string `json:"appId"`
	ThreadID string `json:"threadId"`
	DraftID  string `json:"draftId"`
	Action   string `json:"action"`
	Verified bool   `json:"verified"`
}

func webReviewDraftResultRows(result *WebReviewDraftResult) ([]string, [][]string) {
	return []string{"Section", "Field", "Value"}, [][]string{
		{"Draft", "App ID", result.AppID},
		{"Draft", "Thread ID", result.ThreadID},
		{"Draft", "Draft ID", result.DraftID},
		{"Draft", "Action", result.Action},
		{"Draft", "Verified", fmt.Sprintf("%t", result.Verified)},
	}
}
