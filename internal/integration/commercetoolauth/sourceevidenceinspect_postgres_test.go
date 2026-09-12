//go:build integration

package commercetoolauth_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"strconv"
	"strings"
	"sync"
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

	"task-processor/internal/app/productsourcing"
	"task-processor/internal/authidentity"
	"task-processor/internal/authz"
	"task-processor/internal/commercetool"
	"task-processor/internal/httproute"
	"task-processor/internal/integration/commercetoolauth"
	catalogstore "task-processor/internal/integration/persistence/product/catalog"
	"task-processor/internal/product/catalog"
	"task-processor/internal/product/sourcing"
	tool "task-processor/internal/product/sourcing/tools/sourceevidenceinspect"
	"task-processor/internal/workbenchcontext"
)

// The external IAM source alone is a fixture. The actual GrantResolver,
// LiveWrite policy, ContextAuthorizer, Casbin, Registry, SRC-1 and PostgreSQL
// owners are exercised; no repository, publisher or tool execution is mocked.
type grantSource struct {
	mu    sync.Mutex
	roles []string
	fail  bool
	calls int
}

func (s *grantSource) ListOwnProjectAuthorizations(ctx context.Context, _, _, _ string) ([]authidentity.OrganizationGrant, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls++
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if s.fail {
		return nil, errors.New("private-IAM-dependency")
	}
	roles := append([]string(nil), s.roles...)
	return []authidentity.OrganizationGrant{{OrganizationID: "org-a", ProjectID: "project", Roles: roles}, {OrganizationID: "org-b", ProjectID: "project", Roles: roles}}, nil
}
func (s *grantSource) set(roles []string, fail bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.roles = roles
	s.fail = fail
}
func (s *grantSource) count() int { s.mu.Lock(); defer s.mu.Unlock(); return s.calls }

type liveAccess struct{ resolver *workbenchcontext.Resolver }

func (l liveAccess) ResolveLiveRoles(ctx context.Context, org, actor string) ([]string, error) {
	id, ok := authidentity.AuthenticatedIdentityFromContext(ctx)
	if !ok || id.UserID != actor || id.EffectiveOrganizationID != org || id.TenantID != org {
		return nil, sourcing.ErrPublicationForbidden
	}
	resolved, err := l.resolver.Resolve(ctx, httproute.OrganizationAccessPolicyLiveWrite, workbenchcontext.ResolveInput{Identity: id, BearerToken: "isolated-verified-IAM-fixture", RequestedOrganizationID: org})
	if err != nil {
		if errors.Is(err, workbenchcontext.ErrOrganizationAccessDenied) || errors.Is(err, workbenchcontext.ErrOrganizationAccessRevoked) {
			return nil, sourcing.ErrPublicationForbidden
		}
		return nil, sourcing.ErrSourcePublicationUnavailable
	}
	return resolved.Roles, nil
}

type auditSink struct {
	mu      sync.Mutex
	records []commercetool.AuditRecord
}

func (a *auditSink) RecordToolCall(_ context.Context, r commercetool.AuditRecord) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.records = append(a.records, r)
	return nil
}

