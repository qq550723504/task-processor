package main

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/sirupsen/logrus"

	"task-processor/internal/core/config"
	"task-processor/internal/productenrich"
	productenrichenrich "task-processor/internal/productenrich/enrich"
	"task-processor/internal/productimage"
	"task-processor/internal/prompt"
)

type runtimeDeps struct {
	cfg            *config.Config
	closers        []func() error
	llmMgr         productenrich.LLMManager
	inputParser    productenrich.InputParser
	understanding  productenrich.ProductUnderstanding
	imageWorkDir   string
	productService productenrich.ProductService
	imageService   productimage.Service
}

func buildRuntimeDeps(logger *logrus.Logger) (*runtimeDeps, error) {
	cfg := config.LoadConfigFromFile(*configPath)
	var closers []func() error
	imageWorkDir := resolveImageWorkDir(cfg)

	promptsDir := cfg.Prompts.Dir
	if promptsDir == "" {
		promptsDir = "./prompts"
	}
	if err := prompt.InitGlobal(context.Background(), promptsDir, cfg.Prompts.HotReload, logger.WithField("component", "prompt")); err != nil {
		logger.Warnf("提示词注册表初始化失败，使用备用提示词：%v", err)
	}

	llmMgr, err := newLLMManager(cfg.OpenAI)
	if err != nil {
		return nil, fmt.Errorf("创建 LLM 管理器：%w", err)
	}

	productUnderstanding, err := productenrichenrich.NewProductUnderstanding(llmMgr)
	if err != nil {
		return nil, fmt.Errorf("创建产品理解模块：%w", err)
	}

	webScraper := newWebScraper(cfg)
	inputParser, err := productenrichenrich.NewInputParser(logger, &productenrich.InputParserConfig{}, webScraper)
	if err != nil {
		return nil, fmt.Errorf("创建输入解析器：%w", err)
	}

	return &runtimeDeps{
		cfg:           cfg,
		closers:       closers,
		llmMgr:        llmMgr,
		inputParser:   inputParser,
		understanding: productUnderstanding,
		imageWorkDir:  imageWorkDir,
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
