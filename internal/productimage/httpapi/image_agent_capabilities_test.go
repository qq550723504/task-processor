package httpapi

import (
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"

	"task-processor/internal/core/config"
)

func TestBuildImageAgentCapabilitiesFailsClosedWithoutRealProvidersAndPublisher(t *testing.T) {
	_, err := BuildImageAgentCapabilities(RuntimeBuildInput{Logger: logrus.New(), Config: &config.Config{}, ImageWorkDir: t.TempDir()})
	require.ErrorContains(t, err, "faithful-edit and scene-generation providers")
}
