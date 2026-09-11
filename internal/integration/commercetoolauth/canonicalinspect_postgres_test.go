//go:build integration

package commercetoolauth_test

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"testing"
	"time"

	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"go.opentelemetry.io/otel"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"task-processor/internal/authidentity"
	"task-processor/internal/authz"
	"task-processor/internal/commercetool"
	"task-processor/internal/httproute"
	"task-processor/internal/integration/commercetoolauth"
	catalogstore "task-processor/internal/integration/persistence/product/catalog"
	"task-processor/internal/product/catalog"
	canonicalinspect "task-processor/internal/product/catalog/tools/canonicalinspect"
	"task-processor/internal/workbenchcontext"
)

func TestCanonicalInspectPostgresExactBoundedReaderChain(t *testing.T) {
	db := openCanonicalInspectPostgres(t)
	if err := catalogstore.AutoMigrate(db); err != nil {
		t.Fatalf("AutoMigrate(): %v", err)
	}
	repository, err := catalogstore.NewRepository(db)
	if err != nil {
		t.Fatalf("NewRepository(): %v", err)
	}
	publisher, err := catalog.NewPublisher(repository)
	if err != nil {
		t.Fatalf("NewPublisher(): %v", err)
	}
	identity := catalog.SnapshotIdentity{TenantID: "org-a", ProductKey: "product-1"}
	first := publishPostgresSnapshot(t, publisher, identity, "publication-v1", catalog.ProductSnapshot{Title: "Bottle v1"})
	second := publishPostgresSnapshot(t, publisher, identity, "publication-v2", catalog.ProductSnapshot{Title: "Bottle v2"})

	// Arrange a sparse head, then let the Catalog Publisher own creation of the
	// final >2^53 immutable version. This exercises PostgreSQL BIGINT and JSON
	// protocol precision without direct insertion of the final fact.
	const highVersion = uint64(9007199254740993)
	if err := db.Model(&catalogstore.SnapshotHeadRecord{}).
		Where("tenant_id = ? AND product_key = ?", identity.TenantID, identity.ProductKey).
		Update("current_version", highVersion-1).Error; err != nil {
		t.Fatalf("arrange sparse Catalog head: %v", err)
	}
	high := publishPostgresSnapshot(t, publisher, identity, "publication-high", catalog.ProductSnapshot{Title: "Bottle high"})
	if high.Version != highVersion {
		t.Fatalf("high version = %d, want %d", high.Version, highVersion)
	}

	invoker, request, audits := newPostgresCanonicalInvoker(t, db)
	assertPostgresTitle(t, invokePostgresCanonical(t, invoker, request, first.Version, "exact-v1"), "Bottle v1", "1")
	assertPostgresTitle(t, invokePostgresCanonical(t, invoker, request, second.Version, "exact-v2"), "Bottle v2", "2")
	assertPostgresTitle(t, invokePostgresCanonical(t, invoker, request, high.Version, "exact-high"), "Bottle high", "9007199254740993")
	assertPostgresCode(t, invoker, request, 3, "gap-no-latest-fallback", commercetool.ErrorNotFound)

	outputOversize := publishPostgresSnapshot(t, publisher, identity, "publication-output-oversize", catalog.ProductSnapshot{Description: strings.Repeat("x", canonicalinspect.MaxOutputBytes)})
	assertPostgresCode(t, invoker, request, outputOversize.Version, "output-oversize", commercetool.ErrorFailedPrecondition)
	materializationOversize := publishPostgresSnapshot(t, publisher, identity, "publication-materialization-oversize", catalog.ProductSnapshot{Description: strings.Repeat("x", canonicalinspect.MaxCatalogSnapshotBytes)})
	assertPostgresCode(t, invoker, request, materializationOversize.Version, "snapshot-oversize", commercetool.ErrorFailedPrecondition)

	corrupt := publishPostgresSnapshot(t, publisher, identity, "publication-corrupt", catalog.ProductSnapshot{Title: "Before corruption"})
	if err := db.Model(&catalogstore.SnapshotVersionRecord{}).
		Where("tenant_id = ? AND product_key = ? AND version = ?", identity.TenantID, identity.ProductKey, corrupt.Version).
		Update("snapshot_json", []byte(`{"title":"tampered"}`)).Error; err != nil {
		t.Fatalf("inject corrupt payload: %v", err)
	}
	stateBeforeRead := postgresCatalogState(t, db)
	assertPostgresCode(t, invoker, request, corrupt.Version, "corrupt", commercetool.ErrorInternal)
	if stateAfterRead := postgresCatalogState(t, db); stateAfterRead != stateBeforeRead {
		t.Fatalf("read invocation changed durable Catalog state\nbefore=%s\nafter=%s", stateBeforeRead, stateAfterRead)
	}
	if len(audits.records) != 7 {
		t.Fatalf("audit records = %d, want 7", len(audits.records))
	}
}

