//go:build windows

package web

import (
	"context"
	"os"
)

func readTerminalByteWithContext(ctx context.Context, terminal *os.File, buf []byte) (int, error) {
	if ctxErr := ctx.Err(); ctxErr != nil {
		return 0, ctxErr
	}
	return terminal.Read(buf)
}
