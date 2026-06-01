package bootstrap

import (
	"strings"

	"task-processor/internal/core/config"
	"task-processor/internal/sheinlogin"
)

func HasRedisStoreConfig(cfg *config.Config) bool {
	if cfg == nil {
		return false
	}
	redisCfg := cfg.EffectiveSheinCookieRedis()
	return strings.TrimSpace(redisCfg.Host) != ""
}

func BuildRedisStore(cfg *config.Config) (*sheinlogin.RedisStore, error) {
	if !HasRedisStoreConfig(cfg) {
		return nil, nil
	}
	return sheinlogin.NewRedisStore(cfg.EffectiveSheinCookieRedis())
}
