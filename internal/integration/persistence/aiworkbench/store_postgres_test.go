//go:build integration

package aiworkbenchpersistence

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"task-processor/internal/agent"
	"task-processor/internal/aiworkbench"
)

func workbenchDB(t *testing.T) *gorm.DB {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	t.Cleanup(cancel)
	container, err := tcpostgres.Run(ctx, "postgres:16-alpine",
		tcpostgres.WithDatabase("workbench588"),
		tcpostgres.WithUsername("workbench588"),
		tcpostgres.WithPassword("workbench588"),
		tcpostgres.BasicWaitStrategies())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, container.Terminate(context.Background())) })
	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	t.Cleanup(func() {
		raw, err := db.DB()
		if err == nil {
			require.NoError(t, raw.Close())
		}
	})
	require.NoError(t, InstallSchema(db))
	return db
}

func TestMetadataAuditCommitsWithConversationRevision(t *testing.T) {
	db := workbenchDB(t)
	store, err := New(db)
	require.NoError(t, err)
	ctx := context.Background()
	scope := aiworkbench.Scope{OrganizationID: "org-a", ActorID: "user-a"}
	created, _, err := store.Create(ctx, scope, uuid.NewString(), aiworkbench.CreateInput{})
	require.NoError(t, err)
	title, favorite, archived := "Edited title", true, true
	changed, err := store.SetMetadata(ctx, scope, created.ID, created.MetadataRevision,
		aiworkbench.MetadataChange{Title: &title, Favorite: &favorite, Archived: &archived})
	require.NoError(t, err)
	require.Equal(t, created.MetadataRevision+1, changed.MetadataRevision)
	var receipt struct {
		OrganizationID   string
		ActorID          string
		ConversationID   string
		MetadataRevision uint64
		ChangeMask       int
		CreatedAt        time.Time
	}
	require.NoError(t, db.Table("ai_workbench.metadata_audit").Where("conversation_id = ?", created.ID).Take(&receipt).Error)
	require.Equal(t, scope.OrganizationID, receipt.OrganizationID)
	require.Equal(t, scope.ActorID, receipt.ActorID)
	require.Equal(t, created.ID, receipt.ConversationID)
	require.Equal(t, changed.MetadataRevision, receipt.MetadataRevision)
	require.Equal(t, 7, receipt.ChangeMask)
	require.False(t, receipt.CreatedAt.IsZero())

	require.NoError(t, db.Exec(`CREATE FUNCTION ai_workbench.reject_metadata_audit_test() RETURNS trigger LANGUAGE plpgsql AS $$
		BEGIN RAISE EXCEPTION 'forced metadata audit failure'; END $$`).Error)
	require.NoError(t, db.Exec(`CREATE TRIGGER reject_metadata_audit_test BEFORE INSERT ON ai_workbench.metadata_audit
		FOR EACH ROW EXECUTE FUNCTION ai_workbench.reject_metadata_audit_test()`).Error)
	nextTitle := "Must not commit"
	_, err = store.SetMetadata(ctx, scope, created.ID, changed.MetadataRevision, aiworkbench.MetadataChange{Title: &nextTitle})
	require.Error(t, err)
	current, err := store.Get(ctx, scope, created.ID)
	require.NoError(t, err)
	require.Equal(t, changed.Title, current.Title)
	require.Equal(t, changed.MetadataRevision, current.MetadataRevision)
	var count int64
	require.NoError(t, db.Table("ai_workbench.metadata_audit").Where("conversation_id = ?", created.ID).Count(&count).Error)
	require.EqualValues(t, 1, count)
}

func TestWorkbenchRuntimeRoleCanUseReceiptsWithoutDDLOrDeletes(t *testing.T) {
	db := workbenchDB(t)
	const role = "ai_workbench_runtime"
	require.NoError(t, db.Exec("CREATE ROLE "+role+" LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS").Error)
	require.NoError(t, GrantRuntime(db, role))
	var granted bool
	require.NoError(t, db.Raw("SELECT has_schema_privilege(?,'ai_workbench','CREATE')", role).Scan(&granted).Error)
	require.False(t, granted)
	require.NoError(t, db.Raw("SELECT has_table_privilege(?,'ai_workbench.business_tasks','DELETE')", role).Scan(&granted).Error)
	require.False(t, granted)
	require.NoError(t, db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("SET LOCAL ROLE " + role).Error; err != nil {
			return err
		}
		if err := VerifySchema(context.Background(), tx); err != nil {
			return err
		}
		store, err := New(tx)
		if err != nil {
			return err
		}
		scope := aiworkbench.Scope{OrganizationID: "B", ActorID: "operator"}
		conversation, _, err := store.Create(context.Background(), scope, uuid.NewString(), aiworkbench.CreateInput{})
		if err != nil {
			return err
		}
		title := "Updated by runtime"
		if _, err := store.SetMetadata(context.Background(), scope, conversation.ID, conversation.MetadataRevision,
			aiworkbench.MetadataChange{Title: &title}); err != nil {
			return err
		}
		key := uuid.NewString()
		if _, _, err = store.AppendUser(context.Background(), scope, conversation.ID, key,
			aiworkbench.MessageInput{Content: "Improve title", OperationID: "op-1", TargetPlatform: "shein"}, testPlanPreparer); err != nil {
			return err
		}
		proposal := &aiworkbench.ExecutionProposal{Kind: "product.title.optimize", GoalSummary: "Improve title", OperationID: "op-1",
			ProductKey: "product-1", CatalogVersion: "1", PublicationID: "publication-1", TargetPlatform: "shein",
			AgentID: "product.title.agent", AgentVersion: "v1.0.0", ObservedAgentRevision: "revision-1",
			ObservedActivationEpoch: "epoch-1", ExecutionModelProfile: []byte(`{"profile_id":"execution-v1"}`)}
		planned, _, err := store.CompletePlan(context.Background(), scope, key,
			aiworkbench.PlanTerminal{AssistantText: "Ready for review", Mode: aiworkbench.PlanReady, GoalSummary: proposal.GoalSummary, Proposal: proposal})
		if err != nil {
			return err
		}
		saved, err := store.GetProposal(context.Background(), scope, planned.ProposalID)
		if err != nil {
			return err
		}
		confirmKey := uuid.NewString()
		businessTask, _, err := store.Confirm(context.Background(), scope, conversation.ID, saved.ID, confirmKey, preparedTaskFixture(t, saved, confirmKey))
		if err != nil {
			return err
		}
		receipt, replay, err := store.BeginTaskAction(context.Background(), scope, aiworkbench.TaskActionInput{
			TaskID: businessTask.ID, Key: uuid.NewString(), Action: aiworkbench.TaskActionStart})
		if err != nil || replay || receipt.State != aiworkbench.TaskActionClaimed {
			return aiworkbench.ErrUnavailable
		}
		return store.FinishTaskAction(context.Background(), scope, receipt.Key, aiworkbench.TaskActionComplete)
	}))
}

