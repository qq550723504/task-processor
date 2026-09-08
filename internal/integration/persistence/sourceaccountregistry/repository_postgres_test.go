//go:build integration

package sourceaccountregistry_test

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"net/url"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"task-processor/internal/app/schema/sourceaccountregistry"
	"task-processor/internal/authidentity"
	"task-processor/internal/authz"
	registrystore "task-processor/internal/integration/persistence/sourceaccountregistry"
	registry "task-processor/internal/sourceaccountregistry"
)

func TestSourceAccountRegistryPostgresBusinessChain(t *testing.T) {
	db := openRegistryPostgres(t)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	if err := sourceaccountregistry.Migrate(ctx, db); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}
	if err := sourceaccountregistry.Migrate(ctx, db); err != nil {
		t.Fatalf("repeat Migrate() error = %v", err)
	}
	assertTableExists(t, db, "source_account_resources", true)
	assertTableExists(t, db, "source_account_operations", true)
	assertTableExists(t, db, "goose_source_account_registry_version", true)
	assertTableExists(t, db, "organization_source_accounts", false)
	assertTableExists(t, db, "source_account", false)
	assertPublicTableInventory(t, db, []string{"goose_source_account_registry_version", "source_account_operations", "source_account_resources"})

	now := time.Date(2026, 9, 9, 1, 2, 3, 123456789, time.UTC)
	wantTimestamp := now.Truncate(time.Microsecond)
	repository, err := registrystore.NewRepository(db)
	if err != nil {
		t.Fatal(err)
	}
	rollbackID, err := uuid.NewV7()
	if err != nil {
		t.Fatal(err)
	}
	rollbackAccount, err := registry.NewAccount(rollbackID.String(), "org-rollback", "actor-1", "Rollback", registry.Platform1688, now)
	if err != nil {
		t.Fatal(err)
	}
	_, err = repository.Run(ctx, registry.Operation{
		Scope: registry.Scope{OrganizationID: "org-rollback", ActorSubject: "actor-1"}, Key: uuid.NewString(), Kind: registry.OperationRegister, Fingerprint: strings.Repeat("a", 64),
	}, func(tx registry.Transaction) (registry.Account, error) {
		if _, found, replayErr := tx.Replay(); replayErr != nil || found {
			return registry.Account{}, fmt.Errorf("unexpected replay: found=%t err=%v", found, replayErr)
		}
		if _, countErr := tx.CountForCreate(); countErr != nil {
			return registry.Account{}, countErr
		}
		if insertErr := tx.Insert(rollbackAccount); insertErr != nil {
			return registry.Account{}, insertErr
		}
		if completeErr := tx.Complete(rollbackAccount); completeErr != nil {
			return registry.Account{}, completeErr
		}
		return registry.Account{}, errors.New("force rollback after account and receipt")
	})
	if !errors.Is(err, registry.ErrUnavailable) {
		t.Fatalf("forced rollback error = %v", err)
	}
	assertCount(t, db, "source_account_resources", "organization_id = ?", []any{"org-rollback"}, 0)
	assertCount(t, db, "source_account_operations", "organization_id = ?", []any{"org-rollback"}, 0)

	service := newRegistryService(t, db, now)
	requestContext := registryIdentity(now, "org-b", "actor-1", "listingkit_operator")
	key := uuid.NewString()
	created, err := service.Register(requestContext, key, registry.RegisterInput{DisplayName: "Primary", Platform: "1688"})
	if err != nil || created.Replayed {
		t.Fatalf("Register() = %#v, %v", created, err)
	}
	if !created.Account.CreatedAt.Equal(wantTimestamp) || !created.Account.UpdatedAt.Equal(wantTimestamp) {
		t.Fatalf("Register() timestamps = %s/%s, want PostgreSQL precision %s", created.Account.CreatedAt, created.Account.UpdatedAt, wantTimestamp)
	}
	replayed, err := service.Register(requestContext, key, registry.RegisterInput{DisplayName: "Primary", Platform: "1688"})
	if err != nil || !replayed.Replayed || replayed.Account.ID != created.Account.ID || !replayed.Account.CreatedAt.Equal(created.Account.CreatedAt) || !replayed.Account.UpdatedAt.Equal(created.Account.UpdatedAt) {
		t.Fatalf("Register() replay = %#v, %v", replayed, err)
	}
	if _, err := service.Register(requestContext, key, registry.RegisterInput{DisplayName: "Different", Platform: "1688"}); !errors.Is(err, registry.ErrIdempotencyConflict) {
		t.Fatalf("different payload error = %v", err)
	}

	disabled, err := service.Disable(requestContext, uuid.NewString(), created.Account.ID, 1)
	if err != nil || disabled.Account.ManagementStatus != registry.ManagementStatusDisabled || disabled.Account.Version != 2 {
		t.Fatalf("Disable() = %#v, %v", disabled, err)
	}
	createReplay, err := service.Register(requestContext, key, registry.RegisterInput{DisplayName: "Primary", Platform: "1688"})
	if err != nil || !createReplay.Replayed || createReplay.Account.ManagementStatus != registry.ManagementStatusDisabled || createReplay.Account.Version != 2 {
		t.Fatalf("old create replay = %#v, %v", createReplay, err)
	}
	enabled, err := service.Enable(requestContext, uuid.NewString(), created.Account.ID, 2)
	if err != nil || enabled.Account.ManagementStatus != registry.ManagementStatusEnabled || enabled.Account.Version != 3 {
		t.Fatalf("Enable() = %#v, %v", enabled, err)
	}
	if _, err := service.Enable(requestContext, uuid.NewString(), created.Account.ID, 3); !errors.Is(err, registry.ErrInvalidTransition) {
		t.Fatalf("fresh duplicate transition error = %v", err)
	}
	assertCount(t, db, "source_account_operations", "organization_id = ?", []any{"org-b"}, 3)

	before := registryBusinessState(t, db)
	got, err := service.Get(requestContext, created.Account.ID)
	if err != nil || got.Version != 3 {
		t.Fatalf("Get() = %#v, %v", got, err)
	}
	page, err := service.List(requestContext, registry.PageRequest{Limit: 20})
	if err != nil || len(page.Items) != 1 || page.Next != nil {
		t.Fatalf("List() = %#v, %v", page, err)
	}
	if after := registryBusinessState(t, db); after != before {
		t.Fatalf("GET paths wrote business state:\nbefore=%s\nafter=%s", before, after)
	}
	if _, err := service.Get(registryIdentity(now, "org-other", "actor-1", "listingkit_operator"), created.Account.ID); !errors.Is(err, registry.ErrNotFound) {
		t.Fatalf("foreign Organization Get() error = %v", err)
	}

	pageContext := registryIdentity(now, "org-page", "actor-1", "listingkit_operator")
	wantPageIDs := make(map[string]struct{}, 3)
	for _, name := range []string{"First", "Second", "Third"} {
		result, registerErr := service.Register(pageContext, uuid.NewString(), registry.RegisterInput{DisplayName: name, Platform: "1688"})
		if registerErr != nil {
			t.Fatalf("register page fixture %s: %v", name, registerErr)
		}
		wantPageIDs[result.Account.ID] = struct{}{}
	}
	firstPage, err := service.List(pageContext, registry.PageRequest{Limit: 2})
	if err != nil || len(firstPage.Items) != 2 || firstPage.Next == nil {
		t.Fatalf("first stable page = %#v, %v", firstPage, err)
	}
	secondPage, err := service.List(pageContext, registry.PageRequest{Limit: 2, After: firstPage.Next})
	if err != nil || len(secondPage.Items) != 1 || secondPage.Next != nil {
		t.Fatalf("second stable page = %#v, %v", secondPage, err)
	}
	seenPageIDs := make(map[string]struct{}, 3)
	for _, account := range append(firstPage.Items, secondPage.Items...) {
		if _, duplicate := seenPageIDs[account.ID]; duplicate {
			t.Fatalf("pagination repeated account %s", account.ID)
		}
		seenPageIDs[account.ID] = struct{}{}
	}
	if !maps.Equal(seenPageIDs, wantPageIDs) {
		t.Fatalf("pagination IDs = %v, want %v", seenPageIDs, wantPageIDs)
	}

	reconstructed := newRegistryService(t, db, now.Add(time.Minute))
	got, err = reconstructed.Get(registryIdentity(now, "org-b", "actor-1", "listingkit_operator"), created.Account.ID)
	if err != nil || got.Version != 3 || got.ManagementStatus != registry.ManagementStatusEnabled {
		t.Fatalf("reconstructed Get() = %#v, %v", got, err)
	}
	assertCount(t, db, "source_account_resources", "organization_id = ?", []any{"org-b"}, 1)
	assertCount(t, db, "source_account_operations", "organization_id = ?", []any{"org-b"}, 3)
}

