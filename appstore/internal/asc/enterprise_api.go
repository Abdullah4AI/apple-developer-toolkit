package asc

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
)

// ErrEnterpriseProgramAPIUnsupported reports an operation that the Enterprise
// Program API does not offer.
var ErrEnterpriseProgramAPIUnsupported = errors.New("not available on the Enterprise Program API")

// enterpriseOperations holds every operation in Apple's Enterprise Program API
// 1.0 OpenAPI document. Most App Store Connect resources do not exist there, so
// a typed command fails here with a clear error instead of a 404 from Apple.
var enterpriseOperations = map[string]struct{}{
	"POST /v1/bundleIdCapabilities":               {},
	"PATCH /v1/bundleIdCapabilities/{id}":         {},
	"DELETE /v1/bundleIdCapabilities/{id}":        {},
	"GET /v1/bundleIds":                           {},
	"POST /v1/bundleIds":                          {},
	"GET /v1/bundleIds/{id}":                      {},
	"PATCH /v1/bundleIds/{id}":                    {},
	"DELETE /v1/bundleIds/{id}":                   {},
	"GET /v1/bundleIds/{id}/bundleIdCapabilities": {},
	"GET /v1/bundleIds/{id}/profiles":             {},
	"GET /v1/certificates":                        {},
	"POST /v1/certificates":                       {},
	"GET /v1/certificates/{id}":                   {},
	"DELETE /v1/certificates/{id}":                {},
	"GET /v1/certificates/{id}/passTypeId":        {},
	"GET /v1/devices":                             {},
	"POST /v1/devices":                            {},
	"GET /v1/devices/{id}":                        {},
	"PATCH /v1/devices/{id}":                      {},
	"GET /v1/passTypeIds":                         {},
	"POST /v1/passTypeIds":                        {},
	"GET /v1/passTypeIds/{id}":                    {},
	"PATCH /v1/passTypeIds/{id}":                  {},
	"DELETE /v1/passTypeIds/{id}":                 {},
	"GET /v1/passTypeIds/{id}/certificates":       {},
	"GET /v1/profiles":                            {},
	"POST /v1/profiles":                           {},
	"GET /v1/profiles/{id}":                       {},
	"DELETE /v1/profiles/{id}":                    {},
	"GET /v1/profiles/{id}/bundleId":              {},
	"GET /v1/profiles/{id}/certificates":          {},
	"GET /v1/profiles/{id}/devices":               {},
	"GET /v1/userInvitations":                     {},
	"POST /v1/userInvitations":                    {},
	"GET /v1/userInvitations/{id}":                {},
	"DELETE /v1/userInvitations/{id}":             {},
	"GET /v1/users":                               {},
	"GET /v1/users/{id}":                          {},
	"PATCH /v1/users/{id}":                        {},
	"DELETE /v1/users/{id}":                       {},
}

type unlistedOperationsKey struct{}

// WithUnlistedOperations lets requests sent with the returned context skip the
// Enterprise Program API operation check. `asc api` uses it to stay an escape
// hatch for operations Apple adds after this list was written.
func WithUnlistedOperations(ctx context.Context) context.Context {
	return context.WithValue(ctx, unlistedOperationsKey{}, true)
}

// IsEnterpriseProgramAPI reports whether the client targets the Enterprise
// Program API.
func (c *Client) IsEnterpriseProgramAPI() bool {
	return c.enterprise
}

func (c *Client) validateEnterpriseOperation(ctx context.Context, method, target string) error {
	if !c.enterprise || ctx.Value(unlistedOperationsKey{}) != nil {
		return nil
	}

	parsedURL, err := url.Parse(target)
	if err != nil {
		return fmt.Errorf("invalid request URL: %w", err)
	}
	segments := strings.Split(parsedURL.EscapedPath(), "/")
	if len(segments) > 3 && segments[3] != "" && segments[3] != "." && segments[3] != ".." {
		segments[3] = "{id}"
	}
	if _, ok := enterpriseOperations[strings.ToUpper(method)+" "+strings.Join(segments, "/")]; !ok {
		return fmt.Errorf("%s %s: %w", strings.ToUpper(method), parsedURL.Path, ErrEnterpriseProgramAPIUnsupported)
	}
	return nil
}
