//go:build integration

package commercetoolauth_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/netip"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/network"
	"github.com/testcontainers/testcontainers-go"
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
	foreign := publishPostgresSnapshot(t, publisher, catalog.SnapshotIdentity{TenantID: "org-b", ProductKey: "product-1"}, "publication-org-b", catalog.ProductSnapshot{Title: "Organization B bottle"})

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
	outputOversize := publishPostgresSnapshot(t, publisher, identity, "publication-output-oversize", catalog.ProductSnapshot{Description: strings.Repeat("x", canonicalinspect.MaxOutputBytes)})
	materializationOversize := publishPostgresSnapshot(t, publisher, identity, "publication-materialization-oversize", catalog.ProductSnapshot{Description: strings.Repeat("x", canonicalinspect.MaxCatalogSnapshotBytes)})
	corrupt := publishPostgresSnapshot(t, publisher, identity, "publication-corrupt", catalog.ProductSnapshot{Title: "Before corruption"})
	if err := db.Model(&catalogstore.SnapshotVersionRecord{}).
		Where("tenant_id = ? AND product_key = ? AND version = ?", identity.TenantID, identity.ProductKey, corrupt.Version).
		Update("snapshot_json", []byte(`{"title":"tampered"}`)).Error; err != nil {
		t.Fatalf("inject corrupt payload: %v", err)
	}

	invoker, request, audits, authorization, grantResolver := newPostgresCanonicalInvoker(t, db)
	stateBeforeRead := postgresCatalogState(t, db)
	assertPostgresTitle(t, invokePostgresCanonical(t, invoker, request, first.Version, "exact-v1"), "Bottle v1", "1")
	assertPostgresTitle(t, invokePostgresCanonical(t, invoker, request, second.Version, "exact-v2"), "Bottle v2", "2")
	assertPostgresTitle(t, invokePostgresCanonical(t, invoker, request, high.Version, "exact-high"), "Bottle high", "9007199254740993")
	assertPostgresCode(t, invoker, request, 3, "gap-no-latest-fallback", commercetool.ErrorNotFound)

	assertPostgresCode(t, invoker, request, outputOversize.Version, "output-oversize", commercetool.ErrorFailedPrecondition)
	assertPostgresCode(t, invoker, request, materializationOversize.Version, "snapshot-oversize", commercetool.ErrorFailedPrecondition)
	assertPostgresCode(t, invoker, request, corrupt.Version, "corrupt", commercetool.ErrorInternal)

	// The selected Organization is authority: the same product key cannot cross
	// streams, and a grant is required before either stream is read.
	requestB := request
	requestB.RequestedOrganizationID = "org-b"
	assertPostgresCode(t, invoker, requestB, foreign.Version, "org-b-without-grant", commercetool.ErrorIdentityIntegrity)
	authorization.grants = []authidentity.OrganizationGrant{
		{OrganizationID: "org-a", ProjectID: "project-1", Roles: []string{"listingkit_operator"}},
		{OrganizationID: "org-b", ProjectID: "project-1", Roles: []string{"listingkit_operator"}},
	}
	grantResolver.Invalidate("user-1", "project-1")
	assertPostgresTitle(t, invokePostgresCanonical(t, invoker, requestB, foreign.Version, "org-b-exact"), "Organization B bottle", "1")

	authorization.grants = []authidentity.OrganizationGrant{{OrganizationID: "org-a", ProjectID: "project-1", Roles: []string{"listingkit_viewer"}}}
	grantResolver.Invalidate("user-1", "project-1")
	assertPostgresCode(t, invoker, request, first.Version, "permission-revoked", commercetool.ErrorPermissionDenied)
	authorization.err = errors.New("authorization backend unavailable")
	grantResolver.Invalidate("user-1", "project-1")
	assertPostgresCode(t, invoker, request, first.Version, "authorization-dependency", commercetool.ErrorIdentityIntegrity)

	authorization.err = nil
	authorization.grants = []authidentity.OrganizationGrant{{OrganizationID: "org-a", ProjectID: "project-1", Roles: []string{"listingkit_operator"}}}
	grantResolver.Invalidate("user-1", "project-1")
	assertPostgresTitle(t, invokePostgresCanonical(t, invoker, request, first.Version, "authorization-restored"), "Bottle v1", "1")
	audits.fail = true
	recorderFailure := invokePostgresCanonical(t, invoker, request, first.Version, "audit-recorder-failure")
	if recorderFailure.AuditStatus != commercetool.AuditStatusRecordFailed {
		t.Fatalf("audit recorder failure status = %q", recorderFailure.AuditStatus)
	}
	audits.fail = false
	expired := request
	expired.Identity.TokenExpiresAt = time.Date(2026, 9, 11, 12, 0, 0, 0, time.UTC)
	assertPostgresCode(t, invoker, expired, first.Version, "expired", commercetool.ErrorIdentityIntegrity)
	grantResolver.Invalidate("user-1", "project-1")
	canceledContext, cancel := context.WithCancel(context.Background())
	cancel()
	assertPostgresCodeWithContext(t, canceledContext, invoker, request, first.Version, "canceled", commercetool.ErrorIdentityIntegrity)

	// Rebuild every callable adapter against the same PostgreSQL facts.
	restarted, restartedRequest, restartedAudits, _, _ := newPostgresCanonicalInvoker(t, db)
	assertPostgresTitle(t, invokePostgresCanonical(t, restarted, restartedRequest, first.Version, "restart"), "Bottle v1", "1")
	if stateAfterRead := postgresCatalogState(t, db); stateAfterRead != stateBeforeRead {
		t.Fatalf("read invocation changed durable Catalog state\nbefore=%s\nafter=%s", stateBeforeRead, stateAfterRead)
	}
	assertPostgresAudits(t, audits.records, []commercetool.ErrorCode{"", "", "", commercetool.ErrorNotFound, commercetool.ErrorFailedPrecondition, commercetool.ErrorFailedPrecondition, commercetool.ErrorInternal, commercetool.ErrorIdentityIntegrity, "", commercetool.ErrorPermissionDenied, commercetool.ErrorIdentityIntegrity, "", "", commercetool.ErrorIdentityIntegrity, commercetool.ErrorIdentityIntegrity})
	assertPostgresAudits(t, restartedAudits.records, []commercetool.ErrorCode{""})
}