func TestWorkbenchRuntimeRoleRejectsInheritedCrossOwnerPrivileges(t *testing.T) {
	db := workbenchDB(t)
	const workbenchRole = "ai_workbench_runtime"
	const productRole = "product_agent_runtime"
	require.NoError(t, db.Exec("CREATE ROLE "+workbenchRole+" LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS").Error)
	require.NoError(t, db.Exec("CREATE ROLE "+productRole+" LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS").Error)
	require.NoError(t, db.Exec("CREATE SCHEMA product_agent_boundary").Error)
	require.NoError(t, db.Exec("CREATE TABLE product_agent_boundary.private_fact (id integer PRIMARY KEY)").Error)
	require.NoError(t, db.Exec("GRANT USAGE ON SCHEMA product_agent_boundary TO "+productRole).Error)
	require.NoError(t, db.Exec("GRANT SELECT ON product_agent_boundary.private_fact TO "+productRole).Error)

	require.NoError(t, db.Exec("GRANT "+productRole+" TO "+workbenchRole).Error)
	var granted bool
	require.NoError(t, db.Raw("SELECT has_table_privilege(?,'product_agent_boundary.private_fact','SELECT')", workbenchRole).Scan(&granted).Error)
	require.True(t, granted, "role membership confers cross-owner read access")
	require.ErrorIs(t, GrantRuntime(db, workbenchRole), aiworkbench.ErrInvalid)
	require.NoError(t, db.Exec("REVOKE "+productRole+" FROM "+workbenchRole).Error)
	require.NoError(t, GrantRuntime(db, workbenchRole))

	require.NoError(t, db.Exec("GRANT "+workbenchRole+" TO "+productRole).Error)
	require.NoError(t, db.Raw("SELECT has_table_privilege(?,'ai_workbench.conversations','SELECT')", productRole).Scan(&granted).Error)
	require.True(t, granted, "the Product Agent login can inherit private Workbench reads")
	require.ErrorIs(t, GrantRuntime(db, workbenchRole), aiworkbench.ErrInvalid)
	require.ErrorIs(t, db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("SET LOCAL ROLE " + workbenchRole).Error; err != nil {
			return err
		}
		return VerifySchema(context.Background(), tx)
	}), aiworkbench.ErrUnavailable, "serving startup must reject role membership added after initialization")
}

func TestWorkbenchRuntimeRoleRejectsDirectCrossOwnerPrivileges(t *testing.T) {
	db := workbenchDB(t)
	const role = "ai_workbench_runtime"
	require.NoError(t, db.Exec("CREATE ROLE "+role+" LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS").Error)
	require.NoError(t, db.Exec("CREATE SCHEMA product_agent_boundary").Error)
	require.NoError(t, db.Exec("CREATE TABLE product_agent_boundary.private_fact (id integer PRIMARY KEY, secret text)").Error)
	require.NoError(t, db.Exec("GRANT USAGE ON SCHEMA product_agent_boundary TO "+role).Error)
	require.NoError(t, db.Exec("GRANT SELECT ON product_agent_boundary.private_fact TO "+role).Error)
	require.ErrorIs(t, GrantRuntime(db, role), aiworkbench.ErrInvalid)
	require.NoError(t, db.Exec("REVOKE SELECT ON product_agent_boundary.private_fact FROM "+role).Error)
	require.NoError(t, GrantRuntime(db, role))
	require.NoError(t, db.Exec("GRANT SELECT (secret) ON product_agent_boundary.private_fact TO "+role).Error)
	require.ErrorIs(t, db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("SET LOCAL ROLE " + role).Error; err != nil {
			return err
		}
		return VerifySchema(context.Background(), tx)
	}), aiworkbench.ErrUnavailable, "serving startup must reject a later column grant")
	require.NoError(t, db.Exec("REVOKE SELECT (secret) ON product_agent_boundary.private_fact FROM "+role).Error)
	require.NoError(t, db.Exec("CREATE SEQUENCE product_agent_boundary.private_sequence").Error)
	require.NoError(t, db.Exec("GRANT USAGE ON SEQUENCE product_agent_boundary.private_sequence TO "+role).Error)
	require.ErrorIs(t, db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("SET LOCAL ROLE " + role).Error; err != nil {
			return err
		}
		return VerifySchema(context.Background(), tx)
	}), aiworkbench.ErrUnavailable, "serving startup must reject a later sequence grant")
}

func TestWorkbenchRuntimeRoleRejectsLaterInSchemaPrivilegeDrift(t *testing.T) {
	db := workbenchDB(t)
	const role = "ai_workbench_runtime"
	require.NoError(t, db.Exec("CREATE ROLE "+role+" LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS").Error)
	require.NoError(t, GrantRuntime(db, role))
	verify := func() error {
		return db.Transaction(func(tx *gorm.DB) error {
			if err := tx.Exec("SET LOCAL ROLE " + role).Error; err != nil {
				return err
			}
			return VerifySchema(context.Background(), tx)
		})
	}
	require.NoError(t, verify())
	for _, tc := range []struct{ grant, revoke string }{
		{"CREATE ON SCHEMA ai_workbench", "CREATE ON SCHEMA ai_workbench"},
		{"DELETE ON ai_workbench.business_tasks", "DELETE ON ai_workbench.business_tasks"},
		{"TRUNCATE ON ai_workbench.conversations", "TRUNCATE ON ai_workbench.conversations"},
		{"TRIGGER ON ai_workbench.conversations", "TRIGGER ON ai_workbench.conversations"},
		{"UPDATE (content) ON ai_workbench.messages", "UPDATE (content) ON ai_workbench.messages"},
		{"UPDATE ON ai_workbench.conversations", "UPDATE ON ai_workbench.conversations"},
	} {
		require.NoError(t, db.Exec("GRANT "+tc.grant+" TO "+role).Error)
		require.ErrorIs(t, verify(), aiworkbench.ErrUnavailable, "unsafe runtime grant: %s", tc.grant)
		require.NoError(t, db.Exec("REVOKE "+tc.revoke+" FROM "+role).Error)
		if tc.grant == "UPDATE ON ai_workbench.conversations" {
			require.NoError(t, GrantRuntime(db, role), "REVOKE UPDATE also removes column grants in this fixture")
		}
		require.NoError(t, verify(), "restored after %s", tc.grant)
	}
	for _, tc := range []struct{ privilege string }{
		{"USAGE ON SCHEMA ai_workbench"},
		{"INSERT ON ai_workbench.messages"},
		{"SELECT ON ai_workbench.business_tasks"},
		{"UPDATE (state) ON ai_workbench.commands"},
	} {
		require.NoError(t, db.Exec("REVOKE "+tc.privilege+" FROM "+role).Error)
		require.ErrorIs(t, verify(), aiworkbench.ErrUnavailable, "missing runtime grant: %s", tc.privilege)
		require.NoError(t, db.Exec("GRANT "+tc.privilege+" TO "+role).Error)
		require.NoError(t, verify())
	}
}

