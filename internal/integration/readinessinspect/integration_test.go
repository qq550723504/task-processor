//go:build integration

// Package readinessinspect_test records the TOOL-V1 admission gap. It does not
// implement a Tool or establish marketplace readiness from Product inputs.
package readinessinspect_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/network"
	"github.com/stretchr/testify/require"
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
	tool "task-processor/internal/listing/readiness/tools/readinessinspect"
	"task-processor/internal/workbenchcontext"

	assetstore "task-processor/internal/integration/persistence/product/asset"
	catalogstore "task-processor/internal/integration/persistence/product/catalog"
	"task-processor/internal/listing/readiness"
	"task-processor/internal/listing/record"
	"task-processor/internal/marketplace/shein/draft"
	sheinvalidator "task-processor/internal/marketplace/shein/validator"
	contract "task-processor/internal/marketplace/validator"
	"task-processor/internal/product/asset"
	"task-processor/internal/product/catalog"
	sheinpub "task-processor/internal/publishing/shein"
)

type liveProvider struct {
	grants []authidentity.OrganizationGrant
	err    error
	calls  int
}

func (p *liveProvider) ListOwnProjectAuthorizations(ctx context.Context, _, _, _ string) ([]authidentity.OrganizationGrant, error) {
	p.calls++
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return p.grants, p.err
}

type toolAudits struct{ records []commercetool.AuditRecord }

func (a *toolAudits) RecordToolCall(_ context.Context, r commercetool.AuditRecord) error {
	a.records = append(a.records, r)
	return nil
}

