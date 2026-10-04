package shared

import (
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/asc"
)

func notCancellableAPIError() error {
	return &asc.APIError{
		Code:       "CONFLICT",
		Title:      "Resource state is invalid.",
		Detail:     "Resource is not in cancellable state",
		StatusCode: http.StatusConflict,
	}
}

func TestIsReviewSubmissionNotCancellableError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "nil", err: nil, want: false},
		{name: "apple refusal", err: notCancellableAPIError(), want: true},
		{name: "wrapped refusal", err: errors.Join(errors.New("context"), notCancellableAPIError()), want: true},
		{name: "not found", err: &asc.APIError{Code: "NOT_FOUND", Title: "Not found", StatusCode: http.StatusNotFound}, want: false},
		{name: "other state conflict", err: &asc.APIError{Code: "CONFLICT", Title: "Resource state is invalid.", Detail: "Resource is not in submittable state", StatusCode: http.StatusConflict}, want: false},
		{
			name: "refusal in a later entry",
			err: &asc.APIError{
				Code:       "CONFLICT",
				Title:      "Resource state is invalid.",
				Detail:     "Another conflict",
				StatusCode: http.StatusConflict,
				Entries:    []asc.APIErrorEntry{{Code: "CONFLICT", Detail: "Another conflict"}, {Code: "CONFLICT", Detail: "Resource is not in cancellable state"}},
			},
			want: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := IsReviewSubmissionNotCancellableError(test.err); got != test.want {
				t.Fatalf("IsReviewSubmissionNotCancellableError() = %v, want %v", got, test.want)
			}
		})
	}
}

func TestNewReviewSubmissionNotCancellableErrorExplainsEachState(t *testing.T) {
	tests := []struct {
		state asc.ReviewSubmissionState
		want  string
	}{
		{state: asc.ReviewSubmissionStateReadyForReview, want: "unsubmitted draft"},
		{state: asc.ReviewSubmissionStateWaitingForReview, want: "may be changing state"},
		{state: asc.ReviewSubmissionStateInReview, want: "may be changing state"},
		{state: asc.ReviewSubmissionStateUnresolvedIssues, want: "Resolution Center"},
		{state: asc.ReviewSubmissionStateCanceling, want: "already being canceled"},
		{state: asc.ReviewSubmissionStateCompleting, want: "App Review is finishing it"},
		{state: asc.ReviewSubmissionStateComplete, want: "already finished"},
		{state: asc.ReviewSubmissionState("NEW_STATE"), want: "check its current state"},
	}
	for _, test := range tests {
		t.Run(string(test.state), func(t *testing.T) {
			original := notCancellableAPIError()
			err := NewReviewSubmissionNotCancellableError("sub-1", test.state, original)

			message := err.Error()
			for _, want := range []string{
				"review submission sub-1 cannot be canceled in state " + string(test.state),
				test.want,
				"(App Store Connect: Resource state is invalid.: Resource is not in cancellable state)",
			} {
				if !strings.Contains(message, want) {
					t.Fatalf("error %q does not contain %q", message, want)
				}
			}
			if !errors.Is(err, original) || !errors.Is(err, asc.ErrConflict) {
				t.Fatalf("error %v does not wrap the original conflict", err)
			}
		})
	}
}

func TestNewReviewSubmissionNotCancellableErrorKeepsOriginalWithoutState(t *testing.T) {
	original := notCancellableAPIError()
	err := NewReviewSubmissionNotCancellableError("sub-1", "  ", original)
	if _, explained := errors.AsType[*ReviewSubmissionNotCancellableError](err); explained || !errors.Is(err, original) {
		t.Fatalf("error = %v, want the original error unchanged", err)
	}
}
