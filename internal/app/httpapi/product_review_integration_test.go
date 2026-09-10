package httpapi

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"task-processor/internal/app/productsourcing"
	"task-processor/internal/authidentity"
	"task-processor/internal/authz"
	catalogstore "task-processor/internal/integration/persistence/product/catalog"
	"task-processor/internal/product/catalog"
	"task-processor/internal/product/enrichment"
	"task-processor/internal/product/review"
	"task-processor/internal/product/sourcing"
	"task-processor/internal/workbenchcontext"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestProductReviewHTTPCommittedResponseLost(t *testing.T) {
	f := newTitleFixture(t)
	normal := f.server(t)
	v := titleDecision(t, normal, titleCreate(t, normal, "create"), "accept", "admin", "", 200)
	// Actual TCP close after the real handler commits but before response bytes.
	lost := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := httptest.NewRecorder()
		normal.Config.Handler.ServeHTTP(response, r)
		if response.Code != 200 {
			w.WriteHeader(response.Code)
			_, _ = w.Write(response.Body.Bytes())
			return
		}
		conn, _, e := w.(http.Hijacker).Hijack()
		if e == nil {
			_ = conn.Close()
		}
	}))
	defer lost.Close()
	_, _, err := titleRequest(lost, "POST", titleBasePath+"/"+v.ID+"/apply", "admin", "B", "lost", `{"expected_revision":2}`)
	require.Error(t, err)
	recovered := titleApply(t, f.server(t), v, "lost", 200)
	require.Equal(t, uint64(2), recovered.Receipt.ProductVersion)
	current, e := f.reader.GetCurrentSnapshot(context.Background(), f.base.Identity)
	require.NoError(t, e)
	require.Equal(t, uint64(2), current.Version)
}
func TestProductReviewHTTPBodyDeadline(t *testing.T) {
	f := newTitleFixture(t)
	server := f.server(t)
	conn, e := net.Dial("tcp", server.Listener.Addr().String())
	require.NoError(t, e)
	defer conn.Close()
	require.NoError(t, conn.SetDeadline(time.Now().Add(review.Timeout+4*time.Second)))
	_, e = fmt.Fprintf(conn, "POST %s HTTP/1.1\r\nHost: localhost\r\nAuthorization: Bearer operator\r\nX-Requested-Organization-ID: B\r\nIdempotency-Key: slow\r\nContent-Length: 100\r\n\r\n{", titleBasePath)
	require.NoError(t, e)
	response, e := http.ReadResponse(bufio.NewReader(conn), nil)
	require.NoError(t, e)
	defer response.Body.Close()
	require.Equal(t, 504, response.StatusCode)
	require.Zero(t, f.g.calls.Load())
}
func TestProductReviewPostgresCanceledLock(t *testing.T) {
	f := newTitleFixture(t)
	s := f.server(t)
	v := titleDecision(t, s, titleCreate(t, s, "create"), "accept", "admin", "", 200)
	lock := f.db.Begin()
	require.NoError(t, lock.Error)
	defer lock.Rollback()
	require.NoError(t, lock.Exec("SELECT 1 FROM product_title_proposals WHERE org = 'B' AND id = ? FOR UPDATE", v.ID).Error)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	req, e := http.NewRequestWithContext(ctx, "POST", s.URL+titleBasePath+"/"+v.ID+"/apply", strings.NewReader(`{"expected_revision":2}`))
	require.NoError(t, e)
	req.Header.Set("Authorization", "Bearer admin")
	req.Header.Set("X-Requested-Organization-ID", "B")
	req.Header.Set("Idempotency-Key", "canceled")
	_, e = s.Client().Do(req)
	require.Error(t, e)
	require.NoError(t, lock.Rollback().Error)
	result := titleApply(t, s, v, "canceled", 200)
	require.Equal(t, uint64(2), result.Receipt.ProductVersion)
	current, e := f.reader.GetCurrentSnapshot(context.Background(), f.base.Identity)
	require.NoError(t, e)
	require.Equal(t, uint64(2), current.Version)
}

type titleGenerator struct {
	calls         atomic.Int32
	failed        atomic.Bool
	afterGenerate func()
}

func (g *titleGenerator) Generate(_ context.Context, r enrichment.GenerationRequest) (enrichment.Candidate, error) {
	g.calls.Add(1)
	if g.failed.Load() {
		return enrichment.Candidate{}, enrichment.ErrExternalCapabilityUnavailable
	}
	id, e := enrichment.CanonicalEvidenceID(r.Source)
	if g.afterGenerate != nil {
		g.afterGenerate()
	}
	return enrichment.Candidate{Changes: []enrichment.FieldChange{{Field: "title", Value: "Suggested bottle title", EvidenceIDs: []string{id}}}}, e
}

type titleGrants struct {
	revoked  atomic.Bool
	failed   atomic.Bool
	live     atomic.Int32
	cached   atomic.Int32
	revokeAt atomic.Int32
}

