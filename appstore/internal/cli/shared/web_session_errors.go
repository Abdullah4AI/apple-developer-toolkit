package shared

import "errors"

// ErrMissingWebSession reports that an `asc web` command needs a signed-in
// Apple web session that is not cached and cannot be created without a
// terminal. It keeps the usage exit code that this failure has always had, but
// it is not reported by the command and does not wrap flag.ErrHelp: the root
// renderer prints its message and hint instead of the command's usage page.
var ErrMissingWebSession = errors.New("missing Apple web session")

// MissingWebSessionError is the rendered form of ErrMissingWebSession. Hint
// carries the next step the root error renderer prints after the message.
type MissingWebSessionError struct {
	Message string
	Hint    string
}

func (e *MissingWebSessionError) Error() string { return e.Message }

// Is lets errors.Is match ErrMissingWebSession through any wrapping.
func (e *MissingWebSessionError) Is(target error) bool { return target == ErrMissingWebSession }

// UsageErrorKind classifies the failure as missing required input, as the
// usage errors it replaced were, so telemetry and command diagnostics keep
// their classification.
func (e *MissingWebSessionError) UsageErrorKind() UsageErrorKind { return UsageErrorMissingRequired }
