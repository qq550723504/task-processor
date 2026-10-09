package temporal

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"task-processor/internal/imageagent"
)

func TestImageSetTemporalStartRequiresOriginalAdmissionAndPreservesItsDeadline(t *testing.T) {
	_, repo, input := imageSetPersistenceFixture(t)
	current, err := repo.GetProjection(context.Background(), imageagent.RunScope{TenantID: input.Identity.TenantID, OwnerUserID: input.Identity.UserID, RunID: input.RunID})
	require.NoError(t, err)
	raw := &recordingSDKClient{}
	start := imageagent.WorkflowStart{Run: current.Run, Plan: current.Plan, Identity: input.Identity, AssetCatalog: current.AssetCatalog, MaxConcurrentSlots: 1}
	require.NoError(t, NewOrganizationClient(raw).StartManual(context.Background(), start))
	require.Equal(t, current.Run.ImageAdmission.AdmittedAt, raw.workflowInput.StartedAt)
	require.Equal(t, current.Run.ImageAdmission.Deadline, raw.workflowInput.DeadlineAt)
	require.Nil(t, raw.workflowInput.ImagePolicyContext)
	for _, kind := range []string{"missing_admission", "changed_time", "changed_budget", "changed_member"} {
		t.Run(kind, func(t *testing.T) {
			bad := start
			switch kind {
			case "missing_admission":
				bad.Run.ImageAdmission = nil
			case "changed_time":
				bad.Run.StartedAt = bad.Run.StartedAt.Add(1)
			case "changed_budget":
				bad.Run.Budget.MaxImages++
			case "changed_member":
				bad.Run.MemberID = "other"
			}
			rejected := &recordingSDKClient{}
			require.Error(t, NewOrganizationClient(rejected).StartManual(context.Background(), bad))
			require.Empty(t, rejected.workflowName)
		})
	}
}

func TestSetExecutionRejectsProjectionWithNoWholeRunAdmission(t *testing.T) {
	a, repo, input := imageSetPersistenceFixture(t)
	current, err := repo.GetProjection(context.Background(), imageagent.RunScope{TenantID: input.Identity.TenantID, OwnerUserID: input.Identity.UserID, RunID: input.RunID})
	require.NoError(t, err)
	current.Run.ImageAdmission = nil
	a.repository = setPlanRepository{projection: current}
	require.ErrorIs(t, a.validatePersistedImageSetExecution(context.Background(), slotExecutionInputV3(input)), imageagent.ErrRevisionConflict)
}
