//go:build integration

package knowledge_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	store "task-processor/internal/integration/persistence/knowledge"
	"task-processor/internal/knowledge"
)

func TestKnowledgePostgresAdmissionPromotionAndRecovery(t *testing.T) {
	ctx := context.Background()
	container, err := tcpostgres.Run(ctx, "postgres:17-alpine", tcpostgres.WithDatabase("knowledge"), tcpostgres.WithUsername("knowledge_owner"), tcpostgres.WithPassword("isolated-test-password"), tcpostgres.BasicWaitStrategies())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = container.Terminate(context.Background()) })
	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB()
	t.Cleanup(func() { _ = sqlDB.Close() })
	sqlDB.SetMaxOpenConns(8)
	if err := store.Install(ctx, db); err != nil {
		t.Fatal(err)
	}
	repo, err := store.NewRepository(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	scope := knowledge.Scope{OrganizationID: "org-a", ActorID: "actor-a"}
	apply := func(c knowledge.Command) knowledge.Result {
		t.Helper()
		c.Scope = scope
		if c.Key == "" {
			c.Key = uuid.NewString()
		}
		if c.Fingerprint == "" {
			c.Fingerprint = knowledge.Digest([]byte(c.Key))
		}
		r, e := repo.Apply(ctx, c)
		if e != nil {
			t.Fatal(e)
		}
		return r
	}
	base := apply(knowledge.Command{Kind: "base_create", Name: "Brand"}).Base
	upload := func(key string) knowledge.Command {
		return knowledge.Command{Scope: scope, Kind: "source_create", Key: key, Fingerprint: knowledge.Digest([]byte(key)), BaseID: base.ID, Name: "Guideline", Upload: &knowledge.Revision{Filename: "brand.txt", ContentType: "text/plain", SizeBytes: 5, SHA256: knowledge.Digest([]byte("hello"))}}
	}
	key := uuid.NewString()
	first := apply(upload(key))
	again := apply(upload(key))
	if first.Revision.ID != again.Revision.ID {
		t.Fatal("replay allocated new revision")
	}
	changed := upload(key)
	changed.Fingerprint = knowledge.Digest([]byte("changed"))
	if _, e := repo.Apply(ctx, changed); !errors.Is(e, knowledge.ErrConflict) {
		t.Fatalf("changed command: %v", e)
	}
	var wg sync.WaitGroup
	var mu sync.Mutex
	success := 0
	for range 10 {
		wg.Go(func() {
			_, e := repo.Apply(ctx, upload(uuid.NewString()))
			mu.Lock()
			defer mu.Unlock()
			if e == nil {
				success++
			} else if !errors.Is(e, knowledge.ErrSourceLimit) {
				t.Errorf("admit: %v", e)
			}
		})
	}
	wg.Wait()
	if success != 3 {
		t.Fatalf("expected exactly three remaining slots, got %d", success)
	}
	if _, e := repo.GetSource(ctx, "org-b", first.Source.ID); !errors.Is(e, knowledge.ErrNotFound) {
		t.Fatalf("cross org: %v", e)
	}
	r, claimed, e := repo.ClaimUpload(ctx, scope.OrganizationID, first.Revision.ID, "upload-a")
	if e != nil || !claimed {
		t.Fatalf("claim %v %v", claimed, e)
	}
	if e := repo.ConfirmObject(ctx, r); e != nil {
		t.Fatal(e)
	}
	claimedRevisions, e := repo.ClaimProcessing(ctx, "parser-a", 4)
	if e != nil {
		t.Fatal(e)
	}
	var parsed knowledge.Revision
	for _, rev := range claimedRevisions {
		if rev.ID == r.ID {
			parsed = rev
		}
	}
	if parsed.ID == "" {
		t.Fatal("stored object not resumed")
	}
	if e := repo.Finish(ctx, parsed, knowledge.ParseResult{Text: "hello"}); e != nil {
		t.Fatal(e)
	}
	source, e := repo.GetSource(ctx, scope.OrganizationID, r.SourceID)
	if e != nil {
		t.Fatal(e)
	}
	if source.CurrentReadableRevisionID != r.ID {
		t.Fatal("readable promotion missing")
	}
	replacement := knowledge.Command{Scope: scope, Kind: "revision_create", Key: uuid.NewString(), SourceID: source.ID, Version: source.Version, Name: source.Name, Upload: upload(uuid.NewString()).Upload}
	replacement.Fingerprint = knowledge.Digest([]byte(replacement.Key))
	newRev := apply(replacement)
	busy := replacement
	busy.Key = uuid.NewString()
	busy.Fingerprint = knowledge.Digest([]byte(busy.Key))
	busy.Version = newRev.Source.Version
	if _, e := repo.Apply(ctx, busy); !errors.Is(e, knowledge.ErrRevisionBusy) {
		t.Fatalf("second replacement: %v", e)
	}
	source, e = repo.GetSource(ctx, scope.OrganizationID, source.ID)
	if e != nil || source.CurrentReadableRevisionID != r.ID {
		t.Fatal("processing hid readable version")
	}
	lease, ok, e := repo.ClaimUpload(ctx, scope.OrganizationID, newRev.Revision.ID, "upload-b")
	if e != nil || !ok {
		t.Fatal(e)
	}
	if e := repo.Finish(ctx, lease, knowledge.ParseResult{Failure: "UPLOAD_INCOMPLETE"}); e != nil {
		t.Fatal(e)
	}
	lease, ok, e = repo.ClaimUpload(ctx, scope.OrganizationID, lease.ID, "resume-b")
	if e != nil || !ok || lease.ID != newRev.Revision.ID {
		t.Fatalf("same incomplete identity resume: %v %v", ok, e)
	}
	if e := repo.ConfirmObject(ctx, lease); e != nil {
		t.Fatal(e)
	}
	revisions, e := repo.ClaimProcessing(ctx, "parser-b", 4)
	if e != nil {
		t.Fatal(e)
	}
	parsed = knowledge.Revision{}
	for _, rev := range revisions {
		if rev.ID == lease.ID {
			parsed = rev
		}
	}
	if parsed.ID == "" {
		t.Fatalf("replacement was not claimed: %+v", revisions)
	}
	if e := repo.Finish(ctx, parsed, knowledge.ParseResult{Failure: "CORRUPT_DOCUMENT"}); e != nil {
		t.Fatal(e)
	}
	source, _ = repo.GetSource(ctx, scope.OrganizationID, source.ID)
	if source.CurrentReadableRevisionID != r.ID {
		t.Fatal("failed replacement overwrote old readable")
	}
	if _, e := repo.Preview(ctx, scope.OrganizationID, source.ID, r.ID); e != nil {
		t.Fatal(e)
	}
	disabled := apply(knowledge.Command{Kind: "source_disable", SourceID: source.ID, Version: source.Version}).Source
	if disabled.State != knowledge.Disabled {
		t.Fatal("Slice A disable did not converge")
	}
	if _, e := repo.Preview(ctx, scope.OrganizationID, source.ID, r.ID); !errors.Is(e, knowledge.ErrInactive) {
		t.Fatalf("disabled preview: %v", e)
	}
	apply(upload(uuid.NewString())) // disable freed exactly one slot
	// Stale workers must not finish a newer lease, even after process restart.
	if e := repo.Finish(ctx, parsed, knowledge.ParseResult{Text: "late success"}); !errors.Is(e, knowledge.ErrLeaseLost) {
		t.Fatalf("stale finish: %v", e)
	}
	_ = time.Second
}
