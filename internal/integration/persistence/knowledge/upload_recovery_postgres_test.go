//go:build integration

package knowledge_test

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/google/uuid"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	store "task-processor/internal/integration/persistence/knowledge"
	k "task-processor/internal/knowledge"
)

// Faults are confined to the external object port. Admission, leases and
// recovery all use the real PostgreSQL repository and application service.
func TestKnowledgeUploadAmbiguityAndRestartKeepOriginalIdentity(t *testing.T) {
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
	pool, _ := db.DB()
	t.Cleanup(func() { _ = pool.Close() })
	if err = store.Install(ctx, db); err != nil {
		t.Fatal(err)
	}
	repo, err := store.NewRepository(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	objects := &ambiguousKnowledgeObjects{data: map[string]k.Object{}}
	service, err := k.NewService(repo, objects)
	if err != nil {
		t.Fatal(err)
	}
	scope := k.Scope{OrganizationID: "org-a", ActorID: "actor-a"}
	base, err := service.Mutate(ctx, k.Command{Scope: scope, Key: uuid.NewString(), Kind: "base_create", Name: "Brand"})
	if err != nil {
		t.Fatal(err)
	}
	command := k.Command{Scope: scope, Key: uuid.NewString(), Kind: "source_create", BaseID: base.Base.ID, Name: "Guideline"}
	first, err := service.Upload(ctx, command, "brand.txt", []byte("hello"))
	if err != nil || first.Revision.State != k.ObjectStored {
		t.Fatalf("adopt lost put response: %+v %v", first, err)
	}
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			replay, e := service.Upload(ctx, command, "brand.txt", []byte("hello"))
			if e != nil || replay.Revision.ID != first.Revision.ID {
				t.Errorf("concurrent replay: %+v %v", replay, e)
			}
		})
	}
	wg.Wait()
	if _, err = service.Upload(ctx, command, "brand.txt", []byte("changed")); !errors.Is(err, k.ErrConflict) {
		t.Fatalf("changed bytes: %v", err)
	}
	if objects.puts != 1 {
		t.Fatalf("replay wrote %d objects", objects.puts)
	}
	processor, _ := k.NewProcessor(repo, objects, unusedKnowledgeParser{})
	if err = processor.Sweep(ctx); err != nil {
		t.Fatal(err)
	}
	preview, err := service.Preview(ctx, scope, first.Source.ID, first.Revision.ID)
	if err != nil || preview.Text != "hello" {
		t.Fatalf("stored restart: %+v %v", preview, err)
	}

	// A transport failure during inspection cannot be classified as absence.
	objects.inspectUnavailable = true
	command.Key = uuid.NewString()
	admitted, err := service.Upload(ctx, command, "brand.txt", []byte("hello"))
	if err != nil || admitted.Revision.State != k.Admitted || objects.puts != 1 {
		t.Fatalf("unknown inspection: %+v %v", admitted, err)
	}
	if err = db.Exec("UPDATE knowledge_revisions SET lease_until=clock_timestamp()-interval '1 second',next_attempt_at=clock_timestamp()-interval '1 second' WHERE id=?", admitted.Revision.ID).Error; err != nil {
		t.Fatal(err)
	}
	if err = processor.Sweep(ctx); err != nil {
		t.Fatal(err)
	}
	source, _ := repo.GetSource(ctx, scope.OrganizationID, admitted.Source.ID)
	if source.LatestRevision.State != k.Admitted {
		t.Fatal("transport failure invented missing object")
	}
	objects.inspectUnavailable = false
	if err = db.Exec("UPDATE knowledge_revisions SET lease_until=clock_timestamp()-interval '1 second',next_attempt_at=clock_timestamp()-interval '1 second' WHERE id=?", admitted.Revision.ID).Error; err != nil {
		t.Fatal(err)
	}
	// A fresh process observes confirmed absence and terminates that identity.
	processor, _ = k.NewProcessor(repo, objects, unusedKnowledgeParser{})
	if err = processor.Sweep(ctx); err != nil {
		t.Fatal(err)
	}
	source, _ = repo.GetSource(ctx, scope.OrganizationID, admitted.Source.ID)
	if source.LatestRevision.Failure != "UPLOAD_INCOMPLETE" {
		t.Fatalf("confirmed absence did not become incomplete: %+v", source.LatestRevision)
	}
	resumed, err := service.Upload(ctx, command, "brand.txt", []byte("hello"))
	if err != nil || resumed.Revision.ID != admitted.Revision.ID || resumed.Revision.State != k.ObjectStored {
		t.Fatalf("same upload resume: %+v %v", resumed, err)
	}

	// An existing object with different bytes must fail closed without overwriting.
	objects.mismatch = true
	command.Key = uuid.NewString()
	if _, err = service.Upload(ctx, command, "brand.txt", []byte("hello")); !errors.Is(err, k.ErrIntegrity) {
		t.Fatalf("mismatch: %v", err)
	}
	if objects.puts != 2 {
		t.Fatal("mismatch performed another put")
	}
}

type ambiguousKnowledgeObjects struct {
	mu                           sync.Mutex
	data                         map[string]k.Object
	puts                         int
	inspectUnavailable, mismatch bool
}

func (s *ambiguousKnowledgeObjects) PutImmutable(_ context.Context, o k.Object) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.puts++
	s.data[o.Key] = o
	return errors.New("lost response after object persisted")
}
func (s *ambiguousKnowledgeObjects) Inspect(_ context.Context, o k.Object) (k.Inspection, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.inspectUnavailable {
		return k.Inspection{}, k.ErrUnavailable
	}
	if s.mismatch {
		return k.Inspection{Exists: true, SHA256: k.Digest([]byte("other")), SizeBytes: 5}, nil
	}
	stored, ok := s.data[o.Key]
	return k.Inspection{Exists: ok, SHA256: stored.SHA256, SizeBytes: stored.SizeBytes}, nil
}
func (s *ambiguousKnowledgeObjects) ReadBounded(_ context.Context, o k.Object, _ int64) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.data[o.Key].Data, nil
}

type unusedKnowledgeParser struct{}

func (unusedKnowledgeParser) Parse(context.Context, string, []byte) k.ParseResult {
	panic("TXT must not call external parser")
}
