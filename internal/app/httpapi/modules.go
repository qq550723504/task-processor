package httpapi

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"

	amazonlistinghttpapi "task-processor/internal/amazonlisting/httpapi"
	"task-processor/internal/core/config"
	"task-processor/internal/infra/worker"
	listingkithttpapi "task-processor/internal/listingkit/httpapi"
	"task-processor/internal/productenrich"
	productenrichhttpapi "task-processor/internal/productenrich/httpapi"
	productimage "task-processor/internal/productimage"
	productimagehttpapi "task-processor/internal/productimage/httpapi"
	promptmgmtapi "task-processor/internal/promptmgmt/api"
	sdsclient "task-processor/internal/sds/client"
	sdshttpapi "task-processor/internal/sds/httpapi"
	sdsusecase "task-processor/internal/sds/usecase"
	"task-processor/internal/sdslogin"
	sheinclient "task-processor/internal/shein/client"
	"task-processor/internal/sheinlogin"
	sheinloginmanaged "task-processor/internal/sheinloginmanaged"
	"task-processor/internal/taskrpcapi"
)

var newSDSSyncServiceForHTTPAPI = func(imageSvc productimage.Service, cfg *sdsclient.Config) (sdsusecase.Service, *sdsclient.AuthState, error) {
	if cfg == nil {
		cfg = sdsclient.DefaultConfig()
	}
	sdsHTTPClient, err := sdsclient.New(cfg)
	if err != nil {
		return nil, nil, err
	}

	authState := sdsHTTPClient.AuthState()
	svc, err := sdsusecase.NewService(sdsusecase.Config{
		SDSClient:    sdsHTTPClient,
		ImageService: imageSvc,
	})
	if err != nil {
		return nil, authState, err
	}

	return svc, authState, nil
}

func buildBootstrap(logger *logrus.Logger, options Options) (*appBootstrap, error) {
	deps, err := buildRuntimeDeps(logger, options.ConfigPath)
	if err != nil {
		return nil, err
	}
	configureSheinLoginService(deps.cfg)

	productModule, err := buildProductModule(logger, deps)
	if err != nil {
		return nil, err
	}

	imageModule, err := buildImageModule(logger, deps)
	if err != nil {
		return nil, err
	}

	amazonListingModule, err := buildAmazonListingModule(logger, deps)
	if err != nil {
		return nil, err
	}

	sheinLoginHandler, sheinLoginCloser, err := buildSheinLoginModule(deps)
	if err != nil {
		return nil, err
	}
	if sheinLoginCloser != nil {
		deps.closers = append(deps.closers, sheinLoginCloser)
	}
	sdsLoginHandler, sdsLoginCloser, err := buildSDSLoginModule(deps)
	if err != nil {
		return nil, err
	}
	if sdsLoginCloser != nil {
		deps.closers = append(deps.closers, sdsLoginCloser)
	}
	listingKitModule, err := buildListingKitModule(logger, deps)
	if err != nil {
		return nil, err
	}
	promptTemplateHandler := promptmgmtapi.BuildHandler(deps.tenantPromptStore)

	localTaskHealthProvider := buildLocalTaskHealthProvider(map[string]worker.WorkerPool{
		"product_enrich": productModule.Pool,
		"product_image":  imageModule.Pool,
		"amazon_listing": amazonListingModule.Pool,
		"listing_kit":    listingKitModule.Pool,
	})

	var taskRPCHandler taskrpcapi.Handler
	taskRPCHandler, err = taskrpcapi.BuildHandler(deps.managementClient(), localTaskHealthProvider)
	if err != nil {
		return nil, err
	}

	sdsCatalogHandler := sdshttpapi.BuildCatalogHandler(logger, deps.cfg)

	handlers := httpModuleHandlers{
		product:        productModule.Handler,
		image:          imageModule.Handler,
		amazonListing:  amazonListingModule.Handler,
		listingKit:     listingKitModule.Handler,
		promptTemplate: promptTemplateHandler,
		studioSession:  listingKitModule.StudioSessionHandler,
		sheinLogin:     sheinLoginHandler,
		sdsLogin:       sdsLoginHandler,
		taskRPC:        taskRPCHandler,
		sdsCatalog:     sdsCatalogHandler,
	}
	server, routes, err := buildHTTPServerBundleFromHandlers(options.Port, deps.cfg, handlers)
	if err != nil {
		return nil, err
	}
	return &appBootstrap{
		productHandler:        productModule.Handler,
		imageHandler:          imageModule.Handler,
		amazonListingHandler:  amazonListingModule.Handler,
		listingKitHandler:     listingKitModule.Handler,
		promptTemplateHandler: promptTemplateHandler,
		studioSessionHandler:  listingKitModule.StudioSessionHandler,
		sdsCatalogHandler:     sdsCatalogHandler,
		sheinLoginHandler:     sheinLoginHandler,
		sdsLoginHandler:       sdsLoginHandler,
		taskRPCHandler:        taskRPCHandler,
		server:                server,
		routes:                routes,
		pools:                 []worker.WorkerPool{productModule.Pool, imageModule.Pool, amazonListingModule.Pool, listingKitModule.Pool},
		closers:               deps.closers,
	}, nil
}

