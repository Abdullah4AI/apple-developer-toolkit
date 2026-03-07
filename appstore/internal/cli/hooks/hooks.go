package hooks

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/Abdullah4AI/apple-developer-toolkit/internal/hooks"
)

// HooksCommand returns the top-level hooks ffcli command.
func HooksCommand() *ffcli.Command {
	return &ffcli.Command{
		Name:       "hooks",
		ShortUsage: "hooks <subcommand> [flags]",
		ShortHelp:  "Manage lifecycle hooks.",
		LongHelp: `Manage lifecycle hooks for build and store events.

Hooks are configured in ~/.appledev/hooks.yaml (global) and .appledev/hooks.yaml (project).
Project hooks extend and can override global hooks.

Examples:
  hooks init --template indie
  hooks list
  hooks fire build.done STATUS=success APP_NAME=MyApp
  hooks validate`,
		Subcommands: []*ffcli.Command{
			initCommand(),
			listCommand(),
			fireCommand(),
			validateCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

func initCommand() *ffcli.Command {
	fs := flag.NewFlagSet("hooks init", flag.ExitOnError)
	templateName := fs.String("template", "indie", "Template to use: indie, team, ci")
	project := fs.Bool("project", false, "Create project-local config (.appledev/hooks.yaml) instead of global")

	return &ffcli.Command{
		Name:       "init",
		ShortUsage: "hooks init [--template indie|team|ci] [--project]",
		ShortHelp:  "Initialize hooks configuration from a template.",
		FlagSet:    fs,
		Exec: func(ctx context.Context, args []string) error {
			content, err := hooks.GetTemplate(*templateName)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				return err
			}

			var configPath string
			if *project {
				configPath = filepath.Join(".appledev", "hooks.yaml")
			} else {
				home, err := os.UserHomeDir()
				if err != nil {
					return fmt.Errorf("cannot determine home directory: %w", err)
				}
				configPath = filepath.Join(home, ".appledev", "hooks.yaml")
			}

			// Create parent directories
			dir := filepath.Dir(configPath)
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return fmt.Errorf("create directory %s: %w", dir, err)
			}

			// Create hook-logs dir
			home, _ := os.UserHomeDir()
			logDir := filepath.Join(home, ".appledev", "hook-logs")
			_ = os.MkdirAll(logDir, 0o755)

			// Check if file exists
			if _, err := os.Stat(configPath); err == nil {
				fmt.Fprintf(os.Stderr, "Config already exists: %s\nUse a text editor to modify it.\n", configPath)
				return nil
			}

			if err := os.WriteFile(configPath, content, 0o644); err != nil {
				return fmt.Errorf("write config: %w", err)
			}

			fmt.Fprintf(os.Stderr, "Created %s (template: %s)\n", configPath, *templateName)
			fmt.Fprintf(os.Stderr, "Edit the file to configure your notifiers and hooks.\n")
			return nil
		},
	}
}

func listCommand() *ffcli.Command {
	fs := flag.NewFlagSet("hooks list", flag.ExitOnError)
	eventPattern := fs.String("event", "", "Filter by event name (glob pattern)")

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "hooks list [--event <pattern>]",
		ShortHelp:  "List configured hooks grouped by event.",
		FlagSet:    fs,
		Exec: func(ctx context.Context, args []string) error {
			hooks.ResetConfigCache()
			cfg, err := hooks.LoadConfig()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				return err
			}
			if cfg == nil {
				fmt.Fprintln(os.Stderr, "No hooks configured. Run 'hooks init' to get started.")
				return nil
			}

			// Sort events for consistent output
			events := make([]string, 0, len(cfg.Hooks))
			for event := range cfg.Hooks {
				if *eventPattern != "" {
					matched, _ := filepath.Match(*eventPattern, event)
					if !matched {
						continue
					}
				}
				events = append(events, event)
			}
			sort.Strings(events)

			if len(events) == 0 {
				fmt.Fprintln(os.Stderr, "No hooks found matching the filter.")
				return nil
			}

			for _, event := range events {
				fmt.Fprintf(os.Stderr, "\n%s:\n", event)
				for _, h := range cfg.Hooks[event] {
					when := h.When
					if when == "" {
						when = "always"
					}
					if h.Run != "" {
						fmt.Fprintf(os.Stderr, "  - %s (run, when=%s): %s\n", h.Name, when, h.Run)
					} else if h.Notify != "" {
						fmt.Fprintf(os.Stderr, "  - %s (notify=%s, when=%s): %s\n", h.Name, h.Notify, when, h.Template)
					}
				}
			}
			fmt.Fprintln(os.Stderr)

			// Summary
			totalHooks := 0
			for _, event := range events {
				totalHooks += len(cfg.Hooks[event])
			}
			fmt.Fprintf(os.Stderr, "%d hooks across %d events\n", totalHooks, len(events))

			return nil
		},
	}
}

