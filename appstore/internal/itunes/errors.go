package itunes

import (
	"fmt"

	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/asc"
)

type httpStatusError struct {
	operation  string
	statusCode int
}

func (e *httpStatusError) Error() string {
	return fmt.Sprintf("%s request returned status %d", e.operation, e.statusCode)
}

func (e *httpStatusError) HTTPStatusCode() int {
	return e.statusCode
}

func (e *httpStatusError) PublicStorefrontError() bool {
	return true
}

type notFoundError struct {
	message string
}

func (e *notFoundError) Error() string {
	return e.message
}

func (e *notFoundError) Unwrap() error {
	return asc.ErrNotFound
}
