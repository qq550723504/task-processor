package imageagentworker

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"task-processor/internal/aicapability/store"
	"task-processor/internal/app/httpapi"
	accountschema "task-processor/internal/app/schema/sourceaccountregistry"
	"task-processor/internal/authidentity"
	"task-processor/internal/authz"
	"task-processor/internal/imageagent"
	"task-processor/internal/imageagent/tools"
	openai "task-processor/internal/integration/openai"
	accountallocationstore "task-processor/internal/integration/persistence/accountallocation"
	"task-processor/internal/listingsubscription"
	"task-processor/internal/workbenchcontext"
)

type controlledAuditVerifier struct{}

func (controlledAuditVerifier) Verify(context.Context, string) (authidentity.AuthenticatedIdentity, error) {
	return authidentity.AuthenticatedIdentity{UserID: "actor-1", HomeOrganizationID: "org-1", TokenExpiresAt: time.Now().Add(time.Hour)}, nil
}

type controlledAuditGrants struct{}

func (controlledAuditGrants) Invalidate(string, string) {}
func (controlledAuditGrants) Load(_ context.Context, source workbenchcontext.GrantSource, _ workbenchcontext.GrantRequest) (workbenchcontext.GrantResult, error) {
	return workbenchcontext.GrantResult{Source: source, Grants: []authidentity.OrganizationGrant{{OrganizationID: "org-1", ProjectID: "project", Roles: []string{"listingkit_viewer"}}}}, nil
}