func TestSourceEvidencePostgresRegistryExactReadChain(t *testing.T) {
	db := openSourceEvidencePostgres(t)
	if err := productsourcing.InstallSchema(db); err != nil {
		t.Fatal(err)
	}
	policy, err := authz.NewListingKitAuthorizer(nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	grants := &grantSource{roles: []string{"listingkit_operator"}}
	resolver := workbenchcontext.NewResolver(workbenchcontext.NewGrantResolver(grants, workbenchcontext.NewGrantCache(time.Now)), "project", "v1", nil)
	live := liveAccess{resolver: resolver}
	producer, err := productsourcing.NewInternalProducer(db, live, policy)
	if err != nil {
		t.Fatal(err)
	}
	identity := authidentity.AuthenticatedIdentity{UserID: "actor", HomeOrganizationID: "org-a", TokenExpiresAt: time.Now().Add(time.Hour)}
	resolved, err := resolver.Resolve(context.Background(), httproute.OrganizationAccessPolicyLiveWrite, workbenchcontext.ResolveInput{Identity: identity, BearerToken: "isolated-verified-IAM-fixture", RequestedOrganizationID: "org-a"})
	if err != nil {
		t.Fatal(err)
	}
	ctx := authidentity.WithAuthenticatedIdentity(context.Background(), resolved)
	other := resolved
	other.TenantID = "org-b"
	other.EffectiveOrganizationID = "org-b"
	ctxB := authidentity.WithAuthenticatedIdentity(context.Background(), other)
	first := publishEvidence(t, ctx, producer, "publication-1", "first")
	second := publishEvidence(t, ctx, producer, "publication-2", "second")
	foreign := publishEvidence(t, ctxB, producer, "publication-1", "foreign")
	corrupt := publishEvidence(t, ctx, producer, "publication-corrupt", "corrupt")
	noReceipt := publishEvidence(t, ctx, producer, "publication-no-receipt", "no receipt")
	if first.CatalogVersion != 1 || second.CatalogVersion != 2 || foreign.CatalogVersion != 1 {
		t.Fatal("unexpected publication sequence")
	}
	// A current Catalog publisher creates a legitimate version with no SRC-1
	// publication. Exact source reads must fail rather than search older versions.
	repository, err := catalogstore.NewRepository(db)
	if err != nil {
		t.Fatal(err)
	}
	publisher, err := catalog.NewPublisher(repository)
	if err != nil {
		t.Fatal(err)
	}
	catalogOnly, err := publisher.Publish(ctx, catalog.PublishRequest{Identity: catalog.SnapshotIdentity{TenantID: "org-a", ProductKey: "product"}, PublicationID: "catalog-only", Snapshot: catalog.ProductSnapshot{Title: "no source"}})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("UPDATE product_source_publications SET envelope_hash = ? WHERE organization_id = ? AND publication_id = ?", strings.Repeat("f", 64), "org-a", corrupt.PublicationID).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("DELETE FROM product_source_publication_receipts WHERE organization_id = ? AND publication_id = ?", "org-a", noReceipt.PublicationID).Error; err != nil {
		t.Fatal(err)
	}
	invoker, audits := assembleSourceInvoker(t, db, producer, policy)
	before := businessState(t, db)
	callsBefore := grants.count()
	one := invokeEvidence(t, ctx, invoker, first.CatalogVersion, "first", "")
	two := invokeEvidence(t, ctx, invoker, second.CatalogVersion, "second", "")
	if one.Lineage.EnvelopeHash != first.EnvelopeHash || one.CatalogVersion != "1" || two.Lineage.EnvelopeHash != second.EnvelopeHash || two.CatalogVersion != "2" {
		t.Fatal("exact hashes/version lost")
	}
	if one.SourceIdentity.SourceID != "123456" || one.Disclosure.WarningCodesOmitted != 2 || one.Disclosure.WarningFieldsOmitted != 1 || one.Disclosure.MissingFactsOmitted != 1 || !one.Disclosure.DiagnosticsPartial || one.Disclosure.RawTextReturned || one.Disclosure.SourceURLProvided {
		t.Fatalf("wrong safe projection %#v", one)
	}
	same := invokeEvidence(t, ctx, invoker, first.CatalogVersion, "replay", "")
	a, _ := json.Marshal(one)
	b, _ := json.Marshal(same)
	if string(a) != string(b) {
		t.Fatal("exact result changed after head advance")
	}
	otherOutput := invokeEvidence(t, ctxB, invoker, foreign.CatalogVersion, "other-org", "")
	if otherOutput.Lineage.InputHash == one.Lineage.InputHash {
		t.Fatal("cross-org product confused")
	}
	invokeEvidence(t, ctxB, invoker, second.CatalogVersion, "wrong-org", commercetool.ErrorNotFound)
	invokeEvidence(t, ctx, invoker, 99, "missing-version", commercetool.ErrorNotFound)
	invokeEvidence(t, ctx, invoker, catalogOnly.Version, "missing-publication", commercetool.ErrorNotFound)
	invokeEvidence(t, ctx, invoker, corrupt.CatalogVersion, "corrupt-evidence", commercetool.ErrorInternal)
	invokeEvidence(t, ctx, invoker, noReceipt.CatalogVersion, "missing-receipt", commercetool.ErrorInternal)
	// Keep the verified request's old roles unchanged. The source owner must
	// freshly deny this request after revocation without cache invalidation.
	grants.set([]string{"listingkit_viewer"}, false)
	invokeEvidence(t, ctx, invoker, first.CatalogVersion, "source-revoked", commercetool.ErrorPermissionDenied)
	grants.set([]string{"listingkit_operator"}, true)
	invokeEvidence(t, ctx, invoker, first.CatalogVersion, "IAM-unavailable", commercetool.ErrorDependencyUnavailable)
	grants.set([]string{"listingkit_operator"}, false)
	if grants.count()-callsBefore < 9 {
		t.Fatalf("fresh authorizations missing: %d", grants.count()-callsBefore)
	}
	expired := resolved
	expired.TokenExpiresAt = time.Now().Add(-time.Minute)
	invokeEvidence(t, authidentity.WithAuthenticatedIdentity(context.Background(), expired), invoker, first.CatalogVersion, "expired", commercetool.ErrorIdentityIntegrity)
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	invokeEvidence(t, canceled, invoker, first.CatalogVersion, "canceled", commercetool.ErrorPermissionDenied)
	rebuiltProducer, err := productsourcing.NewInternalProducer(db, live, policy)
	if err != nil {
		t.Fatal(err)
	}
	rebuilt, _ := assembleSourceInvoker(t, db, rebuiltProducer, policy)
	rebuiltOutput := invokeEvidence(t, ctx, rebuilt, first.CatalogVersion, "reconstruction", "")
	b, _ = json.Marshal(rebuiltOutput)
	if string(a) != string(b) {
		t.Fatal("reconstructed exact read changed")
	}
	// Parallel read-only invocations use one immutable executor with owner reads.
	var wg sync.WaitGroup
	for n := 0; n < 8; n++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			invokeEvidence(t, ctx, rebuilt, first.CatalogVersion, fmt.Sprintf("concurrent-%d", n), "")
		}(n)
	}
	wg.Wait()
	if after := businessState(t, db); after != before {
		t.Fatal("read-only tool changed source/Catalog durable state")
	}
	for _, record := range audits.records {
		if record.ToolID != tool.Definition().Ref.ID || record.ToolVersion != "v1.0.0" || record.BusinessTaskID != "synthetic-correlation-only" || record.AIInvocationID != "" {
			t.Fatalf("wrong audit metadata %#v", record)
		}
	}
	t.Log("PASS: real SRC-1 publish -> Catalog -> Registry/permission -> SRC-1 fresh live authorization -> exact PostgreSQL read; head advance/reconstruction/revocation/corruption/no fallback/concurrent reads; four durable owner tables unchanged. IAM source=fixture, real IAM/provider=NOT_RUN.")
}

