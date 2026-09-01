package configadapter

import (
	coreconfig "task-processor/internal/core/config"
	platformredis "task-processor/internal/platform/redis"
)

// Redis translates the application Redis schema into platform runtime configuration.
func Redis(cfg *coreconfig.RedisConfig) *platformredis.Config {
	if cfg == nil {
		return nil
	}

	return &platformredis.Config{
		Host:     cfg.Host,
		Port:     cfg.Port,
		Password: cfg.Password,
		DB:       cfg.DB,
		PoolSize: cfg.PoolSize,
	}
}
