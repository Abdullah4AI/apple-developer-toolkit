package asc

import (
	"context"
	"errors"
	"net"
	"sync"
	"time"
)

type (
	imageUploadProgressKey struct{}
	requestObservationKey  struct{}
	requestObservation     struct {
		status func(int)
		retry  func(error)
	}
)

// WithAssetLibraryImageUploadProgress observes upload stages synchronously. The
// callback must not call back into the uploader or modify the supplied context.
func WithAssetLibraryImageUploadProgress(ctx context.Context, observe func(AssetLibraryImageUploadEvent)) context.Context {
	return context.WithValue(ctx, imageUploadProgressKey{}, observe)
}

func observeRequestStatus(ctx context.Context, status int) {
	if ctx == nil {
		return
	}
	if observer, ok := ctx.Value(requestObservationKey{}).(*requestObservation); ok && observer.status != nil {
		observer.status(status)
	}
}

func observeRequestRetry(ctx context.Context, err error) {
	if ctx == nil {
		return
	}
	if observer, ok := ctx.Value(requestObservationKey{}).(*requestObservation); ok && observer.retry != nil {
		observer.retry(err)
	}
}

type imageUploadStage struct {
	mu      sync.Mutex
	started time.Time
	event   AssetLibraryImageUploadEvent
	observe func(AssetLibraryImageUploadEvent)
}

func startImageUploadStage(ctx context.Context, stage string) (context.Context, *imageUploadStage) {
	observer, _ := ctx.Value(imageUploadProgressKey{}).(func(AssetLibraryImageUploadEvent))
	s := &imageUploadStage{started: time.Now(), event: AssetLibraryImageUploadEvent{Event: "assetLibraryImageUpload", Stage: stage}, observe: observer}
	s.emit("started", nil)
	return context.WithValue(ctx, requestObservationKey{}, &requestObservation{
		status: func(status int) { s.mu.Lock(); defer s.mu.Unlock(); s.event.HTTPStatus = status },
		retry:  s.retry,
	}), s
}

func (s *imageUploadStage) retry(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.event.RetryCount++
	s.emitLocked("retrying", err)
}

func (s *imageUploadStage) emit(status string, err error) AssetLibraryImageUploadEvent {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.emitLocked(status, err)
}

func (s *imageUploadStage) emitLocked(status string, err error) AssetLibraryImageUploadEvent {
	event := s.event
	event.Status = status
	event.DurationMs = time.Since(s.started).Milliseconds()
	if err != nil {
		event.ErrorCode = imageUploadErrorCode(err)
		event.HTTPStatus = 0
		var status interface{ HTTPStatusCode() int }
		if errors.As(err, &status) {
			event.HTTPStatus = status.HTTPStatusCode()
		} else if errors.Is(err, errImageUploadInvalidResponse) || errors.Is(err, errImageProcessingFailed) {
			event.HTTPStatus = s.event.HTTPStatus
		}
	}
	if s.observe != nil {
		s.observe(event)
	}
	return event
}

func imageUploadErrorCode(err error) string {
	switch {
	case errors.Is(err, context.Canceled):
		return "canceled"
	case errors.Is(err, context.DeadlineExceeded):
		return "timeout"
	}
	var status interface{ HTTPStatusCode() int }
	if errors.As(err, &status) && status.HTTPStatusCode() > 0 {
		return "http_error"
	}
	var network net.Error
	if errors.As(err, &network) {
		if network.Timeout() {
			return "timeout"
		}
		return "transport_error"
	}
	if errors.Is(err, errImageUploadInvalidResponse) {
		return "invalid_response"
	}
	if errors.Is(err, errImageProcessingFailed) {
		return "processing_failed"
	}
	return "unknown"
}

var (
	errImageUploadInvalidResponse = errors.New("invalid image response")
	errImageProcessingFailed      = errors.New("image processing reached FAILED")
)
