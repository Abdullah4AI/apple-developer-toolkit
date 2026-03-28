package cmdtest

import (
	"context"
	"errors"
	"flag"
	"io"
	"strings"
	"testing"
)

func runValidationTests(t *testing.T, tests []struct {
	name    string
	args    []string
	wantErr string
},
) {
	t.Helper()

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse(test.args); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				err := root.Run(context.Background())
				if !errors.Is(err, flag.ErrHelp) {
					t.Fatalf("expected ErrHelp, got %v", err)
				}
			})

			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if !strings.Contains(stderr, test.wantErr) {
				t.Fatalf("expected error %q, got %q", test.wantErr, stderr)
			}
		})
	}
}

func TestBuildsParityValidationErrors(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")

	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "builds links missing type",
			args:    []string{"builds", "links", "view", "--build-id", "BUILD_ID"},
			wantErr: "--type is required",
		},
		{
			name:    "builds links missing build",
			args:    []string{"builds", "links", "view", "--type", "app"},
			wantErr: "--build-id or --app is required",
		},
		{
			name:    "builds links invalid type",
			args:    []string{"builds", "links", "view", "--build-id", "BUILD_ID", "--type", "nope"},
			wantErr: "--type must be one of",
		},
		{
			name:    "builds links invalid limit for single",
			args:    []string{"builds", "links", "view", "--build-id", "BUILD_ID", "--type", "app", "--limit", "10"},
			wantErr: "only valid for to-many relationships",
		},
		{
			name:    "builds metrics beta-usages missing build id",
			args:    []string{"builds", "metrics", "beta-usages"},
			wantErr: "--build-id or --app is required",
		},
		{
			name:    "builds metrics beta-usages invalid limit",
			args:    []string{"builds", "metrics", "beta-usages", "--build-id", "BUILD_ID", "--limit", "300"},
			wantErr: "--limit must be between 1 and 200",
		},
		{
			name:    "builds individual-testers list missing build id",
			args:    []string{"builds", "individual-testers", "list"},
			wantErr: "--build-id or --app is required",
		},
		{
			name:    "builds individual-testers add missing build id",
			args:    []string{"builds", "individual-testers", "add", "--tester", "TESTER_ID"},
			wantErr: "--build-id or --app is required",
		},
		{
			name:    "builds individual-testers add missing tester",
			args:    []string{"builds", "individual-testers", "add", "--build-id", "BUILD_ID"},
			wantErr: "--tester is required",
		},
		{
			name:    "builds individual-testers remove missing build id",
			args:    []string{"builds", "individual-testers", "remove", "--tester", "TESTER_ID"},
			wantErr: "--build-id or --app is required",
		},
		{
			name:    "builds individual-testers remove missing tester",
			args:    []string{"builds", "individual-testers", "remove", "--build-id", "BUILD_ID"},
			wantErr: "--tester is required",
		},
		{
			name:    "builds uploads list missing app",
			args:    []string{"builds", "uploads", "list"},
			wantErr: "--app is required",
		},
		{
			name:    "builds uploads list invalid limit",
			args:    []string{"builds", "uploads", "list", "--app", "APP_ID", "--limit", "300"},
			wantErr: "--limit must be between 1 and 200",
		},
		{
			name:    "builds uploads list invalid sort",
			args:    []string{"builds", "uploads", "list", "--app", "APP_ID", "--sort", "nope"},
			wantErr: "--sort must be one of",
		},
		{
			name:    "builds uploads view missing id",
			args:    []string{"builds", "uploads", "view"},
			wantErr: "--id is required",
		},
		{
			name:    "builds uploads delete missing id",
			args:    []string{"builds", "uploads", "delete"},
			wantErr: "--id is required",
		},
		{
			name:    "builds uploads delete missing confirm",
			args:    []string{"builds", "uploads", "delete", "--id", "UPLOAD_ID"},
			wantErr: "--confirm is required",
		},
		{
			name:    "builds uploads files list missing upload",
			args:    []string{"builds", "uploads", "files", "list"},
			wantErr: "--upload is required",
		},
		{
			name:    "builds uploads files view missing id",
			args:    []string{"builds", "uploads", "files", "view"},
			wantErr: "--id is required",
		},
		{
			name:    "builds app-encryption-declaration view missing build id",
			args:    []string{"builds", "app-encryption-declaration", "view"},
			wantErr: "--build-id or --app is required",
		},
	}

	runValidationTests(t, tests)
}

