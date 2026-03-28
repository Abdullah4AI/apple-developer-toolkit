package builds

import (
	"fmt"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/cli/shared"
)

func deprecatedBuildsGetAlias(viewCmd *ffcli.Command, canonicalPath string) *ffcli.Command {
	canonicalPath = strings.TrimSpace(canonicalPath)
	legacyPath := strings.Replace(canonicalPath, " view", " get", 1)

	legacyShortUsage := strings.TrimSpace(viewCmd.ShortUsage)
	if legacyShortUsage == "" {
		legacyShortUsage = canonicalPath + " [flags]"
	}
	legacyShortUsage = strings.Replace(legacyShortUsage, canonicalPath, legacyPath, 1)

	return shared.DeprecatedAliasLeafCommand(
		viewCmd,
		"get",
		legacyShortUsage,
		canonicalPath,
		fmt.Sprintf("Warning: `%s` is deprecated. Use `%s`.", legacyPath, canonicalPath),
	)
}
