package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAICapabilityRoutingDefaultsToLegacy(t *testing.T) {
	t.Setenv("TASK_PROCESSOR_AI_CAPABILITY_STUDIO_IMAGE_ROUTING_MODE", "")

	cfg, err := LoadFromBytes(validMinimalConfigYAML())
	require.NoError(t, err)
	assert.Equal(t, "legacy", cfg.AICapability.StudioImageRoutingMode)
}

func validMinimalConfigYAML() []byte {
	return []byte(`
openai:
  apiKey: test-key
  model: test-model
  baseURL: https://example.test/v1
  timeout: 30
`)
}

func TestAICapabilityRoutingModeUsesEnvironmentOverride(t *testing.T) {
	t.Setenv("TASK_PROCESSOR_AI_CAPABILITY_STUDIO_IMAGE_ROUTING_MODE", "shadow")

	cfg, err := LoadFromBytes(validMinimalConfigYAML())
	require.NoError(t, err)
	assert.Equal(t, "shadow", cfg.AICapability.StudioImageRoutingMode)
}

func TestAICapabilityRoutingAcceptsCaseInsensitiveModes(t *testing.T) {
	for _, mode := range []string{"SHADOW", "Active"} {
		t.Run(mode, func(t *testing.T) {
			errors := ValidateAICapabilityConfig(&AICapabilityConfig{StudioImageRoutingMode: mode})
			require.Empty(t, errors)
		})
	}
}

func TestAICapabilityRoutingRejectsUnknownMode(t *testing.T) {
	t.Setenv("TASK_PROCESSOR_AI_CAPABILITY_STUDIO_IMAGE_ROUTING_MODE", "")

	_, err := LoadFromBytes(append(validMinimalConfigYAML(), []byte("\naiCapability:\n  studioImageRoutingMode: automatic\n")...))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "aiCapability.studioImageRoutingMode")
}

func TestAICapabilityRoutingRejectsEmptyMode(t *testing.T) {
	errors := ValidateAICapabilityConfig(&AICapabilityConfig{})
	require.Len(t, errors, 1)
	assert.Contains(t, errors[0].Error(), "aiCapability.studioImageRoutingMode")
}