func (g *titleGrants) Load(_ context.Context, source workbenchcontext.GrantSource, r workbenchcontext.GrantRequest) (workbenchcontext.GrantResult, error) {
	liveCall := int32(0)
	if source == workbenchcontext.GrantLive {
		liveCall = g.live.Add(1)
	} else {
		g.cached.Add(1)
	}
	if g.failed.Load() {
		return workbenchcontext.GrantResult{}, fmt.Errorf("controlled grant dependency failure")
	}
	roles := []string{"listingkit_operator"}
	if r.Subject == "admin" || r.Subject == "admin2" {
		roles = []string{"listingkit_admin"}
	}
	if r.Subject == "viewer" {
		roles = []string{"listingkit_viewer"}
	}
	if r.Subject == "readonly" {
		roles = []string{"admin"}
	}
	grants := []authidentity.OrganizationGrant{{OrganizationID: "B", ProjectID: "project", Roles: roles}, {OrganizationID: "A", ProjectID: "project", Roles: []string{"listingkit_admin"}}}
	if g.revoked.Load() || g.revokeAt.Load() > 0 && liveCall >= g.revokeAt.Load() {
		grants = nil
	}
	return workbenchcontext.GrantResult{Source: source, Grants: grants}, nil
}
func (*titleGrants) Invalidate(string, string) {}

type titleVerifier struct{}

func (titleVerifier) Verify(_ context.Context, token string) (authidentity.AuthenticatedIdentity, error) {
	if token == "bad" {
		return authidentity.AuthenticatedIdentity{}, fmt.Errorf("bad token")
	}
	return authidentity.AuthenticatedIdentity{UserID: token, HomeOrganizationID: "A", Roles: []string{"listingkit_admin"}, TokenExpiresAt: time.Now().Add(time.Hour)}, nil
}

type titleFixture struct {
	db             *gorm.DB
	source         sourcing.SourceEnvelope
	sourceProducer *sourcing.InternalProducer
	g              *titleGenerator
	grants         *titleGrants
	publisher      *catalog.Publisher
	reader         catalog.Repository
	base           catalog.PublishedSnapshot
}

type titleSetupLiveAccess struct{}

func (titleSetupLiveAccess) ResolveLiveRoles(context.Context, string, string) ([]string, error) {
	return []string{"listingkit_admin"}, nil
}

func newTitleFixture(t *testing.T) *titleFixture {
	t.Helper()
	dsn := os.Getenv("ISSUE382_TEST_DSN")
	if dsn == "" {
		t.Skip("requires isolated PostgreSQL ISSUE382_TEST_DSN")
	}
	cfg := &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)}
	root, e := gorm.Open(postgres.Open(dsn), cfg)
	require.NoError(t, e)
	schema := "review333_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	require.NoError(t, root.Exec("CREATE SCHEMA "+schema).Error)
	db, e := gorm.Open(postgres.Open(dsn+" search_path="+schema), cfg)
	require.NoError(t, e)
	t.Cleanup(func() {
		raw, _ := db.DB()
		_ = raw.Close()
		_ = root.Exec("DROP SCHEMA " + schema + " CASCADE").Error
		raw, _ = root.DB()
		_ = raw.Close()
	})
	require.NoError(t, InstallProductReviewSchema(db))
	repo, e := catalogstore.NewRepository(db)
	require.NoError(t, e)
	publisher, e := catalog.NewPublisher(repo)
	require.NoError(t, e)
	authorizer, e := authz.NewListingKitAuthorizer([]string{"operator"}, nil)
	require.NoError(t, e)
	sourceProducer, e := productsourcing.NewInternalProducer(db, titleSetupLiveAccess{}, authorizer)
	require.NoError(t, e)
	source := sourcing.SourceEnvelope{Identity: sourcing.SourceIdentity{SourceType: sourcing.SourceTypeManualImport, SourcePlatform: "fixture", SourceID: "source1", SourceVersion: "v1"}, RawReference: sourcing.RawSourceReference{ReferenceType: "captured", ReferenceID: "evidence1", SnapshotID: "capture1", Checksum: sourcing.RawSnapshotChecksum("controlled evidence"), CapturedAt: time.Date(2026, 9, 6, 0, 0, 0, 0, time.UTC)}, ProductCandidate: sourcing.ProductCandidate{Title: "Original bottle", Description: "unchanged description", Brand: "unchanged brand", Variants: []sourcing.ProductVariantCandidate{{SourceID: "v1", SKU: "sku", Price: 12, Currency: "USD", Stock: 8}}}, AssetCandidates: []sourcing.AssetCandidate{{URL: "https://example.invalid/image.png", MediaType: "image"}}, Warnings: []sourcing.SourceWarning{{Code: "review", Message: "source needs review"}}}
	publish := func(org, actor, key, publicationID string, envelope sourcing.SourceEnvelope) catalog.PublishedSnapshot {
		ctx := authidentity.WithAuthenticatedIdentity(context.Background(), authidentity.AuthenticatedIdentity{
			TenantID: org, EffectiveOrganizationID: org, UserID: actor, Roles: []string{"listingkit_admin"}, TokenExpiresAt: time.Now().Add(time.Hour),
		})
		receipt, publishErr := sourceProducer.Publish(ctx, sourcing.PublicationCommand{
			PublicationID: publicationID,
			Producer:      sourcing.ProducerDescriptor{Kind: sourcing.ControlledSnapshotProducerKind, Version: sourcing.ControlledSnapshotProducerVersion},
			ProductKey:    key, Envelope: envelope,
		})
		require.NoError(t, publishErr)
		exactReader, readerErr := catalogstore.NewBoundedSnapshotReader(db, sourcing.MaxEncodedSnapshotBytes)
		require.NoError(t, readerErr)
		published, readErr := exactReader.GetSnapshot(context.Background(), catalog.SnapshotIdentity{TenantID: org, ProductKey: key}, receipt.CatalogVersion)
		require.NoError(t, readErr)
		return published
	}
	base := publish("B", "seed", "product", "controlled-initial", source)
	sourceA := source
	sourceA.Identity.SourceID = "source-a"
	sourceA.RawReference.ReferenceID = "evidence-a"
	sourceA.RawReference.SnapshotID = "capture-a"
	_ = publish("A", "seed", "product-a", "controlled-initial-a", sourceA)
	return &titleFixture{db: db, source: source, sourceProducer: sourceProducer, g: &titleGenerator{}, grants: &titleGrants{}, publisher: publisher, reader: repo, base: base}
}

