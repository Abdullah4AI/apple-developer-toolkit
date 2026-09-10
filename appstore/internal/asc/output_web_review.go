package asc

import "fmt"

// WebReviewReplyResult is the redacted receipt for a
// Resolution Center reply sent through an authenticated web session.
type WebReviewReplyResult struct {
	ThreadID  string `json:"threadId"`
	DraftID   string `json:"draftId"`
	MessageID string `json:"messageId"`
	Verified  bool   `json:"verified"`
}

func webReviewReplyResultRows(result *WebReviewReplyResult) ([]string, [][]string) {
	return []string{"Section", "Field", "Value"}, [][]string{
		{"Reply", "Thread ID", result.ThreadID},
		{"Reply", "Draft ID", result.DraftID},
		{"Reply", "Message ID", result.MessageID},
		{"Reply", "Verified", fmt.Sprintf("%t", result.Verified)},
	}
}
