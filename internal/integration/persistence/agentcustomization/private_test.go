package agentcustomizationpersistence

import (
	"context"
	"encoding/json"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	d "task-processor/internal/agentcustomization"
	"testing"
	"time"
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
	inspector := &privateDraftFixture{owner: c.Scope, snapshot: testDraft()}
	s.WithDrafts(inspector)
	command := d.RunCommand{Scope: c.Scope, Key: uuid.NewString(), DeliveryID: delivery.ID, Input: d.DraftSelection{RecordID: inspector.snapshot.RecordID, ExpectedRevision: 1}}
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
	bad.Input.ExpectedRevision = 2
	_, e = s.RunQuality(ctx, bad)
	require.ErrorIs(t, e, d.ErrConflict)
	bad = command
	bad.Scope = foreign
	_, e = s.RunQuality(ctx, bad)
	require.ErrorIs(t, e, d.ErrNotFound)
	reports, e := s.QualityRuns(ctx, c.Scope, delivery.ID, "")
	require.NoError(t, e)
	require.Len(t, reports.Items, 1)
	require.Equal(t, copies[0].ID, reports.Items[0].ID)
	require.Equal(t, 1, reports.Items[0].FindingCount)
	single, e := s.QualityRun(ctx, c.Scope, delivery.ID, copies[0].ID)
	require.NoError(t, e)
	require.Equal(t, copies[0], single)
	otherActor := c.Scope
	otherActor.ActorID = "another-member"
	private, e := s.QualityRuns(ctx, otherActor, delivery.ID, "")
	require.NoError(t, e)
	require.Empty(t, private.Items)
	_, e = s.QualityRun(ctx, otherActor, delivery.ID, single.ID)
	require.ErrorIs(t, e, d.ErrNotFound)
	inspector.stale.Store(true)
	replayedRun, e := s.RunQuality(ctx, command)
	require.NoError(t, e)
	require.Equal(t, single, replayedRun, "head drift cannot replace the original receipt")
	newCommand := command
	newCommand.Key = uuid.NewString()
	_, e = s.RunQuality(ctx, newCommand)
	require.ErrorIs(t, e, d.ErrRevision)
	inspector.denied.Store(true)
	_, e = s.RunQuality(ctx, command)
	require.ErrorIs(t, e, d.ErrForbidden)
	_, e = s.QualityRuns(ctx, c.Scope, delivery.ID, "")
	require.ErrorIs(t, e, d.ErrForbidden)
	_, e = s.QualityRun(ctx, c.Scope, delivery.ID, single.ID)
	require.ErrorIs(t, e, d.ErrForbidden)
	inspector.denied.Store(false)
	// Reconstructing the owner reads the durable report; no memory execution store.
	fresh, e := d.NewService(repo)
	require.NoError(t, e)
	fresh.WithDrafts(inspector)
	saved, e := fresh.QualityRuns(ctx, c.Scope, delivery.ID, "")
	require.NoError(t, e)
	require.Equal(t, reports, saved)
	// A failed insert must not fabricate a successful report or lose the key.
	failed := command
	failed.Key = uuid.NewString()
	_, e = db.Exec(`CREATE FUNCTION agent_customization.reject_run_test() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.key='` + failed.Key + `' THEN RAISE EXCEPTION 'synthetic failure'; END IF; RETURN NEW; END $$; CREATE TRIGGER reject_run BEFORE INSERT ON agent_customization.quality_runs FOR EACH ROW EXECUTE FUNCTION agent_customization.reject_run_test()`)
	require.NoError(t, e)
	inspector.stale.Store(false)
	_, e = s.RunQuality(ctx, failed)
	require.Error(t, e)
	var count int
	require.NoError(t, db.QueryRow("SELECT count(*) FROM agent_customization.quality_runs WHERE key=$1", failed.Key).Scan(&count))
	require.Zero(t, count)
	_, e = db.Exec("DROP TRIGGER reject_run ON agent_customization.quality_runs")
	require.NoError(t, e)
	// Full issue lists are retained once; pages expose only bounded summaries.
	inspector.snapshot.Issues = make([]d.DraftIssue, 4096)
	for i := range inspector.snapshot.Issues {
		inspector.snapshot.Issues[i] = d.DraftIssue{Code: "missing", Field: "category_id", Message: "完整问题不得在摘要或详情中截断"}
	}
	for i := 0; i < 21; i++ {
		failed.Key = uuid.NewString()
		_, e = s.RunQuality(ctx, failed)
		require.NoError(t, e)
	}
	firstPage, e := s.QualityRuns(ctx, c.Scope, delivery.ID, "")
	require.NoError(t, e)
	require.Len(t, firstPage.Items, 20)
	require.NotEmpty(t, firstPage.NextCursor)
	lastPage, e := s.QualityRuns(ctx, c.Scope, delivery.ID, firstPage.NextCursor)
	require.NoError(t, e)
	require.Len(t, lastPage.Items, 2)
	require.Empty(t, lastPage.NextCursor)
	summaryJSON, e := json.Marshal(firstPage)
	require.NoError(t, e)
	require.Less(t, len(summaryJSON), 64<<10)
	require.NotContains(t, string(summaryJSON), "完整问题")
	full, e := s.QualityRun(ctx, c.Scope, delivery.ID, firstPage.Items[0].ID)
	require.NoError(t, e)
	if firstPage.Items[0].FindingCount == 4096 {
		require.Len(t, full.Draft.Issues, 4096)
	} else {
		full, e = s.QualityRun(ctx, c.Scope, delivery.ID, firstPage.Items[1].ID)
		require.NoError(t, e)
		require.Len(t, full.Draft.Issues, 4096)
	}
}