func TestTaskActionReceiptReplaysCommittedResultWithoutReclaim(t *testing.T) {
	db := workbenchDB(t)
	store, err := New(db)
	require.NoError(t, err)
	ctx := context.Background()
	scope := aiworkbench.Scope{OrganizationID: "B", ActorID: "operator"}
	conversation, proposal := readyTaskFixture(t, store, scope)
	confirmKey := uuid.NewString()
	task, _, err := store.Confirm(ctx, scope, conversation.ID, proposal.ID, confirmKey, preparedTaskFixture(t, proposal, confirmKey))
	require.NoError(t, err)
	input := aiworkbench.TaskActionInput{TaskID: task.ID, Key: uuid.NewString(), Action: aiworkbench.TaskActionResume,
		Revision: 2, Feedback: "Use the verified source wording"}
	claimed, replay, err := store.BeginTaskAction(ctx, scope, input)
	require.NoError(t, err)
	require.False(t, replay)
	require.Equal(t, aiworkbench.TaskActionClaimed, claimed.State)
	current, replay, err := store.BeginTaskAction(ctx, scope, input)
	require.NoError(t, err)
	require.True(t, replay)
	require.Equal(t, claimed, current)
	changed := input
	changed.Feedback = "A different response"
	_, _, err = store.BeginTaskAction(ctx, scope, changed)
	require.ErrorIs(t, err, aiworkbench.ErrIdempotencyConflict)
	changed = input
	changed.Action = aiworkbench.TaskActionStart
	changed.Revision, changed.Feedback = 0, ""
	_, _, err = store.BeginTaskAction(ctx, scope, changed)
	require.ErrorIs(t, err, aiworkbench.ErrIdempotencyConflict)
	require.NoError(t, store.FinishTaskAction(ctx, scope, input.Key, aiworkbench.TaskActionComplete))
	completed, replay, err := store.BeginTaskAction(ctx, scope, input)
	require.NoError(t, err)
	require.True(t, replay)
	require.Equal(t, aiworkbench.TaskActionComplete, completed.State)
	failedInput := aiworkbench.TaskActionInput{TaskID: task.ID, Key: uuid.NewString(), Action: aiworkbench.TaskActionStart}
	_, replay, err = store.BeginTaskAction(ctx, scope, failedInput)
	require.NoError(t, err)
	require.False(t, replay)
	require.NoError(t, store.FinishTaskAction(ctx, scope, failedInput.Key, aiworkbench.TaskActionFailed, "REVISION_MISMATCH"))
	failed, replay, err := store.BeginTaskAction(ctx, scope, failedInput)
	require.NoError(t, err)
	require.True(t, replay)
	require.Equal(t, aiworkbench.TaskActionFailed, failed.State)
	require.Equal(t, "REVISION_MISMATCH", failed.ErrorCode)
	unknownInput := aiworkbench.TaskActionInput{TaskID: task.ID, Key: uuid.NewString(), Action: aiworkbench.TaskActionReview}
	_, replay, err = store.BeginTaskAction(ctx, scope, unknownInput)
	require.NoError(t, err)
	require.False(t, replay)
	require.NoError(t, store.FinishTaskAction(ctx, scope, unknownInput.Key, aiworkbench.TaskActionUnknown))
	unknown, replay, err := store.BeginTaskAction(ctx, scope, unknownInput)
	require.NoError(t, err)
	require.True(t, replay)
	require.Equal(t, aiworkbench.TaskActionUnknown, unknown.State)
	require.ErrorIs(t, store.FinishTaskAction(ctx, scope, unknownInput.Key, aiworkbench.TaskActionComplete), aiworkbench.ErrIdempotencyConflict)
	_, _, err = store.BeginTaskAction(ctx, aiworkbench.Scope{OrganizationID: "A", ActorID: "operator"}, input)
	require.ErrorIs(t, err, aiworkbench.ErrNotFound)
}

func readyTaskFixture(t *testing.T, store *Store, scope aiworkbench.Scope) (aiworkbench.Conversation, aiworkbench.ExecutionProposal) {
	t.Helper()
	ctx := context.Background()
	conversation, _, err := store.Create(ctx, scope, uuid.NewString(), aiworkbench.CreateInput{})
	require.NoError(t, err)
	key := uuid.NewString()
	_, _, err = store.AppendUser(ctx, scope, conversation.ID, key,
		aiworkbench.MessageInput{Content: "Improve title", OperationID: "op-1", TargetPlatform: "shein"}, testPlanPreparer)
	require.NoError(t, err)
	proposal := &aiworkbench.ExecutionProposal{Kind: "product.title.optimize", GoalSummary: "Improve title",
		OperationID: "op-1", ProductKey: "product-1", CatalogVersion: "1", PublicationID: "publication-1",
		TargetPlatform: "shein", AgentID: "product.title.agent", AgentVersion: "v1.0.0",
		ObservedAgentRevision: "revision-1", ObservedActivationEpoch: "epoch-1",
		ExecutionModelProfile: []byte(`{"profile_id":"execution-v1"}`)}
	completed, _, err := store.CompletePlan(ctx, scope, key, aiworkbench.PlanTerminal{
		AssistantText: "Review the proposed work", Mode: aiworkbench.PlanReady, GoalSummary: proposal.GoalSummary, Proposal: proposal})
	require.NoError(t, err)
	saved, err := store.GetProposal(ctx, scope, completed.ProposalID)
	require.NoError(t, err)
	return conversation, saved
}

func preparedTaskFixture(t *testing.T, p aiworkbench.ExecutionProposal, key string) aiworkbench.PreparedTask {
	t.Helper()
	ref := agent.ConfigurationSnapshotRef{Kind: "agent-configuration-v1", ID: uuid.NewString(), Digest: fmt.Sprintf("%064x", 99)}
	request := agent.Request{Key: key,
		Binding: agent.Binding{ContextKind: "acquisition", ContextID: p.OperationID, ProductKey: p.ProductKey,
			CatalogVersion: p.CatalogVersion, PublicationID: p.PublicationID, TargetPlatform: p.TargetPlatform},
		ConfigurationSnapshotRef: ref, GoalSummary: p.GoalSummary, PolicyVersion: "policy-v1", PromptVersion: "prompt-v1",
		Limits: agent.Limits{Steps: 12, ModelCalls: 6, Tokens: 500000, CostMicros: 500000, Currency: "USD", Runtime: time.Minute}}
	raw, err := json.Marshal(request)
	require.NoError(t, err)
	return aiworkbench.PreparedTask{ProposalDigest: p.Digest, ConfigurationSnapshotRef: ref, ExecutionRequest: raw}
}

