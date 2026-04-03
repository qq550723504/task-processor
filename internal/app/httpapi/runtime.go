package httpapi

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/sirupsen/logrus"

	appbootstrap "task-processor/internal/app/bootstrap"
	"task-processor/internal/core/config"
	"task-processor/internal/productenrich"
	productenrichenrich "task-processor/internal/productenrich/enrich"
	"task-processor/internal/prompt"
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

	llmMgr, err := newLLMManager(cfg.OpenAI)
	if err != nil {
		return nil, fmt.Errorf("create LLM manager: %w", err)
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

	shared, err := appbootstrap.BuildSharedResources(cfg, logger, appbootstrap.SharedResourceOptions{})
	if err != nil {
		return nil, fmt.Errorf("build shared resources: %w", err)
	}

	return &runtimeDeps{
		cfg:              cfg,
		closers:          nil,
		llmMgr:           llmMgr,
		inputParser:      inputParser,
		understanding:    productUnderstanding,
		imageWorkDir:     imageWorkDir,
		shared:           shared,
		managementClient: shared.ManagementClient,
	}, nil
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