func publishEvidence(t *testing.T, ctx context.Context, p *sourcing.InternalProducer, publication, title string) sourcing.PublicationReceipt {
	t.Helper()
	command := sourcing.PublicationCommand{PublicationID: publication, ProductKey: "product", Producer: sourcing.ProducerDescriptor{Kind: sourcing.ControlledSnapshotProducerKind, Version: sourcing.ControlledSnapshotProducerVersion}, Envelope: sourcing.SourceEnvelope{
		Identity:         sourcing.SourceIdentity{SourceType: sourcing.SourceTypeCrawler, SourcePlatform: "1688", SourceID: "123456", SourceURL: "https://source.invalid/path?cookie=PRIVATE-RAW", SourceVersion: "PRIVATE-VERSION"},
		ProductCandidate: sourcing.ProductCandidate{Title: title, Description: "<html>PRIVATE-RAW</html>"},
		RawReference:     sourcing.RawSourceReference{ReferenceID: "PRIVATE-RAW", URL: "https://source.invalid?token=PRIVATE-RAW", Checksum: sourcing.RawSnapshotChecksum(title), CapturedAt: time.Date(2026, 9, 12, 1, 0, 0, 0, time.UTC), Metadata: map[string]string{"innocent": "PRIVATE-RAW", "cookie": "PRIVATE-RAW"}},
		Warnings:         []sourcing.SourceWarning{{Code: "producer-warning", Field: "title", Message: "PRIVATE-RAW"}, {Code: "PRIVATE-CODE", Field: "PRIVATE-FIELD", Message: "PRIVATE-RAW"}},
		MissingFacts:     []sourcing.MissingFact{{Field: "images", Reason: "PRIVATE-RAW"}, {Field: "PRIVATE-FIELD", Reason: "PRIVATE-RAW"}},
		Trace:            sourcing.SourceTrace{Notes: []string{"PRIVATE-RAW"}, SourceRunID: "PRIVATE-RAW", RequestID: "PRIVATE-RAW"},
	}}
	receipt, err := p.Publish(ctx, command)
	if err != nil {
		t.Fatal(err)
	}
	return receipt
}
func assembleSourceInvoker(t *testing.T, db *gorm.DB, source tool.SourceReader, policy *authz.ListingKitAuthorizer) (*tool.Invoker, *auditSink) {
	t.Helper()
	reader, err := catalogstore.NewBoundedSnapshotReader(db, tool.MaxCatalogSnapshotBytes)
	if err != nil {
		t.Fatal(err)
	}
	auth, err := commercetoolauth.NewCasbinAuthorizer(policy)
	if err != nil {
		t.Fatal(err)
	}
	audit := &auditSink{}
	invoker, err := tool.NewInvoker(reader, source, commercetool.AgentDefinition{ID: "tool.s1.fixture", Version: "v1.0.0", AllowedTools: []commercetool.ToolRef{tool.Definition().Ref}},
		commercetool.InvocationDependencies{PrincipalResolver: commercetoolauth.ContextPrincipalResolver{}, Authorizer: auth, Recorder: audit, Tracer: otel.Tracer("tool-s1-postgres"), Now: time.Now, AuditTimeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	return invoker, audit
}
func invokeEvidence(t *testing.T, ctx context.Context, i *tool.Invoker, version uint64, call string, want commercetool.ErrorCode) tool.Output {
	t.Helper()
	result, err := i.Invoke(ctx, commercetool.CallMetadata{CallID: call, AgentID: "tool.s1.fixture", AgentVersion: "v1.0.0", AgentRunID: "synthetic-run", BusinessTaskID: "synthetic-correlation-only"}, tool.Input{ProductKey: "product", CatalogVersion: strconv.FormatUint(version, 10)})
	if want != "" {
		if err == nil || commercetool.CodeOf(err) != want || len(result.Output) != 0 {
			t.Errorf("%s: expected %s got %v output=%s", call, want, err, result.Output)
		}
		return tool.Output{}
	}
	if err != nil {
		t.Errorf("%s: %v", call, err)
		return tool.Output{}
	}
	if strings.Contains(string(result.Output), "PRIVATE") || strings.Contains(string(result.Output), "source.invalid") {
		t.Errorf("%s: raw source leaked", call)
	}
	var out tool.Output
	if err := json.Unmarshal(result.Output, &out); err != nil {
		t.Error(err)
	}
	return out
}
func businessState(t *testing.T, db *gorm.DB) string {
	t.Helper()
	var state string
	query := `SELECT jsonb_build_object(
 'versions',(SELECT jsonb_agg(to_jsonb(r) ORDER BY tenant_id,product_key,version) FROM product_snapshot_versions r),
 'heads',(SELECT jsonb_agg(to_jsonb(r) ORDER BY tenant_id,product_key) FROM product_snapshot_heads r),
 'sources',(SELECT jsonb_agg(to_jsonb(r) ORDER BY organization_id,publication_id) FROM product_source_publications r),
 'receipts',(SELECT jsonb_agg(to_jsonb(r) ORDER BY organization_id,publication_id) FROM product_source_publication_receipts r))::text`
	if err := db.Raw(query).Scan(&state).Error; err != nil {
		t.Fatal(err)
	}
	return state
}
func openSourceEvidencePostgres(t *testing.T) *gorm.DB {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	port := network.MustParsePort("5432/tcp")
	pg, err := tcpostgres.Run(ctx, "postgres:16-alpine", tcpostgres.WithDatabase("tool_s1"), tcpostgres.WithUsername("tool_s1"), tcpostgres.WithPassword("isolated_tool_s1"), tcpostgres.BasicWaitStrategies(),
		testcontainers.WithHostConfigModifier(func(c *container.HostConfig) {
			if c.PortBindings == nil {
				c.PortBindings = network.PortMap{}
			}
			c.PortBindings[port] = []network.PortBinding{{HostIP: netip.MustParseAddr("127.0.0.1"), HostPort: "0"}}
		}))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := pg.Terminate(ctx); err != nil {
			t.Error(err)
		}
	})
	inspect, err := pg.Inspect(ctx)
	if err != nil {
		t.Fatal(err)
	}
	bindings := inspect.NetworkSettings.Ports[port]
	if len(bindings) != 1 || !bindings[0].HostIP.IsLoopback() || bindings[0].HostPort == "" || bindings[0].HostPort == "0" {
		t.Fatalf("nonisolated binding: %#v", bindings)
	}
	t.Logf("task-owned container=%s loopback=%s:%s", pg.GetContainerID(), bindings[0].HostIP, bindings[0].HostPort)
	dsn, err := pg.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := sqlDB.Close(); err != nil {
			t.Error(err)
		}
	})
	return db
}