func (f *titleFixture) publishSource(t *testing.T, org, key, publicationID string, source sourcing.SourceEnvelope) catalog.PublishedSnapshot {
	t.Helper()
	ctx := authidentity.WithAuthenticatedIdentity(context.Background(), authidentity.AuthenticatedIdentity{
		TenantID: org, EffectiveOrganizationID: org, UserID: "seed", Roles: []string{"listingkit_admin"}, TokenExpiresAt: time.Now().Add(time.Hour),
	})
	receipt, err := f.sourceProducer.Publish(ctx, sourcing.PublicationCommand{
		PublicationID: publicationID,
		Producer:      sourcing.ProducerDescriptor{Kind: sourcing.ControlledSnapshotProducerKind, Version: sourcing.ControlledSnapshotProducerVersion},
		ProductKey:    key, Envelope: source,
	})
	require.NoError(t, err)
	exactReader, err := catalogstore.NewBoundedSnapshotReader(f.db, sourcing.MaxEncodedSnapshotBytes)
	require.NoError(t, err)
	published, err := exactReader.GetSnapshot(context.Background(), catalog.SnapshotIdentity{TenantID: org, ProductKey: key}, receipt.CatalogVersion)
	require.NoError(t, err)
	return published
}
func (f *titleFixture) server(t *testing.T) *httptest.Server {
	t.Helper()
	auth, e := authz.NewListingKitAuthorizer([]string{"operator"}, nil)
	require.NoError(t, e)
	app, e := NewProductReviewApplication(f.db, titleVerifier{}, workbenchcontext.NewResolver(f.grants, "project", "v1", nil), auth, f.g)
	require.NoError(t, e)
	ts := httptest.NewUnstartedServer(app.Handler)
	ts.Config.ReadTimeout = app.ReadTimeout
	ts.Config.WriteTimeout = app.WriteTimeout
	ts.Start()
	t.Cleanup(ts.Close)
	return ts
}

const titleBasePath = "/api/product/text-proposals"

func titleRequest(server *httptest.Server, method, path, subject, org, key, body string) (int, []byte, error) {
	req, e := http.NewRequest(method, server.URL+path, strings.NewReader(body))
	if e != nil {
		return 0, nil, e
	}
	if subject != "" {
		req.Header.Set("Authorization", "Bearer "+subject)
	}
	req.Header.Set("X-Requested-Organization-ID", org)
	req.Header.Set("X-User-Roles", "listingkit_admin")
	req.Header.Set("X-Tenant-ID", "forged")
	req.Header.Set("Idempotency-Key", key)
	response, e := server.Client().Do(req)
	if e != nil {
		return 0, nil, e
	}
	defer response.Body.Close()
	raw, e := io.ReadAll(response.Body)
	return response.StatusCode, raw, e
}
func titleCall(t *testing.T, server *httptest.Server, method, path, actor, org, key, body string, status int) review.View {
	t.Helper()
	code, raw, e := titleRequest(server, method, path, actor, org, key, body)
	require.NoError(t, e)
	require.Equal(t, status, code, string(raw))
	var v review.View
	if code == 200 {
		v = decodeTitleView(t, raw)
	}
	return v
}

