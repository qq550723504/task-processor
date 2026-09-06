package httpapi

import (
	"context"
	"encoding/json"
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
	catalogstore "task-processor/internal/integration/persistence/product/catalog"
	listingrecord "task-processor/internal/listing/record"
	listingtask "task-processor/internal/listing/task"
	"task-processor/internal/product/catalog"
	"task-processor/internal/workbenchcontext"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type recordGrants struct {
	calls   atomic.Int32
	revoked atomic.Bool
}

func (g *recordGrants) Load(_ context.Context, source workbenchcontext.GrantSource, request workbenchcontext.GrantRequest) (workbenchcontext.GrantResult, error) {
	g.calls.Add(1)
	if source != workbenchcontext.GrantLive {
		return workbenchcontext.GrantResult{}, fmt.Errorf("POST did not request live grants")
	}
	roles := []string{"listingkit_operator"}
	switch request.Subject {
	case "viewer":
		roles = []string{"listingkit_viewer"}
	case "admin":
		roles = []string{"listingkit_admin"}
	case "readonly":
		roles = []string{"admin"}
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
	dsn := os.Getenv("ISSUE319_TEST_DSN")
	if dsn == "" {
		t.Skip("requires task-isolated PostgreSQL ISSUE319_TEST_DSN")
	}
	base, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	name := "issue319_" + strings.ReplaceAll(uuid.NewString(), "-", "")
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
	schema, err := os.ReadFile("../listingrecordstore/schema.sql")
	require.NoError(t, err)
	require.NoError(t, db.Exec(string(schema)).Error)
	return db
}
func publishRecordProduct(t *testing.T, db *gorm.DB, org, key string) catalog.PublishedSnapshot {
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
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, server.URL+"/api/listing/shein-records", strings.NewReader(body))
	require.NoError(t, err)
	if subject != "" {
		req.Header.Set("Authorization", "Bearer "+subject)
	}
	req.Header.Set("Idempotency-Key", key)
	req.Header.Set("X-Requested-Organization-ID", "200")
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

const recordBody = `{"product_key":"product","snapshot_version":1,"country":"US","language":"en"}`

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
	var durable []byte
	require.NoError(t, db.Raw("SELECT payload FROM listing_shein_records WHERE id = ?", receipt.RecordID).Row().Scan(&durable))
	require.Equal(t, durable, stored.Payload)
	pkg, err := sheinpub.DecodePersistedPackageStrict(stored.Payload)
	require.NoError(t, err)
	require.Empty(t, pkg.Images)
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
	publishRecordProduct(t, db, "100", "foreign")
	server, _ := recordApplication(t, db, &recordGrants{})
	tests := []struct {
		name, user, body string
		status           int
	}{
		{"no identity", "", recordBody, 401}, {"no grant", "no-grant", recordBody, 403}, {"viewer", "viewer", recordBody, 403}, {"read only", "readonly", recordBody, 403},
		{"wrong org", "operator", strings.Replace(recordBody, "product\"", "foreign\"", 1), 404},
		{"missing version", "operator", strings.Replace(recordBody, `"snapshot_version":1`, `"snapshot_version":99`, 1), 404},
		{"zero version", "operator", strings.Replace(recordBody, `"snapshot_version":1`, `"snapshot_version":0`, 1), 400},
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

	var count int64
	require.NoError(t, db.Table("listing_shein_records").Count(&count).Error)
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
	server, _ := recordApplication(t, db, &recordGrants{})
	status, body := recordPost(t, server, "operator", "large-package", recordBody)
	require.Equal(t, http.StatusRequestEntityTooLarge, status, string(body))
	require.JSONEq(t, `{"error":"input_too_large"}`, string(body))
	var count int64
	require.NoError(t, db.Table("listing_shein_records").Count(&count).Error)
	require.Zero(t, count)
}