func openCanonicalInspectPostgres(t *testing.T) *gorm.DB {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	postgresPort := network.MustParsePort("5432/tcp")
	postgresContainer, err := tcpostgres.Run(ctx, "postgres:16-alpine",
		tcpostgres.WithDatabase("tool_c1"), tcpostgres.WithUsername("tool_c1"), tcpostgres.WithPassword("tool_c1"), tcpostgres.BasicWaitStrategies(),
		testcontainers.WithHostConfigModifier(func(hostConfig *container.HostConfig) {
			if hostConfig.PortBindings == nil {
				hostConfig.PortBindings = network.PortMap{}
			}
			hostConfig.PortBindings[postgresPort] = []network.PortBinding{{HostIP: netip.MustParseAddr("127.0.0.1"), HostPort: "0"}}
		}),
	)
	if err != nil {
		t.Fatalf("start task-owned PostgreSQL: %v", err)
	}
	t.Cleanup(func() {
		if err := postgresContainer.Terminate(context.Background()); err != nil {
			t.Errorf("terminate task-owned PostgreSQL: %v", err)
		}
	})
	inspection, err := postgresContainer.Inspect(ctx)
	if err != nil {
		t.Fatalf("inspect task-owned PostgreSQL: %v", err)
	}
	configuredBindings := inspection.HostConfig.PortBindings[postgresPort]
	if len(configuredBindings) != 1 || !configuredBindings[0].HostIP.IsLoopback() || configuredBindings[0].HostPort != "0" {
		t.Fatalf("PostgreSQL configured binding is not one dynamic loopback port: %#v", configuredBindings)
	}
	actualBindings := inspection.NetworkSettings.Ports[postgresPort]
	if len(actualBindings) != 1 || !actualBindings[0].HostIP.IsLoopback() || actualBindings[0].HostPort == "" || actualBindings[0].HostPort == "0" {
		t.Fatalf("PostgreSQL actual binding is not one allocated loopback port: %#v", actualBindings)
	}
	t.Logf("task-owned PostgreSQL binding verified: %s:%s", actualBindings[0].HostIP, actualBindings[0].HostPort)
	dsn, err := postgresContainer.ConnectionString(ctx, "sslmode=disable")
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
	t.Cleanup(func() {
		if err := sqlDB.Close(); err != nil {
			t.Errorf("close task-owned PostgreSQL pool: %v", err)
		}
	})
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

type postgresAuthorizationClient struct {
	grants []authidentity.OrganizationGrant
	err    error
}

func (c *postgresAuthorizationClient) ListOwnProjectAuthorizations(ctx context.Context, _, _, _ string) ([]authidentity.OrganizationGrant, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return append([]authidentity.OrganizationGrant(nil), c.grants...), c.err
}

type postgresAuditRecorder struct {
	records []commercetool.AuditRecord
	fail    bool
}

func (r *postgresAuditRecorder) RecordToolCall(_ context.Context, record commercetool.AuditRecord) error {
	r.records = append(r.records, record)
	if r.fail {
		return errors.New("audit store unavailable")
	}
	return nil
}

func newPostgresCanonicalInvoker(t *testing.T, db *gorm.DB) (*canonicalinspect.Invoker, commercetoolauth.OrganizationRequest, *postgresAuditRecorder, *postgresAuthorizationClient, *workbenchcontext.GrantResolver) {
	t.Helper()
	reader, err := catalogstore.NewBoundedSnapshotReader(db, canonicalinspect.MaxCatalogSnapshotBytes)
	if err != nil {
		t.Fatalf("NewBoundedSnapshotReader(): %v", err)
	}
	now := time.Date(2026, 9, 11, 12, 0, 0, 0, time.UTC)
	authorization := &postgresAuthorizationClient{grants: []authidentity.OrganizationGrant{{OrganizationID: "org-a", ProjectID: "project-1", Roles: []string{"listingkit_operator"}}}}
	grantResolver := workbenchcontext.NewGrantResolver(authorization, workbenchcontext.NewGrantCache(func() time.Time { return now }))
	organizationResolver := workbenchcontext.NewResolver(grantResolver, "project-1", "v1", nil, workbenchcontext.WithResolverClock(func() time.Time { return now }))
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
	return invoker, request, audits, authorization, grantResolver
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
	assertPostgresCodeWithContext(t, context.Background(), invoker, request, version, callID, want)
}

func assertPostgresCodeWithContext(t *testing.T, ctx context.Context, invoker *canonicalinspect.Invoker, request commercetoolauth.OrganizationRequest, version uint64, callID string, want commercetool.ErrorCode) {
	t.Helper()
	ctx = commercetoolauth.WithOrganizationRequest(ctx, request)
	_, err := invoker.Invoke(ctx, commercetool.CallMetadata{CallID: callID, AgentID: "postgres.product-agent", AgentVersion: "v1.0.0", AgentRunID: "run-1", BusinessTaskID: "correlation-only"}, canonicalinspect.Input{ProductKey: "product-1", CatalogVersion: strconv.FormatUint(version, 10)})
	if commercetool.CodeOf(err) != want {
		t.Fatalf("Invoke(%s) code=%s want=%s error=%v", callID, commercetool.CodeOf(err), want, err)
	}
	if err != nil && (strings.Contains(err.Error(), "tampered") || strings.Contains(err.Error(), "snapshot_json")) {
		t.Fatalf("Invoke(%s) leaked persistence detail: %v", callID, err)
	}
}

func assertPostgresAudits(t *testing.T, records []commercetool.AuditRecord, wantCodes []commercetool.ErrorCode) {
	t.Helper()
	if len(records) != len(wantCodes) {
		t.Fatalf("audit records = %d, want %d", len(records), len(wantCodes))
	}
	for index, record := range records {
		if record.ToolID != "product.canonical.inspect" || record.ToolVersion != "v2.0.0" || record.Owner != "product.catalog" || record.BusinessTaskID != "correlation-only" || record.AIInvocationID != "" {
			t.Fatalf("audit[%d] identity fields = %#v", index, record)
		}
		wantCode := wantCodes[index]
		if wantCode == "" {
			if record.Outcome != commercetool.AuditOutcomeSucceeded || record.ErrorCode != "" || record.TenantID == "" || record.UserID != "user-1" {
				t.Fatalf("audit[%d] success = %#v", index, record)
			}
			continue
		}
		if record.Outcome != commercetool.AuditOutcomeFailed || record.ErrorCode != wantCode {
			t.Fatalf("audit[%d] failure = %#v, want code %s", index, record, wantCode)
		}
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
