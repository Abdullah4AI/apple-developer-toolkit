package web

import (
	"context"

	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/cli/shared"
	webcore "github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/web"
)

func SetResolveWebAuthCredentials(fn func(string) (shared.ResolvedAuthCredentials, error)) func() {
	prev := resolveWebAuthCredentialsFn
	resolveWebAuthCredentialsFn = fn
	return func() {
		resolveWebAuthCredentialsFn = prev
	}
}

func SetResolveWebSession(fn func(context.Context, string, string, string) (*webcore.AuthSession, string, error)) func() {
	prev := resolveSessionFn
	resolveSessionFn = fn
	return func() {
		resolveSessionFn = prev
	}
}

func SetNewWebAuthClient(fn func(*webcore.AuthSession) *webcore.Client) func() {
	prev := newWebAuthClientFn
	newWebAuthClientFn = fn
	return func() {
		newWebAuthClientFn = prev
	}
}

func SetLookupWebAuthKey(fn func(context.Context, *webcore.Client, string) (*webcore.APIKeyRoleLookup, error)) func() {
	prev := lookupWebAuthKeyFn
	lookupWebAuthKeyFn = fn
	return func() {
		lookupWebAuthKeyFn = prev
	}
}
