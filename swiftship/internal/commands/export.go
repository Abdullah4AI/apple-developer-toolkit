package commands

import "github.com/spf13/cobra"

// RootCmd returns the root cobra command for embedding in a parent CLI.
func RootCmd() *cobra.Command {
	return rootCmd
}
