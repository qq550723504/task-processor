package bootstrap

import (
	"net"
	"os"
	"strconv"
	"testing"

	miniredis "github.com/alicebob/miniredis/v2"
	"github.com/stretchr/testify/require"

	"task-processor/internal/core/config"
)

func TestHasRedisStoreConfig(t *testing.T) {
	t.Parallel()

	require.False(t, HasRedisStoreConfig(nil))
	require.False(t, HasRedisStoreConfig(&config.Config{}))
	require.False(t, HasRedisStoreConfig(&config.Config{
		Platforms: config.PlatformsConfig{
			Shein: config.PlatformConfig{
				CookieRedis: config.RedisConfig{Host: "   "},
			},
		},
	}))
	require.True(t, HasRedisStoreConfig(&config.Config{
		Platforms: config.PlatformsConfig{
			Shein: config.PlatformConfig{
				CookieRedis: config.RedisConfig{Host: "127.0.0.1"},
			},
		},
	}))
}

func TestBuildRedisStoreReturnsNilWithoutRedisConfig(t *testing.T) {
	t.Parallel()

	store, err := BuildRedisStore(&config.Config{})
	require.NoError(t, err)
	require.Nil(t, store)
}

func TestBuildRedisStoreReturnsStoreWhenRedisConfigured(t *testing.T) {
	t.Parallel()

	redisServer := miniredis.RunT(t)
	host, portText, err := net.SplitHostPort(redisServer.Addr())
	require.NoError(t, err)
	port, err := strconv.Atoi(portText)
	require.NoError(t, err)

	store, err := BuildRedisStore(&config.Config{
		Platforms: config.PlatformsConfig{
			Shein: config.PlatformConfig{
				CookieRedis: config.RedisConfig{Host: host, Port: port},
			},
		},
	})
	require.NoError(t, err)
	require.NotNil(t, store)
	require.NoError(t, store.Close())
}

func TestBuildHandlerUsesRedisStoreConfigHelper(t *testing.T) {
	t.Parallel()

	src := mustReadFile(t, "build.go")
	require.Contains(t, src, "HasRedisStoreConfig(input.Config)")
	require.NotContains(t, src, "strings.TrimSpace(redisCfg.Host)")
}

func mustReadFile(t *testing.T, name string) string {
	t.Helper()

	data, err := os.ReadFile(name)
	require.NoError(t, err)
	return string(data)
}
