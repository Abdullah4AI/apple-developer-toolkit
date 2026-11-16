package storekit

import (
	"strconv"

	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/asc"
	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/cli/shared"
)

func printOutput(d *shared.Deps, data any, format string, pretty bool, headers []string, rows [][]string) error {
	return d.PrintOutputWithRenderers(
		data,
		format,
		pretty,
		func() error { asc.RenderTableTo(d.IO.Out, headers, rows); return nil },
		func() error { asc.RenderMarkdownTo(d.IO.Out, headers, rows); return nil },
	)
}

func boolString(value bool) string { return strconv.FormatBool(value) }
