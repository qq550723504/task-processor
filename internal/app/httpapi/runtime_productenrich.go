package httpapi

import (
	"fmt"

	"github.com/sirupsen/logrus"

	"task-processor/internal/aicapability"
	"task-processor/internal/core/config"
	openaiclient "task-processor/internal/infra/clients/openai"
	"task-processor/internal/productenrich"
	productenrichenrich "task-processor/internal/productenrich/enrich"
	productenrichhttpapi "task-processor/internal/productenrich/httpapi"
)

type productEnrichRuntimeDeps struct {
	llmMgr        productenrich.LLMManager
	inputParser   productenrich.InputParser
	understanding productenrich.ProductUnderstanding
}

func buildProductEnrichRuntimeDeps(logger *logrus.Logger, cfg *config.Config, openaiMgr *openaiclient.Manager, credentialResolver openaiclient.ClientConfigResolver, recorder aicapability.InvocationRecorder) (productEnrichRuntimeDeps, error) {
	llmMgr, err := productenrich.NewLLMManagerAdapterFromManager(openaiMgr)
	if err != nil {
		return productEnrichRuntimeDeps{}, fmt.Errorf("create LLM manager: %w", err)
	}
	if cfg.Debug.ProductEnrichMockLLM {
		logger.WithField("config", "debug.productEnrichMockLLM").Warn("productenrich mock LLM enabled for local runtime")
		llmMgr = productenrich.NewLocalMockLLMManager()
	}
	if err := productenrich.ValidateMockLLMManager(llmMgr); err != nil {
		return productEnrichRuntimeDeps{}, fmt.Errorf("validate LLM manager: %w", err)
	}

	var productUnderstanding productenrich.ProductUnderstanding
	if cfg.AICapability.ProductEnrichTextEnabled {
		if credentialResolver == nil {
			return productEnrichRuntimeDeps{}, fmt.Errorf("create product enrich text capability: credential resolver is required")
		}
		textGenerator, textErr := productenrichenrich.NewGovernedTextGenerator(llmMgr, productenrichenrich.GovernedTextGeneratorConfig{
			Router:   productenrichhttpapi.BuildProductEnrichTextCapabilityRouter(credentialResolver, cfg.AICapability.ProductEnrichTextAllowedTenantIDs),
			Recorder: recorder,
		})
		if textErr != nil {
			return productEnrichRuntimeDeps{}, fmt.Errorf("create product enrich text capability: %w", textErr)
		}
		productUnderstanding, err = productenrichenrich.NewProductUnderstandingWithTextGenerator(llmMgr, textGenerator)
	} else {
		productUnderstanding, err = productenrichenrich.NewProductUnderstanding(llmMgr)
	}
	if err != nil {
		return productEnrichRuntimeDeps{}, fmt.Errorf("create product understanding: %w", err)
	}

	webScraper := newWebScraper(cfg)
	inputParser, err := productenrichenrich.NewInputParser(logger, &productenrich.InputParserConfig{}, webScraper)
	if err != nil {
		return productEnrichRuntimeDeps{}, fmt.Errorf("create input parser: %w", err)
	}

	return productEnrichRuntimeDeps{
		llmMgr:        llmMgr,
		inputParser:   inputParser,
		understanding: productUnderstanding,
	}, nil
}