func decodeTitleView(t *testing.T, raw []byte) review.View {
	t.Helper()
	var dto productReviewViewDTO
	require.NoError(t, json.Unmarshal(raw, &dto))
	require.Equal(t, productReviewSchemaVersion, dto.SchemaVersion)
	require.Equal(t, productReviewCoverage, dto.Coverage)
	baseVersion, err := strconv.ParseUint(dto.Input.BaseVersion, 10, 64)
	require.NoError(t, err)
	revision, err := strconv.ParseUint(dto.Revision, 10, 64)
	require.NoError(t, err)
	view := review.View{ID: dto.ProposalID, Owner: dto.Owner, Input: review.CreateInput{ProductKey: dto.Input.ProductKey, BaseVersion: baseVersion}, Before: dto.Before, Title: dto.After, OriginalTitle: dto.OriginalTitle, Policy: dto.Policy, State: dto.State, Revision: revision, Evidence: dto.Evidence, Quality: enrichment.QualityScore{Overall: dto.Quality.Overall, EvidenceCoverage: dto.Quality.EvidenceCoverage, RequiredFieldCoverage: dto.Quality.RequiredFieldCoverage}, Unresolved: dto.Unresolved}
	for _, decision := range dto.Decisions {
		decisionRevision, parseErr := strconv.ParseUint(decision.Revision, 10, 64)
		require.NoError(t, parseErr)
		at, parseErr := time.Parse(time.RFC3339Nano, decision.At)
		require.NoError(t, parseErr)
		view.History = append(view.History, review.Decision{Action: decision.Action, Actor: decision.Actor, Revision: decisionRevision, Before: decision.Before, After: decision.After, At: at})
	}
	if dto.ApplyReceipt != nil {
		receiptRevision, parseErr := strconv.ParseUint(dto.ApplyReceipt.Revision, 10, 64)
		require.NoError(t, parseErr)
		productVersion, parseErr := strconv.ParseUint(dto.ApplyReceipt.ProductVersion, 10, 64)
		require.NoError(t, parseErr)
		at, parseErr := time.Parse(time.RFC3339Nano, dto.ApplyReceipt.At)
		require.NoError(t, parseErr)
		view.Receipt = &review.Receipt{ProposalID: dto.ApplyReceipt.ProposalID, Revision: receiptRevision, ProductVersion: productVersion, PublicationID: dto.ApplyReceipt.PublicationID, Actor: dto.ApplyReceipt.Actor, At: at}
	}
	return view
}
func titleCreate(t *testing.T, s *httptest.Server, key string) review.View {
	return titleCall(t, s, "POST", titleBasePath, "operator", "B", key, `{"product_key":"product","base_version":1}`, 200)
}
func titleCreateFor(t *testing.T, s *httptest.Server, key, actor, org, productKey string) review.View {
	body, err := json.Marshal(review.CreateInput{ProductKey: productKey, BaseVersion: 1})
	require.NoError(t, err)
	return titleCall(t, s, "POST", titleBasePath, actor, org, key, string(body), 200)
}
func titleDecision(t *testing.T, s *httptest.Server, v review.View, action, actor, title string, status int) review.View {
	raw, e := json.Marshal(review.DecisionInput{Action: action, ExpectedRevision: v.Revision, Title: title})
	require.NoError(t, e)
	return titleCall(t, s, "POST", titleBasePath+"/"+v.ID+"/decisions", actor, "B", uuid.NewString(), string(raw), status)
}
func titleApply(t *testing.T, s *httptest.Server, v review.View, key string, status int) review.View {
	return titleCall(t, s, "POST", titleBasePath+"/"+v.ID+"/apply", "admin", "B", key, fmt.Sprintf(`{"expected_revision":%d}`, v.Revision), status)
}
func TestProductReviewHTTPPostgresLifecycle(t *testing.T) {
	f := newTitleFixture(t)
	s := f.server(t)
	liveBeforeCreate := f.grants.live.Load()
	v := titleCreate(t, s, "create")
	require.Equal(t, liveBeforeCreate+3, f.grants.live.Load(), "create must resolve the route and recheck SRC-1 inside the Review UoW")
	require.Equal(t, "Original bottle", v.Before)
	require.Equal(t, "pending", v.State)
	require.Len(t, v.Evidence, 1)
	require.Nil(t, v.Receipt)
	titleApply(t, s, v, "unaccepted", 409)
	titleCall(t, s, "GET", titleBasePath+"/"+v.ID, "other", "B", "", "", 404)
	titleCall(t, s, "GET", titleBasePath+"/"+v.ID, "admin", "A", "", "", 404)
	titleDecision(t, s, v, "accept", "operator", "", 403)
	titleDecision(t, s, v, "edit", "other", "hijack", 404)
	v = titleDecision(t, s, v, "accept", "admin", "", 200)
	accepted := v
	v = titleDecision(t, s, v, "edit", "operator", "Human revised title", 200)
	require.Equal(t, "pending", v.State)
	titleApply(t, s, accepted, "old-approval", 409)
	titleApply(t, s, v, "unreviewed-edit", 409)
	v = titleDecision(t, s, v, "accept", "admin", "", 200)
	applied := titleApply(t, s, v, "apply", 200)
	require.Equal(t, uint64(2), applied.Receipt.ProductVersion)
	require.Equal(t, "operator", applied.History[1].Actor)
	require.Equal(t, "Suggested bottle title", applied.History[1].Before)
	current, e := f.reader.GetCurrentSnapshot(context.Background(), f.base.Identity)
	require.NoError(t, e)
	expected := f.base.Snapshot
	expected.Title = "Human revised title"
	require.Equal(t, expected, current.Snapshot)
	// Advance head after successful Apply; replay must return the original receipt.
	_, e = f.publisher.Publish(context.Background(), catalog.PublishRequest{Identity: f.base.Identity, PublicationID: "later", Snapshot: catalog.ProductSnapshot{Title: "Another writer"}})
	require.NoError(t, e)
	s2 := f.server(t)
	replayed := titleApply(t, s2, v, "apply", 200)
	require.Equal(t, applied, replayed)
	read := titleCall(t, s2, "GET", titleBasePath+"/"+v.ID, "admin", "B", "", "", 200)
	require.Equal(t, applied, read)
	titleApply(t, s2, v, "different-op", 409)
	titleCall(t, s2, "POST", titleBasePath+"/"+v.ID+"/apply", "admin", "B", "apply", `{"expected_revision":1}`, 409)
	f.g.failed.Store(true)
	created := titleCreate(t, s2, "create")
	require.Equal(t, uint64(1), created.Revision)
	require.Equal(t, int32(1), f.g.calls.Load())
	f.grants.revoked.Store(true)
	titleApply(t, s2, v, "apply", 403)
	require.Positive(t, f.grants.live.Load())
	require.Positive(t, f.grants.cached.Load())
}

