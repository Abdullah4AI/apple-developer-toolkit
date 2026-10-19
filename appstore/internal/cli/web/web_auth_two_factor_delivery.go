package web

import (
	"context"
	"flag"
	"os"
	"strconv"
	"strings"

	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/appleauth"
	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/cli/shared"
)

const (
	webTwoFactorMethodEnv  = "ASC_WEB_2FA_METHOD"
	webTwoFactorPhoneIDEnv = "ASC_WEB_2FA_PHONE_ID"
	twoFactorMethodAuto    = "auto"
)

// twoFactorDeliveryFlags selects how Apple delivers the verification code.
type twoFactorDeliveryFlags struct {
	flagSet *flag.FlagSet
	method  *string
	phoneID *int
}

func bindTwoFactorDeliveryFlags(fs *flag.FlagSet) twoFactorDeliveryFlags {
	return twoFactorDeliveryFlags{
		flagSet: fs,
		method:  fs.String("two-factor-method", "", "2FA code delivery: auto (trusted devices first) or sms (text a trusted phone number); defaults to "+webTwoFactorMethodEnv+", then auto"),
		phoneID: fs.Int("two-factor-phone-id", 0, "Trusted phone number ID for --two-factor-method sms when the account has several; defaults to "+webTwoFactorPhoneIDEnv),
	}
}

// resolve merges the flags with their environment fallbacks and validates the
// result before any sign-in side effect.
func (f twoFactorDeliveryFlags) resolve() (appleauth.TwoFactorPreference, error) {
	method := ""
	if f.method != nil {
		method = *f.method
	}
	phoneID := 0
	if f.phoneID != nil {
		phoneID = *f.phoneID
	}
	// An explicit --two-factor-phone-id 0 must fail rather than fall back to
	// the environment or the only trusted number.
	phoneIDSet := false
	if f.flagSet != nil {
		f.flagSet.Visit(func(fl *flag.Flag) {
			if fl.Name == "two-factor-phone-id" {
				phoneIDSet = true
			}
		})
	}
	return resolveTwoFactorPreference(method, phoneID, phoneIDSet)
}

func resolveTwoFactorPreference(methodFlag string, phoneIDFlag int, phoneIDFlagSet bool) (appleauth.TwoFactorPreference, error) {
	method := strings.ToLower(strings.TrimSpace(methodFlag))
	methodSource := "--two-factor-method"
	if method == "" {
		method = strings.ToLower(strings.TrimSpace(os.Getenv(webTwoFactorMethodEnv)))
		methodSource = webTwoFactorMethodEnv
	}
	switch method {
	case "", twoFactorMethodAuto, appleauth.TwoFactorDeliverySMS:
	default:
		return appleauth.TwoFactorPreference{}, shared.UsageError(methodSource + " must be one of: auto, sms")
	}

	phoneID := phoneIDFlag
	phoneIDSource := "--two-factor-phone-id"
	if phoneID < 0 || (phoneIDFlagSet && phoneID == 0) {
		return appleauth.TwoFactorPreference{}, shared.UsageError(phoneIDSource + " must be a positive trusted phone number ID")
	}
	if phoneID == 0 {
		if value := strings.TrimSpace(os.Getenv(webTwoFactorPhoneIDEnv)); value != "" {
			phoneIDSource = webTwoFactorPhoneIDEnv
			parsed, err := strconv.Atoi(value)
			if err != nil || parsed <= 0 {
				return appleauth.TwoFactorPreference{}, shared.UsageError(phoneIDSource + " must be a positive trusted phone number ID")
			}
			phoneID = parsed
		}
	}

	if method != appleauth.TwoFactorDeliverySMS {
		if phoneID != 0 {
			return appleauth.TwoFactorPreference{}, shared.UsageError(phoneIDSource + " requires --two-factor-method sms")
		}
		return appleauth.TwoFactorPreference{}, nil
	}
	return appleauth.TwoFactorPreference{Delivery: appleauth.TwoFactorDeliverySMS, PhoneID: phoneID}, nil
}

type twoFactorPreferenceContextKey struct{}

// contextWithTwoFactorPreference carries a validated delivery preference from
// command flags to the sign-in flow.
func contextWithTwoFactorPreference(ctx context.Context, preference appleauth.TwoFactorPreference) context.Context {
	return context.WithValue(ctx, twoFactorPreferenceContextKey{}, preference)
}

// twoFactorPreferenceForSignIn returns the flag-resolved preference, falling
// back to the environment for callers without delivery flags.
func twoFactorPreferenceForSignIn(ctx context.Context) (appleauth.TwoFactorPreference, error) {
	if preference, ok := ctx.Value(twoFactorPreferenceContextKey{}).(appleauth.TwoFactorPreference); ok {
		return preference, nil
	}
	return resolveTwoFactorPreference("", 0, false)
}
