package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/viper"
)

var retiredProductRuntimeYAMLKeys = map[string]string{
	"aicapability.studioimageroutingmode":               "aiCapability.studioImageRoutingMode",
	"debug.productenrichmockllm":                        "debug.productEnrichMockLLM",
	"aicapability.productimagesceneenabled":             "aiCapability.productImageSceneEnabled",
	"aicapability.productimagesceneallowedtenantids":    "aiCapability.productImageSceneAllowedTenantIDs",
	"aicapability.productenrichtextenabled":             "aiCapability.productEnrichTextEnabled",
	"aicapability.productenrichtextallowedtenantids":    "aiCapability.productEnrichTextAllowedTenantIDs",
	"aicapability.productenrichvisionenabled":           "aiCapability.productEnrichVisionEnabled",
	"aicapability.productenrichvisionallowedtenantids":  "aiCapability.productEnrichVisionAllowedTenantIDs",
	"aicapability.productenrichlistingenabled":          "aiCapability.productEnrichListingEnabled",
	"aicapability.productenrichlistingallowedtenantids": "aiCapability.productEnrichListingAllowedTenantIDs",
}

var retiredProductRuntimeEnvironmentVariables = map[string]struct{}{
	"TASK_PROCESSOR_AI_CAPABILITY_STUDIO_IMAGE_ROUTING_MODE":                 {},
	"TASK_PROCESSOR_AICAPABILITY_STUDIOIMAGEROUTINGMODE":                     {},
	"TASK_PROCESSOR_DEBUG_PRODUCTENRICHMOCKLLM":                              {},
	"TASK_PROCESSOR_DEBUG_PRODUCTENRICH_MOCK_LLM":                            {},
	"TASK_PROCESSOR_AI_CAPABILITY_PRODUCT_IMAGE_SCENE_ENABLED":               {},
	"TASK_PROCESSOR_AI_CAPABILITY_PRODUCT_IMAGE_SCENE_ALLOWED_TENANT_IDS":    {},
	"TASK_PROCESSOR_AI_CAPABILITY_PRODUCT_ENRICH_TEXT_ENABLED":               {},
	"TASK_PROCESSOR_AI_CAPABILITY_PRODUCT_ENRICH_TEXT_ALLOWED_TENANT_IDS":    {},
	"TASK_PROCESSOR_AI_CAPABILITY_PRODUCT_ENRICH_VISION_ENABLED":             {},
	"TASK_PROCESSOR_AI_CAPABILITY_PRODUCT_ENRICH_VISION_ALLOWED_TENANT_IDS":  {},
	"TASK_PROCESSOR_AI_CAPABILITY_PRODUCT_ENRICH_LISTING_ENABLED":            {},
	"TASK_PROCESSOR_AI_CAPABILITY_PRODUCT_ENRICH_LISTING_ALLOWED_TENANT_IDS": {},
	"TASK_PROCESSOR_AICAPABILITY_PRODUCTIMAGESCENEENABLED":                   {},
	"TASK_PROCESSOR_AICAPABILITY_PRODUCTIMAGESCENEALLOWEDTENANTIDS":          {},
	"TASK_PROCESSOR_AICAPABILITY_PRODUCTENRICHTEXTENABLED":                   {},
	"TASK_PROCESSOR_AICAPABILITY_PRODUCTENRICHTEXTALLOWEDTENANTIDS":          {},
	"TASK_PROCESSOR_AICAPABILITY_PRODUCTENRICHVISIONENABLED":                 {},
	"TASK_PROCESSOR_AICAPABILITY_PRODUCTENRICHVISIONALLOWEDTENANTIDS":        {},
	"TASK_PROCESSOR_AICAPABILITY_PRODUCTENRICHLISTINGENABLED":                {},
	"TASK_PROCESSOR_AICAPABILITY_PRODUCTENRICHLISTINGALLOWEDTENANTIDS":       {},
}

var retiredProductRuntimeEnvironmentPrefixes = []string{
	"TASK_PROCESSOR_PRODUCTIMAGE_",
	"TASK_PROCESSOR_PRODUCT_IMAGE_",
	"PRODUCTIMAGE_",
	"PRODUCT_IMAGE_",
	"TASK_PROCESSOR_PRODUCTENRICH_",
	"TASK_PROCESSOR_PRODUCT_ENRICH_",
	"PRODUCTENRICH_",
	"PRODUCT_ENRICH_",
}

func rejectRetiredProductRuntimeConfig(v *viper.Viper) error {
	if v != nil {
		if v.InConfig("productimage") {
			return fmt.Errorf("retired configuration key productimage is not supported; use owner-specific imageagent and listingkit settings")
		}
		for _, key := range v.AllKeys() {
			lowerKey := strings.ToLower(key)
			if v.InConfig(key) && strings.HasPrefix(lowerKey, "productimage.") {
				return fmt.Errorf("retired configuration key %s is not supported; use owner-specific imageagent and listingkit settings", key)
			}
			if canonical, retired := retiredProductRuntimeYAMLKeys[lowerKey]; retired && v.InConfig(key) {
				return fmt.Errorf("retired configuration key %s is not supported", canonical)
			}
		}
	}

	for _, assignment := range os.Environ() {
		name, _, _ := strings.Cut(assignment, "=")
		upperName := strings.ToUpper(strings.TrimSpace(name))
		if _, retired := retiredProductRuntimeEnvironmentVariables[upperName]; retired {
			return fmt.Errorf("retired environment variable %s is not supported", name)
		}
		for _, prefix := range retiredProductRuntimeEnvironmentPrefixes {
			if strings.HasPrefix(upperName, prefix) {
				return fmt.Errorf("retired environment variable %s is not supported", name)
			}
		}
	}
	return nil
}