func TestProductReviewHTTPCreateUsesOneDatabaseConnection(t *testing.T) {
	f := newTitleFixture(t)
	raw, err := f.db.DB()
	require.NoError(t, err)
	raw.SetMaxOpenConns(1)
	raw.SetMaxIdleConns(1)

	v := titleCreate(t, f.server(t), "single-connection-create")
	require.Equal(t, "pending", v.State)
	require.Equal(t, uint64(1), v.Revision)
}

func TestProductReviewHTTPRevokedBetweenRouteAndSourceRead(t *testing.T) {
	f := newTitleFixture(t)
	s := f.server(t)
	f.grants.revokeAt.Store(f.grants.live.Load() + 2)
	titleCall(t, s, "POST", titleBasePath, "operator", "B", "revoked-second-check", `{"product_key":"product","base_version":1}`, 403)
	require.Zero(t, f.g.calls.Load())
}

func TestProductReviewHTTPRevokedBeforeCreateCommitWritesNothing(t *testing.T) {
	f := newTitleFixture(t)
	f.g.afterGenerate = func() {
		f.grants.revokeAt.Store(f.grants.live.Load() + 1)
	}
	titleCall(t, f.server(t), "POST", titleBasePath, "operator", "B", "revoked-before-commit", `{"product_key":"product","base_version":1}`, 403)
	require.Equal(t, int32(1), f.g.calls.Load())
	for _, table := range []string{"product_title_proposals", "product_title_operations"} {
		var count int64
		require.NoError(t, f.db.Table(table).Count(&count).Error)
		require.Zero(t, count, table)
	}
}

func TestProductReviewHTTPEachEvidenceUseRefreshesSourceAuthorizationWithoutSourceWrites(t *testing.T) {
	f := newTitleFixture(t)
	s := f.server(t)
	var sourceRowsBefore int64
	require.NoError(t, f.db.Table("product_source_publications").Count(&sourceRowsBefore).Error)

	before := f.grants.live.Load()
	v := titleCreate(t, s, "fresh-create")
	require.Equal(t, before+3, f.grants.live.Load())
	require.Equal(t, "pending", v.State)

	before = f.grants.live.Load()
	v = titleDecision(t, s, v, "accept", "admin", "", 200)
	require.Equal(t, before+2, f.grants.live.Load())

	before = f.grants.live.Load()
	titleApply(t, s, v, "fresh-apply", 200)
	require.Equal(t, before+2, f.grants.live.Load())

	var sourceRowsAfter int64
	require.NoError(t, f.db.Table("product_source_publications").Count(&sourceRowsAfter).Error)
	require.Equal(t, sourceRowsBefore, sourceRowsAfter, "Review must never write SRC-1 publications")
}

