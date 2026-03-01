package web

import (
	"context"
	"errors"
	"flag"
	"fmt"

	webcore "github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/web"
)

type webSessionFlags struct {
	appleID       *string
	twoFactorCode *string
}

func bindWebSessionFlags(fs *flag.FlagSet) webSessionFlags {
	return webSessionFlags{
		appleID:       fs.String("apple-id", "", "Apple ID email used to scope a user-owned session cache (optional when a cached session exists)"),
		twoFactorCode: fs.String("two-factor-code", "", "2FA code if your account requires verification"),
	}
}

func resolveWebSessionForCommand(ctx context.Context, flags webSessionFlags) (*webcore.AuthSession, error) {
	session, _, err := resolveSessionFn(
		ctx,
		*flags.appleID,
		"",
		*flags.twoFactorCode,
	)
	if err != nil {
		return nil, err
	}
	return session, nil
}

func withWebAuthHint(err error, operation string) error {
	if err == nil {
		return nil
	}
	var apiErr *webcore.APIError
	if errors.As(err, &apiErr) && (apiErr.Status == 401 || apiErr.Status == 403) {
		return fmt.Errorf("%s failed: web session is unauthorized or expired (run 'asc web auth login'): %w", operation, err)
	}
	return fmt.Errorf("%s failed: %w", operation, err)
}
