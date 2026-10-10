package httpapi

import (
	"context"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
	"math"
	"net/http"
	"reflect"
	"task-processor/internal/agentconfig"
	imageapp "task-processor/internal/app/imageagent"
	appruntime "task-processor/internal/app/runtime"
	supplyapp "task-processor/internal/app/supplychain"
	imageworker "task-processor/internal/app/worker/imageagent"
	"task-processor/internal/authidentity"
	"task-processor/internal/authruntime/zitadel"
	"task-processor/internal/authz"
	"task-processor/internal/core/config"
	"task-processor/internal/imageagent"
	"task-processor/internal/imageagent/assetpublication"
	imagestore "task-processor/internal/imageagent/store"
	configstore "task-processor/internal/integration/persistence/agentconfig"
	assetstore "task-processor/internal/integration/persistence/product/asset"
	"task-processor/internal/listing/preparation"
	"task-processor/internal/product/asset"
	"task-processor/internal/product/collection"
	productimage "task-processor/internal/product/image"
	"task-processor/internal/workbenchcontext"
	"time"
)

type fullImageApplication struct {
	service         *imageagent.Service
	readSources     imageapp.ImageSetSourceReader
	contexts        *imageapp.ImageSetContextReader
	quotes          imageagent.ImageSetQuoteReader
	selections      *asset.ImageSetService
	inventories     asset.ImageSetInventoryReader
	approvals       asset.ApprovalCommitReader
	publicURLs      imageagent.DurableAssetPublicURLResolver
	trial           *imageagent.IsolatedTrialGeneratedURLPolicy
	configuration   *configstore.Store
	recent          imageagent.ImageSetRunReader
	gate            imageagent.TenantAllowlistStartGate
	rules           supplyapp.ImageSetTargetRules
	manualAvailable bool
}

type FullImageSetDependencies = appruntime.FullImageSetDependencies

type imageMediaScopeAuthority struct {
	current supplyapp.OrganizationExecutionAuthorizer
}

func (a imageMediaScopeAuthority) Authorize(ctx context.Context, permission string) (collection.Scope, error) {
	id, ok := authidentity.AuthenticatedIdentityFromContext(ctx)
	if !ok || id.EffectiveOrganizationID != id.TenantID {
		return collection.Scope{}, collection.ErrForbidden
	}
	scope := collection.Scope{OrganizationID: id.TenantID, ActorID: id.UserID, MemberID: id.EffectiveMemberID}
	if permission != collection.PermissionManage || scope.Validate() != nil {
		return collection.Scope{}, collection.ErrForbidden
	}
	if err := a.current.AuthorizeExecution(ctx, scope, permission); err != nil {
		return collection.Scope{}, err
	}
	return scope, nil
}

func WithFullImageSet(db *gorm.DB, workflows imageagent.WorkflowClient, d FullImageSetDependencies) CurrentApplicationOption {
	return func(o *currentApplicationOptions) {
		o.imageAgents++
		o.imageAgentDB = db
		o.imageAgentWorkflows = workflows
		copy := d
		o.fullImages = &copy
	}
}

// Source evidence uses its own live permission. Image write is separately
// checked by the route and by the generation worker before every dispatch.
type imageSourceScopeAuthority struct {
	current supplyapp.OrganizationExecutionAuthorizer
}

func (a imageSourceScopeAuthority) AuthorizeImageExecution(ctx context.Context, scope collection.Scope) error {
	return a.current.AuthorizeImageSource(ctx, scope)
}