func TestToolInvokeFreshAuthorizationExactPostgresInputDiagnostics(t *testing.T) {
	db := openReadinessPostgres(t)
	ctx := context.Background()
	require.NoError(t, catalogstore.AutoMigrate(db))
	require.NoError(t, assetstore.AutoMigrate(db))
	repo, err := catalogstore.NewRepository(db)
	require.NoError(t, err)
	publisher, err := catalog.NewPublisher(repo)
	require.NoError(t, err)
	id := catalog.SnapshotIdentity{TenantID: "org-a", ProductKey: "product-1"}
	publication := 0
	publish := func(snapshot catalog.ProductSnapshot) catalog.PublishedSnapshot {
		publication++
		p, publishErr := publisher.Publish(ctx, catalog.PublishRequest{Identity: id, PublicationID: "publication-" + strconv.Itoa(publication), Snapshot: snapshot})
		require.NoError(t, publishErr)
		return p
	}
	p1 := publish(catalog.ProductSnapshot{Title: "Bottle"})
	p2 := publish(catalog.ProductSnapshot{Title: "Bottle reviewed", Review: &catalog.ReviewState{NeedsReview: true, Reasons: []string{"missing dimensions"}}, Warnings: []catalog.Warning{{Code: "missing_dimensions", Field: "dimensions", Message: "missing dimensions"}}})
	p3 := publish(catalog.ProductSnapshot{Title: "Corrupted later"})
	p4 := publish(catalog.ProductSnapshot{Title: "Oversize", Description: strings.Repeat("x", tool.MaxSnapshotBytes)})
	p5 := publish(catalog.ProductSnapshot{Title: "Corrupt inventory"})
	p6 := publish(catalog.ProductSnapshot{Title: "Oversize single asset"})
	p7 := publish(catalog.ProductSnapshot{Title: "Oversize asset set"})
	assets, err := assetstore.NewRepository(db)
	require.NoError(t, err)
	for _, c := range []struct {
		version     uint64
		count, size int
	}{{p6.Version, 1, tool.MaxSnapshotBytes + 1}, {p7.Version, 3, tool.MaxSnapshotBytes / 2}} {
		items := make([]asset.ApprovedAsset, c.count)
		for index := range items {
			items[index] = asset.ApprovedAsset{ID: fmt.Sprintf("large-%d-%d", c.version, index), RunID: "run", PlanRevision: 1, SlotID: fmt.Sprintf("slot-%d", index), Attempt: 1, Role: asset.RoleMain, URL: "https://controlled.invalid/" + strings.Repeat("x", c.size)}
		}
		_, err = assets.CommitApproval(ctx, asset.ApprovalCommit{TenantID: id.TenantID, ProductKey: id.ProductKey, TargetPlatform: "shein", SourceSnapshotVersion: c.version, ActionID: fmt.Sprintf("large-%d", c.version), Assets: items})
		require.NoError(t, err)
		// Retain the size while making decoding invalid. The size precheck must
		// win before the malformed row is decoded (which would return internal).
		require.NoError(t, db.Exec("UPDATE product_approved_assets SET payload_json = ? WHERE asset_id = ?", []byte(`{"id":"`+strings.Repeat("x", c.size)+`"}`), items[0].ID).Error)
	}
	for _, version := range []uint64{p1.Version, p5.Version} {
		_, err = assets.CommitApproval(ctx, asset.ApprovalCommit{TenantID: id.TenantID, ProductKey: id.ProductKey, TargetPlatform: "shein", SourceSnapshotVersion: version,
			ActionID: "approval-" + strconv.FormatUint(version, 10), Assets: []asset.ApprovedAsset{{ID: "asset-" + strconv.FormatUint(version, 10), RunID: "run", PlanRevision: 1, SlotID: "main", Attempt: 1, Role: asset.RoleMain, URL: "https://controlled.invalid/main.jpg"}}})
		require.NoError(t, err)
	}
	// An unversioned head must not satisfy the exact missing v2 inventory.
	_, err = assets.CommitApproval(ctx, asset.ApprovalCommit{TenantID: id.TenantID, ProductKey: id.ProductKey, TargetPlatform: "shein", ActionID: "unversioned", Assets: []asset.ApprovedAsset{{ID: "unversioned", RunID: "run", PlanRevision: 1, SlotID: "main", Attempt: 1, Role: asset.RoleMain, URL: "https://controlled.invalid/unversioned.jpg"}}})
	require.NoError(t, err)
	require.NoError(t, db.Model(&catalogstore.SnapshotVersionRecord{}).Where("tenant_id = ? AND product_key = ? AND version = ?", id.TenantID, id.ProductKey, p3.Version).Update("snapshot_json", []byte(`{"title":"tampered"}`)).Error)
	// Corrupt only this test-owned row, after committing via the Asset owner.
	require.NoError(t, db.Exec("UPDATE product_approved_assets SET payload_json = ? WHERE asset_id = ?", []byte(`{"id":"wrong"}`), "asset-"+strconv.FormatUint(p5.Version, 10)).Error)

	err = db.Transaction(func(tx *gorm.DB) error {
		require.NoError(t, tx.Exec("SET TRANSACTION READ ONLY").Error)
		products, readErr := catalogstore.NewBoundedSnapshotReader(tx, tool.MaxSnapshotBytes)
		require.NoError(t, readErr)
		assetReader, readErr := assetstore.NewBoundedApprovedInventoryReader(tx, tool.MaxSnapshotBytes)
		require.NoError(t, readErr)
		now := time.Now()
		provider := &liveProvider{grants: []authidentity.OrganizationGrant{{OrganizationID: "org-a", ProjectID: "project", Roles: []string{"listingkit_operator"}}, {OrganizationID: "org-b", ProjectID: "project", Roles: []string{"listingkit_operator"}}}}
		grants := workbenchcontext.NewGrantResolver(provider, workbenchcontext.NewGrantCache(func() time.Time { return now }))
		owner := workbenchcontext.NewResolver(grants, "project", "v1", nil, workbenchcontext.WithResolverClock(func() time.Time { return now }))
		request := commercetoolauth.OrganizationRequest{Identity: authidentity.AuthenticatedIdentity{UserID: "actor", HomeOrganizationID: "home", TokenExpiresAt: now.Add(time.Minute)}, BearerToken: "controlled-secret", RequestedOrganizationID: "org-a"}
		_, readErr = owner.Resolve(ctx, httproute.OrganizationAccessPolicyCachedRead, workbenchcontext.ResolveInput{Identity: request.Identity, BearerToken: request.BearerToken, RequestedOrganizationID: request.RequestedOrganizationID})
		require.NoError(t, readErr)
		fresh, readErr := commercetoolauth.NewFreshWorkbenchPrincipalResolver(commercetoolauth.FreshOrganizationResolverFunc(func(ctx context.Context, r commercetoolauth.OrganizationRequest) (authidentity.AuthenticatedIdentity, error) {
			return owner.Resolve(ctx, httproute.OrganizationAccessPolicyLiveWrite, workbenchcontext.ResolveInput{Identity: r.Identity, BearerToken: r.BearerToken, RequestedOrganizationID: r.RequestedOrganizationID})
		}), func() time.Time { return now })
		require.NoError(t, readErr)
		policy, readErr := authz.NewListingKitAuthorizer(nil, nil)
		require.NoError(t, readErr)
		authorizer, readErr := commercetoolauth.NewCasbinAuthorizer(policy)
		require.NoError(t, readErr)
		audits := &toolAudits{}
		invoker, readErr := tool.NewInvoker(products, assetReader, fresh, commercetool.AgentDefinition{ID: "test.agent", Version: "v1.0.0", AllowedTools: []commercetool.ToolRef{tool.Definition().Ref}}, commercetool.InvocationDependencies{Authorizer: authorizer, Recorder: audits, Tracer: otel.Tracer("readiness-pg"), Now: func() time.Time { return now }, AuditTimeout: time.Second})
		require.NoError(t, readErr)
		invoke := func(version uint64) (commercetool.Result, error) {
			return invoker.Invoke(commercetoolauth.WithOrganizationRequest(ctx, request), commercetool.CallMetadata{CallID: "call", AgentID: "test.agent", AgentVersion: "v1.0.0", AgentRunID: "run", BusinessTaskID: "correlation"}, tool.Input{ProductKey: id.ProductKey, CatalogVersion: strconv.FormatUint(version, 10), TargetPlatform: "shein"})
		}
		first, invokeErr := invoke(p1.Version)
		require.NoError(t, invokeErr)
		var result tool.Output
		require.NoError(t, json.Unmarshal(first.Output, &result))
		require.Equal(t, "ready", result.Status)
		require.Equal(t, "product.inputs", result.Scope)
		require.Equal(t, "not_evaluated", result.Marketplace.Status)
		again, invokeErr := invoke(p1.Version)
		require.NoError(t, invokeErr)
		require.Equal(t, first.Output, again.Output)
		require.Equal(t, 3, provider.calls, "warm cache does not replace two live lookups")
		missing, invokeErr := invoke(p2.Version)
		require.NoError(t, invokeErr)
		require.NoError(t, json.Unmarshal(missing.Output, &result))
		require.Equal(t, "blocked", result.Status)
		require.Contains(t, result.Reasons, "approved_assets_not_ready")
		require.Contains(t, result.Reasons, "source_review_required")
		require.Equal(t, "missing", result.AssetBinding.Status)
		require.Len(t, result.Diagnostics.Warnings, 1)
		for _, c := range []struct {
			version uint64
			code    commercetool.ErrorCode
		}{{99, commercetool.ErrorNotFound}, {p3.Version, commercetool.ErrorInternal}, {p4.Version, commercetool.ErrorFailedPrecondition}, {p5.Version, commercetool.ErrorInternal}, {p6.Version, commercetool.ErrorFailedPrecondition}, {p7.Version, commercetool.ErrorFailedPrecondition}} {
			failed, invokeErr := invoke(c.version)
			require.Equal(t, c.code, commercetool.CodeOf(invokeErr))
			require.Empty(t, failed.Output)
		}
		request.RequestedOrganizationID = "org-b"
		_, invokeErr = invoke(p1.Version)
		require.Equal(t, commercetool.ErrorNotFound, commercetool.CodeOf(invokeErr))
		request.RequestedOrganizationID = "org-a"
		provider.grants[0].Roles = []string{"listingkit_viewer"}
		_, invokeErr = invoke(p1.Version)
		require.Equal(t, commercetool.ErrorPermissionDenied, commercetool.CodeOf(invokeErr))
		provider.grants = provider.grants[1:]
		_, invokeErr = invoke(p1.Version)
		require.Equal(t, commercetool.ErrorIdentityIntegrity, commercetool.CodeOf(invokeErr))
		provider.err = errors.New("controlled-secret provider unavailable")
		_, invokeErr = invoke(p1.Version)
		require.Equal(t, commercetool.ErrorIdentityIntegrity, commercetool.CodeOf(invokeErr))
		require.NotContains(t, invokeErr.Error(), "controlled-secret")
		encoded, encodeErr := json.Marshal(audits.records)
		require.NoError(t, encodeErr)
		require.NotContains(t, string(encoded), "controlled-secret")
		for _, audit := range audits.records {
			require.Equal(t, "product.readiness.inspect", audit.ToolID)
			require.Equal(t, "v1.0.0", audit.ToolVersion)
		}
		return nil
	})
	require.NoError(t, err)
}