func TestProductReviewHTTPCorruptSourceEvidenceFailsClosed(t *testing.T) {
	f := newTitleFixture(t)
	require.NoError(t, f.db.Exec(
		"UPDATE product_source_publications SET snapshot_json = ? WHERE organization_id = ? AND publication_id = ?",
		[]byte(`{}`), "B", f.base.PublicationID,
	).Error)
	titleCall(t, f.server(t), "POST", titleBasePath, "operator", "B", "corrupt-source", `{"product_key":"product","base_version":1}`, 503)
	require.Zero(t, f.g.calls.Load())
}

func TestProductReviewApplicationRequiresExplicitSchema(t *testing.T) {
	dsn := os.Getenv("ISSUE382_TEST_DSN")
	if dsn == "" {
		t.Skip("requires isolated PostgreSQL ISSUE382_TEST_DSN")
	}
	cfg := &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)}
	root, err := gorm.Open(postgres.Open(dsn), cfg)
	require.NoError(t, err)
	schema := "review382_uninitialized_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	require.NoError(t, root.Exec("CREATE SCHEMA "+schema).Error)
	db, err := gorm.Open(postgres.Open(dsn+" search_path="+schema), cfg)
	require.NoError(t, err)
	t.Cleanup(func() {
		raw, _ := db.DB()
		_ = raw.Close()
		_ = root.Exec("DROP SCHEMA " + schema + " CASCADE").Error
		raw, _ = root.DB()
		_ = raw.Close()
	})
	authorizer, err := authz.NewListingKitAuthorizer(nil, nil)
	require.NoError(t, err)
	_, err = NewProductReviewApplication(db, titleVerifier{}, workbenchcontext.NewResolver(&titleGrants{}, "project", "v1", nil), authorizer, &titleGenerator{})
	require.ErrorIs(t, err, review.ErrUnavailable)
	var tableCount int64
	require.NoError(t, db.Raw("SELECT count(*) FROM information_schema.tables WHERE table_schema = ?", schema).Scan(&tableCount).Error)
	require.Zero(t, tableCount, "application construction must not execute DDL")

	require.NoError(t, InstallProductReviewSchema(db))
	app, err := NewProductReviewApplication(db, titleVerifier{}, workbenchcontext.NewResolver(&titleGrants{}, "project", "v1", nil), authorizer, &titleGenerator{})
	require.NoError(t, err)
	require.NotNil(t, app)
}
func TestProductReviewRejectedStaleAndEvidence(t *testing.T) {
	f := newTitleFixture(t)
	s := f.server(t)
	rejected := titleDecision(t, s, titleCreate(t, s, "reject"), "reject", "admin", "", 200)
	titleApply(t, s, rejected, "apply-reject", 409)
	stale := titleDecision(t, s, titleCreate(t, s, "stale"), "accept", "admin", "", 200)
	_, e := f.publisher.Publish(context.Background(), catalog.PublishRequest{Identity: f.base.Identity, PublicationID: "concurrent", Snapshot: catalog.ProductSnapshot{Title: "other change"}})
	require.NoError(t, e)
	titleApply(t, s, stale, "stale-apply", 409)
	s2 := f.server(t)
	titleDecision(t, s2, stale, "edit", "operator", "edit after stale Catalog head", 200)
	// Version 2 was written directly through Catalog and has no SRC-1 evidence.
	// The exact source publication is missing, with no process-memory fallback.
	titleCall(t, s, "POST", titleBasePath, "operator", "B", "unbound", `{"product_key":"product","base_version":2}`, 404)
	current, e := f.reader.GetCurrentSnapshot(context.Background(), f.base.Identity)
	require.NoError(t, e)
	require.Equal(t, uint64(2), current.Version)
}
func TestProductReviewHTTPStrictInputsAndScope(t *testing.T) {
	f := newTitleFixture(t)
	s := f.server(t)
	for _, body := range []string{`{"product_key":"product","base_version":1,"actor":"admin"}`, `{"product_key":"product","base_version":1,"base_version":1}`, `{"product_key":"product","base_version":0}`, `{"product_key":"product","base_version":-1}`, `null`, `{"product_key":"product","base_version":9223372036854775808}`} {
		titleCall(t, s, "POST", titleBasePath, "operator", "B", uuid.NewString(), body, 400)
	}
	// A valid integer above JavaScript's safe range reaches domain lookup
	// exactly; it is absent, not malformed or rounded.
	titleCall(t, s, "POST", titleBasePath, "operator", "B", "large-valid-version", `{"product_key":"product","base_version":9007199254740993}`, 404)
	titleCall(t, s, "POST", titleBasePath, "operator", "B", "large", strings.Repeat("x", 32769), 413)
	titleCall(t, s, "POST", titleBasePath, "viewer", "B", "viewer", `{}`, 403)
	titleCall(t, s, "POST", titleBasePath, "readonly", "B", "readonly", `{}`, 403)
	titleCall(t, s, "POST", titleBasePath, "", "B", "anon", `{}`, 401)
	v := titleCreate(t, s, "valid")
	for _, body := range []string{"{\"action\":\"edit\",\"expected_revision\":1,\"title\":\"bad" + string([]byte{255}) + "\"}", `{"action":"edit","expected_revision":1,"title":"bad\ud800"}`, `{"action":"edit","expected_revision":1,"title":"bad\udc00"}`} {
		titleCall(t, s, "POST", titleBasePath+"/"+v.ID+"/decisions", "operator", "B", uuid.NewString(), body, 400)
	}
	titleDecision(t, s, v, "edit", "operator", strings.Repeat("x", 4097), 413)
	titleCall(t, s, "GET", titleBasePath+"/invalid", "operator", "B", "", "", 400)
	titleCall(t, s, "GET", titleBasePath+"/"+v.ID+"?org=A", "operator", "B", "", "", 400)
	titleCall(t, s, "POST", titleBasePath, "operator", "B", "valid", `{"product_key":"other","base_version":1}`, 409)
}
func TestProductReviewPostgresConcurrentApplyAndDecision(t *testing.T) {
	f := newTitleFixture(t)
	s1, s2 := f.server(t), f.server(t)
	v := titleCreate(t, s1, "create")
	path := titleBasePath + "/" + v.ID
	run := func(action string, bodies []string, keys []string) []int {
		statuses := make([]int, 2)
		errs := make([]error, 2)
		var wg sync.WaitGroup
		start := make(chan struct{})
		for i, s := range []*httptest.Server{s1, s2} {
			wg.Add(1)
			go func(i int, s *httptest.Server) {
				defer wg.Done()
				<-start
				statuses[i], _, errs[i] = titleRequest(s, "POST", path+action, "admin", "B", keys[i], bodies[i])
			}(i, s)
		}
		close(start)
		wg.Wait()
		require.NoError(t, errs[0])
		require.NoError(t, errs[1])
		return statuses
	}
	statuses := run("/decisions", []string{`{"action":"accept","expected_revision":1}`, `{"action":"accept","expected_revision":1}`}, []string{"decision1", "decision2"})
	require.ElementsMatch(t, []int{200, 409}, statuses)
	statuses = run("/apply", []string{`{"expected_revision":2}`, `{"expected_revision":2}`}, []string{"apply", "apply"})
	require.Equal(t, []int{200, 200}, statuses)
	current, e := f.reader.GetCurrentSnapshot(context.Background(), f.base.Identity)
	require.NoError(t, e)
	require.Equal(t, uint64(2), current.Version)
}

