package agentcustomizationpersistence

import (
	"context"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"strconv"
	"sync"
	d "task-processor/internal/agentcustomization"
	"task-processor/internal/product/quality"
	"testing"
)

func TestPrivatePublishAndRunAreAtomicScopedAndIdempotent(t *testing.T) {
	db, repo := fixture(t)
	s := service(t, repo)
	ctx := context.Background()
	c := submit()
	receipt, e := s.Execute(ctx, c)
	require.NoError(t, e)
	staff := d.Scope{ActorID: "platform", Platform: true}
	for _, u := range []d.Update{{Stage: d.Evaluating, Note: "评估"}, {Stage: d.Proposed, Note: "方案", Proposal: "质检"}, {Stage: d.Developing, Note: "确认", OfflineConfirmation: "线下确认"}, {Stage: d.Delivered, Note: "人工交付"}} {
		expected, e := strconv.ParseInt(receipt.Version, 10, 64)
		require.NoError(t, e)
		receipt, e = s.Execute(ctx, d.Command{Scope: staff, Key: uuid.NewString(), ID: receipt.RequestID, Operation: "progress", Expected: expected, Update: u})
		require.NoError(t, e)
	}
	list, e := s.Deliveries(ctx, c.Scope, "")
	require.NoError(t, e)
	require.Empty(t, list.Items, "manual delivered does not publish")
	publish := d.Command{Scope: staff, Key: uuid.NewString(), ID: receipt.RequestID, Operation: "progress", Expected: 5, Update: d.Update{Stage: d.Delivered, Note: "实际交付", DeliverQualityAgent: true}}
	// Failed final receipt insertion must roll back the earlier delivery insert.
	_, e = db.Exec(`CREATE FUNCTION agent_customization.reject_test() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.key='` + publish.Key + `' THEN RAISE EXCEPTION 'synthetic failure'; END IF; RETURN NEW; END $$; CREATE TRIGGER reject_receipt BEFORE INSERT ON agent_customization.commands FOR EACH ROW EXECUTE FUNCTION agent_customization.reject_test()`)
	require.NoError(t, e)
	_, e = s.Execute(ctx, publish)
	require.Error(t, e)
	list, e = s.Deliveries(ctx, c.Scope, "")
	require.NoError(t, e)
	require.Empty(t, list.Items)
	_, e = db.Exec("DROP TRIGGER reject_receipt ON agent_customization.commands")
	require.NoError(t, e)
	stale := publish
	stale.Expected = 4
	_, e = s.Execute(ctx, stale)
	require.ErrorIs(t, e, d.ErrRevision)
	receipt, e = s.Execute(ctx, publish)
	require.NoError(t, e)
	replay, e := s.Execute(ctx, publish)
	require.NoError(t, e)
	require.Equal(t, receipt, replay)
	changed := publish
	changed.Update.DeliverQualityAgent = false
	_, e = s.Execute(ctx, changed)
	require.ErrorIs(t, e, d.ErrConflict)
	list, e = s.Deliveries(ctx, c.Scope, "")
	require.NoError(t, e)
	require.Len(t, list.Items, 1)
	delivery := list.Items[0]
	require.Equal(t, c.Scope.OrganizationID, delivery.OrganizationID)
	require.Equal(t, d.QualityDefinition, delivery.Definition)
	require.Equal(t, d.QualityVersion, delivery.Version)
	publish.Key = uuid.NewString()
	publish.Expected = 6
	_, e = s.Execute(ctx, publish)
	require.NoError(t, e)
	list, e = s.Deliveries(ctx, c.Scope, "")
	require.NoError(t, e)
	require.Len(t, list.Items, 1)
	foreign := d.Scope{ActorID: "other", OrganizationID: "org-b"}
	_, e = s.Delivery(ctx, foreign, delivery.ID)
	require.ErrorIs(t, e, d.ErrNotFound)
	_, e = s.Delivery(ctx, staff, delivery.ID)
	require.ErrorIs(t, e, d.ErrForbidden)
	command := d.RunCommand{Scope: c.Scope, Key: uuid.NewString(), DeliveryID: delivery.ID, Input: quality.Input{Name: "盒子", Specifications: []quality.Specification{}}}
	copies := make([]d.QualityRun, 8)
	errs := make([]error, 8)
	var wg sync.WaitGroup
	for i := range copies {
		wg.Add(1)
		go func(i int) { defer wg.Done(); copies[i], errs[i] = s.RunQuality(ctx, command) }(i)
	}
	wg.Wait()
	for i := range copies {
		require.NoError(t, errs[i])
		require.Equal(t, copies[0], copies[i])
	}
	bad := command
	bad.Input.Name = "不同输入"
	_, e = s.RunQuality(ctx, bad)
	require.ErrorIs(t, e, d.ErrConflict)
	bad = command
	bad.Scope = foreign
	_, e = s.RunQuality(ctx, bad)
	require.ErrorIs(t, e, d.ErrNotFound)
	reports, e := s.QualityRuns(ctx, c.Scope, delivery.ID, "")
	require.NoError(t, e)
	require.Len(t, reports.Items, 1)
	require.Equal(t, copies[0], reports.Items[0])
	require.Equal(t, command.Input, reports.Items[0].Input)
	require.NotEmpty(t, reports.Items[0].Report.Findings)
	// Reconstructing the owner reads the durable report; no memory execution store.
	fresh, e := d.NewService(repo)
	require.NoError(t, e)
	saved, e := fresh.QualityRuns(ctx, c.Scope, delivery.ID, "")
	require.NoError(t, e)
	require.Equal(t, reports, saved)
}