func TestBetaLocalizationsValidationErrors(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")

	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "beta-app-localizations list missing app",
			args:    []string{"beta-app-localizations", "list"},
			wantErr: "--app is required",
		},
		{
			name:    "beta-app-localizations create missing app",
			args:    []string{"beta-app-localizations", "create", "--locale", "en-US"},
			wantErr: "--app is required",
		},
		{
			name:    "beta-app-localizations create missing locale",
			args:    []string{"beta-app-localizations", "create", "--app", "APP_ID"},
			wantErr: "--locale is required",
		},
		{
			name:    "beta-app-localizations update missing id",
			args:    []string{"beta-app-localizations", "update"},
			wantErr: "--id is required",
		},
		{
			name:    "beta-app-localizations update missing updates",
			args:    []string{"beta-app-localizations", "update", "--id", "LOC_ID"},
			wantErr: "at least one update flag is required",
		},
		{
			name:    "beta-app-localizations delete missing id",
			args:    []string{"beta-app-localizations", "delete"},
			wantErr: "--id is required",
		},
		{
			name:    "beta-app-localizations delete missing confirm",
			args:    []string{"beta-app-localizations", "delete", "--id", "LOC_ID"},
			wantErr: "--confirm is required",
		},
	}

	runValidationTests(t, tests)
}

func TestTestFlightRelationshipsValidationErrors(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")

	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "beta-groups relationships missing type",
			args:    []string{"testflight", "groups", "links", "view", "--group-id", "GROUP_ID"},
			wantErr: "--type is required",
		},
		{
			name:    "beta-groups relationships missing group-id",
			args:    []string{"testflight", "groups", "links", "view", "--type", "betaTesters"},
			wantErr: "--group-id is required",
		},
		{
			name:    "beta-groups relationships invalid type",
			args:    []string{"testflight", "groups", "links", "view", "--group-id", "GROUP_ID", "--type", "nope"},
			wantErr: "--type must be one of",
		},
		{
			name:    "beta-groups deprecated root links missing type",
			args:    []string{"testflight", "beta-groups", "links", "view", "--group-id", "GROUP_ID"},
			wantErr: "--type is required",
		},
		{
			name:    "beta-testers relationships missing type",
			args:    []string{"testflight", "testers", "links", "view", "--tester-id", "TESTER_ID"},
			wantErr: "--type is required",
		},
		{
			name:    "beta-testers relationships missing tester-id",
			args:    []string{"testflight", "testers", "links", "view", "--type", "apps"},
			wantErr: "--tester-id is required",
		},
		{
			name:    "beta-testers relationships invalid type",
			args:    []string{"testflight", "testers", "links", "view", "--tester-id", "TESTER_ID", "--type", "nope"},
			wantErr: "--type must be one of",
		},
		{
			name:    "beta-testers deprecated root links missing type",
			args:    []string{"testflight", "beta-testers", "links", "view", "--tester-id", "TESTER_ID"},
			wantErr: "--type is required",
		},
		{
			name:    "beta-testers metrics missing tester-id",
			args:    []string{"testflight", "beta-testers", "metrics", "--app", "APP_ID"},
			wantErr: "--tester-id is required",
		},
		{
			name:    "beta-testers metrics missing app",
			args:    []string{"testflight", "beta-testers", "metrics", "--tester-id", "TESTER_ID"},
			wantErr: "--app is required",
		},
		{
			name:    "beta-testers metrics invalid period",
			args:    []string{"testflight", "beta-testers", "metrics", "--tester-id", "TESTER_ID", "--app", "APP_ID", "--period", "P1D"},
			wantErr: "--period must be one of",
		},
		{
			name:    "beta-testers metrics invalid limit",
			args:    []string{"testflight", "beta-testers", "metrics", "--tester-id", "TESTER_ID", "--app", "APP_ID", "--limit", "500"},
			wantErr: "--limit must be between 1 and 200",
		},
	}

	runValidationTests(t, tests)
}

func TestPreReleaseRelationshipsValidationErrors(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")

	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "pre-release links missing type",
			args:    []string{"testflight", "pre-release", "links", "view", "--id", "PR_ID"},
			wantErr: "--type is required",
		},
		{
			name:    "pre-release links missing id",
			args:    []string{"testflight", "pre-release", "links", "view", "--type", "app"},
			wantErr: "--id is required",
		},
		{
			name:    "pre-release links invalid type",
			args:    []string{"testflight", "pre-release", "links", "view", "--id", "PR_ID", "--type", "nope"},
			wantErr: "--type must be one of",
		},
		{
			name:    "pre-release links invalid limit for single",
			args:    []string{"testflight", "pre-release", "links", "view", "--id", "PR_ID", "--type", "app", "--limit", "10"},
			wantErr: "only valid for to-many relationships",
		},
	}

	runValidationTests(t, tests)
}