func TestPrivateRecordedTrialRemainsReadOnly(t *testing.T) {
	db, repo := fixture(t)
	s := service(t, repo)
	ctx := context.Background()
	c := submit()
	request, e := s.Execute(ctx, c)
	require.NoError(t, e)
	delivery := d.Delivery{ID: uuid.NewString(), RequestID: request.RequestID, OrganizationID: c.Scope.OrganizationID, Definition: d.QualityDefinition, Version: "1.0.0", Name: "历史手工试用", CreatedBy: "staff", CreatedAt: time.Now().UTC()}
	raw, e := json.Marshal(delivery)
	require.NoError(t, e)
	_, e = db.Exec("INSERT INTO agent_customization.deliveries(id,request_id,organization_id,payload) VALUES($1,$2,$3,$4)", delivery.ID, delivery.RequestID, delivery.OrganizationID, raw)
	require.NoError(t, e)
	old := d.QualityRun{ID: uuid.NewString(), DeliveryID: delivery.ID, OrganizationID: c.Scope.OrganizationID, ActorID: c.Scope.ActorID, Key: uuid.NewString(), Definition: d.QualityDefinition, Version: "1.0.0", Input: &d.RecordedInput{Name: "原记录"}, Report: &d.RecordedReport{RuleVersion: "1.0.0", Summary: "原提示"}, CreatedAt: time.Now().UTC()}
	raw, e = json.Marshal(old)
	require.NoError(t, e)
	_, e = db.Exec("INSERT INTO agent_customization.quality_runs(id,delivery_id,organization_id,actor_id,key,fingerprint,payload) VALUES($1,$2,$3,$4,$5,$6,$7)", old.ID, old.DeliveryID, old.OrganizationID, old.ActorID, old.Key, strings.Repeat("a", 64), raw)
	require.NoError(t, e)
	read, e := s.QualityRun(ctx, c.Scope, delivery.ID, old.ID)
	require.NoError(t, e)
	require.Equal(t, string(raw), mustJSON(t, read))
	page, e := s.QualityRuns(ctx, c.Scope, delivery.ID, "")
	require.NoError(t, e)
	require.Len(t, page.Items, 1)
	require.Equal(t, "1.0.0", page.Items[0].Version)
	_, e = s.RunQuality(ctx, d.RunCommand{Scope: c.Scope, Key: uuid.NewString(), DeliveryID: delivery.ID, Input: d.DraftSelection{RecordID: uuid.NewString(), ExpectedRevision: 1}})
	require.ErrorIs(t, e, d.ErrConflict)
	other := c.Scope
	other.ActorID = "other-member"
	_, e = s.QualityRun(ctx, other, delivery.ID, old.ID)
	require.ErrorIs(t, e, d.ErrNotFound)
}

func mustJSON(t *testing.T, v any) string {
	t.Helper()
	b, e := json.Marshal(v)
	require.NoError(t, e)
	return string(b)
}

type privateDraftFixture struct {
	owner         d.Scope
	snapshot      d.DraftSnapshot
	stale, denied atomic.Bool
}

func (f *privateDraftFixture) Inspect(_ context.Context, scope d.Scope, in d.DraftSelection, head bool) (d.DraftSnapshot, error) {
	if scope != f.owner || f.denied.Load() {
		return d.DraftSnapshot{}, d.ErrForbidden
	}
	if in.RecordID != f.snapshot.RecordID {
		return d.DraftSnapshot{}, d.ErrNotFound
	}
	if in.ExpectedRevision != f.snapshot.Revision || head && f.stale.Load() {
		return d.DraftSnapshot{}, d.ErrRevision
	}
	return f.snapshot, nil
}
func testDraft() d.DraftSnapshot {
	h := strings.Repeat("a", 64)
	return d.DraftSnapshot{DraftBinding: d.DraftBinding{RecordID: uuid.NewString(), Revision: 1, SourceID: uuid.NewString(), PreparationID: uuid.NewString(), StoreID: uuid.NewString(), Platform: "shein", Site: "shein-us", ProductKey: "own:fixture", ProductVersion: "1", Title: "收纳盒", RecordHash: h, ProductHash: h, RulesHash: h, InventoryHash: h, SavedAt: time.Now().UTC()}, Issues: []d.DraftIssue{{Code: "missing", Field: "category_id", Message: "选择当前店铺可发布的末级类目"}}}
}