// The caller owns the API/worker, configuration, source, resource and Asset
// pools. This assembly installs no schema and starts no provider or worker.
func buildFullImageApplication(ctx context.Context, productDB, imageDB, workerDB, configurationDB, resourceDB, assetDB *gorm.DB, workflows imageagent.WorkflowClient, products imageapp.EffectiveImageProductReader, live supplyapp.OrganizationExecutionAuthorizer, supply *supplyChainModule, current, workerConfig *config.Config, logger *logrus.Logger, mediaReaders ...asset.ManualImageReader) (*fullImageApplication, appruntime.ImageAgentTemporalDependencies, error) {
	var empty appruntime.ImageAgentTemporalDependencies
	if len(mediaReaders) > 1 {
		return nil, empty, imageagent.ErrValidation
	}
	var manual asset.ManualImageReader
	if ctx == nil || productDB == nil || imageDB == nil || workerDB == nil || workerDB == imageDB || configurationDB == nil || resourceDB == nil || assetDB == nil || assetDB == imageDB || assetDB == workerDB || workflows == nil || products.Snapshots == nil || products.Applied == nil || live.Permissions == nil || current == nil || workerConfig == nil || !current.ImageAgent.Generation.Configured() || workerConfig.ImageAgent.Generation != current.ImageAgent.Generation || !reflect.DeepEqual(workerConfig.ImageAgent.Admission, current.ImageAgent.Admission) || workerConfig.ImageAgent.ArtifactStore.PublicBase != current.ImageAgent.ArtifactStore.PublicBase || workerConfig.ImageAgent.ArtifactStore.S3.Bucket != current.ImageAgent.ArtifactStore.S3.Bucket {
		return nil, empty, imageagent.ErrCommandBlocked
	}
	if current.ImageAgent.Generation.PointsPerImage > math.MaxInt64/32 {
		return nil, empty, imageagent.ErrCommandBlocked
	}
	startup, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	if err := imagestore.VerifyOrganizationRuntimePermissions(startup, imageDB); err != nil {
		return nil, empty, err
	}
	if err := imagestore.VerifyOrganizationWorkerRuntimePermissions(startup, workerDB); err != nil {
		return nil, empty, err
	}
	if err := assetstore.VerifySourceRuntimePermissions(startup, assetDB); err != nil {
		return nil, empty, err
	}
	if err := configstore.VerifySchema(startup, configurationDB); err != nil {
		return nil, empty, err
	}
	configuration, err := configstore.New(configurationDB)
	if err != nil {
		return nil, empty, err
	}
	if len(mediaReaders) == 1 {
		manual = mediaReaders[0]
	}
	sourceLive := imageSourceScopeAuthority{current: live}
	imageAuth := newFullImageWorkerAuthorizer(live)
	sourceAuth := imageapp.ScopedImageAuthorizer{Live: sourceLive}
	receipts, err := newImageSetAcquisitionReceipts(startup, productDB, imageapp.ImagePublicationScopeAuthorizer{Live: sourceLive})
	if err != nil {
		return nil, empty, err
	}
	sources := imageapp.ImageSetSourceRouter{Acquisition: imageapp.AcquisitionImageSetSources{Receipts: receipts, Authorization: sourceAuth, Products: products}}
	if supply != nil {
		if supply.app == nil {
			return nil, empty, imageagent.ErrCommandBlocked
		}
		sources.Supply = supplyapp.ImageSetSources{ExecutionSources: supply.app.Sources, ExecutionAuthorization: live, Products: supply.app.Products}
	}
	readSources := sources
	sourceSelections := imageapp.ImageSetSourceSelectionReader{Sources: sources}
	rules := supplyapp.ImageSetTargetRules{}
	if supply != nil {
		readSources.Supply = supplyapp.ImageSetSources{Permission: preparation.PermissionRead, ExecutionSources: supply.app.Sources, ExecutionAuthorization: live, Products: supply.app.Products}
		rules = supplyapp.ImageSetTargetRules{Records: supply.app.Records, Sources: sourceSelections, Rules: supply.app.Rules}
	}
	contexts, err := imageapp.NewImageSetContextReader(sources, rules, newImageSetSourceByteReader(), productimage.MaxInlineArtifactBytes)
	if err != nil {
		return nil, empty, err
	}
	quotes, err := imageworker.NewImageGenerationQuoteReader(imageDB, current.ImageAgent.Generation)
	if err != nil {
		return nil, empty, err
	}
	publicURLs := imageAgentDurableAssetPublicURLResolver(current)
	if publicURLs == nil {
		return nil, empty, imageagent.ErrCommandBlocked
	}
	var trial *imageagent.IsolatedTrialGeneratedURLPolicy
	if current.ImageAgent.ArtifactStore.IsolatedTrialGeneratedURLs {
		trial, err = imageagent.NewIsolatedTrialGeneratedURLPolicy(current.ImageAgent.ArtifactStore.PublicBase, current.ImageAgent.ArtifactStore.S3.Bucket)
		if err != nil {
			return nil, empty, err
		}
	}
	apiRepository, workerRepository := imagestore.NewOrganizationRepository(imageDB), imagestore.NewOrganizationRepository(workerDB)
	recent, ok := apiRepository.(imageagent.ImageSetRunReader)
	if !ok {
		return nil, empty, imageagent.ErrCommandBlocked
	}
	assetRepository, err := assetstore.NewRepository(assetDB)
	if err != nil {
		return nil, empty, err
	}
	inventories, ok := assetRepository.(asset.ImageSetInventoryReader)
	if !ok {
		return nil, empty, asset.ErrRepositoryUnavailable
	}
	approvals, err := assetstore.NewBoundedApprovalCommitReader(assetDB, 2<<20)
	if err != nil {
		return nil, empty, err
	}
	generatedMaterials, err := imageworker.NewImageSetMaterialReader(workerConfig, logger)
	if err != nil {
		return nil, empty, err
	}
	newSelections := func(repository imageagent.Repository) (*asset.ImageSetService, error) {
		facts, ok := repository.(imageagent.GenerationFactRepository)
		if !ok {
			return nil, imageagent.ErrCommandBlocked
		}
		materializations, ok := repository.(assetpublication.ImageMaterializationReader)
		if !ok {
			return nil, imageagent.ErrCommandBlocked
		}
		candidates, err := assetpublication.NewImageSetCandidateReader(repository, facts, materializations, publicURLs, trial)
		if err != nil {
			return nil, err
		}
		images := supplyapp.NewPublicImageProbe()
		targets := imageapp.ImageSetMaterialTargetResolver{ReadBytes: newImageSetSourceByteReader(), ReadGeneratedBytes: generatedMaterials.Read, Official: supplyapp.ImageSetAssetTargetResolver{Rules: rules, Images: images}}
		service, err := asset.NewImageSetService(sourceSelections, assetRepository, inventories, approvals, candidates, targets)
		if err != nil {
			return nil, err
		}
		if manual != nil {
			service.WithManualImages(manual)
		}
		return service, nil
	}
	selections, err := newSelections(apiRepository)
	if err != nil {
		return nil, empty, err
	}
	workerSelections, err := newSelections(workerRepository)
	if err != nil {
		return nil, empty, err
	}
	publisher, err := assetpublication.NewImageSetPublisher(workerRepository, workerSelections)
	if err != nil {
		return nil, empty, err
	}
	gate := imageagent.TenantAllowlistStartGate{Enabled: current.ImageAgent.Admission.Enabled, AllowedTenantIDs: append([]string(nil), current.ImageAgent.Admission.AllowedTenantIDs...)}
	service, err := imageagent.NewService(apiRepository, workflows, closedImageSetCatalog{}, imageagent.WithOrganizationScope(), imageagent.WithTenantStartGate(gate), imageagent.WithImageSetDependencies(imageagent.ImageSetDependencies{Configuration: configuration, Contexts: contexts, Quotes: quotes, HardLimits: agentconfig.ImageRunLimits{Images: 32, Points: 32 * current.ImageAgent.Generation.PointsPerImage, ElapsedSeconds: 3600}}))
	if err != nil {
		return nil, empty, err
	}
	dependencies, err := imageworker.NewImageSetTemporalDependencies(workerConfig, workerDB, resourceDB, imageAuth, contexts, publisher, logger)
	if err != nil {
		return nil, empty, err
	}
	return &fullImageApplication{service: service, readSources: readSources, contexts: contexts, quotes: quotes, selections: selections, inventories: inventories, approvals: approvals, publicURLs: publicURLs, trial: trial, configuration: configuration, recent: recent, gate: gate, rules: rules, manualAvailable: manual != nil}, dependencies, nil
}