func TestConfirmReceiptPrecedesLaterStalenessAndKeepsOneTask(t *testing.T) {
	store, err := New(workbenchDB(t))
	require.NoError(t, err)
	ctx := context.Background()
	scope := aiworkbench.Scope{OrganizationID: "org-a", ActorID: "actor-a"}
	conversation, proposal := readyTaskFixture(t, store, scope)
	key := uuid.NewString()
	prepared := preparedTaskFixture(t, proposal, key)
	var changedRequest agent.Request
	require.NoError(t, json.Unmarshal(prepared.ExecutionRequest, &changedRequest))
	changedRequest.GoalSummary = "A different title goal"
	changed := prepared
	changed.ExecutionRequest, err = json.Marshal(changedRequest)
	require.NoError(t, err)
	_, _, err = store.Confirm(ctx, scope, conversation.ID, proposal.ID, key, changed)
	require.ErrorIs(t, err, aiworkbench.ErrInvalid, "the confirmed proposal goal must match the frozen Agent request")
	task, replay, err := store.Confirm(ctx, scope, conversation.ID, proposal.ID, key, prepared)
	require.NoError(t, err)
	require.False(t, replay)
	require.Equal(t, proposal.Digest, task.ProposalDigest)
	require.Equal(t, key, task.ExecutionRequestKey)
	require.NotEmpty(t, task.ExecutionRequestDigest)
	_, _, err = store.AppendUser(ctx, scope, conversation.ID, uuid.NewString(),
		aiworkbench.MessageInput{Content: "A new goal", OperationID: "op-1", TargetPlatform: "shein"}, testPlanPreparer)
	require.NoError(t, err)
	_, _, err = store.Confirm(ctx, scope, conversation.ID, proposal.ID, uuid.NewString(), prepared)
	require.ErrorIs(t, err, aiworkbench.ErrRevisionMismatch)
	archived := true
	_, err = store.SetMetadata(ctx, scope, conversation.ID, conversation.MetadataRevision, aiworkbench.MetadataChange{Archived: &archived})
	require.NoError(t, err)
	replayed, duplicate, err := store.Confirm(ctx, scope, conversation.ID, proposal.ID, key, aiworkbench.PreparedTask{})
	require.NoError(t, err)
	require.True(t, duplicate)
	require.Equal(t, task.ID, replayed.ID)
	preflightFailure := errors.New("agent or knowledge changed after lookup")
	replayed, duplicate, err = store.ReplayOrFail(ctx, scope, conversation.ID, proposal.ID, key, preflightFailure)
	require.NoError(t, err)
	require.True(t, duplicate)
	require.Equal(t, task.ID, replayed.ID)
	_, _, err = store.ReplayOrFail(ctx, scope, conversation.ID, proposal.ID, uuid.NewString(), preflightFailure)
	require.ErrorIs(t, err, preflightFailure)
	_, _, err = store.ReplayOrFail(ctx, scope, conversation.ID, uuid.NewString(), key, preflightFailure)
	require.ErrorIs(t, err, aiworkbench.ErrIdempotencyConflict)
	_, _, err = store.Confirm(ctx, scope, conversation.ID, uuid.NewString(), key, aiworkbench.PreparedTask{})
	require.ErrorIs(t, err, aiworkbench.ErrIdempotencyConflict)
	_, _, err = store.Confirm(ctx, scope, conversation.ID, proposal.ID, uuid.NewString(), prepared)
	require.ErrorIs(t, err, aiworkbench.ErrConversationArchived)
}

type competingExecution struct {
	entered  chan struct{}
	release  chan struct{}
	prepared aiworkbench.PreparedTask
	prepares atomic.Int32
	starts   atomic.Int32
}

func (*competingExecution) AuthorizeReceipt(context.Context, aiworkbench.Scope) error { return nil }

func (x *competingExecution) Prepare(context.Context, aiworkbench.ExecutionProposal, string) (aiworkbench.PreparedTask, error) {
	if x.prepares.Add(1) == 1 {
		close(x.entered)
		<-x.release
		return aiworkbench.PreparedTask{}, aiworkbench.ErrRevisionMismatch
	}
	return x.prepared, nil
}

func (x *competingExecution) Start(context.Context, aiworkbench.BusinessTask) error {
	x.starts.Add(1)
	return nil
}

func TestConfirmPreflightFailureAdoptsConcurrentCommittedTask(t *testing.T) {
	store, err := New(workbenchDB(t))
	require.NoError(t, err)
	scope := aiworkbench.Scope{OrganizationID: "org-a", ActorID: "actor-a"}
	conversation, proposal := readyTaskFixture(t, store, scope)
	key := uuid.NewString()
	execute := &competingExecution{entered: make(chan struct{}), release: make(chan struct{}),
		prepared: preparedTaskFixture(t, proposal, key)}
	service := aiworkbench.Service{Store: store, Execute: execute}
	type outcome struct {
		task   aiworkbench.BusinessTask
		replay bool
		err    error
	}
	first := make(chan outcome, 1)
	go func() {
		task, replay, err := service.Confirm(context.Background(), scope, conversation.ID, proposal.ID, key)
		first <- outcome{task, replay, err}
	}()
	<-execute.entered // First caller has observed absence and is paused in external preflight.
	winner, replay, err := service.Confirm(context.Background(), scope, conversation.ID, proposal.ID, key)
	require.NoError(t, err)
	require.False(t, replay)
	close(execute.release) // Its stale preflight must now pass through serialized replay-or-fail.
	loser := <-first
	require.NoError(t, loser.err)
	require.True(t, loser.replay)
	require.Equal(t, winner.ID, loser.task.ID)
	require.EqualValues(t, 1, execute.starts.Load())
	var count int64
	require.NoError(t, store.db.Table("ai_workbench.business_tasks").Count(&count).Error)
	require.EqualValues(t, 1, count)
}

func testPlanPreparer(history []aiworkbench.Message, invocationID string) (aiworkbench.PreparedPlan, error) {
	if len(history) == 0 || history[len(history)-1].Author != aiworkbench.AuthorUser || invocationID == "" {
		return aiworkbench.PreparedPlan{}, aiworkbench.ErrInvalid
	}
	return aiworkbench.PreparedPlan{MemberID: "member-a", InputHash: fmt.Sprintf("%064x", len(history)),
		ModelProfile: []byte(`{"profile_id":"test"}`), Deadline: time.Now().Add(time.Minute)}, nil
}