func TestParityRelatedCommandsValidationErrors(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")

	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "builds app view missing build id",
			args:    []string{"builds", "app", "view"},
			wantErr: "--build-id or --app is required",
		},
		{
			name:    "builds pre-release-version view missing build id",
			args:    []string{"builds", "pre-release-version", "view"},
			wantErr: "--build-id or --app is required",
		},
		{
			name:    "builds icons list missing build id",
			args:    []string{"builds", "icons", "list"},
			wantErr: "--build-id or --app is required",
		},
		{
			name:    "builds beta-app-review-submission view missing build id",
			args:    []string{"builds", "beta-app-review-submission", "view"},
			wantErr: "--build-id or --app is required",
		},
		{
			name:    "builds build-beta-detail view missing build id",
			args:    []string{"builds", "build-beta-detail", "view"},
			wantErr: "--build-id or --app is required",
		},
		{
			name:    "beta-groups app get missing group-id",
			args:    []string{"testflight", "beta-groups", "app", "get"},
			wantErr: "--group-id is required",
		},
		{
			name:    "beta-groups recruitment-criteria get missing group-id",
			args:    []string{"testflight", "beta-groups", "beta-recruitment-criteria", "get"},
			wantErr: "--group-id is required",
		},
		{
			name:    "beta-groups compatible-build-check get missing group-id",
			args:    []string{"testflight", "beta-groups", "beta-recruitment-criterion-compatible-build-check", "get"},
			wantErr: "--group-id is required",
		},
		{
			name:    "beta-testers apps list missing tester-id",
			args:    []string{"testflight", "beta-testers", "apps", "list"},
			wantErr: "--tester-id is required",
		},
		{
			name:    "beta-testers beta-groups list missing tester-id",
			args:    []string{"testflight", "beta-testers", "beta-groups", "list"},
			wantErr: "--tester-id is required",
		},
		{
			name:    "beta-testers builds list missing tester-id",
			args:    []string{"testflight", "beta-testers", "builds", "list"},
			wantErr: "--tester-id is required",
		},
		{
			name:    "beta-feedback crash-submissions get missing id",
			args:    []string{"testflight", "beta-feedback", "crash-submissions", "get"},
			wantErr: "--submission-id is required",
		},
		{
			name:    "beta-feedback screenshot-submissions get missing id",
			args:    []string{"testflight", "beta-feedback", "screenshot-submissions", "get"},
			wantErr: "--submission-id is required",
		},
		{
			name:    "beta-feedback crash-log get missing id",
			args:    []string{"testflight", "beta-feedback", "crash-log", "get"},
			wantErr: "--submission-id is required",
		},
		{
			name:    "beta-feedback crash-submissions delete missing id",
			args:    []string{"testflight", "beta-feedback", "crash-submissions", "delete", "--confirm"},
			wantErr: "--submission-id is required",
		},
		{
			name:    "beta-feedback crash-submissions delete missing confirm",
			args:    []string{"testflight", "beta-feedback", "crash-submissions", "delete", "--id", "SUBMISSION_ID"},
			wantErr: "--confirm is required",
		},
		{
			name:    "beta-feedback screenshot-submissions delete missing id",
			args:    []string{"testflight", "beta-feedback", "screenshot-submissions", "delete", "--confirm"},
			wantErr: "--submission-id is required",
		},
		{
			name:    "beta-feedback screenshot-submissions delete missing confirm",
			args:    []string{"testflight", "beta-feedback", "screenshot-submissions", "delete", "--id", "SUBMISSION_ID"},
			wantErr: "--confirm is required",
		},
		{
			name:    "beta-app-localizations app get missing id",
			args:    []string{"beta-app-localizations", "app", "get"},
			wantErr: "--id is required",
		},
		{
			name:    "beta-build-localizations build get removed",
			args:    []string{"beta-build-localizations", "build", "get"},
			wantErr: "No canonical replacement exists yet",
		},
		{
			name:    "pre-release app view missing id",
			args:    []string{"testflight", "pre-release", "app", "view"},
			wantErr: "--id is required",
		},
		{
			name:    "pre-release builds list missing id",
			args:    []string{"testflight", "pre-release", "builds", "list"},
			wantErr: "--id is required",
		},
	}

	runValidationTests(t, tests)
}

