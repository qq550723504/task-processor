package config

import "testing"

func TestEffectiveSDSAuthRedisFallsBackToGlobalRedis(t *testing.T) {
	cfg := &Config{
		Redis: &RedisConfig{
			Host:     "redis.local",
			Port:     6379,
			Password: "secret",
			DB:       9,
			PoolSize: 12,
		},
	}

	got := cfg.EffectiveSDSAuthRedis()
	if got.Host != "redis.local" || got.Port != 6379 || got.Password != "secret" || got.DB != 9 || got.PoolSize != 12 {
		t.Fatalf("EffectiveSDSAuthRedis() = %+v", got)
	}
}

func TestEffectiveSDSAuthRedisPrefersExplicitSDSConfig(t *testing.T) {
	cfg := &Config{
		Redis: &RedisConfig{Host: "global.redis", Port: 6379, DB: 0},
		Platforms: PlatformsConfig{
			SDS: SDSPlatformConfig{
				AuthRedis: RedisConfig{
					Host: "sds.redis",
					Port: 6381,
					DB:   9,
				},
			},
		},
	}

	got := cfg.EffectiveSDSAuthRedis()
	if got.Host != "sds.redis" || got.Port != 6381 || got.DB != 9 {
		t.Fatalf("EffectiveSDSAuthRedis() = %+v", got)
	}
}