func TestProductReviewPostgresTwoProposalsCompete(t *testing.T) {
	f := newTitleFixture(t)
	s1, s2 := f.server(t), f.server(t)
	a := titleDecision(t, s1, titleCreate(t, s1, "a"), "accept", "admin", "", 200)
	b := titleDecision(t, s2, titleCreate(t, s2, "b"), "accept", "admin", "", 200)
	statuses := make([]int, 2)
	errs := make([]error, 2)
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i, v := range []review.View{a, b} {
		wg.Add(1)
		go func(i int, v review.View) {
			defer wg.Done()
			<-start
			statuses[i], _, errs[i] = titleRequest([]*httptest.Server{s1, s2}[i], "POST", titleBasePath+"/"+v.ID+"/apply", "admin", "B", v.ID, fmt.Sprintf(`{"expected_revision":%d}`, v.Revision))
		}(i, v)
	}
	close(start)
	wg.Wait()
	require.NoError(t, errs[0])
	require.NoError(t, errs[1])
	require.ElementsMatch(t, []int{200, 409}, statuses)
	current, e := f.reader.GetCurrentSnapshot(context.Background(), f.base.Identity)
	require.NoError(t, e)
	require.Equal(t, uint64(2), current.Version)
}

func TestProductReviewPostgresBoundedSnapshotRead(t *testing.T) {
	f := newTitleFixture(t)
	source := f.source
	source.ProductCandidate.Description = strings.Repeat("x", 2<<20)
	snapshot, e := sourcing.ToSnapshot(source)
	require.NoError(t, e)
	_, e = f.publisher.Publish(context.Background(), catalog.PublishRequest{Identity: f.base.Identity, ExpectedBaseVersion: &f.base.Version, PublicationID: "large", Snapshot: snapshot})
	require.NoError(t, e)
	s := f.server(t)
	titleCall(t, s, "POST", titleBasePath, "operator", "B", "bounded", `{"product_key":"product","base_version":2}`, 503)
	require.Zero(t, f.g.calls.Load())
}