func buildSheinLoginModule(deps *runtimeDeps) (sheinLoginRouteHandler, func() error, error) {
	if deps == nil || deps.cfg == nil || deps.managementClient() == nil {
		return nil, nil, nil
	}
	redisCfg := deps.cfg.EffectiveSheinCookieRedis()
	if strings.TrimSpace(redisCfg.Host) == "" {
		return nil, nil, nil
	}
	provider, repoCloser, err := buildSheinLoginAccountProvider(deps)
	if err != nil {
		return nil, nil, err
	}
	svc, err := sheinlogin.NewService(deps.cfg.Platforms.Shein.LoginService, redisCfg, deps.cfg.Browser, provider)
	if err != nil {
		if repoCloser != nil {
			_ = repoCloser()
		}
		return nil, nil, err
	}
	svc.ConfigureRuntimeSheinAPIClients()
	svc.ConfigureStoreSyncClientFactory(sheinloginmanaged.NewStoreSyncClientFactory(deps.managementClient()))
	svc.ConfigureDuplicateStoreLookup(sheinloginmanaged.NewDuplicateStoreLookup(deps.managementClient()))
	sheinclient.ConfigureLocalLoginRefresher(svc)
	return sheinlogin.NewHandler(svc), func() error {
		closeErr := svc.Close()
		if repoCloser != nil {
			if err := repoCloser(); err != nil && closeErr == nil {
				closeErr = err
			}
		}
		return closeErr
	}, nil
}

func buildSheinLoginAccountProvider(deps *runtimeDeps) (sheinlogin.AccountProvider, func() error, error) {
	if deps == nil || deps.cfg == nil {
		return nil, nil, fmt.Errorf("runtime deps are incomplete")
	}
	localLogger := logrus.New()
	repo, closers, err := listingkithttpapi.BuildListingAdminStoreRepository(deps.cfg, localLogger)
	if err == nil && repo != nil {
		return sheinlogin.NewListingAdminAccountProvider(repo), joinClosers(closers), nil
	}
	fallback := sheinloginmanaged.NewAccountProvider(deps.managementClient())
	provider := &retryingSheinLoginAccountProvider{
		cfg:      deps.cfg,
		logger:   localLogger,
		fallback: fallback,
		lastErr:  err,
	}
	if err != nil {
		localLogger.WithError(err).Warn("shein login local store repository unavailable at startup; will retry locally and temporarily fall back to management provider")
	}
	return provider, provider.Close, nil
}

type retryingSheinLoginAccountProvider struct {
	cfg      *config.Config
	logger   *logrus.Logger
	fallback sheinlogin.AccountProvider

	mu         sync.Mutex
	local      sheinlogin.AccountProvider
	localClose func() error
	lastErr    error
}

func (p *retryingSheinLoginAccountProvider) ListAccounts(ctx context.Context, tenantID int64) ([]sheinlogin.Account, error) {
	if tenantID <= 0 {
		return nil, fmt.Errorf("tenant id is required")
	}
	if local := p.ensureLocalProvider(); local != nil {
		return local.ListAccounts(ctx, tenantID)
	}
	if p.fallback == nil {
		if p.lastErr != nil {
			return nil, p.lastErr
		}
		return nil, fmt.Errorf("shein login account provider is unavailable")
	}
	return p.fallback.ListAccounts(ctx, tenantID)
}

