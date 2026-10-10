package iosbuild

import (
	"context"
	"os"
	"os/exec"
	"os/signal"
)

// ContextWithSignals lets build and signing operations clean up after interruption.
func ContextWithSignals(ctx context.Context) (context.Context, context.CancelFunc) {
	return signal.NotifyContext(ctx, os.Interrupt)
}

func configureProcessCancellation(command *exec.Cmd) {}
