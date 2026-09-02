package config

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLoadFromBytesRejectsRetiredProductRuntimeYAML(t *testing.T) {
	tests := []struct {
		name string
		yaml string
		key  string
	}{
		{name: "productimage root", yaml: "productimage:\n  publisher:\n    provider: s3\n", key: "productimage"},
		{name: "product enrich debug", yaml: "debug:\n  productEnrichMockLLM: true\n", key: "debug.productEnrichMockLLM"},
		{name: "product image scene enabled", yaml: "aiCapability:\n  productImageSceneEnabled: true\n", key: "aiCapability.productImageSceneEnabled"},
		{name: "product image scene tenants", yaml: "aiCapability:\n  productImageSceneAllowedTenantIDs: [tenant-a]\n", key: "aiCapability.productImageSceneAllowedTenantIDs"},
		{name: "product enrich text", yaml: "aiCapability:\n  productEnrichTextEnabled: true\n", key: "aiCapability.productEnrichTextEnabled"},
		{name: "product enrich vision", yaml: "aiCapability:\n  productEnrichVisionAllowedTenantIDs: [tenant-a]\n", key: "aiCapability.productEnrichVisionAllowedTenantIDs"},
		{name: "product enrich listing", yaml: "aiCapability:\n  productEnrichListingEnabled: true\n", key: "aiCapability.productEnrichListingEnabled"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := LoadFromBytes([]byte("openai:\n  apiKey: test-key\n" + test.yaml))
			require.Error(t, err)
			require.Contains(t, strings.ToLower(err.Error()), strings.ToLower(test.key))
		})
	}
}

func TestLoadFromBytesRejectsRetiredProductRuntimeEnvironment(t *testing.T) {
	variables := []string{
		"TASK_PROCESSOR_PRODUCTIMAGE_PUBLISHER_PROVIDER",
		"TASK_PROCESSOR_PRODUCT_IMAGE_PUBLISHER_PROVIDER",
		"PRODUCTIMAGE_PUBLISHER_PROVIDER",
		"PRODUCT_IMAGE_PUBLISHER_PROVIDER",
		"TASK_PROCESSOR_PRODUCTENRICH_MOCK_LLM",
		"TASK_PROCESSOR_PRODUCT_ENRICH_MOCK_LLM",
		"TASK_PROCESSOR_AI_CAPABILITY_PRODUCT_IMAGE_SCENE_ENABLED",
		"TASK_PROCESSOR_AI_CAPABILITY_PRODUCT_IMAGE_SCENE_ALLOWED_TENANT_IDS",
		"TASK_PROCESSOR_AICAPABILITY_PRODUCTIMAGESCENEENABLED",
		"TASK_PROCESSOR_AI_CAPABILITY_PRODUCT_ENRICH_TEXT_ENABLED",
		"TASK_PROCESSOR_AICAPABILITY_PRODUCTENRICHTEXTENABLED",
		"TASK_PROCESSOR_AI_CAPABILITY_PRODUCT_ENRICH_VISION_ENABLED",
		"TASK_PROCESSOR_AI_CAPABILITY_PRODUCT_ENRICH_LISTING_ENABLED",
	}

	for _, variable := range variables {
		t.Run(variable, func(t *testing.T) {
			t.Setenv(variable, "retired-value")
			_, err := LoadFromBytes([]byte("openai:\n  apiKey: test-key\n"))
			require.Error(t, err)
			require.Contains(t, err.Error(), variable)
		})
	}
}