func TestSourceAccountRegistryPostgresCommitFailureIsOutcomeUnknown(t *testing.T) {
	db := openRegistryPostgres(t)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	if err := sourceaccountregistry.Migrate(ctx, db); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 9, 9, 1, 2, 3, 123456789, time.UTC)
	service := newRegistryService(t, db, now)
	if err := db.Exec(`CREATE TABLE public.source_account_commit_gate (id UUID PRIMARY KEY)`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`ALTER TABLE public.source_account_operations ADD CONSTRAINT source_account_operations_test_commit_fkey FOREIGN KEY (idempotency_key) REFERENCES public.source_account_commit_gate (id) DEFERRABLE INITIALLY DEFERRED`).Error; err != nil {
		t.Fatal(err)
	}

	requestContext := registryIdentity(now, "org-commit", "actor-1", "listingkit_operator")
	key := uuid.NewString()
	if _, err := service.Register(requestContext, key, registry.RegisterInput{DisplayName: "Commit failure", Platform: "1688"}); !errors.Is(err, registry.ErrOutcomeUnknown) {
		t.Fatalf("Register() commit error = %v, want ErrOutcomeUnknown", err)
	}
	assertCount(t, db, "source_account_resources", "organization_id = ?", []any{"org-commit"}, 0)
	assertCount(t, db, "source_account_operations", "organization_id = ?", []any{"org-commit"}, 0)

	if err := db.Exec(`ALTER TABLE public.source_account_operations DROP CONSTRAINT source_account_operations_test_commit_fkey`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`DROP TABLE public.source_account_commit_gate`).Error; err != nil {
		t.Fatal(err)
	}
	created, err := service.Register(requestContext, key, registry.RegisterInput{DisplayName: "Commit failure", Platform: "1688"})
	if err != nil || created.Replayed {
		t.Fatalf("same-key retry after verified rollback = %#v, %v", created, err)
	}
	replayed, err := service.Register(requestContext, key, registry.RegisterInput{DisplayName: "Commit failure", Platform: "1688"})
	if err != nil || !replayed.Replayed || replayed.Account.ID != created.Account.ID {
		t.Fatalf("same-key committed replay = %#v, %v", replayed, err)
	}
	assertCount(t, db, "source_account_resources", "organization_id = ?", []any{"org-commit"}, 1)
	assertCount(t, db, "source_account_operations", "organization_id = ?", []any{"org-commit"}, 1)
}

