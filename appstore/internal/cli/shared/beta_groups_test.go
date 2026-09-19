package shared

import (
	"context"
	"slices"
	"testing"

	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/asc"
)

type betaGroupsMutationClientStub struct {
	buildID  string
	groupIDs []string
	notify   bool
	calls    int
}

func (s *betaGroupsMutationClientStub) AddBetaGroupsToBuildWithNotify(_ context.Context, buildID string, groupIDs []string, notify bool) (asc.BuildBetaGroupsNotificationAction, error) {
	s.calls++
	s.buildID = buildID
	s.groupIDs = append([]string(nil), groupIDs...)
	s.notify = notify
	return asc.BuildBetaGroupsNotificationActionNone, nil
}

func TestAddBuildBetaGroupsSkipsInternalGroupsWithAllBuildsWhenRequested(t *testing.T) {
	client := &betaGroupsMutationClientStub{}
	groups := []ResolvedBetaGroup{
		{ID: "group-internal", IsInternalGroup: true, HasAccessToAllBuilds: true},
	}

	result, err := AddBuildBetaGroups(context.Background(), client, "build-1", groups, AddBuildBetaGroupsOptions{
		SkipInternalWithAllBuilds: true,
		Notify:                    true,
	})
	if err != nil {
		t.Fatalf("AddBuildBetaGroups() error = %v", err)
	}

	if client.calls != 0 {
		t.Fatalf("expected no mutation calls, got %d", client.calls)
	}
	if len(result.AddedGroupIDs) != 0 {
		t.Fatalf("expected no added groups, got %v", result.AddedGroupIDs)
	}
	if len(result.SkippedInternalAllBuildsGroups) != 1 {
		t.Fatalf("expected one skipped internal all-builds group, got %d", len(result.SkippedInternalAllBuildsGroups))
	}
	if result.SkippedInternalAllBuildsGroups[0].ID != "group-internal" {
		t.Fatalf("expected skipped group-internal, got %q", result.SkippedInternalAllBuildsGroups[0].ID)
	}
}

func TestAddBuildBetaGroupsAddsInternalGroupsWithAllBuildsWhenSkipDisabled(t *testing.T) {
	client := &betaGroupsMutationClientStub{}
	groups := []ResolvedBetaGroup{
		{ID: "group-internal", IsInternalGroup: true, HasAccessToAllBuilds: true},
		{ID: "group-external", IsInternalGroup: false},
	}

	result, err := AddBuildBetaGroups(context.Background(), client, "build-1", groups, AddBuildBetaGroupsOptions{
		SkipInternalWithAllBuilds: false,
		Notify:                    true,
	})
	if err != nil {
		t.Fatalf("AddBuildBetaGroups() error = %v", err)
	}

	if client.calls != 1 {
		t.Fatalf("expected one mutation call, got %d", client.calls)
	}
	if client.buildID != "build-1" {
		t.Fatalf("expected build-1, got %q", client.buildID)
	}
	if !client.notify {
		t.Fatal("expected notify=true")
	}
	if len(client.groupIDs) != 2 {
		t.Fatalf("expected two group IDs, got %v", client.groupIDs)
	}
	if client.groupIDs[0] != "group-internal" || client.groupIDs[1] != "group-external" {
		t.Fatalf("unexpected group IDs: %v", client.groupIDs)
	}
	if len(result.SkippedInternalAllBuildsGroups) != 0 {
		t.Fatalf("expected no skipped internal all-builds groups, got %v", result.SkippedInternalAllBuildsGroups)
	}
}

func TestPlanBuildBetaGroupAssignmentUsesTheMutationFilters(t *testing.T) {
	groups := []ResolvedBetaGroup{
		{ID: "internal", IsInternalGroup: true},
		{ID: "internal-all", IsInternalGroup: true, HasAccessToAllBuilds: true},
		{ID: "external", IsInternalGroup: false},
	}
	plan := PlanBuildBetaGroupAssignment(groups, AddBuildBetaGroupsOptions{
		SkipInternal:              true,
		SkipInternalWithAllBuilds: true,
	})

	if got, want := plan.GroupIDsToAdd(), []string{"external"}; !slices.Equal(got, want) {
		t.Fatalf("planned IDs = %v, want %v", got, want)
	}
	if len(plan.SkippedInternalGroups) != 2 || plan.SkippedInternalGroups[0].ID != "internal" || plan.SkippedInternalGroups[1].ID != "internal-all" {
		t.Fatalf("skipped internal groups = %+v", plan.SkippedInternalGroups)
	}
	if len(plan.SkippedInternalAllBuildsGroups) != 0 {
		t.Fatalf("skipped all-builds groups = %+v", plan.SkippedInternalAllBuildsGroups)
	}
	if !plan.IncludesExternalGroup() {
		t.Fatal("plan should include an external group")
	}

	plan = PlanBuildBetaGroupAssignment(groups, AddBuildBetaGroupsOptions{
		SkipInternalWithAllBuilds: true,
	})
	if got, want := plan.GroupIDsToAdd(), []string{"internal", "external"}; !slices.Equal(got, want) {
		t.Fatalf("planned IDs with only all-builds filtering = %v, want %v", got, want)
	}
	if len(plan.SkippedInternalAllBuildsGroups) != 1 || plan.SkippedInternalAllBuildsGroups[0].ID != "internal-all" {
		t.Fatalf("skipped all-builds groups = %+v", plan.SkippedInternalAllBuildsGroups)
	}
}
