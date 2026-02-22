package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	swiftshipcmds "github.com/Abdullah4AI/swiftship/pkg/commands"
	appstorecmd "github.com/Abdullah4AI/appstore/cmd"
)

var (
	version = "1.0.0"
	commit  = "dev"
	date    = "unknown"
)

func versionString() string {
	return fmt.Sprintf("appledev %s (commit: %s, date: %s)", version, commit, date)
}

func main() {
	// Multi-call binary support: check how we were invoked
	invoked := filepath.Base(os.Args[0])

	switch invoked {
	case "swiftship":
		// Direct swiftship mode
		if err := swiftshipcmds.Execute(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		return
	case "appstore":
		// Direct appstore mode
		os.Exit(appstorecmd.Run(os.Args[1:], versionString()))
		return
	}

	// Unified mode: appledev
	rootCmd := &cobra.Command{
		Use:   "appledev",
		Short: "Apple Developer Toolkit - unified CLI for iOS development",
		Long:  "A unified command-line toolkit combining SwiftShip (app builder) and App Store Connect CLI.",
		Version: version,
		CompletionOptions: cobra.CompletionOptions{
			DisableDefaultCmd: true,
		},
	}

	// Add swiftship as "build" subcommand
	buildCmd := swiftshipcmds.RootCmd()
	buildCmd.Use = "build"
	buildCmd.Short = "SwiftShip - Autonomous Apple platform app builder"
	buildCmd.Long = "Build, edit, and fix Apple platform apps using Claude Code as the AI backend."
	buildCmd.Aliases = []string{"b"}
	rootCmd.AddCommand(buildCmd)

	// Add appstore as "store" subcommand (passthrough to ffcli)
	storeCmd := &cobra.Command{
		Use:                "store",
		Short:              "App Store Connect CLI",
		Long:               "A fast, lightweight CLI for App Store Connect. Built for developers and AI agents.",
		Aliases:            []string{"s"},
		DisableFlagParsing: true,
		RunE: func(c *cobra.Command, args []string) error {
			// Check if user just wants help
			if len(args) == 0 {
				// Show appstore's own help
				exitCode := appstorecmd.Run([]string{}, versionString())
				if exitCode != 0 {
					os.Exit(exitCode)
				}
				return nil
			}
			exitCode := appstorecmd.Run(args, versionString())
			if exitCode != 0 {
				os.Exit(exitCode)
			}
			return nil
		},
	}
	rootCmd.AddCommand(storeCmd)

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
