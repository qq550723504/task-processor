package store

import (
	"context"
	"encoding/json"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"task-processor/internal/imageagent"
	"testing"
)

func TestRecentImageSetsBindOrganizationActorMemberAndCursor(t *testing.T) {
	repo := NewOrganizationRepository(newConcurrentSQLite(t)).(*gormRepository)
	ids := map[string]string{}
	for _, owner := range []struct{ org, actor, member string }{{"tenant-a", "user-1", "member-1"}, {"tenant-a", "user-1", "member-1"}, {"tenant-b", "user-1", "member-1"}, {"tenant-a", "other", "member-1"}, {"tenant-a", "user-1", "replacement"}} {
		run := manualRun(uuid.NewString(), owner.org)
		run.UserID = owner.actor
		run.MemberID = owner.member
		run.ScopeProtocol = imageagent.OrganizationScopeProtocol
		run.BusinessTaskID = "operation-1"
		plan := imageSetPlanForStore(t)
		scope := imageagent.ScopeForRun(*run)
		_, err := repo.InitializeRun(context.Background(), imageagent.ProjectionInitialization{Scope: scope, Run: *run, Plan: plan, Catalog: imageagent.AssetCatalog{Assets: []imageagent.AuthorizedAsset{{ID: "source-1", Type: imageagent.AuthorizedAssetSource, URL: "https://images.example.org/source.png"}}}, Snapshot: imageagent.RunProjection{Run: *run, Plan: plan}, CommitID: "start", EventType: "run.initialized", EventPayload: json.RawMessage(`{}`)})
		require.NoError(t, err)
		ids[owner.member] = run.ID
	}
	identity := imageagent.ExecutionIdentity{ScopeProtocol: imageagent.OrganizationScopeProtocol, TenantID: "tenant-a", UserID: "user-1", MemberID: "member-1"}
	first, next, err := repo.ListImageSets(context.Background(), identity, "operation-1", "", 1)
	require.NoError(t, err)
	require.Len(t, first, 1)
	require.NotEmpty(t, next)
	second, last, err := repo.ListImageSets(context.Background(), identity, "operation-1", next, 1)
	require.NoError(t, err)
	require.Len(t, second, 1)
	require.Empty(t, last)
	require.NotEqual(t, first[0].RunID, second[0].RunID)
	_, _, err = repo.ListImageSets(context.Background(), identity, "operation-1", ids["replacement"], 1)
	require.Error(t, err)
}
