package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	sheinvalidator "task-processor/internal/marketplace/shein/validator"
	contract "task-processor/internal/marketplace/validator"
	sheinpub "task-processor/internal/publishing/shein"
	"testing"
	"time"

	"github.com/google/uuid"

	"task-processor/internal/authidentity"
	"task-processor/internal/authz"
	assetstore "task-processor/internal/integration/persistence/product/asset"
	catalogstore "task-processor/internal/integration/persistence/product/catalog"
	listingrecord "task-processor/internal/listing/record"
	listingtask "task-processor/internal/listing/task"
	productasset "task-processor/internal/product/asset"
	"task-processor/internal/product/catalog"
	"task-processor/internal/storecenter"
	"task-processor/internal/workbenchcontext"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type recordGrants struct {
	calls   atomic.Int32
	revoked atomic.Bool
	failed  atomic.Bool
}

func (g *recordGrants) Load(_ context.Context, source workbenchcontext.GrantSource, request workbenchcontext.GrantRequest) (workbenchcontext.GrantResult, error) {
	g.calls.Add(1)
	if g.failed.Load() {
		return workbenchcontext.GrantResult{}, errors.New("controlled grant dependency failure")
	}
	if source != workbenchcontext.GrantLive && source != workbenchcontext.GrantReadCached {
		return workbenchcontext.GrantResult{}, fmt.Errorf("unexpected grant source")
	}
	roles := []string{"listingkit_operator"}
	switch request.Subject {
	case "viewer":
		roles = []string{"listingkit_viewer"}
	case "admin":
		roles = []string{"listingkit_admin"}
	case "readonly":
		roles = []string{"admin"}
	case "store":
		roles = []string{"listingkit_viewer"}
	}
	grants := []authidentity.OrganizationGrant{{OrganizationID: "200", ProjectID: "project", Roles: roles}}
	if request.Subject == "no-grant" || g.revoked.Load() {
		grants = nil
	}
	return workbenchcontext.GrantResult{Source: source, Grants: grants}, nil
}
func (*recordGrants) Invalidate(string, string) {}

type recordVerifier struct{}

func (recordVerifier) Verify(_ context.Context, token string) (authidentity.AuthenticatedIdentity, error) {
	return authidentity.AuthenticatedIdentity{UserID: token, HomeOrganizationID: "100", Roles: []string{"listingkit_admin"}, TokenExpiresAt: time.Now().Add(time.Hour)}, nil
}
func recordTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("ISSUE376_TEST_DSN")
	if dsn == "" {
		t.Skip("requires task-isolated PostgreSQL ISSUE376_TEST_DSN")
	}
	base, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	name := "issue376_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	require.NoError(t, base.Exec("CREATE SCHEMA "+name).Error)
	db, err := gorm.Open(postgres.Open(dsn+" search_path="+name), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	t.Cleanup(func() {
		sqlDB, e := db.DB()
		if e == nil {
			_ = sqlDB.Close()
		}
		_ = base.Exec("DROP SCHEMA " + name + " CASCADE").Error
		raw, e := base.DB()
		if e == nil {
			_ = raw.Close()
		}
	})
	require.NoError(t, catalogstore.AutoMigrate(db))
	require.NoError(t, assetstore.AutoMigrate(db))
	require.NoError(t, storecenter.AutoMigrateStoreRepository(db))
	schema, err := os.ReadFile("../listingrecordstore/schema.sql")
	require.NoError(t, err)
	require.NoError(t, db.Exec(string(schema)).Error)
	return db
}

func requireDraftS1RowCounts(t *testing.T, db *gorm.DB, expected int64) {
	t.Helper()
	var count int64
	require.NoError(t, db.Table("listing_shein_records").Count(&count).Error)
	require.Equal(t, expected, count, "immutable record rows")
	require.NoError(t, db.Table("listing_shein_record_operations").Count(&count).Error)
	require.Equal(t, expected, count, "operation receipt rows")
}
func publishRecordProduct(t *testing.T, db *gorm.DB, org, key string) catalog.PublishedSnapshot {
	published := publishRecordProductOnly(t, db, org, key)
	prepareRecordStore(t, db, org)
	prepareApprovedAssets(t, db, org, key, published.Version)
	return published
}

