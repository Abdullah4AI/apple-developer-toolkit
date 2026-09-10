package web

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/cli/shared"
	webcore "github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/web"
)

const developerTeamFlagUsage = "Developer Portal team ID (or exact team name) to use; required when the Apple Account belongs to multiple Developer Portal teams and none matches the selected App Store Connect provider"

type developerPortalFlags struct {
	fs            *flag.FlagSet
	developerTeam *string
}

func bindDeveloperPortalFlags(fs *flag.FlagSet) developerPortalFlags {
	return developerPortalFlags{
		fs:            fs,
		developerTeam: fs.String("developer-team", "", developerTeamFlagUsage),
	}
}

func (flags developerPortalFlags) developerTeamWasSet() bool {
	if flags.fs == nil {
		return false
	}
	set := false
	flags.fs.Visit(func(flag *flag.Flag) {
		if flag.Name == "developer-team" {
			set = true
		}
	})
	return set
}

func validateDeveloperPortalFlags(flags developerPortalFlags) error {
	if !flags.developerTeamWasSet() {
		return nil
	}
	if flags.developerTeam == nil || strings.TrimSpace(*flags.developerTeam) == "" {
		return shared.UsageError("--developer-team must be a Developer Portal team ID or exact team name")
	}
	return nil
}

func newDeveloperPortalClient(session *webcore.AuthSession, flags developerPortalFlags) *webcore.Client {
	client := newWebClientFn(session)
	if client == nil {
		return nil
	}
	if flags.developerTeamWasSet() {
		client.SetDeveloperTeamSelector(strings.TrimSpace(*flags.developerTeam))
	}
	return client
}

func persistDeveloperPortalSession(session *webcore.AuthSession) {
	if err := persistWebSessionFn(session); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Warning: failed to persist refreshed web session: %v\n", err)
	}
}