func TestConversationRecentPageOrdersByLastActivity(t *testing.T) {
	store, err := New(workbenchDB(t))
	require.NoError(t, err)
	ctx := context.Background()
	scope := aiworkbench.Scope{OrganizationID: "org-a", ActorID: "actor-a"}
	older, _, err := store.Create(ctx, scope, uuid.NewString(), aiworkbench.CreateInput{})
	require.NoError(t, err)
	newer, _, err := store.Create(ctx, scope, uuid.NewString(), aiworkbench.CreateInput{})
	require.NoError(t, err)
	now := time.Now().UTC().Truncate(time.Microsecond)
	require.NoError(t, store.db.Model(&conversationRow{}).Where("id = ?", older.ID).
		Updates(map[string]any{"created_at": now.Add(-2 * time.Hour), "updated_at": now.Add(-2 * time.Hour)}).Error)
	require.NoError(t, store.db.Model(&conversationRow{}).Where("id = ?", newer.ID).
		Updates(map[string]any{"created_at": now.Add(-time.Hour), "updated_at": now.Add(-time.Hour)}).Error)
	title := "Active again"
	_, err = store.SetMetadata(ctx, scope, older.ID, older.MetadataRevision, aiworkbench.MetadataChange{Title: &title})
	require.NoError(t, err)
	first, next, err := store.ListConversations(ctx, scope, "", 1, false, false)
	require.NoError(t, err)
	require.Len(t, first, 1)
	require.Equal(t, older.ID, first[0].ID)
	require.NotEmpty(t, next)
	require.Len(t, next, 32)
	_, _, err = store.ListConversations(ctx, aiworkbench.Scope{OrganizationID: "other-org", ActorID: scope.ActorID}, next, 1, false, false)
	require.ErrorIs(t, err, aiworkbench.ErrNotFound)
	_, _, err = store.ListConversations(ctx, scope, older.ID, 1, false, false)
	require.ErrorIs(t, err, aiworkbench.ErrInvalid, "a row ID is not a frozen page coordinate")
	second, next, err := store.ListConversations(ctx, scope, next, 1, false, false)
	require.NoError(t, err)
	require.Len(t, second, 1)
	require.Equal(t, newer.ID, second[0].ID)
	require.Empty(t, next)
	key := uuid.NewString()
	_, _, err = store.AppendUser(ctx, scope, older.ID, key,
		aiworkbench.MessageInput{Content: "Next request", OperationID: "op-1", TargetPlatform: "shein"}, testPlanPreparer)
	require.NoError(t, err)
	// Isolate the assistant append from the preceding USER timestamp update.
	require.NoError(t, store.db.Model(&conversationRow{}).Where("id = ?", older.ID).
		Update("updated_at", now.Add(-2*time.Hour)).Error)
	_, _, err = store.CompletePlan(ctx, scope, key, aiworkbench.PlanTerminal{AssistantText: "Please clarify", Mode: aiworkbench.PlanClarify})
	require.NoError(t, err)
	first, _, err = store.ListConversations(ctx, scope, "", 1, false, false)
	require.NoError(t, err)
	require.Equal(t, older.ID, first[0].ID, "assistant activity also refreshes recent order")
}

func TestConversationCursorKeepsOriginalBoundaryAfterCursorActivity(t *testing.T) {
	store, err := New(workbenchDB(t))
	require.NoError(t, err)
	ctx := context.Background()
	scope := aiworkbench.Scope{OrganizationID: "org-a", ActorID: "actor-a"}
	ids := make([]string, 3)
	for i := range ids {
		created, _, createErr := store.Create(ctx, scope, uuid.NewString(), aiworkbench.CreateInput{Favorite: true})
		require.NoError(t, createErr)
		ids[i] = created.ID
	}
	now := time.Now().UTC().Truncate(time.Microsecond)
	for i, id := range ids {
		require.NoError(t, store.db.Model(&conversationRow{}).Where("id = ?", id).
			Update("updated_at", now.Add(-time.Duration(i+1)*time.Hour)).Error)
	}
	first, cursor, err := store.ListConversations(ctx, scope, "", 2, true, false)
	require.NoError(t, err)
	require.Equal(t, []string{ids[0], ids[1]}, []string{first[0].ID, first[1].ID})
	require.NotEmpty(t, cursor)
	// The cursor row moves to the top after page one. Page two must use its
	// original sort coordinate, not return page one's first row again.
	title := "Updated while paging"
	_, err = store.SetMetadata(ctx, scope, ids[1], 1, aiworkbench.MetadataChange{Title: &title})
	require.NoError(t, err)
	second, _, err := store.ListConversations(ctx, scope, cursor, 2, true, false)
	require.NoError(t, err)
	require.Len(t, second, 1)
	require.Equal(t, ids[2], second[0].ID)
}

func TestSavedCursorContinuesAfterCursorIsUnfavoritedAndArchived(t *testing.T) {
	store, err := New(workbenchDB(t))
	require.NoError(t, err)
	ctx := context.Background()
	scope := aiworkbench.Scope{OrganizationID: "org-a", ActorID: "actor-a"}
	ids := make([]string, 3)
	for i := range ids {
		created, _, createErr := store.Create(ctx, scope, uuid.NewString(), aiworkbench.CreateInput{Favorite: true})
		require.NoError(t, createErr)
		ids[i] = created.ID
	}
	now := time.Now().UTC().Truncate(time.Microsecond)
	for i, id := range ids {
		require.NoError(t, store.db.Model(&conversationRow{}).Where("id = ?", id).
			Update("updated_at", now.Add(-time.Duration(i+1)*time.Hour)).Error)
	}
	_, cursor, err := store.ListConversations(ctx, scope, "", 2, true, false)
	require.NoError(t, err)
	require.NotEmpty(t, cursor)
	favorite, archived := false, true
	_, err = store.SetMetadata(ctx, scope, ids[1], 1, aiworkbench.MetadataChange{Favorite: &favorite, Archived: &archived})
	require.NoError(t, err)
	second, _, err := store.ListConversations(ctx, scope, cursor, 2, true, false)
	require.NoError(t, err)
	require.Len(t, second, 1)
	require.Equal(t, ids[2], second[0].ID)
}

func TestSavedConversationFilterPrecedesPaginationAndExcludesArchived(t *testing.T) {
	store, err := New(workbenchDB(t))
	require.NoError(t, err)
	ctx := context.Background()
	scope := aiworkbench.Scope{OrganizationID: "org-a", ActorID: "actor-a"}
	oldFavorite, _, err := store.Create(ctx, scope, uuid.NewString(), aiworkbench.CreateInput{Favorite: true})
	require.NoError(t, err)
	for range 3 {
		_, _, err = store.Create(ctx, scope, uuid.NewString(), aiworkbench.CreateInput{})
		require.NoError(t, err)
	}
	saved, next, err := store.ListConversations(ctx, scope, "", 1, true, false)
	require.NoError(t, err)
	require.Len(t, saved, 1)
	require.Equal(t, oldFavorite.ID, saved[0].ID)
	require.Empty(t, next)
	archived := true
	_, err = store.SetMetadata(ctx, scope, oldFavorite.ID, oldFavorite.MetadataRevision, aiworkbench.MetadataChange{Archived: &archived})
	require.NoError(t, err)
	saved, next, err = store.ListConversations(ctx, scope, "", 1, true, false)
	require.NoError(t, err)
	require.Empty(t, saved)
	require.Empty(t, next)
}