func TestProductReviewPostgresCapacityLeavesAcceptedApplyable(t *testing.T) {
	f := newTitleFixture(t)
	source := f.source
	source.RawReference.Metadata = map[string]string{}
	for index := 0; index < 8; index++ {
		source.RawReference.Metadata[fmt.Sprintf("controlled-large-metadata-%d", index)] = strings.Repeat("m", 7500)
	}
	f.publishSource(t, "B", "product", "large-evidence", source)
	s := f.server(t)
	v := titleCall(t, s, "POST", titleBasePath, "operator", "B", "large-create", `{"product_key":"product","base_version":2}`, 200)
	var size int
	require.NoError(t, f.db.Raw("SELECT octet_length(payload) FROM product_title_proposals WHERE org = ? AND id = ?", "B", v.ID).Scan(&size).Error)
	require.Greater(t, size, 60000)
	v = titleDecision(t, s, v, "accept", "admin", "", 200)
	titleDecision(t, s, v, "edit", "operator", strings.Repeat("x", 4096), 413)
	stored := titleCall(t, s, "GET", titleBasePath+"/"+v.ID, "admin", "B", "", "", 200)
	require.Equal(t, v, stored)
	result := titleApply(t, s, v, "large-apply", 200)
	require.Equal(t, uint64(3), result.Receipt.ProductVersion)
}

func TestProductReviewJSONUnicode(t *testing.T) {
	for _, raw := range []string{`{"title":"emoji\ud83d\ude00"}`, `{"title":"literal \\ud800"}`, `{"title":"replacement �"}`, `{"title":"中文"}`} {
		require.True(t, validReviewJSONUnicode([]byte(raw)), raw)
	}
	for _, raw := range []string{`{"title":"\ud800"}`, `{"title":"\udc00"}`, `{"title":"\ud800\u0061"}`, `{"title":"` + string([]byte{255}) + `"}`} {
		require.False(t, validReviewJSONUnicode([]byte(raw)))
	}
}
func TestProductReviewPostgresPublisherRace(t *testing.T) {
	for i := 0; i < 6; i++ {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			f := newTitleFixture(t)
			s := f.server(t)
			v := titleDecision(t, s, titleCreate(t, s, "create"), "accept", "admin", "", 200)
			start := make(chan struct{})
			done := make(chan error, 1)
			go func() {
				<-start
				_, e := f.publisher.Publish(context.Background(), catalog.PublishRequest{Identity: f.base.Identity, PublicationID: "ordinary", Snapshot: catalog.ProductSnapshot{Title: "ordinary writer"}})
				done <- e
			}()
			close(start)
			code, raw, e := titleRequest(s, "POST", titleBasePath+"/"+v.ID+"/apply", "admin", "B", "apply", `{"expected_revision":2}`)
			require.NoError(t, e)
			require.Contains(t, []int{200, 409}, code, string(raw))
			require.NoError(t, <-done)
			current, e := f.reader.GetCurrentSnapshot(context.Background(), f.base.Identity)
			require.NoError(t, e)
			require.Equal(t, "ordinary writer", current.Snapshot.Title)
			if code == 200 {
				require.Equal(t, uint64(3), current.Version)
			} else {
				require.Equal(t, uint64(2), current.Version)
			}
		})
	}
}
func TestProductReviewPostgresAtomicFailuresAndLostResponse(t *testing.T) {
	for _, table := range []string{"product_title_proposals", "product_title_operations"} {
		t.Run(table, func(t *testing.T) {
			f := newTitleFixture(t)
			s := f.server(t)
			v := titleDecision(t, s, titleCreate(t, s, "create"), "accept", "admin", "", 200)
			// Trigger fails only after Catalog's INSERT/head update have run.
			require.NoError(t, f.db.Exec("CREATE FUNCTION reject_review_write() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'isolated injected failure'; END $$").Error)
			require.NoError(t, f.db.Exec("CREATE TRIGGER fail_review BEFORE INSERT OR UPDATE ON "+table+" FOR EACH ROW EXECUTE FUNCTION reject_review_write()").Error)
			titleApply(t, s, v, "apply", 503)
			current, e := f.reader.GetCurrentSnapshot(context.Background(), f.base.Identity)
			require.NoError(t, e)
			require.Equal(t, uint64(1), current.Version)
			read := titleCall(t, s, "GET", titleBasePath+"/"+v.ID, "admin", "B", "", "", 200)
			require.Equal(t, "accepted", read.State)
			require.Nil(t, read.Receipt)
			require.NoError(t, f.db.Exec("DROP TRIGGER fail_review ON "+table).Error)
			committed := titleApply(t, s, v, "apply", 200)
			// Discard committed response and reconstruct the application. Receipt alone
			// resolves the retry, without running generation/publication a second time.
			replay := titleApply(t, f.server(t), v, "apply", 200)
			require.Equal(t, committed, replay)
			current, e = f.reader.GetCurrentSnapshot(context.Background(), f.base.Identity)
			require.NoError(t, e)
			require.Equal(t, uint64(2), current.Version)
		})
	}
}
