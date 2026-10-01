package asc

import "fmt"

// WebICloudContainerCreateResult is the receipt for a verified Developer
// Portal iCloud container create. iCloud containers cannot be deleted, so
// Permanent is always true.
type WebICloudContainerCreateResult struct {
	Operation     string `json:"operation"`
	ContainerID   string `json:"containerId"`
	Identifier    string `json:"identifier"`
	Name          string `json:"name"`
	RequestedName string `json:"requestedName,omitempty"`
	Prefix        string `json:"prefix,omitempty"`
	Hidden        bool   `json:"hidden"`
	Changed       bool   `json:"changed"`
	Verified      bool   `json:"verified"`
	Permanent     bool   `json:"permanent"`
	Status        string `json:"status"`
}

func webICloudContainerCreateRows(result *WebICloudContainerCreateResult) ([]string, [][]string) {
	headers := []string{"Operation", "Container ID", "Identifier", "Name", "Prefix", "Hidden", "Changed", "Verified", "Permanent", "Status"}
	if result == nil {
		return headers, nil
	}
	return headers, [][]string{{
		result.Operation,
		result.ContainerID,
		result.Identifier,
		result.Name,
		result.Prefix,
		fmt.Sprintf("%t", result.Hidden),
		fmt.Sprintf("%t", result.Changed),
		fmt.Sprintf("%t", result.Verified),
		fmt.Sprintf("%t", result.Permanent),
		result.Status,
	}}
}
