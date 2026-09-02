package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func validMinimalConfigYAML() []byte {
	return []byte(`
openai:
  apiKey: test-key
  model: test-model
  baseURL: https://example.test/v1
  timeout: 30
`)
}

func TestLoadFromBytesRejectsRetiredStudioImageRoutingModeYAML(t *testing.T) {
	_, err := LoadFromBytes(append(validMinimalConfigYAML(), []byte("\naiCapability:\n  studioImageRoutingMode: legacy\n")...))
	require.Error(t, err)
	require.Contains(t, err.Error(), "aiCapability.studioImageRoutingMode")
}

func TestLoadFromBytesRejectsRetiredStudioImageRoutingModeEnvironment(t *testing.T) {
	t.Setenv("TASK_PROCESSOR_AI_CAPABILITY_STUDIO_IMAGE_ROUTING_MODE", "legacy")

	_, err := LoadFromBytes(validMinimalConfigYAML())
	require.Error(t, err)
	require.Contains(t, err.Error(), "TASK_PROCESSOR_AI_CAPABILITY_STUDIO_IMAGE_ROUTING_MODE")
}