func (p *retryingSheinLoginAccountProvider) GetAccount(ctx context.Context, tenantID int64, storeID int64) (*sheinlogin.Account, error) {
	if tenantID <= 0 {
		return nil, fmt.Errorf("tenant id is required")
	}
	if local := p.ensureLocalProvider(); local != nil {
		return local.GetAccount(ctx, tenantID, storeID)
	}
	if p.fallback == nil {
		if p.lastErr != nil {
			return nil, p.lastErr
		}
		return nil, fmt.Errorf("shein login account provider is unavailable")
	}
	return p.fallback.GetAccount(ctx, tenantID, storeID)
}

func (p *retryingSheinLoginAccountProvider) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.localClose == nil {
		return nil
	}
	closeFn := p.localClose
	p.localClose = nil
	p.local = nil
	return closeFn()
}

func (p *retryingSheinLoginAccountProvider) ensureLocalProvider() sheinlogin.AccountProvider {
	if p == nil || p.cfg == nil {
		return nil
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.local != nil {
		return p.local
	}
	repo, closers, err := listingkithttpapi.BuildListingAdminStoreRepository(p.cfg, p.logger)
	if err != nil || repo == nil {
		p.lastErr = err
		return nil
	}
	p.local = sheinlogin.NewListingAdminAccountProvider(repo)
	p.localClose = joinClosers(closers)
	p.lastErr = nil
	if p.logger != nil {
		p.logger.Info("shein login account provider switched to local listing admin repository")
	}
	return p.local
}

func joinClosers(closers []func() error) func() error {
	if len(closers) == 0 {
		return nil
	}
	return func() error {
		var closeErr error
		for i := len(closers) - 1; i >= 0; i-- {
			if closers[i] == nil {
				continue
			}
			if err := closers[i](); err != nil && closeErr == nil {
				closeErr = err
			}
		}
		return closeErr
	}
}

func buildSDSLoginModule(deps *runtimeDeps) (sdsLoginRouteHandler, func() error, error) {
	if deps == nil {
		return nil, nil, nil
	}
	result, err := sdslogin.BuildHandler(deps.cfg)
	if err != nil {
		return nil, nil, err
	}
	if result == nil || result.Service == nil {
		return nil, nil, nil
	}
	deps.sdsLoginStatusProvider = result.Service
	sdsclient.ConfigureLocalLoginProvider(result.Service)
	return result.Handler, nil, nil
}

func configureSheinLoginService(cfg *config.Config) {
	if cfg == nil {
		return
	}
	loginService := cfg.Platforms.Shein.LoginService
	identifier := strings.TrimSpace(loginService.Identifier)
	sheinclient.ConfigureLoginAccount("", identifier)
}

func BuildHandlers(logger *logrus.Logger, options Options) (productenrich.ProductHandler, productimage.Handler, []worker.WorkerPool, []func() error, error) {
	bootstrap, err := buildBootstrap(logger, options)
	if err != nil {
		return nil, nil, nil, nil, err
	}

	return bootstrap.productHandler, bootstrap.imageHandler, bootstrap.pools, bootstrap.closers, nil
}

func newWorkerPool(processor worker.Processor, cfg *config.Config) worker.WorkerPool {
	return worker.NewPoolWithConfig(processor, worker.PoolConfig{
		Concurrency:     cfg.Worker.Concurrency,
		BufferSize:      cfg.Worker.BufferSize,
		TaskTimeout:     15 * time.Minute,
		EnableMetrics:   true,
		ShutdownTimeout: 30 * time.Second,
	})
}

func buildProductModule(logger *logrus.Logger, deps *runtimeDeps) (*productenrichhttpapi.Module, error) {
	module, err := productenrichhttpapi.BuildModule(productenrichhttpapi.BuildModuleInput{
		Config:        deps.cfg,
		Logger:        logger,
		LLMManager:    deps.llmMgr,
		InputParser:   deps.inputParser,
		Understanding: deps.understanding,
	})
	if err != nil {
		return nil, err
	}

	deps.closers = append(deps.closers, module.Closers...)
	deps.productService = module.Service

	return module, nil
}

func buildImageModule(logger *logrus.Logger, deps *runtimeDeps) (*productimagehttpapi.Module, error) {
	module, err := productimagehttpapi.BuildModule(productimagehttpapi.BuildModuleInput{
		Config:        deps.cfg,
		Logger:        logger,
		LLMManager:    deps.llmMgr,
		OpenAIManager: deps.openaiMgr,
		InputParser:   deps.inputParser,
		Understanding: deps.understanding,
		ImageWorkDir:  deps.imageWorkDir,
	})
	if err != nil {
		return nil, err
	}

	deps.closers = append(deps.closers, module.Closers...)
	deps.imageService = module.Service
	deps.imageSubjectExtractor = module.SubjectExtractor
	deps.imageWhiteBgRenderer = module.WhiteBackgroundRender
	deps.imageSceneRenderer = module.SceneRenderer

	return module, nil
}

func buildAmazonListingModule(logger *logrus.Logger, deps *runtimeDeps) (*amazonlistinghttpapi.Module, error) {
	module, err := amazonlistinghttpapi.BuildModule(amazonlistinghttpapi.BuildModuleInput{
		Config:         deps.cfg,
		Logger:         logger,
		ProductService: deps.productService,
		ImageService:   deps.imageService,
	})
	if err != nil {
		return nil, err
	}

	deps.closers = append(deps.closers, module.Closers...)
	return module, nil
}

func buildListingKitModule(logger *logrus.Logger, deps *runtimeDeps) (*listingkithttpapi.Module, error) {
	module, err := listingkithttpapi.BuildModule(newListingKitBuildModuleInput(logger, deps))
	if err != nil {
		return nil, err
	}
	deps.closers = append(deps.closers, module.Closers...)
	deps.sdsSyncService = buildSDSSyncService(logger, deps)
	return module, nil
}

func buildLocalTaskHealthProvider(pools map[string]worker.WorkerPool) taskrpcapi.LocalStatusProvider {
	return func() map[string]any {
		summary := map[string]any{
			"poolCount":           0,
			"totalQueueSize":      0,
			"totalBufferSize":     0,
			"totalAvailableSlots": 0,
			"totalSubmitted":      int64(0),
			"totalProcessed":      int64(0),
			"totalSucceeded":      int64(0),
			"totalFailed":         int64(0),
			"totalPanicked":       int64(0),
			"queueFullCount":      int64(0),
		}
		poolSnapshots := make(map[string]any, len(pools))

		for name, pool := range pools {
			if pool == nil {
				continue
			}

			queueStats := pool.GetQueueStats()
			poolSnapshot := map[string]any{
				"queueSize":      queueStats.QueueSize,
				"bufferSize":     queueStats.BufferSize,
				"availableSlots": queueStats.AvailableSlots,
				"usagePercent":   queueStats.UsagePercent,
			}

			summary["poolCount"] = summary["poolCount"].(int) + 1
			summary["totalQueueSize"] = summary["totalQueueSize"].(int) + queueStats.QueueSize
			summary["totalBufferSize"] = summary["totalBufferSize"].(int) + queueStats.BufferSize
			summary["totalAvailableSlots"] = summary["totalAvailableSlots"].(int) + queueStats.AvailableSlots

			if metrics := pool.GetMetrics(); metrics != nil {
				snapshot := metrics.GetSnapshot()
				poolSnapshot["metrics"] = map[string]any{
					"totalSubmitted": snapshot.TotalSubmitted,
					"totalProcessed": snapshot.TotalProcessed,
					"totalSucceeded": snapshot.TotalSucceeded,
					"totalFailed":    snapshot.TotalFailed,
					"totalPanicked":  snapshot.TotalPanicked,
					"queueFullCount": snapshot.QueueFullCount,
					"successRate":    snapshot.SuccessRate(),
					"failureRate":    snapshot.FailureRate(),
					"panicRate":      snapshot.PanicRate(),
					"uptimeSeconds":  int64(snapshot.Uptime.Seconds()),
				}
				summary["totalSubmitted"] = summary["totalSubmitted"].(int64) + snapshot.TotalSubmitted
				summary["totalProcessed"] = summary["totalProcessed"].(int64) + snapshot.TotalProcessed
				summary["totalSucceeded"] = summary["totalSucceeded"].(int64) + snapshot.TotalSucceeded
				summary["totalFailed"] = summary["totalFailed"].(int64) + snapshot.TotalFailed
				summary["totalPanicked"] = summary["totalPanicked"].(int64) + snapshot.TotalPanicked
				summary["queueFullCount"] = summary["queueFullCount"].(int64) + snapshot.QueueFullCount
			}

			poolSnapshots[name] = poolSnapshot
		}

		return map[string]any{
			"summary": summary,
			"pools":   poolSnapshots,
		}
	}
}
