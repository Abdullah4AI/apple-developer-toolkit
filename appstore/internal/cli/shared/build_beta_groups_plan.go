package shared

// BuildBetaGroupAssignmentPlan describes the groups that an assignment would
// send after applying the configured skip rules. It performs no API calls.
type BuildBetaGroupAssignmentPlan struct {
	GroupsToAdd                    []ResolvedBetaGroup
	SkippedInternalGroups          []ResolvedBetaGroup
	SkippedInternalAllBuildsGroups []ResolvedBetaGroup
}

// GroupIDsToAdd returns the beta group IDs that belong in the relationship
// request, without exposing the plan's backing slice.
func (p BuildBetaGroupAssignmentPlan) GroupIDsToAdd() []string {
	ids := make([]string, 0, len(p.GroupsToAdd))
	for _, group := range p.GroupsToAdd {
		ids = append(ids, group.ID)
	}
	return ids
}

// IncludesExternalGroup reports whether the plan contains an external group.
func (p BuildBetaGroupAssignmentPlan) IncludesExternalGroup() bool {
	for _, group := range p.GroupsToAdd {
		if !group.IsInternalGroup {
			return true
		}
	}
	return false
}

// PlanBuildBetaGroupAssignment applies assignment filtering without making a
// mutation or consulting provider state. This is shared by normal execution
// and the command's dry-run preview so the planned IDs cannot drift from the
// eventual relationship payload.
func PlanBuildBetaGroupAssignment(groups []ResolvedBetaGroup, opts AddBuildBetaGroupsOptions) BuildBetaGroupAssignmentPlan {
	plan := BuildBetaGroupAssignmentPlan{
		GroupsToAdd:                    make([]ResolvedBetaGroup, 0, len(groups)),
		SkippedInternalGroups:          make([]ResolvedBetaGroup, 0, len(groups)),
		SkippedInternalAllBuildsGroups: make([]ResolvedBetaGroup, 0, len(groups)),
	}
	for _, group := range groups {
		if group.IsInternalGroup && opts.SkipInternal {
			plan.SkippedInternalGroups = append(plan.SkippedInternalGroups, group)
			continue
		}
		if group.IsInternalGroup && group.HasAccessToAllBuilds && opts.SkipInternalWithAllBuilds {
			plan.SkippedInternalAllBuildsGroups = append(plan.SkippedInternalAllBuildsGroups, group)
			continue
		}
		plan.GroupsToAdd = append(plan.GroupsToAdd, group)
	}
	return plan
}