func TestSourceAccountRegistryPostgresConcurrencyAndCapacity(t *testing.T) {
	db, dsn := openRegistryPostgresWithDSN(t)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	if err := sourceaccountregistry.Migrate(ctx, db); err != nil {
		t.Fatal(err)
	}
	db = openRegistryGORM(t, registryDSNWithDefaultIsolation(t, dsn, "repeatable read"))
	var defaultIsolation string
	if err := db.Raw(`SHOW default_transaction_isolation`).Scan(&defaultIsolation).Error; err != nil || defaultIsolation != "repeatable read" {
		t.Fatalf("default transaction isolation = %q, err=%v", defaultIsolation, err)
	}
	now := time.Date(2026, 9, 9, 1, 2, 3, 0, time.UTC)
	service := newRegistryService(t, db, now)
	requestContext := registryIdentity(now, "org-same", "actor-1", "listingkit_operator")
	key := uuid.NewString()

	const sameKeyCallers = 12
	results := make(chan registry.MutationResult, sameKeyCallers)
	errorsChannel := make(chan error, sameKeyCallers)
	var wait sync.WaitGroup
	for range sameKeyCallers {
		wait.Add(1)
		go func() {
			defer wait.Done()
			result, err := service.Register(requestContext, key, registry.RegisterInput{DisplayName: "Same", Platform: "1688"})
			results <- result
			errorsChannel <- err
		}()
	}
	wait.Wait()
	close(results)
	close(errorsChannel)
	fresh := 0
	accountID := ""
	for err := range errorsChannel {
		if err != nil {
			t.Fatalf("same-key Register() error = %v", err)
		}
	}
	for result := range results {
		if !result.Replayed {
			fresh++
		}
		if accountID == "" {
			accountID = result.Account.ID
		} else if result.Account.ID != accountID {
			t.Fatalf("same-key IDs differ: %q != %q", result.Account.ID, accountID)
		}
	}
	if fresh != 1 {
		t.Fatalf("same-key fresh results = %d", fresh)
	}

	stateErrors := make(chan error, 2)
	for range 2 {
		wait.Add(1)
		go func() {
			defer wait.Done()
			_, err := service.Disable(requestContext, uuid.NewString(), accountID, 1)
			stateErrors <- err
		}()
	}
	wait.Wait()
	close(stateErrors)
	stateSuccess, stateConflict := 0, 0
	for err := range stateErrors {
		switch {
		case err == nil:
			stateSuccess++
		case errors.Is(err, registry.ErrVersionConflict):
			stateConflict++
		default:
			t.Fatalf("concurrent state error = %v", err)
		}
	}
	if stateSuccess != 1 || stateConflict != 1 {
		t.Fatalf("state results success=%d conflict=%d", stateSuccess, stateConflict)
	}

	capacityContext := registryIdentity(now, "org-capacity", "actor-1", "listingkit_operator")
	const capacityCallers = registry.MaxAccountsPerOrganization + 4
	capacityErrors := make(chan error, capacityCallers)
	for index := 0; index < capacityCallers; index++ {
		wait.Add(1)
		go func(index int) {
			defer wait.Done()
			_, err := service.Register(capacityContext, uuid.NewString(), registry.RegisterInput{DisplayName: fmt.Sprintf("Account %03d", index), Platform: "1688"})
			capacityErrors <- err
		}(index)
	}
	wait.Wait()
	close(capacityErrors)
	capacitySuccess, capacityRejected := 0, 0
	for err := range capacityErrors {
		switch {
		case err == nil:
			capacitySuccess++
		case errors.Is(err, registry.ErrResourceLimitReached):
			capacityRejected++
		default:
			t.Fatalf("capacity error = %v", err)
		}
	}
	if capacitySuccess != registry.MaxAccountsPerOrganization || capacityRejected != capacityCallers-registry.MaxAccountsPerOrganization {
		t.Fatalf("capacity success=%d rejected=%d", capacitySuccess, capacityRejected)
	}
	assertCount(t, db, "source_account_resources", "organization_id = ?", []any{"org-capacity"}, registry.MaxAccountsPerOrganization)
}

