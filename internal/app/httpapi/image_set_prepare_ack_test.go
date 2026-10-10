package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"task-processor/internal/agentconfig"
	"task-processor/internal/authidentity"
	"task-processor/internal/imageagent"
	imagestore "task-processor/internal/imageagent/store"
	configstore "task-processor/internal/integration/persistence/agentconfig"
)

type imageSetPrepareACKConfiguration struct {
	agentconfig.ImageConfigurationRepository
}

func (imageSetPrepareACKConfiguration) PrepareImageConfiguration(_ context.Context, in agentconfig.ImageStartCommand) (agentconfig.ImageConfigurationSnapshot, error) {
	return agentconfig.ImageConfigurationSnapshot{
		ID: "91d39d8d-3819-4a7c-ae7f-04ce91951f9a", Digest: strings.Repeat("a", 64),
		Scope: in.Scope, MemberID: in.MemberID, RequestKey: in.RequestKey, ContextID: in.ContextID, RunID: in.RunID,
		TargetPlatform: in.TargetPlatform, SourceDigest: in.SourceDigest, InputDigest: in.InputDigest, Template: *in.Template,
		Parameters:       agentconfig.SetTemplate{Schema: agentconfig.ImageConfigurationSchema, Mode: "standard", ShareOriginals: true, Background: "white", Language: "zh", Carousel: []agentconfig.ContentTask{{ID: "identity", Purpose: "product_identity"}}},
		ParametersDigest: strings.Repeat("b", 64), Epoch: "1", AgentVersion: agentconfig.ImageAgentVersion, HardLimits: in.HardLimits,
	}, nil
}

type imageSetPrepareACKContexts struct {
	preparation imageagent.ImageSetPreparation
}

func (r imageSetPrepareACKContexts) ResolveImageSet(context.Context, imageagent.ExecutionIdentity, imageagent.PrepareImageSetInput) (imageagent.ImageSetPreparation, error) {
	return r.preparation, nil
}
func (imageSetPrepareACKContexts) RevalidateImageSet(context.Context, imageagent.ExecutionIdentity, imageagent.RunProjection) error {
	return nil
}

type imageSetPrepareACKQuote struct{}

func (imageSetPrepareACKQuote) ReadImageGenerationQuote(context.Context, imageagent.ExecutionIdentity) (imageagent.ImageGenerationQuote, error) {
	return imageagent.ImageGenerationQuote{Provider: "grsai", Model: "gpt-image-2.5", Protocol: "grsai-json-sync-v1", Resolution: "1024x1024", Quality: "auto", PriceVersion: "price", Points: 3, RouteReference: "route", CredentialReference: "credential", ConfigurationVersion: "config"}, nil
}

type imageSetPrepareACKRepository struct {
	imageagent.Repository
	initializations int
}

func (r *imageSetPrepareACKRepository) InitializeRun(ctx context.Context, in imageagent.ProjectionInitialization) (imageagent.RunProjection, error) {
	r.initializations++
	return r.Repository.InitializeRun(ctx, in)
}

func TestFullImageCommittedPreparationRetainsOriginalKeyWhenResponseReadFails(t *testing.T) {
	const sourceID = "d1abe8da-b381-4924-8d15-d79bdbfacf70"
	const requestID = "b912e7d4-df80-44a5-8510-3cdf91d5b8dd"
	for _, readError := range []error{errors.New("configuration read temporarily unavailable"), agentconfig.ErrConflict} {
		t.Run(readError.Error(), func(t *testing.T) {
			catalog, err := imageagent.NormalizeAssetCatalog(imageagent.AssetCatalog{ProductContext: imageagent.ProductContextRef{ProductID: "product", Title: "Controlled product", SourceSnapshotVersion: 1}, Assets: []imageagent.AuthorizedAsset{{ID: "source-1", Type: imageagent.AuthorizedAssetSource, URL: "https://images.test/source.png", Width: 1024, Height: 1024}}})
			require.NoError(t, err)
			contexts := imageSetPrepareACKContexts{preparation: imageagent.ImageSetPreparation{Source: imageagent.ImageSourceBinding{ContextKind: imageagent.ImageSourceAcquisition, ProductID: "product", OperationID: sourceID, OriginalPublicationID: "publication", OriginalVersion: 1, EffectiveVersion: 1, CatalogHash: catalog.Manifest.Hash}, Target: imageagent.ImageTarget{Platform: "product"}, Catalog: catalog, Observations: []imageagent.ImageSourceObservation{{AssetID: "source-1", SHA256: strings.Repeat("c", 64), Bytes: 10, Width: 1024, Height: 1024, MediaType: "image/png"}}}}
			repository := &imageSetPrepareACKRepository{Repository: imagestore.NewMemoryRepository()}
			service, err := imageagent.NewService(repository, imageSetHTTPWorkflow{}, closedImageSetCatalog{}, imageagent.WithOrganizationScope(), imageagent.WithImageSetDependencies(imageagent.ImageSetDependencies{Configuration: imageSetPrepareACKConfiguration{}, Contexts: contexts, Quotes: imageSetPrepareACKQuote{}, HardLimits: agentconfig.ImageRunLimits{Images: 32, Points: 100, ElapsedSeconds: 3600}}))
			require.NoError(t, err)
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			t.Cleanup(func() { _ = db.Close() })
			gormDB, err := gorm.Open(postgres.New(postgres.Config{Conn: db}), &gorm.Config{DisableAutomaticPing: true, Logger: logger.Default.LogMode(logger.Silent)})
			require.NoError(t, err)
			configuration, err := configstore.New(gormDB)
			require.NoError(t, err)
			module := fullImageModule{application: &fullImageApplication{service: service, configuration: configuration}, bind: func(ctx context.Context, _ string) (context.Context, error) { return ctx, nil }}
			router := gin.New()
			for _, route := range module.routes() {
				router.Handle(route.Method, route.Path, route.Handler)
			}
			body, err := json.Marshal(fullImagePrepareBody{Template: &agentconfig.TemplateRef{TemplateID: requestID, Revision: "1"}, Target: imageagent.ImageTargetSelection{Platform: "product"}, SharedOriginalIDs: []string{"source-1"}})
			require.NoError(t, err)
			identity := authidentity.AuthenticatedIdentity{TenantID: "org", EffectiveOrganizationID: "org", UserID: "actor", EffectiveMemberID: "member"}
			ctx := authidentity.WithAuthenticatedIdentity(context.Background(), identity)
			for attempt := 0; attempt < 2; attempt++ {
				mock.ExpectQuery("SELECT").WillReturnError(readError)
				request := httptest.NewRequest(http.MethodPost, "/api/v1/workbench/sourcing/1688/acquisitions/"+sourceID+"/images/prepare", strings.NewReader(string(body)))
				request.Header.Set("Content-Type", "application/json")
				request.Header.Set("Idempotency-Key", requestID)
				response := httptest.NewRecorder()
				router.ServeHTTP(response, request.WithContext(ctx))
				original, readErr := service.GetPreparedImageSet(ctx, imageagent.ImageSourceAcquisition, sourceID, requestID)
				require.NoError(t, readErr)
				require.Equal(t, requestID, original.Run.IdempotencyKey)
				require.Equal(t, imageagent.RunStatusAwaitingPlanApproval, original.Run.Status)
				require.Equal(t, 1, repository.initializations)
				require.Equal(t, http.StatusServiceUnavailable, response.Code, response.Body.String())
				require.JSONEq(t, `{"code":"OUTCOME_UNKNOWN"}`, response.Body.String())
			}
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}
