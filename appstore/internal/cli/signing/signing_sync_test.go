package signing

import (
	"strings"
	"testing"
)

func TestSigningSyncCommandLongHelpUsesOutputDirExample(t *testing.T) {
	cmd := SigningSyncCommand()
	if !strings.Contains(cmd.LongHelp, "--output-dir ./signing") {
		t.Fatalf("expected long help to document --output-dir, got %q", cmd.LongHelp)
	}
	if strings.Contains(cmd.LongHelp, "--output ./signing") {
		t.Fatalf("expected long help to avoid --output path example, got %q", cmd.LongHelp)
	}
}

func TestSanitizeRepoURLForOutputRedactsCredentials(t *testing.T) {
	raw := "https://token:secret@example.com/org/repo.git?access_token=abc123"
	got := sanitizeRepoURLForOutput(raw)

	if strings.Contains(got, "token:secret@") || strings.Contains(got, "secret") || strings.Contains(got, "abc123") {
		t.Fatalf("expected credentials to be redacted, got %q", got)
	}
	if !strings.Contains(got, "%5BREDACTED%5D") {
		t.Fatalf("expected sanitized marker, got %q", got)
	}
}

func TestSigningCommandLongHelpUsesOutputDirForSyncPull(t *testing.T) {
	cmd := SigningCommand()
	if !strings.Contains(cmd.LongHelp, "asc signing sync pull --repo git@github.com:team/certs.git --output-dir ./signing") {
		t.Fatalf("expected top-level help to use --output-dir for sync pull, got %q", cmd.LongHelp)
	}
	if strings.Contains(cmd.LongHelp, "asc signing sync pull --repo git@github.com:team/certs.git --output ./signing") {
		t.Fatalf("expected top-level help to avoid --output for sync pull, got %q", cmd.LongHelp)
	}
}

func TestSigningSyncCommandLongHelpPullExampleOmitsUnsupportedFlags(t *testing.T) {
	cmd := SigningSyncCommand()
	if strings.Contains(cmd.LongHelp, "asc signing sync pull --bundle-id") {
		t.Fatalf("expected pull example to omit --bundle-id, got %q", cmd.LongHelp)
	}
	if strings.Contains(cmd.LongHelp, "asc signing sync pull --profile-type") {
		t.Fatalf("expected pull example to omit --profile-type, got %q", cmd.LongHelp)
	}
}