func fireCommand() *ffcli.Command {
	fs := flag.NewFlagSet("hooks fire", flag.ExitOnError)
	dryRun := fs.Bool("dry-run", false, "Show what would be executed without running")

	return &ffcli.Command{
		Name:       "fire",
		ShortUsage: "hooks fire <event> [KEY=VALUE...]",
		ShortHelp:  "Manually fire a hook event.",
		LongHelp: `Manually fire a hook event with optional variables.

Examples:
  hooks fire build.done STATUS=success APP_NAME=MyApp DURATION_SEC=42
  hooks fire --dry-run store.upload.done STATUS=success`,
		FlagSet: fs,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) == 0 {
				fmt.Fprintln(os.Stderr, "Error: event name is required")
				return flag.ErrHelp
			}

			eventName := args[0]
			vars := parseVars(args[1:])

			hooks.ResetConfigCache()
			if *dryRun {
				ctx = hooks.DryRunContext(ctx)
			}

			if err := hooks.Fire(ctx, eventName, vars); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				return err
			}

			if !*dryRun {
				fmt.Fprintf(os.Stderr, "Event %s fired successfully\n", eventName)
			}
			return nil
		},
	}
}

func validateCommand() *ffcli.Command {
	return &ffcli.Command{
		Name:       "validate",
		ShortUsage: "hooks validate",
		ShortHelp:  "Validate hooks configuration.",
		Exec: func(ctx context.Context, args []string) error {
			hooks.ResetConfigCache()

			// Check global config
			home, _ := os.UserHomeDir()
			globalPath := filepath.Join(home, ".appledev", "hooks.yaml")
			projectPath := filepath.Join(".appledev", "hooks.yaml")

			foundAny := false
			hasErrors := false

			for _, path := range []string{globalPath, projectPath} {
				if _, err := os.Stat(path); os.IsNotExist(err) {
					continue
				}
				foundAny = true

				cfg, err := hooks.LoadConfigFromPath(path)
				if err != nil {
					fmt.Fprintf(os.Stderr, "ERROR %s: %v\n", path, err)
					hasErrors = true
					continue
				}

				fmt.Fprintf(os.Stderr, "OK %s (version %d)\n", path, cfg.Version)

				// Check notifiers
				for name, nc := range cfg.Notifiers {
					if !nc.Enabled {
						fmt.Fprintf(os.Stderr, "  notifier %s: disabled\n", name)
						continue
					}
					switch name {
					case "telegram":
						_, err := hooks.ResolveBotToken(nc.BotTokenEnv, nc.BotTokenKeychain)
						if err != nil {
							fmt.Fprintf(os.Stderr, "  WARN notifier %s: %v\n", name, err)
						} else {
							fmt.Fprintf(os.Stderr, "  notifier %s: OK (token resolved)\n", name)
						}
						if nc.ChatID == "" {
							fmt.Fprintf(os.Stderr, "  WARN notifier %s: chat_id not set\n", name)
						}
					case "slack":
						_, err := hooks.ResolveWebhookURL(nc.WebhookURLEnv)
						if err != nil {
							fmt.Fprintf(os.Stderr, "  WARN notifier %s: %v\n", name, err)
						} else {
							fmt.Fprintf(os.Stderr, "  notifier %s: OK (webhook resolved)\n", name)
						}
					}
				}

				// Check hooks
				for event, hks := range cfg.Hooks {
					for _, h := range hks {
						if h.Run == "" && h.Notify == "" {
							fmt.Fprintf(os.Stderr, "  ERROR hook %s/%s: no 'run' or 'notify' defined\n", event, h.Name)
							hasErrors = true
						}
						if h.Notify != "" {
							if _, ok := cfg.Notifiers[h.Notify]; !ok {
								fmt.Fprintf(os.Stderr, "  ERROR hook %s/%s: references undefined notifier %q\n", event, h.Name, h.Notify)
								hasErrors = true
							}
						}
						if h.When != "" && h.When != "always" && h.When != "success" && h.When != "failure" {
							fmt.Fprintf(os.Stderr, "  ERROR hook %s/%s: invalid 'when' value %q (use: always, success, failure)\n", event, h.Name, h.When)
							hasErrors = true
						}
					}
				}
			}

			if !foundAny {
				fmt.Fprintln(os.Stderr, "No hooks configuration found. Run 'hooks init' to get started.")
				return nil
			}

			if hasErrors {
				return fmt.Errorf("validation found errors")
			}
			fmt.Fprintln(os.Stderr, "\nValidation passed")
			return nil
		},
	}
}

// parseVars parses KEY=VALUE pairs from command arguments.
func parseVars(args []string) map[string]string {
	vars := make(map[string]string)
	for _, arg := range args {
		if idx := strings.IndexByte(arg, '='); idx > 0 {
			vars[arg[:idx]] = arg[idx+1:]
		}
	}
	return vars
}
