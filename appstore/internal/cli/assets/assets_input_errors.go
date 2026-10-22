package assets

import (
	"errors"

	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/cli/shared"
)

// mediaInputError marks a failure caused by the local media named by --path,
// with the diagnostic chosen where the failure was detected. Its message is
// unchanged. It carries no diagnostic itself: command entry points apply it
// through ClassifyMediaInputError or mediaPathValidationError.
type mediaInputError struct {
	code shared.DiagnosticCode
	err  error
}

func (e *mediaInputError) Error() string { return e.err.Error() }
func (e *mediaInputError) Unwrap() error { return e.err }

// markMediaInput marks err with code, after mapping a missing or unreadable
// file to file_not_found or file_permission_denied. Any other filesystem
// failure, such as EIO, says nothing about the input and stays unmarked.
func markMediaInput(err error, code shared.DiagnosticCode) error {
	code, ok := shared.LocalFileDiagnostic(err, code)
	if !ok {
		return err
	}
	return &mediaInputError{code: code, err: err}
}

// ClassifyMediaInputError turns a marked local media failure into an
// expected-negative validation failure for parameter, keeping its message and
// exit code. Unmarked errors are returned unchanged.
func ClassifyMediaInputError(err error, parameter string) error {
	if classified, ok := classifyMediaInput(err, parameter); ok {
		return classified
	}
	return err
}

// mediaPathValidationError classifies a --path failure for callers that
// already report every local preflight failure as a validation error.
func mediaPathValidationError(err error) error {
	if classified, ok := classifyMediaInput(err, "--path"); ok {
		return classified
	}
	return shared.NewValidationError(err)
}

func classifyMediaInput(err error, parameter string) (error, bool) {
	var inputErr *mediaInputError
	if !errors.As(err, &inputErr) {
		return nil, false
	}
	return shared.WithDiagnostic(shared.NewValidationError(err), inputErr.code, parameter), true
}

// invalidDeviceTypeError reports an unknown --device-type as an
// expected-negative validation failure, keeping its message and exit code.
func invalidDeviceTypeError(err error) error {
	return shared.WithDiagnostic(shared.NewValidationError(err), shared.DiagnosticInvalidInput, "--device-type")
}

// InvalidDeviceTypeError is invalidDeviceTypeError for other command groups.
func InvalidDeviceTypeError(err error) error {
	return invalidDeviceTypeError(err)
}
