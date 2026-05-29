package httpapi

import (
	"context"
	"strings"

	"github.com/sirupsen/logrus"

	appruntime "task-processor/internal/app/runtime"
	"task-processor/internal/listingkit"
	listingkithttpapi "task-processor/internal/listingkit/httpapi"
	sdsclient "task-processor/internal/sds/client"
	sdsdesign "task-processor/internal/sds/design"
	sdshttpapi "task-processor/internal/sds/httpapi"
	sdstemplate "task-processor/internal/sds/template"
	sdsusecase "task-processor/internal/sds/usecase"
	"task-processor/internal/sheinlogin"
)

func newListingKitRuntimeBuildInput(logger *logrus.Logger, deps *runtimeDeps) listingkithttpapi.RuntimeBuildInput {
	return listingkithttpapi.RuntimeBuildInput{
		Logger: logger,
		Runtime: listingkithttpapi.RuntimeDependencies{
			Config:                     deps.shared.cfg,
			ProductService:             deps.features.productService,
			ImageService:               deps.features.imageService,
			ImageSubjectExtractor:      deps.features.imageSubjectExtractor,
			ImageWhiteBackgroundRender: deps.features.imageWhiteBgRenderer,
			ImageSceneRenderer:         deps.features.imageSceneRenderer,
			AICredentialStore:          deps.shared.aiCredentialStore,
			Support: listingkithttpapi.BuildRuntimeSupport(listingkithttpapi.RuntimeSupportInput{
				SheinCookieStore:          ensureListingKitSheinCookieStore(logger, deps),
				SDSSyncService:            buildSDSSyncService(logger, deps),
				SDSLoginStatusProvider:    deps.features.sdsLoginStatusProvider,
				SDSBaselineRemoteProvider: buildSDSBaselineRemoteProvider(logger, deps),
			}),
			ShouldStartTemporalWorkerInProcess: appruntime.ShouldStartListingKitSheinPublishTemporalWorkerInProcess(),
		},
	}
}

func ensureListingKitSheinCookieStore(logger *logrus.Logger, deps *runtimeDeps) *sheinlogin.RedisStore {
	if deps == nil || deps.shared == nil || deps.shared.cfg == nil {
		return nil
	}
	support := deps.ensureListingKitSupport()
	if support == nil {
		return nil
	}
	if support.sheinCookieStore != nil {
		return support.sheinCookieStore
	}
	redisCfg := deps.shared.cfg.EffectiveSheinCookieRedis()
	if strings.TrimSpace(redisCfg.Host) == "" {
		return nil
	}
	store, err := sheinlogin.NewRedisStore(redisCfg)
	if err != nil {
		if logger != nil {
			logger.WithError(err).Warn("failed to initialize listingkit shein cookie store; shein runtime will degrade")
		}
		return nil
	}
	support.sheinCookieStore = store
	deps.addClosers(store.Close)
	return store
}

func buildSDSSyncService(logger *logrus.Logger, deps *runtimeDeps) sdsusecase.Service {
	if deps == nil || deps.shared == nil || deps.features == nil || deps.features.imageService == nil {
		return nil
	}

	svc, authState, err := newSDSSyncServiceForHTTPAPI(deps.features.imageService, sdshttpapi.BuildClientConfig(deps.shared.cfg))
	if err != nil {
		logger.WithError(err).Warn("failed to initialize SDS client; SDS sync disabled")
		return nil
	}
	if svc == nil {
		logger.Warn("SDS sync service not initialized; SDS sync disabled")
		return nil
	}

	if authState == nil || strings.TrimSpace(authState.AccessToken) == "" {
		logger.Info("SDS auth state not found at startup; keeping SDS sync enabled for request-time auth bootstrap")
	}

	return svc
}

func buildSDSBaselineRemoteProvider(logger *logrus.Logger, deps *runtimeDeps) listingkit.SDSBaselineRemoteProvider {
	if deps == nil || deps.shared == nil {
		return nil
	}
	support := deps.ensureListingKitSupport()
	if support == nil {
		return nil
	}
	if support.sdsBaselineRemoteProvider != nil {
		return support.sdsBaselineRemoteProvider
	}
	client, err := sdsclient.New(sdshttpapi.BuildClientConfig(deps.shared.cfg))
	if err != nil {
		if logger != nil {
			logger.WithError(err).Warn("failed to initialize SDS baseline remote provider; online baseline validation disabled")
		}
		return nil
	}
	support.sdsBaselineRemoteProvider = &listingKitSDSBaselineRemoteProvider{
		design: sdsdesign.NewService(client),
	}
	return support.sdsBaselineRemoteProvider
}

type listingKitSDSBaselineRemoteProvider struct {
	design *sdsdesign.Service
}

func (p *listingKitSDSBaselineRemoteProvider) GetProductDetail(ctx context.Context, parentProductID int64) (*sdstemplate.ProductDetail, error) {
	if p == nil || p.design == nil {
		return nil, nil
	}
	return p.design.GetProductDetail(ctx, parentProductID)
}

func (p *listingKitSDSBaselineRemoteProvider) GetDesignProduct(ctx context.Context, variantID int64) (*sdsdesign.DesignProductPage, error) {
	if p == nil || p.design == nil {
		return nil, nil
	}
	return p.design.GetDesignProduct(ctx, variantID)
}

func (p *listingKitSDSBaselineRemoteProvider) GetPrototypeGroups(ctx context.Context, parentProductID int64) ([]sdsdesign.PrototypeGroup, error) {
	if p == nil || p.design == nil {
		return nil, nil
	}
	return p.design.GetPrototypeGroups(ctx, parentProductID)
}