func publishRecordProductOnly(t *testing.T, db *gorm.DB, org, key string) catalog.PublishedSnapshot {
	t.Helper()
	repo, err := catalogstore.NewRepository(db)
	require.NoError(t, err)
	publisher, err := catalog.NewPublisher(repo)
	require.NoError(t, err)
	// These upstream server values never come from the Listing request.
	upstream := authidentity.AuthenticatedIdentity{EffectiveOrganizationID: org, UserID: "upstream-author"}
	published, err := publisher.Publish(context.Background(), catalog.PublishRequest{Identity: catalog.SnapshotIdentity{TenantID: upstream.EffectiveOrganizationID, ProductKey: key}, PublicationID: uuid.NewString(), Snapshot: catalog.ProductSnapshot{Title: "Bottle", Images: []catalog.Image{{URL: "https://example.com/source.jpg"}}}})
	require.NoError(t, err)
	return published
}

const recordStoreID = "11111111-1111-4111-8111-111111111111"

func prepareRecordStore(t *testing.T, db *gorm.DB, organizationID string) {
	prepareRecordStoreID(t, db, organizationID, recordStoreID)
}

func prepareRecordStoreID(t *testing.T, db *gorm.DB, organizationID, storeID string) {
	t.Helper()
	repository, err := storecenter.NewGormStoreRepository(db)
	require.NoError(t, err)
	if existing, getErr := repository.Get(context.Background(), organizationID, storeID); getErr == nil {
		require.Equal(t, organizationID, existing.OrganizationID())
		return
	} else {
		require.ErrorIs(t, getErr, storecenter.ErrNotFound)
	}
	stored, err := storecenter.NewStore(storecenter.CreateStoreInput{
		ID: storeID, OrganizationID: organizationID, ActorSubject: "store-owner", Name: "Controlled SHEIN Store", Platform: "shein", Region: "US",
		ExternalStoreID: "controlled-store", CreateIdempotencyKey: uuid.NewString(), QuotaAllocationID: uuid.NewString(),
		OccurredAt: time.Date(2026, 9, 9, 0, 0, 0, 0, time.UTC),
	})
	require.NoError(t, err)
	_, replayed, err := repository.CreateOrReplay(context.Background(), organizationID, stored)
	require.NoError(t, err)
	require.False(t, replayed)
}

func prepareApprovedAssets(t *testing.T, db *gorm.DB, organizationID, productKey string, version uint64) {
	t.Helper()
	repository, err := assetstore.NewRepository(db)
	require.NoError(t, err)
	_, err = repository.CommitApproval(context.Background(), productasset.ApprovalCommit{
		TenantID: organizationID, ProductKey: productKey, TargetPlatform: "shein", ActionID: uuid.NewString(), SourceSnapshotVersion: version,
		Assets: []productasset.ApprovedAsset{{
			ID: uuid.NewString(), RunID: uuid.NewString(), PlanRevision: 1, SlotID: "main", Attempt: 1,
			Role: productasset.RoleMain, URL: fmt.Sprintf("https://fixtures.invalid/%s/%d/main.jpg", productKey, version),
		}},
	})
	require.NoError(t, err)
}
func recordApplication(t *testing.T, db *gorm.DB, g *recordGrants) (*httptest.Server, listingrecord.Reader) {
	t.Helper()
	auth, err := authz.NewListingKitAuthorizer(nil, nil)
	require.NoError(t, err)
	server, reader, err := NewSheinRecordApplication(db, recordVerifier{}, workbenchcontext.NewResolver(g, "project", "v1", nil), auth)
	require.NoError(t, err)
	// Exercise actual TCP HTTP using the production registration and server timeouts.
	ts := httptest.NewUnstartedServer(server.Handler)
	ts.Config.ReadTimeout = server.ReadTimeout
	ts.Config.WriteTimeout = server.WriteTimeout
	ts.Start()
	t.Cleanup(ts.Close)
	return ts, reader
}
func recordPost(t *testing.T, server *httptest.Server, subject, key, body string) (int, []byte) {
	return recordPostForOrganization(t, server, subject, key, "200", body)
}

