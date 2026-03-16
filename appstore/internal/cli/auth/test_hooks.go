package auth

import (
	"context"
	"crypto/ecdsa"
	"errors"

	authsvc "github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/auth"
)

// SetStatusValidateCredential replaces the validation hook for tests.
// It returns a restore function to reset the previous handler.
func SetStatusValidateCredential(fn func(context.Context, authsvc.Credential) error) func() {
	previous := statusValidateCredential
	if fn == nil {
		statusValidateCredential = validateStoredCredential
	} else {
		statusValidateCredential = fn
	}
	return func() {
		statusValidateCredential = previous
	}
}

// SetListStoredCredentials replaces the full credential listing hook for tests.
// It returns a restore function to reset the previous handler.
func SetListStoredCredentials(fn func() ([]authsvc.Credential, error)) func() {
	previous := listStoredCredentials
	if fn == nil {
		listStoredCredentials = authsvc.ListCredentials
	} else {
		listStoredCredentials = fn
	}
	return func() {
		listStoredCredentials = previous
	}
}

// SetListCredentialSummaries replaces the metadata-only credential listing hook
// for tests. It returns a restore function to reset the previous handler.
func SetListCredentialSummaries(fn func() ([]authsvc.Credential, error)) func() {
	previous := listCredentialSummaries
	if fn == nil {
		listCredentialSummaries = authsvc.ListCredentialSummaries
	} else {
		listCredentialSummaries = fn
	}
	return func() {
		listCredentialSummaries = previous
	}
}

// NewPermissionWarning builds a permission warning error for tests.
func NewPermissionWarning(err error) error {
	if err == nil {
		err = errors.New("permission warning")
	}
	return &permissionWarning{err: err}
}

// SetLoginJWTGenerator replaces the JWT generator for tests.
// It returns a restore function to reset the previous handler.
func SetLoginJWTGenerator(fn func(string, string, *ecdsa.PrivateKey) (string, error)) func() {
	previous := loginJWTGenerator
	if fn != nil {
		loginJWTGenerator = fn
	}
	return func() {
		loginJWTGenerator = previous
	}
}