func TestSourceAccountRegistrySchemaDriftFailsReadOnlyConstruction(t *testing.T) {
	db := openRegistryPostgres(t)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	if err := sourceaccountregistry.Migrate(ctx, db); err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`ALTER TABLE public.source_account_resources ADD CONSTRAINT source_account_resources_unexpected_check CHECK (display_name <> 'blocked') NOT VALID`).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := registrystore.NewRepository(db); !errors.Is(err, registry.ErrUnavailable) {
		t.Fatalf("NewRepository() NOT VALID drift error = %v", err)
	}
	if err := db.Exec(`ALTER TABLE public.source_account_resources DROP CONSTRAINT source_account_resources_unexpected_check`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`ALTER TABLE public.source_account_resources DROP CONSTRAINT source_account_resources_platform_check`).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := registrystore.NewRepository(db); !errors.Is(err, registry.ErrUnavailable) {
		t.Fatalf("NewRepository() drift error = %v", err)
	}
	if err := db.Exec(`ALTER TABLE public.source_account_resources ADD CONSTRAINT source_account_resources_platform_check CHECK (platform <> '')`).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := registrystore.NewRepository(db); !errors.Is(err, registry.ErrUnavailable) {
		t.Fatalf("NewRepository() same-name constraint drift error = %v", err)
	}
	if err := sourceaccountregistry.Migrate(ctx, db); err == nil {
		t.Fatal("repeat Migrate() repaired drift")
	}
}