func TestArchivedConversationCanBeFoundAndRestoredWithinOwnerScope(t *testing.T) {
	store, err := New(workbenchDB(t))
	require.NoError(t, err)
	ctx := context.Background()
	scope := aiworkbench.Scope{OrganizationID: "org-a", ActorID: "actor-a"}
	archived, _, err := store.Create(ctx, scope, uuid.NewString(), aiworkbench.CreateInput{})
	require.NoError(t, err)
	for range 3 {
		_, _, err = store.Create(ctx, scope, uuid.NewString(), aiworkbench.CreateInput{})
		require.NoError(t, err)
	}
	yes, no := true, false
	archived, err = store.SetMetadata(ctx, scope, archived.ID, archived.MetadataRevision, aiworkbench.MetadataChange{Archived: &yes})
	require.NoError(t, err)
	items, next, err := store.ListConversations(ctx, scope, "", 1, false, true)
	require.NoError(t, err)
	require.Len(t, items, 1, "archived filter runs before pagination")
	require.Equal(t, archived.ID, items[0].ID)
	require.True(t, items[0].Archived)
	require.Empty(t, next)
	other := aiworkbench.Scope{OrganizationID: "org-a", ActorID: "actor-b"}
	items, _, err = store.ListConversations(ctx, other, "", 1, false, true)
	require.NoError(t, err)
	require.Empty(t, items)
	_, err = store.SetMetadata(ctx, other, archived.ID, archived.MetadataRevision, aiworkbench.MetadataChange{Archived: &no})
	require.ErrorIs(t, err, aiworkbench.ErrNotFound)
	restored, err := store.SetMetadata(ctx, scope, archived.ID, archived.MetadataRevision, aiworkbench.MetadataChange{Archived: &no})
	require.NoError(t, err)
	items, _, err = store.ListConversations(ctx, scope, "", 1, false, true)
	require.NoError(t, err)
	require.Empty(t, items)
	items, _, err = store.ListConversations(ctx, scope, "", 50, false, false)
	require.NoError(t, err)
	require.False(t, restored.Archived)
	require.NotEmpty(t, items)
	require.Equal(t, restored.ID, items[0].ID)
	require.False(t, items[0].Archived)
}

func TestConversationCreateReceiptIsAtomicAndScoped(t *testing.T) {
	db := workbenchDB(t)
	store, err := New(db)
	require.NoError(t, err)
	ctx := context.Background()
	scope := aiworkbench.Scope{OrganizationID: "org-a", ActorID: "user-a"}
	key := uuid.NewString()
	input := aiworkbench.CreateInput{Favorite: true}
	created, replayed, err := store.Create(ctx, scope, key, input)
	require.NoError(t, err)
	require.False(t, replayed)
	require.Equal(t, scope, created.Scope)
	require.True(t, created.Favorite)

	restarted, err := New(db)
	require.NoError(t, err)
	same, replayed, err := restarted.Create(ctx, scope, key, input)
	require.NoError(t, err)
	require.True(t, replayed)
	require.Equal(t, created.ID, same.ID)
	_, _, err = restarted.Create(ctx, scope, key, aiworkbench.CreateInput{Favorite: false})
	require.ErrorIs(t, err, aiworkbench.ErrIdempotencyConflict)

	// Create and message share one operation namespace.
	_, _, err = restarted.AppendUser(ctx, scope, created.ID, key,
		aiworkbench.MessageInput{Content: "优化标题", OperationID: "op-a", TargetPlatform: "shein"},
		testPlanPreparer)
	require.ErrorIs(t, err, aiworkbench.ErrIdempotencyConflict)

	other, replayed, err := restarted.Create(ctx, aiworkbench.Scope{OrganizationID: "org-b", ActorID: scope.ActorID}, key, input)
	require.NoError(t, err)
	require.False(t, replayed)
	require.NotEqual(t, created.ID, other.ID)
	other, replayed, err = restarted.Create(ctx, aiworkbench.Scope{OrganizationID: scope.OrganizationID, ActorID: "user-b"}, key, input)
	require.NoError(t, err)
	require.False(t, replayed)
	require.NotEqual(t, created.ID, other.ID)
	_, err = restarted.Get(ctx, aiworkbench.Scope{OrganizationID: "org-b", ActorID: "user-a"}, created.ID)
	require.ErrorIs(t, err, aiworkbench.ErrNotFound)

	title := "手动标题"
	archived := true
	changed, err := restarted.SetMetadata(ctx, scope, created.ID, created.MetadataRevision,
		aiworkbench.MetadataChange{Title: &title, Archived: &archived})
	require.NoError(t, err)
	require.Equal(t, "手动标题", changed.Title)
	require.True(t, changed.Archived)
	same, replayed, err = restarted.Create(ctx, scope, key, input)
	require.NoError(t, err)
	require.True(t, replayed)
	require.Equal(t, changed.ID, same.ID)
	require.Equal(t, changed.Title, same.Title)
	require.True(t, same.Archived)

	// Force receipt insert failure after Conversation insert: neither row may survive.
	failingKey := uuid.NewString()
	triggerSQL := fmt.Sprintf("CREATE FUNCTION ai_workbench.reject_test_receipt() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.idempotency_key = '%s' THEN RAISE EXCEPTION 'forced receipt failure'; END IF; RETURN NEW; END $$", failingKey)
	require.NoError(t, db.Exec(triggerSQL).Error)
	require.NoError(t, db.Exec("CREATE TRIGGER reject_test_receipt BEFORE INSERT ON ai_workbench.commands FOR EACH ROW EXECUTE FUNCTION ai_workbench.reject_test_receipt()").Error)
	_, _, err = store.Create(ctx, scope, failingKey, aiworkbench.CreateInput{})
	require.Error(t, err)
	var count int64
	require.NoError(t, db.Table("ai_workbench.conversations").Where("organization_id = ? AND owner_user_id = ?", scope.OrganizationID, scope.ActorID).Count(&count).Error)
	require.EqualValues(t, 1, count)
	require.NoError(t, db.Table("ai_workbench.commands").Where("organization_id = ? AND actor_id = ? AND idempotency_key = ?", scope.OrganizationID, scope.ActorID, failingKey).Count(&count).Error)
	require.Zero(t, count)
}

