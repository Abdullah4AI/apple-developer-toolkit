package validation

import "testing"

func countCheckID(checks []CheckResult, id string) int {
	count := 0
	for _, check := range checks {
		if check.ID == id {
			count++
		}
	}
	return count
}

func TestBuildChecks_MissingBuild(t *testing.T) {
	checks := buildChecks(nil)
	if !hasCheckID(checks, "build.required.missing") {
		t.Fatalf("expected build.required.missing check, got %v", checks)
	}
}

func TestBuildChecks_InvalidProcessingState(t *testing.T) {
	checks := buildChecks(&Build{
		ID:              "build-1",
		ProcessingState: "PROCESSING",
	})
	if !hasCheckID(checks, "build.invalid.processing_state") {
		t.Fatalf("expected build.invalid.processing_state check, got %v", checks)
	}
}

func TestBuildChecks_ExpiredBuild(t *testing.T) {
	checks := buildChecks(&Build{
		ID:              "build-1",
		ProcessingState: "VALID",
		Expired:         true,
	})
	if !hasCheckID(checks, "build.invalid.expired") {
		t.Fatalf("expected build.invalid.expired check, got %v", checks)
	}
}

func TestBuildChecks_Pass(t *testing.T) {
	checks := buildChecks(&Build{
		ID:              "build-1",
		ProcessingState: "VALID",
		Expired:         false,
		UsesNonExemptEncryption: func() *bool {
			value := false
			return &value
		}(),
	})
	if len(checks) != 0 {
		t.Fatalf("expected no checks, got %d (%v)", len(checks), checks)
	}
}

func TestSubmissionBuildChecks_MissingEncryptionState(t *testing.T) {
	checks := buildSubmissionChecks(&Build{
		ID:              "build-1",
		ProcessingState: "VALID",
		Expired:         false,
	})
	if !hasCheckID(checks, "build.encryption.missing") {
		t.Fatalf("expected build.encryption.missing check, got %v", checks)
	}
}

func TestSubmissionBuildChecks_NonExemptEncryptionMissingDeclaration(t *testing.T) {
	usesNonExemptEncryption := true
	checks := buildSubmissionChecks(&Build{
		ID:                      "build-1",
		ProcessingState:         "VALID",
		Expired:                 false,
		UsesNonExemptEncryption: &usesNonExemptEncryption,
	})
	if !hasCheckID(checks, "build.encryption.declaration_missing") {
		t.Fatalf("expected build.encryption.declaration_missing check, got %v", checks)
	}
}

func TestValidate_DoesNotDuplicateBuildChecks(t *testing.T) {
	usesNonExemptEncryption := true
	report := Validate(Input{
		Build: &Build{
			ID:                      "build-1",
			ProcessingState:         "PROCESSING",
			UsesNonExemptEncryption: &usesNonExemptEncryption,
		},
	}, false)

	if got := countCheckID(report.Checks, "build.invalid.processing_state"); got != 1 {
		t.Fatalf("expected one build.invalid.processing_state check, got %d (%v)", got, report.Checks)
	}
	if got := countCheckID(report.Checks, "build.encryption.declaration_missing"); got != 1 {
		t.Fatalf("expected one build.encryption.declaration_missing check, got %d (%v)", got, report.Checks)
	}
}