func TestSourceAccountRegistryRuntimeAlwaysUsesVerifiedPublicSchema(t *testing.T) {
	db, dsn := openRegistryPostgresWithDSN(t)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	if err := sourceaccountregistry.Migrate(ctx, db); err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`CREATE SCHEMA shadow`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`CREATE TABLE shadow.source_account_resources (LIKE public.source_account_resources INCLUDING ALL)`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`CREATE TABLE shadow.source_account_operations (LIKE public.source_account_operations INCLUDING ALL)`).Error; err != nil {
		t.Fatal(err)
	}
	shadowOnlyDB := openRegistryGORM(t, registryDSNWithSearchPath(t, dsn, "shadow,pg_catalog"))
	now := time.Date(2026, 9, 9, 1, 2, 3, 0, time.UTC)
	service := newRegistryService(t, shadowOnlyDB, now)

	// Schema admission and runtime queries remain bound to public even when
	// public is absent from search_path and valid same-name shadows pre-exist.

	requestContext := registryIdentity(now, "org-public", "actor-1", "listingkit_operator")
	key := uuid.NewString()
	created, err := service.Register(requestContext, key, registry.RegisterInput{DisplayName: "Public owner", Platform: "1688"})
	if err != nil || created.Replayed {
		t.Fatalf("Register() = %#v, %v", created, err)
	}
	if _, err := service.Register(requestContext, key, registry.RegisterInput{DisplayName: "Public owner", Platform: "1688"}); err != nil {
		t.Fatalf("Register() replay error = %v", err)
	}
	if _, err := service.Get(requestContext, created.Account.ID); err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if page, err := service.List(requestContext, registry.PageRequest{Limit: 20}); err != nil || len(page.Items) != 1 {
		t.Fatalf("List() = %#v, %v", page, err)
	}
	if _, err := service.Disable(requestContext, uuid.NewString(), created.Account.ID, created.Account.Version); err != nil {
		t.Fatalf("Disable() error = %v", err)
	}

	assertCount(t, db, "public.source_account_resources", "organization_id = ?", []any{"org-public"}, 1)
	assertCount(t, db, "public.source_account_operations", "organization_id = ?", []any{"org-public"}, 2)
	assertCount(t, db, "shadow.source_account_resources", "organization_id = ?", []any{"org-public"}, 0)
	assertCount(t, db, "shadow.source_account_operations", "organization_id = ?", []any{"org-public"}, 0)
}

func TestSourceAccountRegistrySchemaHistoryAlwaysUsesPublic(t *testing.T) {
	db, dsn := openRegistryPostgresWithDSN(t)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	if err := db.Exec(`CREATE SCHEMA shadow`).Error; err != nil {
		t.Fatal(err)
	}

	shadowOnlyDB := openRegistryGORM(t, registryDSNWithSearchPath(t, dsn, "shadow,pg_catalog"))
	if err := sourceaccountregistry.Migrate(ctx, shadowOnlyDB); err != nil {
		t.Fatalf("public-excluded Migrate() error = %v", err)
	}
	assertSchemaTableExists(t, db, "public", "goose_source_account_registry_version", true)
	assertSchemaTableExists(t, db, "shadow", "goose_source_account_registry_version", false)

	if err := sourceaccountregistry.Migrate(ctx, db); err != nil {
		t.Fatalf("public-first repeat Migrate() error = %v", err)
	}
	assertPublicTableInventory(t, db, []string{"goose_source_account_registry_version", "source_account_operations", "source_account_resources"})
}

func openRegistryPostgres(t *testing.T) *gorm.DB {
	t.Helper()
	db, _ := openRegistryPostgresWithDSN(t)
	return db
}

func openRegistryPostgresWithDSN(t *testing.T) (*gorm.DB, string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	container, err := tcpostgres.Run(ctx, "postgres:16-alpine", tcpostgres.WithDatabase("sourceaccountregistry"), tcpostgres.WithUsername("sourceaccountregistry"), tcpostgres.WithPassword("sourceaccountregistry"), tcpostgres.BasicWaitStrategies())
	if err != nil {
		t.Fatalf("start PostgreSQL: %v", err)
	}
	t.Cleanup(func() { _ = container.Terminate(context.Background()) })
	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}
	return openRegistryGORM(t, dsn), dsn
}

