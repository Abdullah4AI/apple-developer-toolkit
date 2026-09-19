package shared

import (
	"encoding/json"
	"testing"

	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/asc"
)

func TestEvaluateBuildBetaGroupBuildState(t *testing.T) {
	valid := mustBuildAttributes(t, `{"processingState":"VALID","expired":false,"usesNonExemptEncryption":false,"buildAudienceType":"APP_STORE_ELIGIBLE"}`)

	tests := []struct {
		name             string
		attributes       asc.BuildAttributes
		includesExternal bool
		wantBlocked      bool
		wantMessage      string
	}{
		{name: "processing", attributes: withProcessingState(valid, asc.BuildProcessingStateProcessing), wantBlocked: true, wantMessage: "PROCESSING"},
		{name: "expired", attributes: withExpired(valid, true), wantBlocked: true, wantMessage: "has expired"},
		{name: "missing expiry", attributes: withExpiredUnknown(valid), wantBlocked: true, wantMessage: "expired was not reported"},
		{name: "internal does not need audience or encryption", attributes: mustBuildAttributes(t, `{"processingState":"VALID","expired":false}`), wantBlocked: false},
		{name: "external needs audience", attributes: valid, includesExternal: true, wantBlocked: false},
		{name: "external internal-only audience", attributes: withAudience(valid, asc.BuildAudienceTypeInternalOnly), includesExternal: true, wantBlocked: true, wantMessage: "INTERNAL_ONLY"},
		{name: "external unknown audience", attributes: withAudience(valid, ""), includesExternal: true, wantBlocked: true, wantMessage: "buildAudienceType"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			precondition, blocked := evaluateBuildBetaGroupBuildState("build-1", test.attributes, test.includesExternal)
			if blocked != test.wantBlocked {
				t.Fatalf("blocked = %t, want %t (precondition=%+v)", blocked, test.wantBlocked, precondition)
			}
			if test.wantMessage != "" && !contains(precondition.Summary, test.wantMessage) {
				t.Fatalf("summary = %q, want %q", precondition.Summary, test.wantMessage)
			}
		})
	}
}

func TestEvaluateBuildBetaGroupExternalState(t *testing.T) {
	falseValue := false
	attributes := asc.BuildAttributes{UsesNonExemptEncryption: &falseValue}
	tests := []struct {
		name        string
		state       string
		wantBlocked bool
		wantMessage string
	}{
		{name: "ready for testing", state: externalBetaStateReadyForBetaTesting},
		{name: "legacy ready for testing", state: externalBetaStateReadyForTesting},
		{name: "in beta testing", state: externalBetaStateInBetaTesting},
		{name: "approved", state: externalBetaStateBetaApproved},
		{name: "ready for submission", state: externalBetaStateReadyForBetaSubmission},
		{name: "waiting for review", state: externalBetaStateWaitingForBetaReview, wantBlocked: true, wantMessage: "awaiting beta app review"},
		{name: "in review", state: externalBetaStateInBetaReview, wantBlocked: true, wantMessage: "awaiting beta app review"},
		{name: "legacy not ready for testing", state: externalBetaStateNotReadyForTesting, wantBlocked: true, wantMessage: "not ready for external testing"},
		{name: "missing export compliance", state: externalBetaStateMissingExportCompliance, wantBlocked: true, wantMessage: "MISSING_EXPORT_COMPLIANCE"},
		{name: "unknown", state: "FUTURE_STATE", wantBlocked: true, wantMessage: "not a recognized"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			precondition, blocked := evaluateBuildBetaGroupExternalState("build-1", attributes, test.state)
			if blocked != test.wantBlocked {
				t.Fatalf("blocked = %t, want %t (precondition=%+v)", blocked, test.wantBlocked, precondition)
			}
			if test.wantMessage != "" && !contains(precondition.Summary, test.wantMessage) {
				t.Fatalf("summary = %q, want %q", precondition.Summary, test.wantMessage)
			}
		})
	}

	precondition, blocked := evaluateBuildBetaGroupExternalState("build-1", asc.BuildAttributes{}, externalBetaStateReadyForBetaTesting)
	if !blocked || !contains(precondition.Summary, "usesNonExemptEncryption") {
		t.Fatalf("missing encryption = blocked %t, summary %q", blocked, precondition.Summary)
	}

	precondition, blocked = evaluateBuildBetaGroupExternalState("build-1", asc.BuildAttributes{}, externalBetaStateProcessing)
	if !blocked || !contains(precondition.Summary, "still processing") || contains(precondition.Summary, "usesNonExemptEncryption") {
		t.Fatalf("processing without encryption = blocked %t, summary %q", blocked, precondition.Summary)
	}
}

func withProcessingState(attributes asc.BuildAttributes, state string) asc.BuildAttributes {
	attributes.ProcessingState = state
	return attributes
}

func withExpired(attributes asc.BuildAttributes, expired bool) asc.BuildAttributes {
	attributes.Expired = expired
	return attributes
}

func withExpiredUnknown(attributes asc.BuildAttributes) asc.BuildAttributes {
	return asc.BuildAttributes{
		ProcessingState:         attributes.ProcessingState,
		UsesNonExemptEncryption: attributes.UsesNonExemptEncryption,
		BuildAudienceType:       attributes.BuildAudienceType,
	}
}

func withAudience(attributes asc.BuildAttributes, audience asc.BuildAudienceType) asc.BuildAttributes {
	attributes.BuildAudienceType = audience
	return attributes
}

func contains(value, want string) bool {
	for i := 0; i+len(want) <= len(value); i++ {
		if value[i:i+len(want)] == want {
			return true
		}
	}
	return false
}

func mustBuildAttributes(t *testing.T, raw string) asc.BuildAttributes {
	t.Helper()
	var attributes asc.BuildAttributes
	if err := json.Unmarshal([]byte(raw), &attributes); err != nil {
		t.Fatalf("decode build attributes: %v", err)
	}
	return attributes
}
