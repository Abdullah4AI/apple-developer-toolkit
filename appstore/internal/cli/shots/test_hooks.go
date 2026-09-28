package shots

import (
	"context"

	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/rootfs"
	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/screenshots"
)

// SetFrameFunc replaces the frame implementation for tests, for both single
// inputs and --input-dir batches; a replacement ignores the batch output root.
// It returns a restore function to reset the previous handlers.
func SetFrameFunc(fn func(context.Context, screenshots.FrameRequest) (*screenshots.FrameResult, error)) func() {
	previous, previousInto := shotsFrameFn, shotsFrameIntoFn
	if fn == nil {
		shotsFrameFn = screenshots.Frame
		shotsFrameIntoFn = screenshots.FrameIntoOutputRoot
	} else {
		shotsFrameFn = fn
		shotsFrameIntoFn = func(ctx context.Context, req screenshots.FrameRequest, _ rootfs.Root) (*screenshots.FrameResult, error) {
			return fn(ctx, req)
		}
	}
	return func() {
		shotsFrameFn, shotsFrameIntoFn = previous, previousInto
	}
}

// SetFrameOutputFoldsCase forces whether --input-dir treats output names as
// case-insensitive, for tests. It returns a restore function.
func SetFrameOutputFoldsCase(fold bool) func() {
	previous := frameOutputFoldsCaseFn
	frameOutputFoldsCaseFn = func(string) bool { return fold }
	return func() {
		frameOutputFoldsCaseFn = previous
	}
}