func TestConversationCreateConcurrentSameKeyOneReceipt(t *testing.T) {
	db := workbenchDB(t)
	store, err := New(db)
	require.NoError(t, err)
	scope := aiworkbench.Scope{OrganizationID: "org-a", ActorID: "user-a"}
	key := uuid.NewString()
	const parallel = 10
	ids := make(chan string, parallel)
	var wg sync.WaitGroup
	var created atomic.Int32
	for i := 0; i < parallel; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			row, replay, createErr := store.Create(context.Background(), scope, key, aiworkbench.CreateInput{Favorite: true})
			if createErr != nil {
				t.Error(createErr)
				return
			}
			if !replay {
				created.Add(1)
			}
			ids <- row.ID
		}()
	}
	wg.Wait()
	close(ids)
	require.EqualValues(t, 1, created.Load())
	var first string
	for id := range ids {
		if first == "" {
			first = id
		}
		require.Equal(t, first, id)
	}
	var count int64
	require.NoError(t, db.Table("ai_workbench.conversations").Count(&count).Error)
	require.EqualValues(t, 1, count)
	require.NoError(t, db.Table("ai_workbench.commands").Count(&count).Error)
	require.EqualValues(t, 1, count)
}

func TestUnavailablePlannerStillPersistsUserIntentWithoutDispatchProfile(t *testing.T) {
	db := workbenchDB(t)
	store, err := New(db)
	require.NoError(t, err)
	ctx := context.Background()
	scope := aiworkbench.Scope{OrganizationID: "org-a", ActorID: "user-a"}
	conversation, _, err := store.Create(ctx, scope, uuid.NewString(), aiworkbench.CreateInput{})
	require.NoError(t, err)
	key := uuid.NewString()
	input := aiworkbench.MessageInput{Content: "Please suggest a title", OperationID: "op-1", TargetPlatform: "shein"}
	_, found, err := store.ReplayCommand(ctx, scope, conversation.ID, key, input)
	require.NoError(t, err)
	require.False(t, found)
	prepare := func(history []aiworkbench.Message, invocationID string) (aiworkbench.PreparedPlan, error) {
		require.Len(t, history, 1)
		require.NotEmpty(t, invocationID)
		return aiworkbench.PreparedPlan{MemberID: "member-a", Unavailable: true}, nil
	}
	command, replay, err := store.AppendUser(ctx, scope, conversation.ID, key, input, prepare)
	require.NoError(t, err)
	require.False(t, replay)
	require.Equal(t, aiworkbench.PlanningFailedBeforeDispatch, command.State)
	require.Empty(t, command.ModelProfile)
	require.Equal(t, input.WorkScope(), command.WorkScope)
	preflightReceipt, found, err := store.ReplayCommand(ctx, scope, conversation.ID, key, input)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, command.UserMessageID, preflightReceipt.UserMessageID)
	_, found, err = store.ReplayCommand(ctx, aiworkbench.Scope{OrganizationID: "org-b", ActorID: scope.ActorID}, conversation.ID, key, input)
	require.NoError(t, err)
	require.False(t, found)
	messages, err := store.ListMessages(ctx, scope, conversation.ID, 50)
	require.NoError(t, err)
	require.Len(t, messages, 1)
	require.Equal(t, input.Content, messages[0].Content)
	command, replay, err = store.AppendUser(ctx, scope, conversation.ID, key, input, func([]aiworkbench.Message, string) (aiworkbench.PreparedPlan, error) {
		t.Fatal("replay must not resolve a new profile")
		return aiworkbench.PreparedPlan{}, nil
	})
	require.NoError(t, err)
	require.True(t, replay)
	require.Equal(t, aiworkbench.PlanningFailedBeforeDispatch, command.State)
	changed := input
	changed.Content = "A different title goal"
	_, _, err = store.ReplayCommand(ctx, scope, conversation.ID, key, changed)
	require.ErrorIs(t, err, aiworkbench.ErrIdempotencyConflict)
	_, _, err = store.AppendUser(ctx, scope, conversation.ID, key, changed, prepare)
	require.ErrorIs(t, err, aiworkbench.ErrIdempotencyConflict)
}

func TestPlanningFailureTerminalDoesNotAppendAssistant(t *testing.T) {
	store, err := New(workbenchDB(t))
	require.NoError(t, err)
	ctx := context.Background()
	scope := aiworkbench.Scope{OrganizationID: "org-a", ActorID: "user-a"}
	conversation, _, err := store.Create(ctx, scope, uuid.NewString(), aiworkbench.CreateInput{})
	require.NoError(t, err)
	key := uuid.NewString()
	_, _, err = store.AppendUser(ctx, scope, conversation.ID, key,
		aiworkbench.MessageInput{Content: "Title?", OperationID: "op-1", TargetPlatform: "shein"}, testPlanPreparer)
	require.NoError(t, err)
	failed, err := store.FinalizePlanning(ctx, scope, key, aiworkbench.PlanningFailedBeforeDispatch)
	require.NoError(t, err)
	require.Equal(t, aiworkbench.PlanningFailedBeforeDispatch, failed.State)
	_, err = store.FinalizePlanning(ctx, scope, key, aiworkbench.PlanningUnknown)
	require.ErrorIs(t, err, aiworkbench.ErrIdempotencyConflict)
	_, _, err = store.CompletePlan(ctx, scope, key, aiworkbench.PlanTerminal{AssistantText: "clarify", Mode: aiworkbench.PlanClarify})
	require.ErrorIs(t, err, aiworkbench.ErrIdempotencyConflict)
	messages, err := store.ListMessages(ctx, scope, conversation.ID, 50)
	require.NoError(t, err)
	require.Len(t, messages, 1)
}

