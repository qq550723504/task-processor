package httpapi

import (
	"fmt"

	"task-processor/internal/core/config"
)

func loadHTTPAPIConfig(configPath string) (*config.Config, error) {
	cfg, err := config.LoadConfigFromFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}
	return cfg, nil
}
