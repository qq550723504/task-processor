package tests

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strconv"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	"task-processor/internal/authidentity"
	"task-processor/internal/authz"
	"task-processor/internal/commercetool"
	"task-processor/internal/httproute"
	"task-processor/internal/integration/commercetoolauth"
	catalogpersistence "task-processor/internal/integration/persistence/product/catalog"
	"task-processor/internal/product/catalog"
	canonicalinspect "task-processor/internal/product/catalog/tools/canonicalinspect"
	"task-processor/internal/workbenchcontext"
)

func TestRegistryConformanceCanonicalInspectionVerticalSlice(t *testing.T) {
	db, err := gorm.Open(sqlite.Dialector{DriverName: "sqlite", DSN: "file:" + t.Name() + "?mode=memory&cache=shared"}, &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := catalogpersistence.AutoMigrate(db); err != nil {
		t.Fatalf("migrate catalog: %v", err)
	}
	repository, err := catalogpersistence.NewRepository(db)
	if err != nil {
		t.Fatalf("NewRepository(): %v", err)
	}
	publisher, err := catalog.NewPublisher(repository)
	if err != nil {
		t.Fatalf("NewPublisher(): %v", err)
	}
	identity := catalog.SnapshotIdentity{TenantID: "org-a", ProductKey: "product-1"}
	first := publishConformanceSnapshot(t, publisher, identity, "publication-1", "Bottle v1")
	second := publishConformanceSnapshot(t, publisher, identity, "publication-2", "Bottle v2")

	reader, err := catalogpersistence.NewBoundedSnapshotReader(db, canonicalinspect.MaxCatalogSnapshotBytes)
	if err != nil {
		t.Fatalf("NewBoundedSnapshotReader(): %v", err)
	}
	casbin, err := authz.NewListingKitAuthorizer(nil, nil)
	if err != nil {
		t.Fatalf("NewListingKitAuthorizer(): %v", err)
	}
	authorizer, err := commercetoolauth.NewCasbinAuthorizer(casbin)
	if err != nil {
		t.Fatalf("NewCasbinAuthorizer(): %v", err)
	}

	now := time.Date(2026, 9, 11, 12, 0, 0, 0, time.UTC)
	grants := &conformanceGrantLoader{grants: []authidentity.OrganizationGrant{{OrganizationID: "org-a", ProjectID: "project-1", Roles: []string{"listingkit_operator"}}}}
	organizationResolver := workbenchcontext.NewResolver(grants, "project-1", "v1", nil, workbenchcontext.WithResolverClock(func() time.Time { return now }))
	principalResolver, err := commercetoolauth.NewWorkbenchPrincipalResolver(commercetoolauth.CachedReadOrganizationResolverFunc(func(ctx context.Context, request commercetoolauth.OrganizationRequest) (authidentity.AuthenticatedIdentity, error) {
		return organizationResolver.Resolve(ctx, httproute.OrganizationAccessPolicyCachedRead, workbenchcontext.ResolveInput{
			Identity: request.Identity, BearerToken: request.BearerToken, RequestedOrganizationID: request.RequestedOrganizationID,
		})
	}), func() time.Time { return now })
	if err != nil {
		t.Fatalf("NewWorkbenchPrincipalResolver(): %v", err)
	}

	spanRecorder := tracetest.NewSpanRecorder()
	traceProvider := trace.NewTracerProvider(trace.WithSpanProcessor(spanRecorder))
	t.Cleanup(func() { _ = traceProvider.Shutdown(context.Background()) })
	audits := &conformanceAuditRecorder{}
	agent := commercetool.AgentDefinition{ID: "fake.product-agent", Version: "v1.0.0", AllowedTools: []commercetool.ToolRef{canonicalinspect.Definition().Ref}}
	invoker, err := canonicalinspect.NewInvoker(reader, agent, commercetool.InvocationDependencies{
		PrincipalResolver: principalResolver, Authorizer: authorizer, Recorder: audits,
		Tracer: traceProvider.Tracer("canonicalinspect-conformance"), Now: func() time.Time { return now }, AuditTimeout: time.Second,
	})
	if err != nil {
		t.Fatalf("NewInvoker(): %v", err)
	}

	before := captureConformanceCatalogState(t, db)
	request := conformanceOrganizationRequest(now, "org-a")

	// Exact v1 remains v1 after v2 exists: no current/latest fallback.
	assertConformanceTitle(t, invokeConformance(t, invoker, request, "exact-v1", first.Version), "Bottle v1", first.Version)
	assertConformanceTitle(t, invokeConformance(t, invoker, request, "exact-v2", second.Version), "Bottle v2", second.Version)
	assertConformanceError(t, invoker, request, "missing-version", second.Version+1, commercetool.ErrorNotFound)

	// BusinessTaskID is correlation metadata only and cannot select product identity.
	result := invokeConformanceWithBusinessTask(t, invoker, request, "old-task-id", first.Version)
	assertConformanceTitle(t, result, "Bottle v1", first.Version)
	if string(result.Output) == "" || containsAuthorityField(result.Output) {
		t.Fatalf("output leaked authority fields: %s", result.Output)
	}

	// Effective organization, permission, and current grant resolution all fail closed.
	assertConformanceError(t, invoker, conformanceOrganizationRequest(now, "org-b"), "cross-org", first.Version, commercetool.ErrorIdentityIntegrity)
	grants.grants = []authidentity.OrganizationGrant{{OrganizationID: "org-a", ProjectID: "project-1", Roles: []string{"listingkit_viewer"}}}
	assertConformanceError(t, invoker, request, "permission", first.Version, commercetool.ErrorPermissionDenied)
	grants.err = errors.New("authorization dependency unavailable")
	assertConformanceError(t, invoker, request, "grant-dependency", first.Version, commercetool.ErrorIdentityIntegrity)

	// Reconstructing the callable boundary still reads the immutable fact.
	grants.err = nil
	grants.grants = []authidentity.OrganizationGrant{{OrganizationID: "org-a", ProjectID: "project-1", Roles: []string{"listingkit_operator"}}}
	restarted, err := canonicalinspect.NewInvoker(reader, agent, commercetool.InvocationDependencies{
		PrincipalResolver: principalResolver, Authorizer: authorizer, Recorder: audits,
		Tracer: traceProvider.Tracer("canonicalinspect-conformance-restarted"), Now: func() time.Time { return now }, AuditTimeout: time.Second,
	})
	if err != nil {
		t.Fatalf("restart NewInvoker(): %v", err)
	}
	assertConformanceTitle(t, invokeConformance(t, restarted, request, "restart", first.Version), "Bottle v1", first.Version)

	after := captureConformanceCatalogState(t, db)
	if !reflect.DeepEqual(after, before) {
		t.Fatalf("read-only tool changed durable Catalog state\nbefore=%#v\nafter=%#v", before, after)
	}
	if len(audits.records) != 8 {
		t.Fatalf("audit records = %d, want 8", len(audits.records))
	}
	if grants.calls != 8 {
		t.Fatalf("request-scoped grant resolutions = %d, want 8", grants.calls)
	}
}

func publishConformanceSnapshot(t *testing.T, publisher *catalog.Publisher, identity catalog.SnapshotIdentity, publicationID, title string) catalog.PublishedSnapshot {
	t.Helper()
	published, err := publisher.Publish(context.Background(), catalog.PublishRequest{Identity: identity, PublicationID: publicationID, Snapshot: catalog.ProductSnapshot{Title: title}})
	if err != nil {
		t.Fatalf("publish %s: %v", publicationID, err)
	}
	return published
}

type conformanceGrantLoader struct {
	grants []authidentity.OrganizationGrant
	err    error
	calls  int
}

func (l *conformanceGrantLoader) Load(_ context.Context, source workbenchcontext.GrantSource, _ workbenchcontext.GrantRequest) (workbenchcontext.GrantResult, error) {
	l.calls++
	return workbenchcontext.GrantResult{Grants: append([]authidentity.OrganizationGrant(nil), l.grants...), Source: source}, l.err
}

func (*conformanceGrantLoader) Invalidate(string, string) {}

type conformanceAuditRecorder struct{ records []commercetool.AuditRecord }

func (r *conformanceAuditRecorder) RecordToolCall(_ context.Context, record commercetool.AuditRecord) error {
	r.records = append(r.records, record)
	return nil
}

func conformanceOrganizationRequest(now time.Time, organizationID string) commercetoolauth.OrganizationRequest {
	return commercetoolauth.OrganizationRequest{
		Identity:    authidentity.AuthenticatedIdentity{UserID: "user-1", HomeOrganizationID: "org-a", TokenExpiresAt: now.Add(time.Hour)},
		BearerToken: "verified-bearer", RequestedOrganizationID: organizationID,
	}
}

func invokeConformance(t *testing.T, invoker *canonicalinspect.Invoker, request commercetoolauth.OrganizationRequest, callID string, version uint64) commercetool.Result {
	t.Helper()
	return invokeConformanceWithBusinessTask(t, invoker, request, callID, version)
}

func invokeConformanceWithBusinessTask(t *testing.T, invoker *canonicalinspect.Invoker, request commercetoolauth.OrganizationRequest, businessTaskID string, version uint64) commercetool.Result {
	t.Helper()
	ctx := commercetoolauth.WithOrganizationRequest(context.Background(), request)
	result, err := invoker.Invoke(ctx, commercetool.CallMetadata{
		CallID: "call-" + businessTaskID, AgentID: "fake.product-agent", AgentVersion: "v1.0.0", AgentRunID: "run-1", BusinessTaskID: businessTaskID,
	}, canonicalinspect.Input{ProductKey: "product-1", CatalogVersion: strconv.FormatUint(version, 10)})
	if err != nil {
		t.Fatalf("Invoke(%s) error = %v", businessTaskID, err)
	}
	return result
}

func assertConformanceError(t *testing.T, invoker *canonicalinspect.Invoker, request commercetoolauth.OrganizationRequest, callID string, version uint64, want commercetool.ErrorCode) {
	t.Helper()
	ctx := commercetoolauth.WithOrganizationRequest(context.Background(), request)
	_, err := invoker.Invoke(ctx, commercetool.CallMetadata{
		CallID: "call-" + callID, AgentID: "fake.product-agent", AgentVersion: "v1.0.0", AgentRunID: "run-1", BusinessTaskID: callID,
	}, canonicalinspect.Input{ProductKey: "product-1", CatalogVersion: strconv.FormatUint(version, 10)})
	if commercetool.CodeOf(err) != want {
		t.Fatalf("Invoke(%s) code = %s, want %s; error=%v", callID, commercetool.CodeOf(err), want, err)
	}
}

func assertConformanceTitle(t *testing.T, result commercetool.Result, title string, version uint64) {
	t.Helper()
	var output canonicalinspect.Output
	if err := json.Unmarshal(result.Output, &output); err != nil {
		t.Fatalf("unmarshal output: %v", err)
	}
	if output.Snapshot.Title != title || output.CatalogVersion != strconv.FormatUint(version, 10) || result.AIInvocationID != "" {
		t.Fatalf("output = %#v, result AI invocation = %q", output, result.AIInvocationID)
	}
}

func containsAuthorityField(raw json.RawMessage) bool {
	var output map[string]any
	_ = json.Unmarshal(raw, &output)
	for _, field := range []string{"task_id", "tenant_id", "organization_id", "user_id", "roles", "permission"} {
		if _, exists := output[field]; exists {
			return true
		}
	}
	return false
}

type conformanceCatalogState struct {
	Versions []catalogpersistence.SnapshotVersionRecord
	Heads    []catalogpersistence.SnapshotHeadRecord
}

func captureConformanceCatalogState(t *testing.T, db *gorm.DB) conformanceCatalogState {
	t.Helper()
	state := conformanceCatalogState{}
	if err := db.Order("tenant_id, product_key, version").Find(&state.Versions).Error; err != nil {
		t.Fatalf("load versions: %v", err)
	}
	if err := db.Order("tenant_id, product_key").Find(&state.Heads).Error; err != nil {
		t.Fatalf("load heads: %v", err)
	}
	return state
}
