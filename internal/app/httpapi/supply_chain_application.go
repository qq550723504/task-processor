package httpapi

import (
	"context"
	"errors"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
	"gorm.io/gorm"
	"net/http"
	storeapp "task-processor/internal/app/storecenter"
	supplyapp "task-processor/internal/app/supplychain"
	"task-processor/internal/authruntime/zitadel"
	"task-processor/internal/authz"
	"task-processor/internal/core/config"
	"task-processor/internal/httproute"
	prepstore "task-processor/internal/integration/persistence/listing/preparation"
	recordstore "task-processor/internal/integration/persistence/listing/record"
	submissionstore "task-processor/internal/integration/persistence/listing/submission"
	assetstore "task-processor/internal/integration/persistence/product/asset"
	catalogstore "task-processor/internal/integration/persistence/product/catalog"
	kernelmodule "task-processor/internal/kernel/module"
	"task-processor/internal/listing/preparation"
	record "task-processor/internal/listing/record/target"
	"task-processor/internal/listing/submission"
	"task-processor/internal/product/asset"
	"task-processor/internal/product/collection"
	"task-processor/internal/product/sourcing"
	"task-processor/internal/storecenter"
	"task-processor/internal/workbenchcontext"
	"time"
)

type SupplyChainDependencies struct {
	AssetDB  *gorm.DB
	Workflow client.Client
	Worker   *worker.Worker
}

func WithSupplyChain(d SupplyChainDependencies) CurrentApplicationOption {
	return func(o *currentApplicationOptions) { o.supplyChains++; o.supplyChain = &d }
}

type supplyChainModule struct {
	app    *supplyapp.Application
	worker worker.Worker
	routes []httproute.Descriptor
}

func (s supplyChainModule) Name() string                  { return "supply-chain" }
func (s supplyChainModule) Enabled(c *config.Config) bool { return c != nil && c.Workbench.Enabled }
func (s supplyChainModule) Register(r *kernelmodule.Registry) error {
	r.AddRoutes(s.routes...)
	return nil
}