// Generic Acquisition consumes the existing Product/Review and live IAM ports
// directly; official Store and Supply execution are separate optional consumers.
func buildFullImageProducts(db *gorm.DB, deps routeAuthDependencies, permissions *authz.ListingKitAuthorizer, cfg *config.Config) (imageapp.EffectiveImageProductReader, supplyapp.OrganizationExecutionAuthorizer, error) {
	var products imageapp.EffectiveImageProductReader
	var authorization supplyapp.OrganizationExecutionAuthorizer
	resolver, ok := deps.organizationResolver.(*workbenchcontext.Resolver)
	if !ok || resolver == nil || permissions == nil || cfg == nil || cfg.ListingKit.Zitadel.TenantDirectoryToken == "" {
		return products, authorization, imageagent.ErrCommandBlocked
	}
	reviews, err := buildProductReviewCore(db, resolver, permissions)
	if err != nil {
		return products, authorization, err
	}
	products = imageapp.EffectiveImageProductReader{Snapshots: reviews.reader, Applied: reviews.store}
	authorization = supplyapp.OrganizationExecutionAuthorizer{Client: zitadel.NewAuthorizationClient(cfg.ListingKit.Zitadel.AuthorizationAPIURL, &http.Client{Timeout: 5 * time.Second}), ServiceToken: func(context.Context) (string, error) { return cfg.ListingKit.Zitadel.TenantDirectoryToken, nil }, ProjectID: cfg.ListingKit.Zitadel.ProjectID, Permissions: permissions, OrganizationStatus: resolver.BusinessStatusChecker()}
	return products, authorization, nil
}

func newFullImageWorkerAuthorizer(live supplyapp.OrganizationExecutionAuthorizer) imageagent.ExecutionAuthorizer {
	// Activities validate the persisted Run subject before live authorization,
	// then restore its context. They never inherit a browser request identity.
	return imageworker.OrganizationExecutionAuthorizer{
		Client: live.Client, ServiceToken: live.ServiceToken, ProjectID: live.ProjectID,
		Authorizer: live.Permissions, OrganizationStatus: live.OrganizationStatus,
	}
}

type closedImageSetCatalog struct{}

func (closedImageSetCatalog) Resolve(context.Context, imageagent.AssetCatalogScope) (imageagent.AssetCatalog, error) {
	return imageagent.AssetCatalog{}, imageagent.ErrCommandBlocked
}
