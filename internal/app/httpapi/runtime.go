package httpapi

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/sirupsen/logrus"

	amazonlistinghttpapi "task-processor/internal/amazonlisting/httpapi"
	appbootstrap "task-processor/internal/app/bootstrap"
	"task-processor/internal/core/config"
	"task-processor/internal/infra/clients/management"
	openaiclient "task-processor/internal/infra/clients/openai"
	listingkithttpapi "task-processor/internal/listingkit/httpapi"
	"task-processor/internal/productenrich"
	productenrichenrich "task-processor/internal/productenrich/enrich"
	productenrichhttpapi "task-processor/internal/productenrich/httpapi"
	productimagehttpapi "task-processor/internal/productimage/httpapi"
	"task-processor/internal/prompt"
	sdsloginbootstrap "task-processor/internal/sdslogin/bootstrap"
)

func buildRuntimeDeps(logger *logrus.Logger, configPath string) (*runtimeDeps, error) {
	cfg, err := config.LoadConfigFromFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}

	imageWorkDir := resolveImageWorkDir(cfg)
	promptsDir := cfg.Prompts.Dir
	if promptsDir == "" {
		promptsDir = "./prompts"
	}
	if err := prompt.InitGlobal(context.Background(), promptsDir, cfg.Prompts.HotReload, logger.WithField("component", "prompt")); err != nil {
		logger.Warnf("prompt registry initialization failed, fallback prompts will be used: %v", err)
	}

	openaiMgr, err := newOpenAIManager(cfg.OpenAI)
	if err != nil {
		return nil, fmt.Errorf("create OpenAI manager: %w", err)
	}
	closers := make([]func() error, 0)
	var aiCredentialStore *openaiclient.GormCredentialResolver
	var tenantPromptStore prompt.TenantPromptStore
	if cfg.Database != nil {
		var closer func() error
		tenantPromptStore, closer, err = newDBTenantPromptStore(cfg.Database, logger)
		if err != nil {
			return nil, fmt.Errorf("create tenant prompt store: %w", err)
		}
		if err := prompt.SetTenantPromptStore(tenantPromptStore); err != nil {
			return nil, fmt.Errorf("attach tenant prompt store: %w", err)
		}
		if closer != nil {
			closers = append(closers, closer)
		}

		credentialResolver, closer, err := newDBOpenAICredentialResolver(cfg.Database, logger)
		if err != nil {
			return nil, fmt.Errorf("create OpenAI credential resolver: %w", err)
		}
		aiCredentialStore = credentialResolver
		openaiMgr.SetConfigResolver(credentialResolver)
		if closer != nil {
			closers = append(closers, closer)
		}
	}
	llmMgr, err := productenrich.NewLLMManagerAdapterFromManager(openaiMgr)
	if err != nil {
		return nil, fmt.Errorf("create LLM manager: %w", err)
	}
	if cfg.Debug.ProductEnrichMockLLM {
		logger.WithField("config", "debug.productEnrichMockLLM").Warn("productenrich mock LLM enabled for local runtime")
		llmMgr = productenrich.NewLocalMockLLMManager()
	}
	if err := productenrich.ValidateMockLLMManager(llmMgr); err != nil {
		return nil, fmt.Errorf("validate LLM manager: %w", err)
	}

	productUnderstanding, err := productenrichenrich.NewProductUnderstanding(llmMgr)
	if err != nil {
		return nil, fmt.Errorf("create product understanding: %w", err)
	}

	webScraper := newWebScraper(cfg)
	inputParser, err := productenrichenrich.NewInputParser(logger, &productenrich.InputParserConfig{}, webScraper)
	if err != nil {
		return nil, fmt.Errorf("create input parser: %w", err)
	}

	shared, err := appbootstrap.BuildSharedResources(cfg, logger, appbootstrap.SharedResourceOptions{
		AllowMissingManagementAuth: true,
		SkipManagementAuth:         true,
	})
	if err != nil {
		return nil, fmt.Errorf("build shared resources: %w", err)
	}

	return &runtimeDeps{
		cfg:               cfg,
		closers:           closers,
		openaiMgr:         openaiMgr,
		aiCredentialStore: aiCredentialStore,
		tenantPromptStore: tenantPromptStore,
		llmMgr:            llmMgr,
		inputParser:       inputParser,
		understanding:     productUnderstanding,
		imageWorkDir:      imageWorkDir,
		shared:            shared,
	}, nil
}

func (d *runtimeDeps) managementClient() *management.ClientManager {
	if d == nil {
		return nil
	}
	if d.shared == nil {
		return nil
	}
	return d.shared.ManagementClient
}

func (d *runtimeDeps) ensureListingKitSupport() *listingKitSupport {
	if d == nil {
		return nil
	}
	if d.listingKitSupport == nil {
		d.listingKitSupport = &listingKitSupport{}
	}
	return d.listingKitSupport
}

func (d *runtimeDeps) addClosers(closers ...func() error) {
	if d == nil {
		return
	}
	for _, closer := range closers {
		if closer == nil {
			continue
		}
		d.closers = append(d.closers, closer)
	}
}

func (d *runtimeDeps) attachProductModule(module *productenrichhttpapi.Module) {
	if d == nil || module == nil {
		return
	}
	d.addClosers(module.Closers...)
	d.productService = module.Service
}

func (d *runtimeDeps) attachImageModule(module *productimagehttpapi.Module) {
	if d == nil || module == nil {
		return
	}
	d.addClosers(module.Closers...)
	d.imageService = module.Service
	d.imageSubjectExtractor = module.SubjectExtractor
	d.imageWhiteBgRenderer = module.WhiteBackgroundRender
	d.imageSceneRenderer = module.SceneRenderer
}

func (d *runtimeDeps) attachAmazonListingModule(module *amazonlistinghttpapi.Module) {
	if d == nil || module == nil {
		return
	}
	d.addClosers(module.Closers...)
}

func (d *runtimeDeps) attachListingKitModule(module *listingkithttpapi.Module) {
	if d == nil || module == nil {
		return
	}
	d.addClosers(module.Closers...)
}

func (d *runtimeDeps) attachSDSLoginResult(result *sdsloginbootstrap.BuildResult) {
	if d == nil || result == nil {
		return
	}
	d.sdsLoginStatusProvider = result.StatusProvider
}

func resolveImageWorkDir(cfg *config.Config) string {
	if cfg == nil {
		return filepath.Join(".", "tmp", "productimage")
	}

	workDir := filepath.Clean(cfg.ProductImage.WorkDir)
	if workDir == "" || workDir == "." {
		return filepath.Join(".", "tmp", "productimage")
	}

	return workDir
}
