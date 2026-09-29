//go:build integration

package knowledge_test

import (
	"context"
	"errors"
	"github.com/google/uuid"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"net/url"
	store "task-processor/internal/integration/persistence/knowledge"
	k "task-processor/internal/knowledge"
	"testing"
)

func TestKnowledgeRuntimePrivilegesCrashBudgetAndDisableFence(t *testing.T) {
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
	owner, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	sqlOwner, _ := owner.DB()
	t.Cleanup(func() { _ = sqlOwner.Close() })
	if err = store.Install(ctx, owner); err != nil {
		t.Fatal(err)
	}
	for _, query := range []string{
		"CREATE ROLE knowledge_runtime LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS PASSWORD 'isolated-runtime-password'",
		"CREATE DATABASE unrelated",
		"REVOKE ALL ON DATABASE knowledge,unrelated,postgres,template1 FROM PUBLIC",
		"REVOKE CREATE ON SCHEMA public FROM PUBLIC",
	} {
		if err = owner.Exec(query).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err = store.GrantRuntime(ctx, owner); err != nil {
		t.Fatal(err)
	}
	target, err := url.Parse(dsn)
	if err != nil {
		t.Fatal(err)
	}
	target.User = url.UserPassword("knowledge_runtime", "isolated-runtime-password")
	runtimeDB, err := gorm.Open(postgres.Open(target.String()), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	pool, _ := runtimeDB.DB()
	pool.SetMaxOpenConns(4)
	t.Cleanup(func() { _ = pool.Close() })
	if err = store.VerifyRuntime(ctx, runtimeDB); err != nil {
		t.Fatal(err)
	}
	if err = runtimeDB.Exec("CREATE TABLE public.forbidden(id int)").Error; err == nil {
		t.Fatal("runtime performed DDL")
	}
	if err = runtimeDB.Exec("CREATE TEMP TABLE forbidden(id int)").Error; err == nil {
		t.Fatal("runtime created temporary table")
	}
	target.Path = "/unrelated"
	foreign, err := gorm.Open(postgres.Open(target.String()), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err == nil {
		foreignPool, _ := foreign.DB()
		_ = foreignPool.Close()
		t.Fatal("runtime connected to another database")
	}
	repo, err := store.NewRepository(ctx, runtimeDB)
	if err != nil {
		t.Fatal(err)
	}
	scope := k.Scope{OrganizationID: "org-a", ActorID: "actor-a"}
	apply := func(command k.Command) k.Result {
		t.Helper()
		command.Scope = scope
		command.Key = uuid.NewString()
		command.Fingerprint = k.Digest([]byte(command.Key))
		result, e := repo.Apply(ctx, command)
		if e != nil {
			t.Fatal(e)
		}
		return result
	}
	base := apply(k.Command{Kind: "base_create", Name: "Brand"}).Base
	admit := func() k.Result {
		return apply(k.Command{Kind: "source_create", BaseID: base.ID, Name: "document", Upload: &k.Revision{Filename: "document.txt", ContentType: "text/plain", SizeBytes: 5, SHA256: k.Digest([]byte("hello"))}})
	}
	first := admit()
	if err = runtimeDB.Exec("UPDATE knowledge_revisions SET sha256=? WHERE id=?", k.Digest([]byte("other")), first.Revision.ID).Error; err == nil {
		t.Fatal("runtime changed immutable digest")
	}
	upload, ok, err := repo.ClaimUpload(ctx, scope.OrganizationID, first.Revision.ID, "upload")
	if err != nil || !ok {
		t.Fatal(err)
	}
	if err = repo.ConfirmObject(ctx, upload); err != nil {
		t.Fatal(err)
	}
	var previous k.Revision
	for attempt := 1; attempt <= 3; attempt++ {
		claimed, e := repo.ClaimProcessing(ctx, "process-"+uuid.NewString(), 2)
		if e != nil || len(claimed) != 1 || claimed[0].Attempts != attempt {
			t.Fatalf("attempt %d: %+v %v", attempt, claimed, e)
		}
		if attempt > 1 && !errors.Is(repo.Finish(ctx, previous, k.ParseResult{Text: "stale"}), k.ErrLeaseLost) {
			t.Fatal("stale process finished replacement lease")
		}
		previous = claimed[0]
		if e = owner.Exec("UPDATE knowledge_revisions SET lease_until=now()-interval '1 second' WHERE id=?", first.Revision.ID).Error; e != nil {
			t.Fatal(e)
		}
	}
	claimed, err := repo.ClaimProcessing(ctx, "fourth-process", 2)
	if err != nil || len(claimed) != 0 {
		t.Fatal("fourth parser attempt admitted", err)
	}
	source, err := repo.GetSource(ctx, scope.OrganizationID, first.Source.ID)
	if err != nil || source.LatestRevision.State != k.Failed || source.LatestRevision.Failure != "PARSER_RETRIES_EXHAUSTED" || source.CurrentReadableRevision != nil {
		t.Fatalf("crash exhaustion: %+v %v", source, err)
	}
	second := admit()
	upload, ok, err = repo.ClaimUpload(ctx, scope.OrganizationID, second.Revision.ID, "upload-second")
	if err != nil || !ok {
		t.Fatal(err)
	}
	if err = repo.ConfirmObject(ctx, upload); err != nil {
		t.Fatal(err)
	}
	claimed, err = repo.ClaimProcessing(ctx, "partial-process", 2)
	if err != nil || len(claimed) != 1 {
		t.Fatal(err)
	}
	if err = repo.Finish(ctx, claimed[0], k.ParseResult{Text: "usable portion", Warning: "INCOMPLETE_EXTRACTION"}); err != nil {
		t.Fatal(err)
	}
	source, err = repo.GetSource(ctx, scope.OrganizationID, second.Source.ID)
	if err != nil || source.CurrentReadableRevision == nil || source.CurrentReadableRevision.State != k.Partial {
		t.Fatal("partial promotion missing", err)
	}
	third := admit()
	upload, ok, err = repo.ClaimUpload(ctx, scope.OrganizationID, third.Revision.ID, "upload-third")
	if err != nil || !ok {
		t.Fatal(err)
	}
	if err = repo.ConfirmObject(ctx, upload); err != nil {
		t.Fatal(err)
	}
	claimed, err = repo.ClaimProcessing(ctx, "disabled-process", 2)
	if err != nil || len(claimed) != 1 {
		t.Fatalf("disabled process claims %+v: %v", claimed, err)
	}
	baseNow, err := repo.GetBase(ctx, scope.OrganizationID, base.ID)
	if err != nil {
		t.Fatal(err)
	}
	apply(k.Command{Kind: "base_disable", BaseID: base.ID, Version: baseNow.Version})
	if err = repo.Finish(ctx, claimed[0], k.ParseResult{Text: "late success"}); err != nil {
		t.Fatal(err)
	}
	source, err = repo.GetSource(ctx, scope.OrganizationID, third.Source.ID)
	if err != nil || source.CurrentReadableRevision != nil || source.BaseState != k.Disabled {
		t.Fatal("disabled base promoted late result", err)
	}
	if _, err = repo.Preview(ctx, scope.OrganizationID, second.Source.ID, second.Revision.ID); !errors.Is(err, k.ErrInactive) {
		t.Fatal("disabled base permitted preview", err)
	}
}