func openRegistryGORM(t *testing.T, dsn string) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	// Keep the client pool below PostgreSQL's default connection ceiling while
	// still exercising concurrent operation and Organization locks.
	sqlDB.SetMaxOpenConns(32)
	t.Cleanup(func() { _ = sqlDB.Close() })
	return db
}

func registryDSNWithSearchPath(t *testing.T, dsn, searchPath string) string {
	t.Helper()
	parsed, err := url.Parse(dsn)
	if err != nil {
		t.Fatal(err)
	}
	query := parsed.Query()
	query.Set("search_path", searchPath)
	parsed.RawQuery = query.Encode()
	return parsed.String()
}

func registryDSNWithDefaultIsolation(t *testing.T, dsn, isolation string) string {
	t.Helper()
	parsed, err := url.Parse(dsn)
	if err != nil {
		t.Fatal(err)
	}
	query := parsed.Query()
	query.Set("default_transaction_isolation", isolation)
	parsed.RawQuery = query.Encode()
	return parsed.String()
}

func newRegistryService(t *testing.T, db *gorm.DB, now time.Time) *registry.Service {
	t.Helper()
	repository, err := registrystore.NewRepository(db)
	if err != nil {
		t.Fatal(err)
	}
	authorizer, err := authz.NewListingKitAuthorizer(nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	service, err := registry.NewService(repository, authorizer, registry.WithClock(func() time.Time { return now }))
	if err != nil {
		t.Fatal(err)
	}
	return service
}

func registryIdentity(now time.Time, organizationID, actor, role string) context.Context {
	return authidentity.WithAuthenticatedIdentity(context.Background(), authidentity.AuthenticatedIdentity{TenantID: organizationID, EffectiveOrganizationID: organizationID, UserID: actor, Roles: []string{role}, TokenExpiresAt: now.Add(time.Hour)})
}

func assertTableExists(t *testing.T, db *gorm.DB, table string, want bool) {
	t.Helper()
	assertSchemaTableExists(t, db, "public", table, want)
}

func assertSchemaTableExists(t *testing.T, db *gorm.DB, schema, table string, want bool) {
	t.Helper()
	var count int64
	if err := db.Raw(`SELECT count(*) FROM information_schema.tables WHERE table_schema = ? AND table_name = ?`, schema, table).Scan(&count).Error; err != nil {
		t.Fatal(err)
	}
	if got := count == 1; got != want {
		t.Fatalf("table %s.%s exists=%t, want %t", schema, table, got, want)
	}
}

func assertPublicTableInventory(t *testing.T, db *gorm.DB, want []string) {
	t.Helper()
	var tables []string
	if err := db.Raw(`SELECT table_name FROM information_schema.tables WHERE table_schema = 'public' AND table_type = 'BASE TABLE' ORDER BY table_name`).Scan(&tables).Error; err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(tables, want) {
		t.Fatalf("public table inventory = %v, want %v", tables, want)
	}
}

func assertCount(t *testing.T, db *gorm.DB, table, predicate string, args []any, want int) {
	t.Helper()
	var count int64
	if err := db.Table(table).Where(predicate, args...).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != int64(want) {
		t.Fatalf("%s count=%d, want %d", table, count, want)
	}
}

func registryBusinessState(t *testing.T, db *gorm.DB) string {
	t.Helper()
	var state string
	query := `SELECT jsonb_build_object(
  'resources', COALESCE((SELECT jsonb_agg(to_jsonb(row_value) ORDER BY organization_id, id) FROM (SELECT *, xmin::text AS row_version FROM public.source_account_resources) AS row_value), '[]'::jsonb),
  'operations', COALESCE((SELECT jsonb_agg(to_jsonb(row_value) ORDER BY organization_id, actor_subject, idempotency_key) FROM (SELECT *, xmin::text AS row_version FROM public.source_account_operations) AS row_value), '[]'::jsonb)
)::text`
	if err := db.Raw(query).Scan(&state).Error; err != nil {
		t.Fatal(err)
	}
	return state
}