// A PASS here proves the missing domain input, not the requested ready Must.
// Country/language/actions below are explicit test cases, never Tool defaults.
// Setup writes only controlled samples in a new task-owned PostgreSQL container;
// the observed read/build/diagnose path runs inside a READ ONLY transaction.
func TestReadinessAdmissionGapExactProductAssetsRemainMarketplaceBlocked(t *testing.T) {
	db := openReadinessPostgres(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	require.NoError(t, catalogstore.AutoMigrate(db))
	require.NoError(t, assetstore.AutoMigrate(db))
	products, err := catalogstore.NewRepository(db)
	require.NoError(t, err)
	publisher, err := catalog.NewPublisher(products)
	require.NoError(t, err)
	identity := catalog.SnapshotIdentity{TenantID: "tool-v1-org", ProductKey: "bottle"}
	published, err := publisher.Publish(ctx, catalog.PublishRequest{
		Identity: identity, PublicationID: "tool-v1-publication",
		Snapshot: catalog.ProductSnapshot{
			Title: "Stainless steel bottle", Description: "Reusable stainless steel water bottle.",
			CategoryPath: []string{"Home", "Drinkware"},
			Attributes:   []catalog.Attribute{{Name: "Material", Value: "Stainless steel"}},
			Variants:     []catalog.Variant{{SKU: "bottle-1", Stock: 10, Price: &catalog.Price{Currency: "USD", Amount: 20}}},
		},
	})
	require.NoError(t, err)
	assets, err := assetstore.NewRepository(db)
	require.NoError(t, err)
	_, err = assets.CommitApproval(ctx, asset.ApprovalCommit{
		TenantID: identity.TenantID, ProductKey: identity.ProductKey, TargetPlatform: "shein",
		SourceSnapshotVersion: published.Version, ActionID: "controlled-approval",
		Assets: []asset.ApprovedAsset{{ID: "approved-main", RunID: "controlled-run", PlanRevision: 1,
			SlotID: "main", Attempt: 1, Role: asset.RoleMain, URL: "https://controlled.invalid/main.jpg"}},
	})
	require.NoError(t, err)

	err = db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		require.NoError(t, tx.Exec("SET TRANSACTION READ ONLY").Error)
		reader, readErr := catalogstore.NewBoundedSnapshotReader(tx, 2<<20)
		require.NoError(t, readErr)
		exact, readErr := reader.GetSnapshot(ctx, identity, published.Version)
		require.NoError(t, readErr)
		require.Equal(t, published, exact)
		assetReader, readErr := assetstore.NewBoundedApprovedInventoryReader(tx, tool.MaxSnapshotBytes)
		require.NoError(t, readErr)
		scope := asset.InventoryScope{TenantID: identity.TenantID, ProductKey: identity.ProductKey,
			TargetPlatform: "shein", SourceSnapshotVersion: published.Version}
		inventory, readErr := assetReader.GetApprovedInventory(ctx, scope)
		require.NoError(t, readErr)
		require.Equal(t, scope, inventory.Scope)
		require.Len(t, inventory.Assets, 1)
		require.True(t, readiness.ProductInputs(exact, &inventory, "shein").Ready,
			"the Product input gate is satisfied; this is not marketplace-ready")
		for _, action := range []contract.Action{contract.SaveDraft, contract.Publish} {
			t.Run(string(action), func(t *testing.T) {
				input := record.Input{ProductKey: identity.ProductKey, SnapshotVersion: published.Version,
					Country: "US", Language: "en", Action: action}
				raw, buildErr := (draft.Builder{}).Build(ctx, exact.Snapshot, inventory, input)
				require.NoError(t, buildErr)
				pkg, decodeErr := sheinpub.DecodePersistedPackageStrict(raw)
				require.NoError(t, decodeErr)
				require.Zero(t, pkg.CategoryID)
				require.Nil(t, pkg.ProductTypeID)
				require.Nil(t, pkg.CategoryResolution)
				observed := time.Date(2026, 9, 12, 0, 0, 0, 0, time.UTC)
				request := contract.BoundRequest[[]byte]{Input: raw,
					Target: contract.Target{Marketplace: "shein"}, Action: action,
					RuleVersion: sheinvalidator.DiagnosticRuleVersion, BindingVersion: sheinvalidator.BindingVersion,
					ReadAt: observed, EvaluatedAt: observed, Freshness: contract.ExternalFreshness{Status: contract.NotEvaluated}}
				result, evaluateErr := (sheinvalidator.ExactApprovedAssetValidator{}).Validate(request)
				require.NoError(t, evaluateErr)
				require.Equal(t, contract.Blocked, result.OfflineChecks.Status)
				require.True(t, result.DiagnosticOnly)
				require.Equal(t, contract.NotEvaluated, result.Freshness.Status)
				require.NotContains(t, result.NotEvaluated, "approved_asset_provenance_and_consent")
				var blockerRules []string
				for _, blocker := range result.OfflineChecks.Blockers {
					blockerRules = append(blockerRules, blocker.Rule)
				}
				require.Contains(t, blockerRules, "category")
				t.Logf("gap evidence only: action=%s productVersion=%d rule=%s status=%s blockers=%v",
					action, exact.Version, result.RuleVersion, result.OfflineChecks.Status, blockerRules)
				first, encodeErr := json.Marshal(result)
				require.NoError(t, encodeErr)
				for range 3 {
					again, repeatErr := (draft.Builder{}).Build(ctx, exact.Snapshot, inventory, input)
					require.NoError(t, repeatErr)
					require.Equal(t, raw, again)
					request.Input = again
					repeated, repeatErr := (sheinvalidator.ExactApprovedAssetValidator{}).Validate(request)
					require.NoError(t, repeatErr)
					encoded, encodeErr := json.Marshal(repeated)
					require.NoError(t, encodeErr)
					require.Equal(t, first, encoded)
				}
			})
		}
		return nil
	})
	require.NoError(t, err)
}

