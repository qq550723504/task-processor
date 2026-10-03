//go:build integration

package aiworkbenchpersistence

import (
	"context"
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
		aiworkbench.PreparedPlan{InputHash: fmt.Sprintf("%064x", 1), ModelProfile: []byte("{\"profile_id\":\"test\"}"), Deadline: time.Now().Add(time.Minute)})
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
	prepared := aiworkbench.PreparedPlan{InputHash: fmt.Sprintf("%064x", 2), ModelProfile: []byte("{\"profile_id\":\"test\"}"), Deadline: time.Now().Add(time.Minute)}
	command, replay, err := store.AppendUser(ctx, scope, conversation.ID, key, request, prepared)
	require.NoError(t, err)
	require.False(t, replay)
	require.EqualValues(t, 1, command.SourceSequence)
	require.Equal(t, aiworkbench.PlanningReadyToDispatch, command.State)
	require.NotEmpty(t, command.PlannerInvocationID)

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

	_, _, err = store.CompletePlan(ctx, scope, key, aiworkbench.PlanTerminal{AssistantText: "不同结果", Mode: aiworkbench.PlanClarify})
	require.ErrorIs(t, err, aiworkbench.ErrIdempotencyConflict)
	_, err = store.GetCommand(ctx, aiworkbench.Scope{OrganizationID: "org-b", ActorID: scope.ActorID}, key)
	require.ErrorIs(t, err, aiworkbench.ErrNotFound)
}
