//go:build integration

package assetinspect_test

import (
	"context"
	"crypto/sha256"
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
	assetstore "task-processor/internal/integration/persistence/product/asset"
	catalogstore "task-processor/internal/integration/persistence/product/catalog"
	"task-processor/internal/product/asset"
	"task-processor/internal/product/asset/tools/assetinspect"
	"task-processor/internal/product/catalog"
	"task-processor/internal/workbenchcontext"
)

func TestAssetInspectPostgresExactFreshReadChain(t *testing.T) {
	db := openAssetPostgres(t)
	if err := catalogstore.AutoMigrate(db); err != nil {
		t.Fatal(err)
	}
	if err := assetstore.AutoMigrate(db); err != nil {
		t.Fatal(err)
	}
	products, err := catalogstore.NewRepository(db)
	if err != nil {
		t.Fatal(err)
	}
	publisher, err := catalog.NewPublisher(products)
	if err != nil {
		t.Fatal(err)
	}
	assets, err := assetstore.NewRepository(db)
	if err != nil {
		t.Fatal(err)
	}
	identity := catalog.SnapshotIdentity{TenantID: "org-a", ProductKey: "product-1"}
	publish := func(key, id, title, description string) catalog.PublishedSnapshot {
		t.Helper()
		p, err := publisher.Publish(context.Background(), catalog.PublishRequest{Identity: catalog.SnapshotIdentity{TenantID: identity.TenantID, ProductKey: key}, PublicationID: id, Snapshot: catalog.ProductSnapshot{Title: title, Description: description}})
		if err != nil {
			t.Fatal(err)
		}
		return p
	}
	approve := func(key, platform, id string, version uint64, url string) {
		t.Helper()
		_, err := assets.CommitApproval(context.Background(), asset.ApprovalCommit{TenantID: identity.TenantID, ProductKey: key, TargetPlatform: platform, ActionID: "approve-" + id, SourceSnapshotVersion: version, Assets: []asset.ApprovedAsset{{ID: id, RunID: "run-" + id, PlanRevision: 1, SlotID: "main", Attempt: 1, Role: asset.RoleMain, URL: url}}})
		if err != nil {
			t.Fatal(err)
		}
	}
	v1 := publish("product-1", "publication-v1", "Bottle v1", "")
	v2 := publish("product-1", "publication-v2", "Bottle v2", "")
	for _, v := range []catalog.PublishedSnapshot{v1, v2} {
		approve("product-1", "shein", "shein-v"+strconv.FormatUint(v.Version, 10), v.Version, "https://assets.example/image.png")
	}
	approve("product-1", "amazon", "amazon-v1", v1.Version, "https://assets.example/amazon.png")
	missing := publish("product-1", "publication-no-assets", "No approved assets", "")
	neutral := publish("neutral-only", "neutral-v1", "Neutral assets", "")
	approve("neutral-only", "", "neutral", neutral.Version, "https://assets.example/neutral.png")
	unversioned := publish("unversioned-only", "unversioned-v1", "Unversioned assets", "")
	approve("unversioned-only", "shein", "unversioned", 0, "https://assets.example/unversioned.png")
	corrupt := publish("corrupt", "corrupt-v1", "Corrupt assets", "")
	approve("corrupt", "shein", "corrupt", corrupt.Version, "https://assets.example/corrupt.png")
	if err := db.Model(&assetstore.ApprovedAssetRecord{}).Where("asset_id = ?", "corrupt").Update("payload_json", []byte(`{"id":"other-id"}`)).Error; err != nil {
		t.Fatal(err)
	}
	corruptProduct := publish("corrupt-product", "corrupt-product-v1", "Before corruption", "")
	if err := db.Model(&catalogstore.SnapshotVersionRecord{}).Where("product_key = ?", "corrupt-product").Update("snapshot_json", []byte(`{"title":"tampered"}`)).Error; err != nil {
		t.Fatal(err)
	}
	largeProduct := publish("large-product", "large-product-v1", "Large", strings.Repeat("x", assetinspect.MaxCatalogSnapshotBytes))
	largeAssets := publish("large-assets", "large-assets-v1", "Large inventory", "")
	approve("large-assets", "shein", "large-assets", largeAssets.Version, "https://assets.example/"+strings.Repeat("x", assetinspect.MaxInventoryBytes))
	combined := publish("combined", "combined-v1", "Combined oversize", strings.Repeat("x", assetinspect.MaxOutputBytes/2))
	approve("combined", "shein", "combined", combined.Version, "https://assets.example/"+strings.Repeat("x", assetinspect.MaxOutputBytes/2))

	invoker, request, client, now := newAssetInvoker(t, db)
	before := assetDatabaseState(t, db)
	invoke := func(key, platform string, version uint64) (commercetool.Result, error) {
		return invoker.Invoke(commercetoolauth.WithOrganizationRequest(context.Background(), request), assetMetadata(), assetinspect.Input{ProductKey: key, CatalogVersion: strconv.FormatUint(version, 10), TargetPlatform: platform})
	}
	for _, tc := range []struct {
		key, platform     string
		version           uint64
		wantID, wantTitle string
		code              commercetool.ErrorCode
	}{
		{"product-1", "shein", v1.Version, "shein-v1", "Bottle v1", ""},
		{"product-1", "shein", v2.Version, "shein-v2", "Bottle v2", ""},
		{"product-1", "amazon", v1.Version, "amazon-v1", "Bottle v1", ""},
		{"product-1", "shein", 999, "", "", commercetool.ErrorNotFound},
		{"product-1", "shein", missing.Version, "", "", commercetool.ErrorNotFound},
		{"product-1", "other", v1.Version, "", "", commercetool.ErrorNotFound},
		{"neutral-only", "shein", neutral.Version, "", "", commercetool.ErrorNotFound},
		{"unversioned-only", "shein", unversioned.Version, "", "", commercetool.ErrorNotFound},
		{"corrupt", "shein", corrupt.Version, "", "", commercetool.ErrorInternal},
		{"corrupt-product", "shein", corruptProduct.Version, "", "", commercetool.ErrorInternal},
		{"large-product", "shein", largeProduct.Version, "", "", commercetool.ErrorFailedPrecondition},
		{"large-assets", "shein", largeAssets.Version, "", "", commercetool.ErrorFailedPrecondition},
		{"combined", "shein", combined.Version, "", "", commercetool.ErrorFailedPrecondition},
	} {
		t.Run(tc.key+"/"+tc.platform+"/"+strconv.FormatUint(tc.version, 10), func(t *testing.T) {
			result, err := invoke(tc.key, tc.platform, tc.version)
			if tc.code != "" {
				if err == nil || commercetool.CodeOf(err) != tc.code || len(result.Output) != 0 {
					t.Fatalf("want %s got %v output=%s", tc.code, err, result.Output)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			var out assetinspect.Output
			if err := json.Unmarshal(result.Output, &out); err != nil {
				t.Fatal(err)
			}
			if out.Snapshot.Title != tc.wantTitle || len(out.ApprovedAssets) != 1 || out.ApprovedAssets[0].ID != tc.wantID || out.AssetBinding.TargetPlatform != tc.platform || out.AssetBinding.SourceSnapshotVersion != strconv.FormatUint(tc.version, 10) {
				t.Fatalf("exact facts mismatch: %s", result.Output)
			}
		})
	}
	// Same Invoker, warm current-grant cache, no Invalidate: every next read
	// must query the actual grant owner again and enforce the latest read role.
	checks := []struct {
		name   string
		change func()
		code   commercetool.ErrorCode
	}{
		{"revoked read", func() { client.grants[0].Roles = []string{"listingkit_viewer"} }, commercetool.ErrorPermissionDenied},
		{"provider failure", func() { client.err = errors.New("provider unavailable") }, commercetool.ErrorIdentityIntegrity},
		{"wrong org", func() { request.RequestedOrganizationID = "org-b" }, commercetool.ErrorIdentityIntegrity},
		{"expired", func() { *now = request.Identity.TokenExpiresAt }, commercetool.ErrorIdentityIntegrity},
	}
	for _, check := range checks {
		t.Run(check.name, func(t *testing.T) {
			client.err = nil
			client.grants[0].Roles = []string{"listingkit_operator"}
			request.RequestedOrganizationID = "org-a"
			*now = request.Identity.TokenExpiresAt.Add(-time.Minute)
			if _, err := invoke("product-1", "shein", 1); err != nil {
				t.Fatal(err)
			}
			check.change()
			result, err := invoke("product-1", "shein", 1)
			if err == nil || commercetool.CodeOf(err) != check.code || len(result.Output) != 0 {
				t.Fatalf("fresh authorization failed: %v", err)
			}
		})
	}
	// Rebuild every adapter against the same facts to prove persistent identity.
	restarted, restartRequest, _, _ := newAssetInvoker(t, db)
	result, err := restarted.Invoke(commercetoolauth.WithOrganizationRequest(context.Background(), restartRequest), assetMetadata(), assetinspect.Input{ProductKey: "product-1", CatalogVersion: "1", TargetPlatform: "shein"})
	if err != nil || !strings.Contains(string(result.Output), `"id":"shein-v1"`) {
		t.Fatalf("restart: %v", err)
	}
	if after := assetDatabaseState(t, db); after != before {
		t.Fatal("read-only invocation changed durable facts")
	}
	t.Logf("actual Catalog Publisher + Asset CommitApproval -> bounded PG adapters -> fresh Workbench/Casbin -> A1 Invoker passed; provider lookups=%d", client.calls)
}

type assetAuthorizationFixture struct {
	grants []authidentity.OrganizationGrant
	err    error
	calls  int
}

func (f *assetAuthorizationFixture) ListOwnProjectAuthorizations(ctx context.Context, _, _, _ string) ([]authidentity.OrganizationGrant, error) {
	f.calls++
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return f.grants, f.err
}

type assetAuditFixture struct{}

func (assetAuditFixture) RecordToolCall(context.Context, commercetool.AuditRecord) error { return nil }

func newAssetInvoker(t *testing.T, db *gorm.DB) (*assetinspect.Invoker, commercetoolauth.OrganizationRequest, *assetAuthorizationFixture, *time.Time) {
	t.Helper()
	now := time.Date(2026, 9, 12, 0, 0, 0, 0, time.UTC)
	client := &assetAuthorizationFixture{grants: []authidentity.OrganizationGrant{{OrganizationID: "org-a", ProjectID: "project", Roles: []string{"listingkit_operator"}}}}
	owner := workbenchcontext.NewResolver(workbenchcontext.NewGrantResolver(client, workbenchcontext.NewGrantCache(func() time.Time { return now })), "project", "v1", nil, workbenchcontext.WithResolverClock(func() time.Time { return now }))
	fresh, err := commercetoolauth.NewFreshWorkbenchPrincipalResolver(commercetoolauth.FreshOrganizationResolverFunc(func(ctx context.Context, r commercetoolauth.OrganizationRequest) (authidentity.AuthenticatedIdentity, error) {
		return owner.Resolve(ctx, httproute.OrganizationAccessPolicyLiveWrite, workbenchcontext.ResolveInput{Identity: r.Identity, BearerToken: r.BearerToken, RequestedOrganizationID: r.RequestedOrganizationID})
	}), func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	policy, err := authz.NewListingKitAuthorizer(nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	authorizer, err := commercetoolauth.NewCasbinAuthorizer(policy)
	if err != nil {
		t.Fatal(err)
	}
	snapshots, err := catalogstore.NewBoundedSnapshotReader(db, assetinspect.MaxCatalogSnapshotBytes)
	if err != nil {
		t.Fatal(err)
	}
	assets, err := assetstore.NewBoundedApprovedInventoryReader(db, assetinspect.MaxInventoryBytes)
	if err != nil {
		t.Fatal(err)
	}
	i, err := assetinspect.NewInvoker(snapshots, assets, fresh, commercetool.AgentDefinition{ID: "asset.agent", Version: "v1.0.0", AllowedTools: []commercetool.ToolRef{assetinspect.Definition().Ref}}, commercetool.InvocationDependencies{Authorizer: authorizer, Recorder: assetAuditFixture{}, Tracer: otel.Tracer("asset-postgres"), Now: func() time.Time { return now }, AuditTimeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	request := commercetoolauth.OrganizationRequest{Identity: authidentity.AuthenticatedIdentity{UserID: "actor", HomeOrganizationID: "org-a", TokenExpiresAt: now.Add(time.Hour)}, BearerToken: "controlled-local-fixture", RequestedOrganizationID: "org-a"}
	return i, request, client, &now
}

func assetMetadata() commercetool.CallMetadata {
	return commercetool.CallMetadata{CallID: "call", AgentID: "asset.agent", AgentVersion: "v1.0.0", AgentRunID: "run", BusinessTaskID: "correlation-only"}
}

func openAssetPostgres(t *testing.T) *gorm.DB {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	port := network.MustParsePort("5432/tcp")
	pg, err := tcpostgres.Run(ctx, "postgres:16-alpine", tcpostgres.WithDatabase("tool_a1_404"), tcpostgres.WithUsername("tool_a1"), tcpostgres.WithPassword("isolated_test_only"), tcpostgres.BasicWaitStrategies(), testcontainers.WithHostConfigModifier(func(config *container.HostConfig) {
		if config.PortBindings == nil {
			config.PortBindings = network.PortMap{}
		}
		config.PortBindings[port] = []network.PortBinding{{HostIP: netip.MustParseAddr("127.0.0.1"), HostPort: "0"}}
	}))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := pg.Terminate(context.Background()); err != nil {
			t.Error(err)
		}
	})
	inspection, err := pg.Inspect(ctx)
	if err != nil {
		t.Fatal(err)
	}
	bindings := inspection.NetworkSettings.Ports[port]
	if len(bindings) != 1 || !bindings[0].HostIP.IsLoopback() {
		t.Fatalf("not exclusive loopback binding: %+v", bindings)
	}
	t.Logf("task-owned PostgreSQL %s loopback port %s", pg.GetContainerID(), bindings[0].HostPort)
	dsn, err := pg.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	pool, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := pool.Close(); err != nil {
			t.Error(err)
		}
	})
	return db
}

func assetDatabaseState(t *testing.T, db *gorm.DB) [32]byte {
	t.Helper()
	var tables []string
	if err := db.Raw("SELECT tablename FROM pg_tables WHERE schemaname='public' ORDER BY tablename").Scan(&tables).Error; err != nil {
		t.Fatal(err)
	}
	var state strings.Builder
	for _, table := range tables {
		var content string
		query := `SELECT COALESCE(jsonb_agg(to_jsonb(r) ORDER BY to_jsonb(r)::text),'[]'::jsonb)::text FROM "` + strings.ReplaceAll(table, `"`, `""`) + `" AS r`
		if err := db.Raw(query).Scan(&content).Error; err != nil {
			t.Fatal(err)
		}
		state.WriteString(table)
		state.WriteString(content)
	}
	return sha256.Sum256([]byte(state.String()))
}