func TestBuildsDeprecatedGetAliasesMatchViewValidation(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")

	tests := []struct {
		name     string
		viewArgs []string
		getArgs  []string
		warning  string
		wantErr  string
	}{
		{
			name:     "builds app get alias",
			viewArgs: []string{"builds", "app", "view"},
			getArgs:  []string{"builds", "app", "get"},
			warning:  "Warning: `asc builds app get` is deprecated. Use `asc builds app view`.",
			wantErr:  "--build-id or --app is required",
		},
		{
			name:     "builds pre-release-version get alias",
			viewArgs: []string{"builds", "pre-release-version", "view"},
			getArgs:  []string{"builds", "pre-release-version", "get"},
			warning:  "Warning: `asc builds pre-release-version get` is deprecated. Use `asc builds pre-release-version view`.",
			wantErr:  "--build-id or --app is required",
		},
		{
			name:     "builds beta-app-review-submission get alias",
			viewArgs: []string{"builds", "beta-app-review-submission", "view"},
			getArgs:  []string{"builds", "beta-app-review-submission", "get"},
			warning:  "Warning: `asc builds beta-app-review-submission get` is deprecated. Use `asc builds beta-app-review-submission view`.",
			wantErr:  "--build-id or --app is required",
		},
		{
			name:     "builds build-beta-detail get alias",
			viewArgs: []string{"builds", "build-beta-detail", "view"},
			getArgs:  []string{"builds", "build-beta-detail", "get"},
			warning:  "Warning: `asc builds build-beta-detail get` is deprecated. Use `asc builds build-beta-detail view`.",
			wantErr:  "--build-id or --app is required",
		},
		{
			name:     "builds app-encryption-declaration get alias",
			viewArgs: []string{"builds", "app-encryption-declaration", "view"},
			getArgs:  []string{"builds", "app-encryption-declaration", "get"},
			warning:  "Warning: `asc builds app-encryption-declaration get` is deprecated. Use `asc builds app-encryption-declaration view`.",
			wantErr:  "--build-id or --app is required",
		},
		{
			name:     "builds uploads get alias",
			viewArgs: []string{"builds", "uploads", "view"},
			getArgs:  []string{"builds", "uploads", "get"},
			warning:  "Warning: `asc builds uploads get` is deprecated. Use `asc builds uploads view`.",
			wantErr:  "--id is required",
		},
		{
			name:     "builds uploads files get alias",
			viewArgs: []string{"builds", "uploads", "files", "view"},
			getArgs:  []string{"builds", "uploads", "files", "get"},
			warning:  "Warning: `asc builds uploads files get` is deprecated. Use `asc builds uploads files view`.",
			wantErr:  "--id is required",
		},
	}

	run := func(t *testing.T, args []string) (string, string) {
		t.Helper()

		root := RootCommand("1.2.3")
		root.FlagSet.SetOutput(io.Discard)

		stdout, stderr := captureOutput(t, func() {
			if err := root.Parse(args); err != nil {
				t.Fatalf("parse error: %v", err)
			}
			err := root.Run(context.Background())
			if !errors.Is(err, flag.ErrHelp) {
				t.Fatalf("expected ErrHelp, got %v", err)
			}
		})

		return stdout, stderr
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			viewStdout, viewStderr := run(t, test.viewArgs)
			getStdout, getStderr := run(t, test.getArgs)

			if viewStdout != "" {
				t.Fatalf("expected empty canonical stdout, got %q", viewStdout)
			}
			if !strings.Contains(viewStderr, test.wantErr) {
				t.Fatalf("expected canonical stderr to contain %q, got %q", test.wantErr, viewStderr)
			}
			if strings.Contains(viewStderr, "deprecated") {
				t.Fatalf("expected canonical command to avoid deprecation warnings, got %q", viewStderr)
			}

			if getStdout != "" {
				t.Fatalf("expected empty alias stdout, got %q", getStdout)
			}
			requireStderrContainsWarning(t, getStderr, test.warning)
			if !strings.Contains(getStderr, test.wantErr) {
				t.Fatalf("expected alias stderr to contain %q, got %q", test.wantErr, getStderr)
			}
		})
	}
}