func recordPostForOrganization(t *testing.T, server *httptest.Server, subject, key, organizationID, body string) (int, []byte) {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, server.URL+"/api/listing/shein-records", strings.NewReader(body))
	require.NoError(t, err)
	if subject != "" {
		req.Header.Set("Authorization", "Bearer "+subject)
	}
	req.Header.Set("Idempotency-Key", key)
	req.Header.Set("X-Requested-Organization-ID", organizationID)
	req.Header.Set("X-Tenant-ID", "forged")
	req.Header.Set("X-User-ID", "forged")
	req.Header.Set("X-User-Roles", "listingkit_admin")
	response, err := server.Client().Do(req)
	require.NoError(t, err)
	defer response.Body.Close()
	raw, err := io.ReadAll(response.Body)
	require.NoError(t, err)
	return response.StatusCode, raw
}

func recordGet(t *testing.T, server *httptest.Server, subject, rawQuery string) (int, []byte) {
	t.Helper()
	path := server.URL + "/api/listing/shein-records"
	if rawQuery != "" {
		path += "?" + rawQuery
	}
	req, err := http.NewRequest(http.MethodGet, path, nil)
	require.NoError(t, err)
	if subject != "" {
		req.Header.Set("Authorization", "Bearer "+subject)
	}
	req.Header.Set("X-Requested-Organization-ID", "200")
	req.Header.Set("X-Tenant-ID", "forged")
	req.Header.Set("X-User-ID", "forged")
	response, err := server.Client().Do(req)
	require.NoError(t, err)
	defer response.Body.Close()
	raw, err := io.ReadAll(response.Body)
	require.NoError(t, err)
	return response.StatusCode, raw
}

const recordBody = `{"product_key":"product","snapshot_version":1,"store_id":"11111111-1111-4111-8111-111111111111","country":"US","language":"en","action":"save_draft"}`

func TestDraftS1ExactApprovedAssetMissDoesNotUseUnversionedHead(t *testing.T) {
	db := recordTestDB(t)
	published := publishRecordProductOnly(t, db, "200", "product")
	prepareRecordStore(t, db, "200")
	repository, err := assetstore.NewRepository(db)
	require.NoError(t, err)
	_, err = repository.CommitApproval(context.Background(), productasset.ApprovalCommit{
		TenantID: "200", ProductKey: "product", TargetPlatform: "shein", ActionID: "unversioned-only",
		Assets: []productasset.ApprovedAsset{{
			ID: "unversioned-main", RunID: "run-1", PlanRevision: 1, SlotID: "main", Attempt: 1,
			Role: productasset.RoleMain, URL: "https://example.test/unversioned-main.jpg",
		}},
	})
	require.NoError(t, err)
	require.EqualValues(t, 1, published.Version)

	server, _ := recordApplication(t, db, &recordGrants{})
	status, body := recordPost(t, server, "operator", "draft-s1-exact-miss", recordBody)
	require.Equal(t, http.StatusUnprocessableEntity, status, string(body))
	require.JSONEq(t, `{"error":"not_ready"}`, string(body))

	var records int64
	require.NoError(t, db.Table("listing_shein_records").Count(&records).Error)
	require.Zero(t, records)
	require.NoError(t, db.Table("listing_shein_record_operations").Count(&records).Error)
	require.Zero(t, records)
}

