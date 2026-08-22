package httpapi

import (
	"fmt"
	"strings"

	"github.com/sirupsen/logrus"

	"task-processor/internal/aicapability"
	"task-processor/internal/core/config"
	openaiclient "task-processor/internal/infra/clients/openai"
	"task-processor/internal/productenrich"
	productenrichenrich "task-processor/internal/productenrich/enrich"
	productenrichhttpapi "task-processor/internal/productenrich/httpapi"
)

type productEnrichRuntimeDeps struct {
	llmMgr               productenrich.LLMManager
	inputParser          productenrich.InputParser
	understanding        productenrich.ProductUnderstanding
	contentGenerator     productenrichenrich.TextGenerator
	specsGenerator       productenrichenrich.TextGenerator
	variantsGenerator    productenrichenrich.TextGenerator
	fusionGenerator      productenrichenrich.TextGenerator
	scoringTextGenerator productenrichenrich.TextGenerator
	scoringImageAnalyzer productenrichenrich.ImageAnalyzer
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
	var fusionGenerator productenrichenrich.TextGenerator
	var scoringTextGenerator productenrichenrich.TextGenerator
	var scoringImageAnalyzer productenrichenrich.ImageAnalyzer
	if cfg.AICapability.ProductEnrichTextEnabled || cfg.AICapability.ProductEnrichVisionEnabled || cfg.AICapability.ProductEnrichListingEnabled {
		if credentialResolver == nil {
			return productEnrichRuntimeDeps{}, fmt.Errorf("create product enrich capability: credential resolver is required")
		}
	}
	legacyRouteMetadata := productenrichhttpapi.BuildProductEnrichLegacyRouteMetadataResolver(credentialResolver)
	if cfg.AICapability.ProductEnrichTextEnabled {
		router := productenrichhttpapi.BuildProductEnrichTextCapabilityRouter(credentialResolver)
		textGenerator, err = productenrichenrich.NewGovernedTextGenerator(llmMgr, productenrichenrich.GovernedTextGeneratorConfig{
			Planner: productenrichhttpapi.BuildProductEnrichExecutionPlanner(
				router, cfg.AICapability.ProductEnrichTextAllowedTenantIDs, []string{"fast", "default"},
			),
			LegacyRouteMetadata: legacyRouteMetadata,
			Recorder:            recorder,
			OnRecordError:       productEnrichInvocationErrorHandler(logger),
		})
		if err != nil {
			return productEnrichRuntimeDeps{}, fmt.Errorf("create product enrich text capability: %w", err)
		}
	}
	if cfg.AICapability.ProductEnrichVisionEnabled {
		router := productenrichhttpapi.BuildProductEnrichVisionCapabilityRouter(credentialResolver)
		imageAnalyzer, err = productenrichenrich.NewGovernedImageAnalyzer(llmMgr, productenrichenrich.GovernedImageAnalyzerConfig{
			Planner: productenrichhttpapi.BuildProductEnrichExecutionPlanner(
				router, cfg.AICapability.ProductEnrichVisionAllowedTenantIDs, []string{"vision", "default"},
			),
			LegacyRouteMetadata: legacyRouteMetadata,
			Recorder:            recorder,
			OnRecordError:       productEnrichInvocationErrorHandler(logger),
		})
		if err != nil {
			return productEnrichRuntimeDeps{}, fmt.Errorf("create product enrich vision capability: %w", err)
		}
	}
	if cfg.AICapability.ProductEnrichListingEnabled {
		router := productenrichhttpapi.BuildProductEnrichListingCapabilityRouter(credentialResolver)
		contentGenerator, err = productenrichenrich.NewGovernedTextGenerator(llmMgr, productenrichenrich.GovernedTextGeneratorConfig{
			Planner: productenrichhttpapi.BuildProductEnrichExecutionPlanner(
				router, cfg.AICapability.ProductEnrichListingAllowedTenantIDs, []string{"default"},
			),
			LegacyRouteMetadata: legacyRouteMetadata,
			Recorder:            recorder,
			OnRecordError:       productEnrichInvocationErrorHandler(logger),
			Capability:          aicapability.CapabilityProductEnrichListing,
			Operation:           aicapability.OperationProductEnrichJSONGenerate,
			RequiredFeature:     aicapability.FeatureTextGenerate,
			PromptKey:           "productenrich.listing.generate_json",
			PromptVersion:       "v1",
			PromptScope:         "product_enrich",
		})
		if err != nil {
			return productEnrichRuntimeDeps{}, fmt.Errorf("create product enrich listing capability: %w", err)
		}
		specsGenerator, err = productenrichenrich.NewGovernedTextGenerator(llmMgr, productenrichenrich.GovernedTextGeneratorConfig{
			Planner: productenrichhttpapi.BuildProductEnrichExecutionPlanner(
				router, cfg.AICapability.ProductEnrichListingAllowedTenantIDs, []string{"default"},
			),
			LegacyRouteMetadata: legacyRouteMetadata,
			Recorder:            recorder,
			OnRecordError:       productEnrichInvocationErrorHandler(logger),
			Capability:          aicapability.CapabilityProductEnrichListing,
			Operation:           aicapability.OperationProductEnrichSpecsGenerate,
			RequiredFeature:     aicapability.FeatureTextGenerate,
			PromptKey:           "productenrich.listing.generate_specs",
			PromptVersion:       "v1",
			PromptScope:         "product_enrich",
		})
		if err != nil {
			return productEnrichRuntimeDeps{}, fmt.Errorf("create product enrich specs capability: %w", err)
		}
		variantsGenerator, err = productenrichenrich.NewGovernedTextGenerator(llmMgr, productenrichenrich.GovernedTextGeneratorConfig{
			Planner: productenrichhttpapi.BuildProductEnrichExecutionPlanner(
				router, cfg.AICapability.ProductEnrichListingAllowedTenantIDs, []string{"default"},
			),
			LegacyRouteMetadata: legacyRouteMetadata,
			Recorder:            recorder,
			OnRecordError:       productEnrichInvocationErrorHandler(logger),
			Capability:          aicapability.CapabilityProductEnrichListing,
			Operation:           aicapability.OperationProductEnrichVariantsGenerate,
			RequiredFeature:     aicapability.FeatureTextGenerate,
			PromptKey:           "productenrich.listing.generate_variants",
			PromptVersion:       "v1",
			PromptScope:         "product_enrich",
		})
		if err != nil {
			return productEnrichRuntimeDeps{}, fmt.Errorf("create product enrich variants capability: %w", err)
		}
	}
	if cfg.AICapability.ProductEnrichTextEnabled || cfg.AICapability.ProductEnrichVisionEnabled {
		allowedTenants := unionTenantIDs(cfg.AICapability.ProductEnrichTextAllowedTenantIDs, cfg.AICapability.ProductEnrichVisionAllowedTenantIDs)
		router := productenrichhttpapi.BuildProductEnrichFusionCapabilityRouter(credentialResolver)
		fusionGenerator, err = productenrichenrich.NewGovernedTextGenerator(llmMgr, productenrichenrich.GovernedTextGeneratorConfig{
			Planner: productenrichhttpapi.BuildProductEnrichExecutionPlanner(
				router, allowedTenants, []string{"default"},
			),
			LegacyRouteMetadata: legacyRouteMetadata,
			Recorder:            recorder,
			OnRecordError:       productEnrichInvocationErrorHandler(logger),
			Capability:          aicapability.CapabilityProductEnrichFusion,
			Operation:           aicapability.OperationProductEnrichMultimodalFuse,
			RequiredFeature:     aicapability.FeatureTextGenerate,
			PromptKey:           "productenrich.understanding.fuse_multimodal",
			PromptVersion:       "v1",
			PromptScope:         "product_enrich",
		})
		if err != nil {
			return productEnrichRuntimeDeps{}, fmt.Errorf("create product enrich fusion capability: %w", err)
		}
	}
	if cfg.AICapability.ProductEnrichTextEnabled {
		scoringClient := scorerClientName(cfg, "fast")
		router := productenrichhttpapi.BuildProductEnrichTextQualityCapabilityRouter(credentialResolver, scoringClient)
		scoringTextGenerator, err = productenrichenrich.NewGovernedTextGenerator(llmMgr, productenrichenrich.GovernedTextGeneratorConfig{
			Planner: productenrichhttpapi.BuildProductEnrichExecutionPlanner(
				router, cfg.AICapability.ProductEnrichTextAllowedTenantIDs, uniqueProductEnrichClientNames(scoringClient, "default"),
			),
			LegacyRouteMetadata: legacyRouteMetadata,
			Recorder:            recorder,
			OnRecordError:       productEnrichInvocationErrorHandler(logger),
			Capability:          aicapability.CapabilityProductEnrichText,
			Operation:           aicapability.OperationProductEnrichTextQualityScore,
			RequiredFeature:     aicapability.FeatureTextGenerate,
			PromptKey:           "productenrich.quality_score.text",
			PromptVersion:       "v1",
			PromptScope:         "product_enrich",
		})
		if err != nil {
			return productEnrichRuntimeDeps{}, fmt.Errorf("create product enrich text scoring capability: %w", err)
		}
	}
	if cfg.AICapability.ProductEnrichVisionEnabled {
		scoringClient := scorerClientName(cfg, "vision")
		router := productenrichhttpapi.BuildProductEnrichVisionQualityCapabilityRouter(credentialResolver, scoringClient)
		scoringImageAnalyzer, err = productenrichenrich.NewGovernedImageAnalyzer(llmMgr, productenrichenrich.GovernedImageAnalyzerConfig{
			Planner: productenrichhttpapi.BuildProductEnrichExecutionPlanner(
				router, cfg.AICapability.ProductEnrichVisionAllowedTenantIDs, uniqueProductEnrichClientNames(scoringClient, "default"),
			),
			LegacyRouteMetadata: legacyRouteMetadata,
			Recorder:            recorder,
			OnRecordError:       productEnrichInvocationErrorHandler(logger),
			Capability:          aicapability.CapabilityProductEnrichVision,
			Operation:           aicapability.OperationProductEnrichVisionQualityScore,
			RequiredFeature:     aicapability.FeatureVisionAnalyze,
		})
		if err != nil {
			return productEnrichRuntimeDeps{}, fmt.Errorf("create product enrich image scoring capability: %w", err)
		}
	}
	if textGenerator == nil && imageAnalyzer == nil && fusionGenerator == nil {
		productUnderstanding, err = productenrichenrich.NewProductUnderstanding(llmMgr)
	} else {
		productUnderstanding, err = productenrichenrich.NewProductUnderstandingWithAllCapabilities(llmMgr, textGenerator, imageAnalyzer, fusionGenerator)
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
		llmMgr:               llmMgr,
		inputParser:          inputParser,
		understanding:        productUnderstanding,
		contentGenerator:     contentGenerator,
		specsGenerator:       specsGenerator,
		variantsGenerator:    variantsGenerator,
		fusionGenerator:      fusionGenerator,
		scoringTextGenerator: scoringTextGenerator,
		scoringImageAnalyzer: scoringImageAnalyzer,
	}, nil
}

func unionTenantIDs(groups ...[]string) []string {
	seen := map[string]struct{}{}
	var result []string
	for _, group := range groups {
		for _, id := range group {
			id = strings.TrimSpace(id)
			if id != "" {
				if _, ok := seen[id]; !ok {
					seen[id] = struct{}{}
					result = append(result, id)
				}
			}
		}
	}
	return result
}

func scorerClientName(cfg *config.Config, fallback string) string {
	if cfg != nil {
		if _, ok := cfg.OpenAI.Clients["scorer"]; ok {
			return "scorer"
		}
	}
	return fallback
}

func uniqueProductEnrichClientNames(clientNames ...string) []string {
	seen := make(map[string]struct{}, len(clientNames))
	result := make([]string, 0, len(clientNames))
	for _, clientName := range clientNames {
		clientName = strings.TrimSpace(clientName)
		if clientName == "" {
			continue
		}
		if _, ok := seen[clientName]; ok {
			continue
		}
		seen[clientName] = struct{}{}
		result = append(result, clientName)
	}
	return result
}
