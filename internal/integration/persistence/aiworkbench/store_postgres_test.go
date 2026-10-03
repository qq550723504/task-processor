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
		store, err := New(tx)
		if err != nil {
			return err
		}
		scope := aiworkbench.Scope{OrganizationID: "B", ActorID: "operator"}
		conversation, _, err := store.Create(context.Background(), scope, uuid.NewString(), aiworkbench.CreateInput{})
		if err != nil {
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
		_, _, err = store.Confirm(context.Background(), scope, conversation.ID, saved.ID, confirmKey, preparedTaskFixture(t, saved, confirmKey))
		return err
	}))
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
		ConfigurationSnapshotRef: ref, PolicyVersion: "policy-v1", PromptVersion: "prompt-v1",
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

func testPlanPreparer(history []aiworkbench.Message, invocationID string) (aiworkbench.PreparedPlan, error) {
	if len(history) == 0 || history[len(history)-1].Author != aiworkbench.AuthorUser || invocationID == "" {
		return aiworkbench.PreparedPlan{}, aiworkbench.ErrInvalid
	}
	return aiworkbench.PreparedPlan{MemberID: "member-a", InputHash: fmt.Sprintf("%064x", len(history)),
		ModelProfile: []byte(`{"profile_id":"test"}`), Deadline: time.Now().Add(time.Minute)}, nil
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
