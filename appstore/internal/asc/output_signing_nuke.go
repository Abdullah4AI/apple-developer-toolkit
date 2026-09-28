package asc

import "strings"

// Signing sync nuke publication states.
const (
	SigningSyncNukePublicationSkipped   = "skipped"
	SigningSyncNukePublicationNotNeeded = "not-needed"
	SigningSyncNukePublicationSucceeded = "succeeded"
	SigningSyncNukePublicationFailed    = "failed"
)

// SigningSyncNukeResult is the receipt for signing sync nuke. Planned lists
// every resource and encrypted file selected before any mutation. Deleted,
// Revoked, and Removed list only the operations that completed; Failed lists
// the App Store Connect operations that returned an error. With DryRun,
// nothing is deleted, revoked, removed, or published.
type SigningSyncNukeResult struct {
	Operation        string                      `json:"operation"`
	RepoURL          string                      `json:"repoUrl"`
	ProfileType      string                      `json:"profileType"`
	CertificateTypes []string                    `json:"certificateTypes"`
	DryRun           bool                        `json:"dryRun"`
	Profiles         SigningSyncNukeProfiles     `json:"profiles"`
	Certificates     SigningSyncNukeCertificates `json:"certificates"`
	RepositoryFiles  SigningSyncNukeFiles        `json:"repositoryFiles"`
	PublicationState string                      `json:"publicationState"`
	Partial          bool                        `json:"partial,omitempty"`
}

// SigningSyncNukeProfiles reports the profiles selected and deleted by nuke.
type SigningSyncNukeProfiles struct {
	Planned []SigningSyncNukeResource `json:"planned"`
	Deleted []SigningSyncNukeResource `json:"deleted"`
	Failed  []SigningSyncNukeFailure  `json:"failed,omitempty"`
}

// SigningSyncNukeCertificates reports the certificates selected and revoked
// by nuke.
type SigningSyncNukeCertificates struct {
	Planned []SigningSyncNukeResource `json:"planned"`
	Revoked []SigningSyncNukeResource `json:"revoked"`
	Failed  []SigningSyncNukeFailure  `json:"failed,omitempty"`
}

// SigningSyncNukeResource is one App Store Connect profile or certificate.
type SigningSyncNukeResource struct {
	ID             string `json:"id"`
	Name           string `json:"name,omitempty"`
	Type           string `json:"type"`
	SerialNumber   string `json:"serialNumber,omitempty"`
	ExpirationDate string `json:"expirationDate,omitempty"`
}

// SigningSyncNukeFailure is one profile deletion or certificate revocation
// that App Store Connect rejected.
type SigningSyncNukeFailure struct {
	ID    string `json:"id"`
	Name  string `json:"name,omitempty"`
	Error string `json:"error"`
}

// SigningSyncNukeFiles reports encrypted repository artifacts. Kept lists
// planned artifacts retained because their App Store Connect resource was
// not deleted or revoked.
type SigningSyncNukeFiles struct {
	Planned []string `json:"planned"`
	Removed []string `json:"removed"`
	Kept    []string `json:"kept,omitempty"`
}

func signingSyncNukeRows(result *SigningSyncNukeResult) ([]string, [][]string) {
	headers := []string{"Kind", "ID", "Name", "Type", "Status"}
	if result == nil {
		return headers, nil
	}
	rows := make([][]string, 0)
	profileStatus := signingSyncNukeStatuses(result.DryRun, "deleted", result.Profiles.Deleted, result.Profiles.Failed)
	for _, profile := range result.Profiles.Planned {
		rows = append(rows, []string{"profile", profile.ID, profile.Name, profile.Type, profileStatus(profile.ID)})
	}
	certificateStatus := signingSyncNukeStatuses(result.DryRun, "revoked", result.Certificates.Revoked, result.Certificates.Failed)
	for _, certificate := range result.Certificates.Planned {
		name := certificate.Name
		if name == "" {
			name = certificate.SerialNumber
		}
		rows = append(rows, []string{"certificate", certificate.ID, name, certificate.Type, certificateStatus(certificate.ID)})
	}
	removed := make(map[string]struct{}, len(result.RepositoryFiles.Removed))
	for _, path := range result.RepositoryFiles.Removed {
		removed[path] = struct{}{}
	}
	for _, path := range result.RepositoryFiles.Planned {
		status := "kept"
		switch {
		case result.DryRun:
			status = "planned"
		case hasSigningSyncNukeKey(removed, path):
			status = "removed"
		}
		rows = append(rows, []string{"file", path, "", "", status})
	}
	return headers, rows
}

func signingSyncNukeStatuses(dryRun bool, done string, completed []SigningSyncNukeResource, failed []SigningSyncNukeFailure) func(string) string {
	completedIDs := make(map[string]struct{}, len(completed))
	for _, item := range completed {
		completedIDs[item.ID] = struct{}{}
	}
	failures := make(map[string]string, len(failed))
	for _, item := range failed {
		failures[item.ID] = item.Error
	}
	return func(id string) string {
		switch {
		case dryRun:
			return "planned"
		case hasSigningSyncNukeKey(completedIDs, id):
			return done
		case failures[id] != "":
			return "failed: " + strings.TrimSpace(failures[id])
		default:
			return "not attempted"
		}
	}
}

func hasSigningSyncNukeKey(set map[string]struct{}, key string) bool {
	_, ok := set[key]
	return ok
}
