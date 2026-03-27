package main

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/sirupsen/logrus"

	"task-processor/internal/core/config"
	"task-processor/internal/productenrich"
	productenrichenrich "task-processor/internal/productenrich/enrich"
	"task-processor/internal/prompt"
)

type runtimeDeps struct {
	cfg           *config.Config
	closers       []func() error
	llmMgr        productenrich.LLMManager
	inputParser   productenrich.InputParser
	understanding productenrich.ProductUnderstanding
	imageWorkDir  string
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
		logger.Warnf("prompt registry init failed, using fallback prompts: %v", err)
	}

	llmMgr, err := newLLMManager(cfg.OpenAI)
	if err != nil {
		return nil, fmt.Errorf("create llm manager: %w", err)
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