func recordActor(user, role string) listingtask.Actor {
	return listingtask.Actor{TenantID: "200", UserID: user, Roles: []string{role}}
}
func TestSheinRecordHTTPPostgresRoundTrip(t *testing.T) {
	db := recordTestDB(t)
	published := publishRecordProduct(t, db, "200", "product")
	g := &recordGrants{}
	server, reader := recordApplication(t, db, g)
	status, body := recordPost(t, server, "operator", "op", recordBody)
	require.Equal(t, 201, status, string(body))
	var receipt listingrecord.Receipt
	require.NoError(t, json.Unmarshal(body, &receipt))
	require.NotEmpty(t, receipt.RecordID)
	_, reader = recordApplication(t, db, g)
	stored, err := reader.ReadOfflinePackage(context.Background(), recordActor("operator", "listingkit_operator"), receipt.RecordID)
	require.NoError(t, err)
	require.Equal(t, "200", stored.OrganizationID)
	require.Equal(t, "operator", stored.OwnerUserID)
	require.Equal(t, published.Version, stored.Input.SnapshotVersion)
	require.Equal(t, recordStoreID, stored.Input.StoreID)
	require.Equal(t, contract.SaveDraft, stored.Input.Action)
	require.Equal(t, published.Version, stored.AssetInventoryVersion)
	require.Equal(t, receipt.InputHash, stored.InputHash)
	require.Equal(t, receipt.DiagnosticHash, stored.DiagnosticHash)
	require.Equal(t, receipt.DiagnosticStatus, stored.DiagnosticStatus)
	require.True(t, receipt.DiagnosticOnly)
	var durable []byte
	require.NoError(t, db.Raw("SELECT payload FROM listing_shein_records WHERE id = ?", receipt.RecordID).Row().Scan(&durable))
	require.Equal(t, durable, stored.Payload)
	pkg, err := sheinpub.DecodePersistedPackageStrict(stored.Payload)
	require.NoError(t, err)
	require.NotNil(t, pkg.Images)
	require.Equal(t, "https://fixtures.invalid/product/1/main.jpg", pkg.Images.MainImage)
	require.NotContains(t, string(stored.Payload), "https://example.com/source.jpg")
	var persistedDiagnostic contract.DiagnosticResult
	require.NoError(t, json.Unmarshal(stored.Diagnostic, &persistedDiagnostic))
	require.True(t, persistedDiagnostic.DiagnosticOnly)
	require.Equal(t, contract.NotEvaluated, persistedDiagnostic.Freshness.Status)
	require.NotContains(t, persistedDiagnostic.NotEvaluated, "approved_asset_provenance_and_consent")
	require.Contains(t, persistedDiagnostic.NotEvaluated, "online_template_freshness")
	for _, item := range []struct {
		actor listingtask.Actor
		deny  bool
	}{{recordActor("upstream-author", "listingkit_operator"), true}, {recordActor("other", "listingkit_operator"), true}, {recordActor("admin", "listingkit_admin"), false}, {listingtask.Actor{TenantID: "100", UserID: "admin", Roles: []string{"listingkit_admin"}}, true}} {
		_, e := reader.ReadOfflinePackage(context.Background(), item.actor, receipt.RecordID)
		if item.deny {
			require.ErrorIs(t, e, listingrecord.ErrNotFound)
		} else {
			require.NoError(t, e)
		}
	}
	result, err := (sheinvalidator.DiagnosticValidator{}).Validate(contract.BoundRequest[[]byte]{Input: stored.Payload, Target: contract.Target{Marketplace: "shein"}, Action: contract.Action("publish"), RuleVersion: sheinvalidator.DiagnosticRuleVersion, BindingVersion: sheinvalidator.BindingVersion, ReadAt: stored.ReadAt, EvaluatedAt: time.Now().UTC(), Freshness: contract.ExternalFreshness{Status: contract.NotEvaluated}})
	require.NoError(t, err)
	require.True(t, result.DiagnosticOnly)
	require.NotEmpty(t, result.OfflineChecks.Blockers)
	require.Equal(t, contract.NotEvaluated, result.Freshness.Status)
	stored.Payload[0] = 'x'
	again, err := reader.ReadOfflinePackage(context.Background(), recordActor("operator", "listingkit_operator"), receipt.RecordID)
	require.NoError(t, err)
	require.Equal(t, durable, again.Payload)
	status, replay := recordPost(t, server, "operator", "op", recordBody)
	require.Equal(t, 201, status)
	require.JSONEq(t, string(body), string(replay))
	require.EqualValues(t, 2, g.calls.Load())
	source, err := catalogstore.NewRepository(db)
	require.NoError(t, err)
	unchanged, err := source.GetCurrentSnapshot(context.Background(), published.Identity)
	require.NoError(t, err)
	require.Equal(t, published, unchanged)
	g.revoked.Store(true)
	status, _ = recordPost(t, server, "operator", "op", recordBody)
	require.Equal(t, 403, status)
}
func TestSheinRecordHTTPNegativeInputs(t *testing.T) {
	db := recordTestDB(t)
	publishRecordProduct(t, db, "200", "product")
	publishRecordProductOnly(t, db, "100", "foreign")
	server, _ := recordApplication(t, db, &recordGrants{})
	tests := []struct {
		name, user, body string
		status           int
	}{
		{"no identity", "", recordBody, 401}, {"no grant", "no-grant", recordBody, 403}, {"viewer", "viewer", recordBody, 403}, {"read only", "readonly", recordBody, 403},
		{"wrong org", "operator", strings.Replace(recordBody, "product\"", "foreign\"", 1), 404},
		{"missing exact store", "operator", strings.Replace(recordBody, recordStoreID, "22222222-2222-4222-8222-222222222222", 1), 404},
		{"missing version", "operator", strings.Replace(recordBody, `"snapshot_version":1`, `"snapshot_version":99`, 1), 404},
		{"zero version", "operator", strings.Replace(recordBody, `"snapshot_version":1`, `"snapshot_version":0`, 1), 400},
		{"invalid store", "operator", strings.Replace(recordBody, recordStoreID, "store", 1), 400},
		{"unsupported action", "operator", strings.Replace(recordBody, `"save_draft"`, `"preview"`, 1), 400},
		{"owner spoof", "operator", strings.Replace(recordBody, `"country"`, `"user_id":"victim","country"`, 1), 400},
		{"org spoof", "operator", strings.Replace(recordBody, `"country"`, `"tenant_id":"100","country"`, 1), 400},
		{"trusted spoof", "operator", strings.Replace(recordBody, `"country"`, `"trusted":true,"country"`, 1), 400},
		{"package spoof", "operator", strings.Replace(recordBody, `"country"`, `"package":{},"country"`, 1), 400},
		{"freshness spoof", "operator", strings.Replace(recordBody, `"country"`, `"freshness":{},"country"`, 1), 400},
		{"duplicate key", "operator", strings.Replace(recordBody, `"country"`, `"product_key":"other","country"`, 1), 400},
		{"case key", "operator", strings.Replace(recordBody, "product_key", "PRODUCT_KEY", 1), 400},
		{"oversize", "operator", `{"product_key":"` + strings.Repeat("x", 1500) + `"}`, 400},
		{"unsupported language", "operator", strings.Replace(recordBody, `"en"`, `"zh"`, 1), 400},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			status, body := recordPost(t, server, test.user, uuid.NewString(), test.body)
			require.Equal(t, test.status, status, string(body))
			require.NotContains(t, string(body), "Bottle")
		})
	}
	var count int64
	require.NoError(t, db.Table("listing_shein_records").Count(&count).Error)
	require.Zero(t, count)
	require.NoError(t, db.Table("listing_shein_record_operations").Count(&count).Error)
	require.Zero(t, count)
}