func TestOrganizationMainControlledProviderSettlesOnlyObservedReviewTokens(t *testing.T) {
	db := controlledMainAuditPostgres(t)
	require.NoError(t, db.Exec(`UPDATE saas_tenant_entitlements SET limits = ? WHERE tenant_id = ?`, `{"ai_tokens":1000000}`, "org-1").Error)
	require.NoError(t, db.AutoMigrate(&openai.AIClientCredential{}))
	require.NoError(t, store.AutoMigrateInvocationLedger(db))
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	img.Set(0, 0, color.RGBA{R: 255, G: 255, B: 255, A: 255})
	var pngBody bytes.Buffer
	require.NoError(t, png.Encode(&pngBody, img))
	imageBytes := pngBody.Bytes()
	var imageCalls, reviewCalls, unexpectedCalls atomic.Int32
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/images/edits":
			imageCalls.Add(1)
			_ = json.NewEncoder(w).Encode(map[string]any{"data": []any{map[string]string{"b64_json": base64.StdEncoding.EncodeToString(imageBytes)}}})
		case "/v1/chat/completions":
			reviewCalls.Add(1)
			_, _ = io.WriteString(w, `{"id":"controlled-review","choices":[{"message":{"role":"assistant","content":"{\"score\":0.9,\"needs_human_review\":false,\"reasons\":[]}"}}],"usage":{"prompt_tokens":3,"completion_tokens":4,"total_tokens":7}}`)
		default:
			unexpectedCalls.Add(1)
			http.NotFound(w, r)
		}
	}))
	defer provider.Close()
	credentials := openai.NewGormCredentialResolver(db)
	for _, name := range []string{"default", "image_gpt_image_2"} {
		require.NoError(t, credentials.SaveCredential(context.Background(), openai.AIClientCredential{TenantID: "org-1", UserID: "actor-1", ClientName: name, APIKey: "controlled-local-only", BaseURL: provider.URL + "/v1", Model: "review-test", Enabled: true, TimeoutSecond: 2}))
	}
	reference := &http.Client{Transport: controlledImageReference{body: imageBytes}}
	defaultConfig := openai.NewClientConfig("unused", "review-test", provider.URL+"/v1", 2)
	imageConfig := openai.NewClientConfig("unused", "image-test", provider.URL+"/v1", 2)
	defaultConfig.ImageReferenceHTTPClient, imageConfig.ImageReferenceHTTPClient = reference, reference
	manager, err := openai.NewManager(&openai.ManagerConfig{Clients: map[string]*openai.ClientConfig{"default": defaultConfig, "image_gpt_image_2": imageConfig}, DefaultClient: "default"})
	require.NoError(t, err)
	recorder := store.NewGormInvocationRecorder(db)
	recorder.SetUsageSettler(listingsubscription.AIInvocationUsageAdapter{Repository: listingsubscription.NewGormRepository(db)})
	capabilities, err := BuildOrganizationImageCapabilities(manager, db, OrganizationReviewOptions{Recorder: recorder, Logger: logrus.New()})
	require.NoError(t, err)
	delegate := tools.NewProductImageSlotExecutor(tools.Dependencies{SubjectExtractor: capabilities.SubjectExtractor, WhiteBackgroundRenderer: capabilities.WhiteBackgroundRenderer, SceneRenderer: capabilities.SceneRenderer, Reviewer: capabilities.Reviewer, UsageQuoter: capabilities.UsageQuoter, ProfileResolver: capabilities.ProfileResolver})
	executor := organizationMainSlotExecutor{delegate: delegate, quoter: capabilities.UsageQuoter, reservation: recorder}
	input := imageagent.SlotExecutionInput{RunID: "run-1", TenantID: "org-1", UserID: "actor-1", PlanRevision: 1, Attempt: 1, IdempotencyKey: "attempt-1", TargetPlatform: "product", ImagePolicyContext: &imageagent.ImagePolicyContext{Country: "zz", Family: "default", SceneCategory: "general"}, Slot: imageagent.Slot{ID: "main-1", Role: imageagent.SlotRoleMain, SourceAssetIDs: []string{"catalog-image-1"}, IdempotencyKey: "slot-1"}, AssetCatalog: imageagent.AssetCatalog{ProductContext: imageagent.ProductContextRef{ProductID: "product-1", Title: "Controlled product", SourceSnapshotVersion: 1}, Assets: []imageagent.AuthorizedAsset{{ID: "catalog-image-1", Type: imageagent.AuthorizedAssetSource, URL: "https://source.example/product.png", Width: 1200, Height: 1200}}}, ProductContext: imageagent.ProductContextRef{ProductID: "product-1", Title: "Controlled product", SourceSnapshotVersion: 1}}
	ctx := mainAdmissionContext()
	quote, err := executor.QuoteSlot(ctx, input, imageagent.BudgetPolicy{})
	require.NoError(t, err)
	require.EqualValues(t, 3, quote.Maximum.Images)
	output, err := executor.GenerateQuotedSlot(ctx, input, quote)
	require.NoError(t, err)
	require.Len(t, output.Assets, 1)
	require.Equal(t, imageBytes, output.Assets[0].Bytes)
	require.EqualValues(t, 2, imageCalls.Load())
	require.EqualValues(t, 1, reviewCalls.Load())
	require.Zero(t, unexpectedCalls.Load())
	var usage []struct {
		EventID  string
		SourceID string
		Quantity int64
		MemberID string
		Status   string
	}
	require.NoError(t, db.Table("saas_usage_events").Where("source_type = ?", "ai_invocation").Find(&usage).Error)
	require.Len(t, usage, 1, "Account/Audit read the canonical commercial invocation fact")
	require.EqualValues(t, 7, usage[0].Quantity)
	require.Equal(t, "member-1", usage[0].MemberID)
	require.Equal(t, "committed", usage[0].Status)
	// Rebuild the existing Account Audit HTTP module over the same canonical PG
	// ledger event emitted by the controlled provider path above.
	require.NoError(t, accountschema.Migrate(context.Background(), db))
	require.NoError(t, accountallocationstore.AutoMigrate(db))
	authorizer, err := authz.NewListingKitAuthorizer(nil, nil)
	require.NoError(t, err)
	server, err := httpapi.NewAccountAuditApplication(context.Background(), db, controlledAuditVerifier{}, workbenchcontext.NewResolver(controlledAuditGrants{}, "project", "v1", nil), authorizer)
	require.NoError(t, err)
	get := func(org, query string) *httptest.ResponseRecorder {
		request := httptest.NewRequest(http.MethodGet, "/api/v1/account/audit"+query, nil)
		request.Header.Set("Authorization", "Bearer controlled")
		request.Header.Set("X-Requested-Organization-ID", org)
		response := httptest.NewRecorder()
		server.Handler.ServeHTTP(response, request)
		return response
	}
	response := get("org-1", "?limit=1")
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	var audit struct {
		Items []struct {
			EventType, Actor, ObjectReference string
			Relation                          struct{ Reference string }
			Usage                             struct {
				MemberID string
				Quantity int64
			}
		}
	}
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &audit))
	require.Len(t, audit.Items, 1)
	require.Equal(t, "account_ai_tokens.committed", audit.Items[0].EventType)
	require.Empty(t, audit.Items[0].Actor)
	require.Equal(t, usage[0].SourceID, audit.Items[0].ObjectReference)
	require.Equal(t, usage[0].EventID, audit.Items[0].Relation.Reference)
	require.Equal(t, "member-1", audit.Items[0].Usage.MemberID)
	require.EqualValues(t, 7, audit.Items[0].Usage.Quantity)
	require.Equal(t, http.StatusForbidden, get("org-2", "?limit=1").Code)
}

