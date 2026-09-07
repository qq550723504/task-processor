package httpapi

import (
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"task-processor/internal/core/config"
	"testing"
)

func TestCommercialReadModuleRequiresDurableConfiguration(t *testing.T) {
	result, err := buildCommercialReadModule(&config.Config{}, logrus.New())
	require.NoError(t, err)
	require.Nil(t, result.module)
	_, err = buildCommercialReadModule(&config.Config{Workbench: config.WorkbenchConfig{Enabled: true}}, logrus.New())
	require.Error(t, err)
}