func TestSheinRecordGrantDependencyFailureDoesNotReadOrWrite(t *testing.T) {
	db := recordTestDB(t)
	publishRecordProduct(t, db, "200", "product")
	grants := &recordGrants{}
	grants.failed.Store(true)
	server, _ := recordApplication(t, db, grants)
	status, body := recordPost(t, server, "operator", "grant-failure", recordBody)
	require.Equal(t, http.StatusServiceUnavailable, status, string(body))
	require.Contains(t, string(body), `"code":"DEPENDENCY_UNAVAILABLE"`)
	require.NotContains(t, string(body), "controlled grant dependency failure")
	var count int64
	require.NoError(t, db.Table("listing_shein_records").Count(&count).Error)
	require.Zero(t, count)
	require.NoError(t, db.Table("listing_shein_record_operations").Count(&count).Error)
	require.Zero(t, count)
}

func TestDraftS1CorruptExactApprovedAssetDoesNotCreateReceipt(t *testing.T) {
	db := recordTestDB(t)
	publishRecordProduct(t, db, "200", "product")
	require.NoError(t, db.Table("product_approved_assets").Where("tenant_id = ? AND product_key = ? AND target_platform = ? AND source_snapshot_version = ?", "200", "product", "shein", 1).Update("payload_json", []byte(`{"id":"wrong"}`)).Error)
	server, _ := recordApplication(t, db, &recordGrants{})
	status, body := recordPost(t, server, "operator", "corrupt-exact-asset", recordBody)
	require.Equal(t, http.StatusServiceUnavailable, status, string(body))
	require.JSONEq(t, `{"error":"unavailable"}`, string(body))
	requireDraftS1RowCounts(t, db, 0)
}
func TestSheinRecordConcurrentAndConflictingHTTP(t *testing.T) {
	db := recordTestDB(t)
	publishRecordProduct(t, db, "200", "product")
	publishRecordProduct(t, db, "200", "another")
	first, _ := recordApplication(t, db, &recordGrants{})
	second, _ := recordApplication(t, db, &recordGrants{})
	var wg sync.WaitGroup
	ids := make(chan string, 12)
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			server := first
			if i%2 == 0 {
				server = second
			}
			status, body := recordPost(t, server, "operator", "concurrent", recordBody)
			require.Equal(t, 201, status, string(body))
			var receipt listingrecord.Receipt
			require.NoError(t, json.Unmarshal(body, &receipt))
			ids <- receipt.RecordID
		}(i)
	}
	wg.Wait()
	close(ids)
	var id string
	for got := range ids {
		if id == "" {
			id = got
		}
		require.Equal(t, id, got)
	}
	status, _ := recordPost(t, first, "operator", "concurrent", strings.Replace(recordBody, `"product"`, `"another"`, 1))
	require.Equal(t, 409, status)
	status, _ = recordPost(t, first, "operator", "concurrent", strings.Replace(recordBody, `"snapshot_version":1`, `"snapshot_version":99`, 1))
	require.Equal(t, 409, status)
	status, _ = recordPost(t, first, "operator", "concurrent", strings.Replace(recordBody, `"save_draft"`, `"publish"`, 1))
	require.Equal(t, 409, status)
	status, _ = recordPost(t, first, "other", "concurrent", recordBody)
	require.Equal(t, 409, status)

	var count int64
	require.NoError(t, db.Table("listing_shein_records").Count(&count).Error)
	require.EqualValues(t, 1, count)
	require.NoError(t, db.Table("listing_shein_record_operations").Count(&count).Error)
	require.EqualValues(t, 1, count)
}

func TestSheinRecordOversizedGeneratedPackageHTTP(t *testing.T) {
	db := recordTestDB(t)
	repository, err := catalogstore.NewRepository(db)
	require.NoError(t, err)
	publisher, err := catalog.NewPublisher(repository)
	require.NoError(t, err)
	_, err = publisher.Publish(context.Background(), catalog.PublishRequest{
		Identity: catalog.SnapshotIdentity{TenantID: "200", ProductKey: "product"}, PublicationID: "oversized-setup",
		Snapshot: catalog.ProductSnapshot{Title: "Bottle", SellingPoints: []string{strings.Repeat("detail ", 350000)}},
	})
	require.NoError(t, err)
	prepareRecordStore(t, db, "200")
	prepareApprovedAssets(t, db, "200", "product", 1)
	server, _ := recordApplication(t, db, &recordGrants{})
	status, body := recordPost(t, server, "operator", "large-package", recordBody)
	require.Equal(t, http.StatusServiceUnavailable, status, string(body))
	require.JSONEq(t, `{"error":"unavailable"}`, string(body))
	var count int64
	require.NoError(t, db.Table("listing_shein_records").Count(&count).Error)
	require.Zero(t, count)
}
