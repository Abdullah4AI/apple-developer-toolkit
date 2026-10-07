package asc

import "strconv"

// StatusNextCommand is a runnable asc command suggested by the status
// dashboard for the current release state.
type StatusNextCommand struct {
	Command string `json:"command"`
	Reason  string `json:"reason"`
	Mutates bool   `json:"mutates"`
}

// StatusUntilResult is the final record of asc status --until. Reached is false
// only when the wait ended before the condition reached a terminal outcome.
type StatusUntilResult struct {
	Until   string `json:"until"`
	Reached bool   `json:"reached"`
	Outcome string `json:"outcome"`
	State   string `json:"state,omitempty"`
	Polls   int    `json:"polls"`
}

func statusUntilResultRows(result *StatusUntilResult) ([]string, [][]string) {
	headers := []string{"Until", "Reached", "Outcome", "State", "Polls"}
	rows := [][]string{{
		result.Until,
		strconv.FormatBool(result.Reached),
		result.Outcome,
		result.State,
		strconv.Itoa(result.Polls),
	}}
	return headers, rows
}
