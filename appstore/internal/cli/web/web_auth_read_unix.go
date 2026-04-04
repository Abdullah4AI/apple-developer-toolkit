//go:build !windows

package web

import (
	"context"
	"errors"
	"os"
	"time"

	"golang.org/x/sys/unix"
)

func readTerminalByteWithContext(ctx context.Context, terminal *os.File, buf []byte) (int, error) {
	for {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return 0, ctxErr
		}

		timeoutMs := int(passwordReadPollInterval / time.Millisecond)
		if deadline, ok := ctx.Deadline(); ok {
			remaining := time.Until(deadline)
			if remaining <= 0 {
				return 0, context.DeadlineExceeded
			}
			if remaining < passwordReadPollInterval {
				timeoutMs = max(1, int(remaining/time.Millisecond))
			}
		}

		fds := []unix.PollFd{{
			Fd:     int32(terminal.Fd()),
			Events: unix.POLLIN,
		}}
		n, err := unix.Poll(fds, timeoutMs)
		if err != nil {
			if errors.Is(err, unix.EINTR) {
				continue
			}
			return 0, err
		}
		if n == 0 {
			continue
		}
		return terminal.Read(buf)
	}
}