func TestReadyPlannerCommitsAssistantAndImmutableProposalTogether(t *testing.T) {
	store, err := New(workbenchDB(t))
	require.NoError(t, err)
	ctx := context.Background()
	scope := aiworkbench.Scope{OrganizationID: "org-a", ActorID: "user-a"}
	conversation, _, err := store.Create(ctx, scope, uuid.NewString(), aiworkbench.CreateInput{})
	require.NoError(t, err)
	key := uuid.NewString()
	request := aiworkbench.MessageInput{Content: "Improve title", OperationID: "op-1", TargetPlatform: "shein"}
	command, _, err := store.AppendUser(ctx, scope, conversation.ID, key, request, testPlanPreparer)
	require.NoError(t, err)
	proposal := &aiworkbench.ExecutionProposal{Kind: "product.title.optimize", GoalSummary: "Improve product title",
		OperationID: "op-1", ProductKey: "product-1", CatalogVersion: "1", PublicationID: "publication-1",
		TargetPlatform: "shein", AgentID: "product.title.agent", AgentVersion: "v1.0.0",
		ObservedAgentRevision: "revision-1", ObservedActivationEpoch: "epoch-1",
		ExecutionModelProfile: []byte(`{"profile_id":"execution-v1"}`)}
	terminal := aiworkbench.PlanTerminal{AssistantText: "Review this title goal", Mode: aiworkbench.PlanReady,
		GoalSummary: proposal.GoalSummary, Proposal: proposal}
	completed, replay, err := store.CompletePlan(ctx, scope, key, terminal)
	require.NoError(t, err)
	require.False(t, replay)
	require.Equal(t, aiworkbench.PlanningComplete, completed.State)
	require.Equal(t, command.SourceSequence, completed.SourceSequence)
	require.NotEmpty(t, completed.ProposalID)
	saved, err := store.GetProposal(ctx, scope, completed.ProposalID)
	require.NoError(t, err)
	require.Equal(t, command.UserMessageID, saved.SourceUserMessageID)
	require.Equal(t, completed.AssistantMessageID, saved.AssistantMessageID)
	require.Equal(t, completed.SourceSequence, saved.SourceSequence)
	require.Equal(t, proposal.GoalSummary, saved.GoalSummary)
	require.NotEmpty(t, saved.Digest)
	_, replay, err = store.CompletePlan(ctx, scope, key, terminal)
	require.NoError(t, err)
	require.True(t, replay)
	messages, err := store.ListMessages(ctx, scope, conversation.ID, 50)
	require.NoError(t, err)
	require.Len(t, messages, 2)
}

func TestArchivePreservesInflightPlannerTerminalization(t *testing.T) {
	db := workbenchDB(t)
	store, err := New(db)
	require.NoError(t, err)
	scope := aiworkbench.Scope{OrganizationID: "org-a", ActorID: "user-a"}
	ctx := context.Background()
	conversation, _, err := store.Create(ctx, scope, uuid.NewString(), aiworkbench.CreateInput{})
	require.NoError(t, err)
	key := uuid.NewString()
	request := aiworkbench.MessageInput{Content: "请优化标题", OperationID: "operation-1", TargetPlatform: "shein"}
	var captured []aiworkbench.Message
	prepared := aiworkbench.PlanPreparer(func(history []aiworkbench.Message, invocationID string) (aiworkbench.PreparedPlan, error) {
		captured = append([]aiworkbench.Message(nil), history...)
		return testPlanPreparer(history, invocationID)
	})
	command, replay, err := store.AppendUser(ctx, scope, conversation.ID, key, request, prepared)
	require.NoError(t, err)
	require.False(t, replay)
	require.EqualValues(t, 1, command.SourceSequence)
	require.Equal(t, aiworkbench.PlanningReadyToDispatch, command.State)
	require.NotEmpty(t, command.PlannerInvocationID)
	require.Equal(t, request.WorkScope(), command.WorkScope)
	history, frozen, err := store.HistoryForCommand(ctx, scope, key)
	require.NoError(t, err)
	require.Equal(t, captured, history)
	require.Equal(t, command.InputHash, frozen.InputHash)

	started := make(chan struct{})
	release := make(chan struct{})
	planningCtx, cancelPlanning := context.WithCancel(ctx)
	defer cancelPlanning()
	var providerCalls atomic.Int32
	done := make(chan error, 1)
	go func() {
		providerCalls.Add(1)
		close(started)
		<-release
		_, _, terminalErr := store.CompletePlan(planningCtx, scope, key, aiworkbench.PlanTerminal{AssistantText: "建议突出核心卖点", Mode: aiworkbench.PlanClarify})
		done <- terminalErr
	}()
	<-started
	archived := true
	conversation, err = store.SetMetadata(ctx, scope, conversation.ID, conversation.MetadataRevision, aiworkbench.MetadataChange{Archived: &archived})
	require.NoError(t, err)
	_, _, err = store.AppendUser(ctx, scope, conversation.ID, uuid.NewString(), request, prepared)
	require.ErrorIs(t, err, aiworkbench.ErrConversationArchived)
	cancelPlanning()
	close(release)
	require.NoError(t, <-done)

	terminal, err := store.GetCommand(ctx, scope, key)
	require.NoError(t, err)
	require.Equal(t, aiworkbench.PlanningComplete, terminal.State)
	require.Equal(t, command.SourceSequence, terminal.SourceSequence)
	require.NotEmpty(t, terminal.AssistantMessageID)
	messages, err := store.ListMessages(ctx, scope, conversation.ID, 50)
	require.NoError(t, err)
	require.Len(t, messages, 2)
	require.Equal(t, aiworkbench.AuthorUser, messages[0].Author)
	require.Equal(t, aiworkbench.AuthorAssistant, messages[1].Author)
	require.EqualValues(t, 2, messages[1].Sequence)
	history, frozen, err = store.HistoryForCommand(ctx, scope, key)
	require.NoError(t, err)
	require.Equal(t, captured, history)
	require.Equal(t, request.WorkScope(), frozen.WorkScope)

	replayed, duplicate, err := store.CompletePlan(ctx, scope, key, aiworkbench.PlanTerminal{AssistantText: "建议突出核心卖点", Mode: aiworkbench.PlanClarify})
	require.NoError(t, err)
	require.True(t, duplicate)
	require.Equal(t, terminal.AssistantMessageID, replayed.AssistantMessageID)
	messages, err = store.ListMessages(ctx, scope, conversation.ID, 50)
	require.NoError(t, err)
	require.Len(t, messages, 2)
	require.EqualValues(t, 1, providerCalls.Load())
	_, replay, err = store.AppendUser(ctx, scope, conversation.ID, key, request, prepared)
	require.NoError(t, err)
	require.True(t, replay)
	unarchived := false
	conversation, err = store.SetMetadata(ctx, scope, conversation.ID, conversation.MetadataRevision, aiworkbench.MetadataChange{Archived: &unarchived})
	require.NoError(t, err)
	_, replay, err = store.AppendUser(ctx, scope, conversation.ID, key, request, prepared)
	require.NoError(t, err)
	require.True(t, replay)
	require.EqualValues(t, 1, providerCalls.Load())
	_, replay, err = store.AppendUser(ctx, scope, conversation.ID, uuid.NewString(), request, prepared)
	require.NoError(t, err)
	require.False(t, replay)

	_, _, err = store.CompletePlan(ctx, scope, key, aiworkbench.PlanTerminal{AssistantText: "不同结果", Mode: aiworkbench.PlanClarify})
	require.ErrorIs(t, err, aiworkbench.ErrIdempotencyConflict)
	_, err = store.GetCommand(ctx, aiworkbench.Scope{OrganizationID: "org-b", ActorID: scope.ActorID}, key)
	require.ErrorIs(t, err, aiworkbench.ErrNotFound)
}
