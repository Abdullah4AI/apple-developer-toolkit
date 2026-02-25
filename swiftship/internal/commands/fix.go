package commands

import (
	"github.com/Abdullah4AI/apple-developer-toolkit/swiftship/internal/service"
	"github.com/spf13/cobra"
)

var fixCmd = &cobra.Command{
	Use:   "fix",
	Short: "Auto-fix compilation errors",
	Long:  "Build the project and automatically fix any compilation errors.",
	RunE: func(cmd *cobra.Command, args []string) error {
		svc, err := loadProjectService(service.ServiceOpts{Model: ModelFlag()})
		if err != nil {
			printNoProjectFoundCreateFirst()
			return err
		}
		return svc.Fix(cmd.Context())
	},
}
