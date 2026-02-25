// Package commands re-exports the public surface of swiftship/internal/commands.
package commands

import (
	inner "github.com/Abdullah4AI/apple-developer-toolkit/swiftship/internal/commands"
	"github.com/spf13/cobra"
)

// Execute runs the swiftship root command.
func Execute() error { return inner.Execute() }

// RootCmd returns the cobra root command for embedding in a parent CLI.
func RootCmd() *cobra.Command { return inner.RootCmd() }