func openCanonicalInspectPostgres(t *testing.T) *gorm.DB {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	container, err := tcpostgres.Run(ctx, "postgres:16-alpine", tcpostgres.WithDatabase("tool_c1"), tcpostgres.WithUsername("tool_c1"), tcpostgres.WithPassword("tool_c1"), tcpostgres.BasicWaitStrategies())
	if err != nil {
		t.Fatalf("start task-owned PostgreSQL: %v", err)
	}
	t.Cleanup(func() { _ = container.Terminate(context.Background()) })
	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("PostgreSQL connection string: %v", err)
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("open PostgreSQL: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("PostgreSQL pool: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	return db
}

func publishPostgresSnapshot(t *testing.T, publisher *catalog.Publisher, identity catalog.SnapshotIdentity, publicationID string, snapshot catalog.ProductSnapshot) catalog.PublishedSnapshot {
	t.Helper()
	published, err := publisher.Publish(context.Background(), catalog.PublishRequest{Identity: identity, PublicationID: publicationID, Snapshot: snapshot})
	if err != nil {
		t.Fatalf("Publish(%s): %v", publicationID, err)
	}
	return published
}

type postgresGrantLoader struct{}

func (postgresGrantLoader) Load(_ context.Context, source workbenchcontext.GrantSource, _ workbenchcontext.GrantRequest) (workbenchcontext.GrantResult, error) {
	return workbenchcontext.GrantResult{Source: source, Grants: []authidentity.OrganizationGrant{{OrganizationID: "org-a", ProjectID: "project-1", Roles: []string{"listingkit_operator"}}}}, nil
}

func (postgresGrantLoader) Invalidate(string, string) {}

type postgresAuditRecorder struct{ records []commercetool.AuditRecord }

func (r *postgresAuditRecorder) RecordToolCall(_ context.Context, record commercetool.AuditRecord) error {
	r.records = append(r.records, record)
	return nil
}

func newPostgresCanonicalInvoker(t *testing.T, db *gorm.DB) (*canonicalinspect.Invoker, commercetoolauth.OrganizationRequest, *postgresAuditRecorder) {
	t.Helper()
	reader, err := catalogstore.NewBoundedSnapshotReader(db, canonicalinspect.MaxCatalogSnapshotBytes)
	if err != nil {
		t.Fatalf("NewBoundedSnapshotReader(): %v", err)
	}
	now := time.Date(2026, 9, 11, 12, 0, 0, 0, time.UTC)
	organizationResolver := workbenchcontext.NewResolver(postgresGrantLoader{}, "project-1", "v1", nil, workbenchcontext.WithResolverClock(func() time.Time { return now }))
	principalResolver, err := commercetoolauth.NewWorkbenchPrincipalResolver(commercetoolauth.CachedReadOrganizationResolverFunc(func(ctx context.Context, request commercetoolauth.OrganizationRequest) (authidentity.AuthenticatedIdentity, error) {
		return organizationResolver.Resolve(ctx, httproute.OrganizationAccessPolicyCachedRead, workbenchcontext.ResolveInput{
			Identity: request.Identity, BearerToken: request.BearerToken, RequestedOrganizationID: request.RequestedOrganizationID,
		})
	}), func() time.Time { return now })
	if err != nil {
		t.Fatalf("NewWorkbenchPrincipalResolver(): %v", err)
	}
	policy, err := authz.NewListingKitAuthorizer(nil, nil)
	if err != nil {
		t.Fatalf("NewListingKitAuthorizer(): %v", err)
	}
	authorizer, err := commercetoolauth.NewCasbinAuthorizer(policy)
	if err != nil {
		t.Fatalf("NewCasbinAuthorizer(): %v", err)
	}
	audits := &postgresAuditRecorder{}
	definition := canonicalinspect.Definition()
	invoker, err := canonicalinspect.NewInvoker(reader, commercetool.AgentDefinition{ID: "postgres.product-agent", Version: "v1.0.0", AllowedTools: []commercetool.ToolRef{definition.Ref}}, commercetool.InvocationDependencies{
		PrincipalResolver: principalResolver, Authorizer: authorizer, Recorder: audits,
		Tracer: otel.Tracer("canonicalinspect-postgres"), Now: func() time.Time { return now }, AuditTimeout: time.Second,
	})
	if err != nil {
		t.Fatalf("NewInvoker(): %v", err)
	}
	request := commercetoolauth.OrganizationRequest{Identity: authidentity.AuthenticatedIdentity{UserID: "user-1", HomeOrganizationID: "org-a", TokenExpiresAt: now.Add(time.Hour)}, BearerToken: "verified-bearer", RequestedOrganizationID: "org-a"}
	return invoker, request, audits
}

func invokePostgresCanonical(t *testing.T, invoker *canonicalinspect.Invoker, request commercetoolauth.OrganizationRequest, version uint64, callID string) commercetool.Result {
	t.Helper()
	ctx := commercetoolauth.WithOrganizationRequest(context.Background(), request)
	result, err := invoker.Invoke(ctx, commercetool.CallMetadata{CallID: callID, AgentID: "postgres.product-agent", AgentVersion: "v1.0.0", AgentRunID: "run-1", BusinessTaskID: "correlation-only"}, canonicalinspect.Input{ProductKey: "product-1", CatalogVersion: strconv.FormatUint(version, 10)})
	if err != nil {
		t.Fatalf("Invoke(%s): %v", callID, err)
	}
	return result
}

func assertPostgresCode(t *testing.T, invoker *canonicalinspect.Invoker, request commercetoolauth.OrganizationRequest, version uint64, callID string, want commercetool.ErrorCode) {
	t.Helper()
	ctx := commercetoolauth.WithOrganizationRequest(context.Background(), request)
	_, err := invoker.Invoke(ctx, commercetool.CallMetadata{CallID: callID, AgentID: "postgres.product-agent", AgentVersion: "v1.0.0", AgentRunID: "run-1", BusinessTaskID: "correlation-only"}, canonicalinspect.Input{ProductKey: "product-1", CatalogVersion: strconv.FormatUint(version, 10)})
	if commercetool.CodeOf(err) != want {
		t.Fatalf("Invoke(%s) code=%s want=%s error=%v", callID, commercetool.CodeOf(err), want, err)
	}
	if err != nil && (strings.Contains(err.Error(), "tampered") || strings.Contains(err.Error(), "snapshot_json")) {
		t.Fatalf("Invoke(%s) leaked persistence detail: %v", callID, err)
	}
}

func assertPostgresTitle(t *testing.T, result commercetool.Result, wantTitle, wantVersion string) {
	t.Helper()
	var output canonicalinspect.Output
	if err := json.Unmarshal(result.Output, &output); err != nil {
		t.Fatalf("unmarshal output: %v", err)
	}
	if output.Snapshot.Title != wantTitle || output.CatalogVersion != wantVersion {
		t.Fatalf("output=%#v", output)
	}
}

func postgresCatalogState(t *testing.T, db *gorm.DB) string {
	t.Helper()
	var state string
	query := `SELECT jsonb_build_object(
  'versions', COALESCE((SELECT jsonb_agg(to_jsonb(row_value) ORDER BY tenant_id, product_key, version) FROM product_snapshot_versions AS row_value), '[]'::jsonb),
  'heads', COALESCE((SELECT jsonb_agg(to_jsonb(row_value) ORDER BY tenant_id, product_key) FROM product_snapshot_heads AS row_value), '[]'::jsonb)
)::text`
	if err := db.Raw(query).Scan(&state).Error; err != nil {
		t.Fatalf("capture Catalog state: %v", err)
	}
	return state
}