func buildSupplyChainModule(ctx context.Context, productDB, storeDB *gorm.DB, d SupplyChainDependencies, deps routeAuthDependencies, permissions *authz.ListingKitAuthorizer, apps *storeapp.OfficialApplicationRegistry, cfg *config.Config, productAgent *productAgentApplication) (supplyChainModule, error) {
	var empty supplyChainModule
	resolver, ok := deps.organizationResolver.(*workbenchcontext.Resolver)
	if !ok || resolver == nil || permissions == nil || d.AssetDB == nil || d.Workflow == nil || d.Worker == nil || apps == nil || productDB == nil || storeDB == nil || cfg == nil || cfg.ListingKit.Zitadel.TenantDirectoryToken == "" {
		return empty, preparation.ErrUnavailable
	}
	if err := assetstore.VerifySourceRuntimePermissions(ctx, d.AssetDB); err != nil {
		return empty, err
	}
	if err := storecenter.VerifyCurrentSchema(ctx, storeDB); err != nil {
		return empty, err
	}
	if err := storecenter.VerifyRuntimePermissions(ctx, storeDB); err != nil {
		return empty, err
	}
	collections, err := buildProductCollectionService(ctx, productDB, deps, permissions, true)
	if err != nil {
		return empty, err
	}
	live := &productReviewLiveOrganizationAccess{resolver: resolver, now: time.Now}
	auth, err := preparation.NewContextAuthorizer(live, permissions)
	if err != nil {
		return empty, err
	}
	executionAuth := supplyapp.OrganizationExecutionAuthorizer{Client: zitadel.NewAuthorizationClient(cfg.ListingKit.Zitadel.AuthorizationAPIURL, &http.Client{Timeout: 5 * time.Second}), ServiceToken: func(context.Context) (string, error) { return cfg.ListingKit.Zitadel.TenantDirectoryToken, nil }, ProjectID: cfg.ListingKit.Zitadel.ProjectID, Permissions: permissions, OrganizationStatus: resolver.BusinessStatusChecker()}
	repository, err := prepstore.NewRepository(ctx, productDB)
	if err != nil {
		return empty, err
	}
	preparations, err := preparation.NewService(repository, collections, auth)
	if err != nil {
		return empty, err
	}
	snapshots, err := catalogstore.NewBoundedSnapshotReader(productDB, sourcing.MaxEncodedSnapshotBytes)
	if err != nil {
		return empty, err
	}
	sources, err := preparation.NewSourceSelector(preparations, collections, repository, snapshots)
	if err != nil {
		return empty, err
	}
	sources, err = sources.WithExecution(executionAuth, collection.ExecutionOwnerAuthority{Authorization: executionAuth})
	if err != nil {
		return empty, err
	}
	operationsRepo, err := prepstore.NewOperationRepository(ctx, productDB)
	if err != nil {
		return empty, err
	}
	operations, err := preparation.NewOperationService(preparations, collections, operationsRepo)
	if err != nil {
		return empty, err
	}
	operations, err = operations.WithExecution(executionAuth)
	if err != nil {
		return empty, err
	}
	reviews, err := buildProductReviewCore(productDB, resolver, permissions)
	if err != nil {
		return empty, err
	}
	effective := supplyapp.EffectiveProductReader{Reviews: reviews.store, Snapshots: snapshots}
	storeRepo, err := storecenter.NewMemberScopedStoreRepository(storeDB, currentStoreMemberAuthorizer{authorizer: permissions})
	if err != nil {
		return empty, err
	}
	access, err := storeapp.NewOfficialProductAccess(storeRepo, executionAuth, apps)
	if err != nil {
		return empty, err
	}
	rules := supplyapp.RuleReader{Stores: supplyapp.OfficialRuleStore{Access: access}}
	assets, err := assetstore.NewBoundedApprovedInventoryReader(d.AssetDB, record.MaxPayloadBytes)
	if err != nil {
		return empty, err
	}
	assetRepo, err := assetstore.NewRepository(d.AssetDB)
	if err != nil {
		return empty, err
	}
	approvalsReader, err := assetstore.NewBoundedApprovalCommitReader(d.AssetDB, record.MaxPayloadBytes)
	if err != nil {
		return empty, err
	}
	selections := supplyapp.SourceImageReader{Sources: sources, Products: effective, Authorization: auth}
	approvals, err := asset.NewSourceApprovalService(selections, assetRepo, approvalsReader)
	if err != nil {
		return empty, err
	}
	records, err := recordstore.NewRepository(ctx, productDB)
	if err != nil {
		return empty, err
	}
	targets, err := record.NewTargetService(record.TargetDependencies{Sources: sources, Products: effective, Assets: assets, Rules: rules, Records: records, Images: supplyapp.NewPublicImageProbe(), Authorizer: auth, ExecutionSources: sources, ExecutionAuthorization: executionAuth})
	if err != nil {
		return empty, err
	}
	official, err := submissionstore.NewOfficialRepository(ctx, productDB)
	if err != nil {
		return empty, err
	}
	effects, err := submissionstore.NewOfficialEffectsRepository(productDB)
	if err != nil {
		return empty, err
	}
	kernel, err := submission.NewExecutionKernel(effects)
	if err != nil {
		return empty, err
	}
	stageProjection := supplyapp.ReviewProjection{Facts: repository, Reviews: reviews.store}
	uploader, err := supplyapp.NewUploadService(supplyapp.UploadDependencies{Sources: sources, Products: effective, Assets: assets, Rules: rules, Records: records, Authorization: executionAuth, Stores: supplyapp.OfficialExecutionStore{Access: access}, Images: supplyapp.NewPublicImageProbe(), Kernel: kernel, Intents: official, Receipts: official, ReviewGate: stageProjection.RequireUploadReady})
	if err != nil {
		return empty, err
	}
	app := &supplyapp.Application{Preparations: preparations, Sources: sources, Operations: operations, Execution: supplyapp.OperationApplication{Service: operations, Repository: operationsRepo, Starter: supplyapp.TemporalOperationStarter{Client: d.Workflow}}, Targets: targets, Records: records, Products: effective, Rules: rules, Assets: assets, Approvals: approvals, Authorization: auth, PublicationReceipts: official, PublicationStores: supplyapp.OfficialRuleStore{Access: access}}
	var optimizer supplyapp.OperationOptimizer
	app.StageProjection = stageProjection
	if productAgent != nil {
		bridge, e := connectSupplyProductAgent(ctx, productAgent.config.ReviewDB, app, productAgent, executionAuth)
		if e != nil {
			return empty, e
		}
		optimizer = bridge
		app.AuthorizeOptimization = bridge.authorizeRequest
		app.OptimizationOptions = bridge.options
	}
	currentWorker, err := supplyapp.NewSupplyWorker(d.Workflow, &supplyapp.OperationActivities{Operations: operations, Repository: operationsRepo, Sources: sources, Products: effective, Targets: records, Creator: targets, Uploader: uploader, Optimizer: optimizer})
	if err != nil {
		return empty, err
	}
	binder := productReviewCapabilityBinder{now: time.Now}
	return supplyChainModule{app: app, worker: currentWorker, routes: supplyapp.SupplyRoutes(app, binder.Bind)}, nil
}
func validateSupplyDescriptor(route httproute.Descriptor) error {
	for _, expected := range supplyapp.SupplyRoutes(nil, nil) {
		if route.Method == expected.Method && route.Path == expected.Path {
			if route.Module != expected.Module || route.Permission != expected.Permission || route.AuthPolicy != expected.AuthPolicy || route.OrganizationAccessPolicy != expected.OrganizationAccessPolicy || route.OrganizationTargetResolver != nil || route.RequestTimeout != expected.RequestTimeout || route.RejectUnreadRequestBody != expected.RejectUnreadRequestBody || route.Handler == nil {
				return errors.New("supply route loses live private source boundary")
			}
			return nil
		}
	}
	return errors.New("supply route not admitted")
}
