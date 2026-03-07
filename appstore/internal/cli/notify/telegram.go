package notify

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/Abdullah4AI/apple-developer-toolkit/internal/hooks"
)

const (
	defaultChatID    = "1835854"
	defaultParseMode = "Markdown"
)

// TelegramCommand returns the ffcli command for sending Telegram notifications.
func TelegramCommand() *ffcli.Command {
	fs := flag.NewFlagSet("notify telegram", flag.ExitOnError)

	message := fs.String("message", "", "Message to send (required)")
	chatID := fs.String("chat-id", "", "Telegram chat ID (default: from config or "+defaultChatID+")")
	botToken := fs.String("bot-token", "", "Bot token (falls back to env/keychain)")
	silent := fs.Bool("silent", false, "Send with notifications disabled")
	parseMode := fs.String("parse-mode", defaultParseMode, "Parse mode: Markdown or HTML")

	return &ffcli.Command{
		Name:       "telegram",
		ShortUsage: "asc notify telegram --message TEXT [--chat-id ID] [--bot-token TOKEN]",
		ShortHelp:  "Send a message via Telegram bot.",
		LongHelp: `Send a message to Telegram via the Bot API.

The bot token is resolved in order:
  1. --bot-token flag
  2. Environment variable (from hooks.yaml config)
  3. macOS Keychain (from hooks.yaml config)

Examples:
  asc notify telegram --message "Build uploaded"
  asc notify telegram --message "Deploy done" --chat-id 12345
  asc notify telegram --message "Alert" --silent`,
		FlagSet: fs,
		Exec: func(ctx context.Context, args []string) error {
			msg := strings.TrimSpace(*message)
			if msg == "" {
				fmt.Fprintln(os.Stderr, "Error: --message is required")
				return flag.ErrHelp
			}

			// Resolve bot token
			token := strings.TrimSpace(*botToken)
			if token == "" {
				// Try from hooks config
				cfg, _ := hooks.LoadConfig()
				if cfg != nil {
					if nc, ok := cfg.Notifiers["telegram"]; ok {
						var err error
						token, err = hooks.ResolveBotToken(nc.BotTokenEnv, nc.BotTokenKeychain)
						if err != nil {
							fmt.Fprintf(os.Stderr, "Error resolving bot token: %v\n", err)
							return err
						}
					}
				}
				if token == "" {
					// Final fallback: try common env vars
					token = os.Getenv("TELEGRAM_BOT_TOKEN")
					if token == "" {
						fmt.Fprintln(os.Stderr, "Error: bot token not found. Use --bot-token, set TELEGRAM_BOT_TOKEN, or configure hooks.yaml")
						return flag.ErrHelp
					}
				}
			}

			// Resolve chat ID
			cid := strings.TrimSpace(*chatID)
			if cid == "" {
				// Try from hooks config
				cfg, _ := hooks.LoadConfig()
				if cfg != nil {
					if nc, ok := cfg.Notifiers["telegram"]; ok && nc.ChatID != "" {
						cid = nc.ChatID
					}
				}
				if cid == "" {
					cid = defaultChatID
				}
			}

			_ = *parseMode // reserved for future use with NotifyTelegram

			if err := hooks.NotifyTelegram(ctx, token, cid, msg, *silent); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				return err
			}

			fmt.Fprintln(os.Stderr, "Message sent to Telegram successfully")
			return nil
		},
	}
}
