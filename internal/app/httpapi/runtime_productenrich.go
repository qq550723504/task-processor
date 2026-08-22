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
	llmMgr            productenrich.LLMManager
	inputParser       productenrich.InputParser
	understanding     productenrich.ProductUnderstanding
	contentGenerator  productenrichenrich.TextGenerator
	specsGenerator    productenrichenrich.TextGenerator
	variantsGenerator productenrichenrich.TextGenerator
}

func productEnrichInvocationErrorHandler(logger *logrus.Logger) func(aicapability.InvocationRecord, error) {
	return func(record aicapability.InvocationRecord, err error) {
		if logger == nil {
			return
		}
		logger.WithError(err).WithFields(logrus.Fields{
			"invocation_id": string(record.InvocationID),
			"capability":    string(record.Capability),
			"operation":     string(record.Operation),
		}).Warn("ai invocation ledger write failed")
	}
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
	var textGenerator productenrichenrich.TextGenerator
	var imageAnalyzer productenrichenrich.ImageAnalyzer
	var contentGenerator productenrichenrich.TextGenerator
	var specsGenerator productenrichenrich.TextGenerator
	var variantsGenerator productenrichenrich.TextGenerator
	if cfg.AICapability.ProductEnrichTextEnabled || cfg.AICapability.ProductEnrichVisionEnabled || cfg.AICapability.ProductEnrichListingEnabled {
		if credentialResolver == nil {
			return productEnrichRuntimeDeps{}, fmt.Errorf("create product enrich capability: credential resolver is required")
		}
	}
	if cfg.AICapability.ProductEnrichTextEnabled {
		textGenerator, err = productenrichenrich.NewGovernedTextGenerator(llmMgr, productenrichenrich.GovernedTextGeneratorConfig{
			Router:        productenrichhttpapi.BuildProductEnrichTextCapabilityRouter(credentialResolver, cfg.AICapability.ProductEnrichTextAllowedTenantIDs),
			Recorder:      recorder,
			OnRecordError: productEnrichInvocationErrorHandler(logger),
		})
		if err != nil {
			return productEnrichRuntimeDeps{}, fmt.Errorf("create product enrich text capability: %w", err)
		}
	}
	if cfg.AICapability.ProductEnrichVisionEnabled {
		imageAnalyzer, err = productenrichenrich.NewGovernedImageAnalyzer(llmMgr, productenrichenrich.GovernedImageAnalyzerConfig{
			Router:        productenrichhttpapi.BuildProductEnrichVisionCapabilityRouter(credentialResolver, cfg.AICapability.ProductEnrichVisionAllowedTenantIDs),
			Recorder:      recorder,
			OnRecordError: productEnrichInvocationErrorHandler(logger),
		})
		if err != nil {
			return productEnrichRuntimeDeps{}, fmt.Errorf("create product enrich vision capability: %w", err)
		}
	}
	if cfg.AICapability.ProductEnrichListingEnabled {
		contentGenerator, err = productenrichenrich.NewGovernedTextGenerator(llmMgr, productenrichenrich.GovernedTextGeneratorConfig{
			Router:          productenrichhttpapi.BuildProductEnrichListingCapabilityRouter(credentialResolver, cfg.AICapability.ProductEnrichListingAllowedTenantIDs),
			Recorder:        recorder,
			OnRecordError:   productEnrichInvocationErrorHandler(logger),
			Capability:      aicapability.CapabilityProductEnrichListing,
			Operation:       aicapability.OperationProductEnrichJSONGenerate,
			RequiredFeature: aicapability.FeatureTextGenerate,
			PromptKey:       "productenrich.listing.generate_json",
			PromptVersion:   "v1",
			PromptScope:     "product_enrich",
		})
		if err != nil {
			return productEnrichRuntimeDeps{}, fmt.Errorf("create product enrich listing capability: %w", err)
		}
		specsGenerator, err = productenrichenrich.NewGovernedTextGenerator(llmMgr, productenrichenrich.GovernedTextGeneratorConfig{
			Router:          productenrichhttpapi.BuildProductEnrichListingCapabilityRouter(credentialResolver, cfg.AICapability.ProductEnrichListingAllowedTenantIDs),
			Recorder:        recorder,
			OnRecordError:   productEnrichInvocationErrorHandler(logger),
			Capability:      aicapability.CapabilityProductEnrichListing,
			Operation:       aicapability.OperationProductEnrichSpecsGenerate,
			RequiredFeature: aicapability.FeatureTextGenerate,
			PromptKey:       "productenrich.listing.generate_specs",
			PromptVersion:   "v1",
			PromptScope:     "product_enrich",
		})
		if err != nil {
			return productEnrichRuntimeDeps{}, fmt.Errorf("create product enrich specs capability: %w", err)
		}
		variantsGenerator, err = productenrichenrich.NewGovernedTextGenerator(llmMgr, productenrichenrich.GovernedTextGeneratorConfig{
			Router:          productenrichhttpapi.BuildProductEnrichListingCapabilityRouter(credentialResolver, cfg.AICapability.ProductEnrichListingAllowedTenantIDs),
			Recorder:        recorder,
			OnRecordError:   productEnrichInvocationErrorHandler(logger),
			Capability:      aicapability.CapabilityProductEnrichListing,
			Operation:       aicapability.OperationProductEnrichVariantsGenerate,
			RequiredFeature: aicapability.FeatureTextGenerate,
			PromptKey:       "productenrich.listing.generate_variants",
			PromptVersion:   "v1",
			PromptScope:     "product_enrich",
		})
		if err != nil {
			return productEnrichRuntimeDeps{}, fmt.Errorf("create product enrich variants capability: %w", err)
		}
	}
	if textGenerator == nil && imageAnalyzer == nil {
		productUnderstanding, err = productenrichenrich.NewProductUnderstanding(llmMgr)
	} else {
		productUnderstanding, err = productenrichenrich.NewProductUnderstandingWithCapabilities(llmMgr, textGenerator, imageAnalyzer)
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
		llmMgr:            llmMgr,
		inputParser:       inputParser,
		understanding:     productUnderstanding,
		contentGenerator:  contentGenerator,
		specsGenerator:    specsGenerator,
		variantsGenerator: variantsGenerator,
	}, nil
}