func controlledMainAuditPostgres(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("ISSUE487_TEST_DSN")
	if dsn == "" {
		t.Skip("requires isolated PostgreSQL ISSUE487_TEST_DSN")
	}
	root, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	database := "issue487_audit_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	require.NoError(t, root.Exec("CREATE DATABASE "+database).Error)
	db, err := gorm.Open(postgres.Open(dsn+" dbname="+database), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	var actual string
	require.NoError(t, db.Raw("SELECT current_database()").Scan(&actual).Error)
	require.Equal(t, database, actual)
	t.Cleanup(func() {
		pool, _ := db.DB()
		_ = pool.Close()
		if err := root.Exec("DROP DATABASE " + database).Error; err != nil {
			t.Errorf("drop dedicated audit test database %s: %v", database, err)
		}
		rootPool, _ := root.DB()
		_ = rootPool.Close()
	})
	require.NoError(t, listingsubscription.AutoMigrateRepository(db))
	require.NoError(t, db.Exec(`CREATE TABLE account_member_token_locks (organization_id text PRIMARY KEY, updated_at timestamptz NOT NULL)`).Error)
	require.NoError(t, db.Exec(`CREATE TABLE account_member_token_allocations (organization_id text NOT NULL, member_id text NOT NULL, metric text NOT NULL, allocated bigint NOT NULL, version bigint NOT NULL, active boolean NOT NULL, window_start timestamptz NOT NULL, window_end timestamptz NOT NULL, updated_at timestamptz NOT NULL, PRIMARY KEY (organization_id,member_id,metric))`).Error)
	start, end := time.Now().UTC().Add(-time.Hour), time.Now().UTC().Add(time.Hour)
	require.NoError(t, db.Exec(`INSERT INTO saas_tenant_entitlements (tenant_id,module_code,status,starts_at,expires_at,limits) VALUES (?,?,?,?,?,?)`, "org-1", listingsubscription.ModuleListingKit, listingsubscription.StatusActive, start, end, `{"ai_tokens":1000000}`).Error)
	require.NoError(t, db.Exec(`INSERT INTO account_member_token_allocations (organization_id,member_id,metric,allocated,version,active,window_start,window_end,updated_at) VALUES (?,?,?,?,?,?,?,?,?)`, "org-1", "member-1", "token", 10000, 1, true, start, end, start).Error)
	return db
}

type controlledImageReference struct{ body []byte }

func (t controlledImageReference) RoundTrip(request *http.Request) (*http.Response, error) {
	if request.URL.Host != "source.example" || request.URL.Scheme != "https" {
		return nil, imageagent.ErrValidation
	}
	return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"image/png"}}, Body: io.NopCloser(bytes.NewReader(t.body)), Request: request}, nil
}