func openReadinessPostgres(t *testing.T) *gorm.DB {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	port := network.MustParsePort("5432/tcp")
	pg, err := tcpostgres.Run(ctx, "postgres:16-alpine",
		tcpostgres.WithDatabase("tool_v1"), tcpostgres.WithUsername("tool_v1"), tcpostgres.WithPassword("tool_v1"),
		tcpostgres.BasicWaitStrategies(),
		testcontainers.WithHostConfigModifier(func(config *container.HostConfig) {
			if config.PortBindings == nil {
				config.PortBindings = network.PortMap{}
			}
			config.PortBindings[port] = []network.PortBinding{{HostIP: netip.MustParseAddr("127.0.0.1"), HostPort: "0"}}
		}),
	)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, pg.Terminate(context.Background())) })
	inspection, err := pg.Inspect(ctx)
	require.NoError(t, err)
	bindings := inspection.NetworkSettings.Ports[port]
	require.Len(t, bindings, 1)
	require.True(t, bindings[0].HostIP.IsLoopback())
	require.NotEmpty(t, bindings[0].HostPort)
	require.NotEqual(t, "0", bindings[0].HostPort)
	t.Logf("task-owned PostgreSQL %s at %s:%s", pg.GetContainerID(), bindings[0].HostIP, bindings[0].HostPort)
	dsn, err := pg.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	pool, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, pool.Close()) })
	return db
}
