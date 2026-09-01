package configadapter

import (
	"reflect"
	"testing"

	coreconfig "task-processor/internal/core/config"
	platformredis "task-processor/internal/platform/redis"
)

func TestRedisConfigPreservesRuntimeFields(t *testing.T) {
	t.Parallel()

	in := &coreconfig.RedisConfig{
		Host:     "redis",
		Port:     6380,
		Password: "secret",
		DB:       4,
		PoolSize: 23,
	}

	got := Redis(in)
	want := &platformredis.Config{
		Host:     "redis",
		Port:     6380,
		Password: "secret",
		DB:       4,
		PoolSize: 23,
	}
	if !reflect.DeepEqual(want, got) {
		t.Fatalf("config = %#v, want %#v", got, want)
	}
}

func TestRedisConfigPreservesNil(t *testing.T) {
	t.Parallel()

	if got := Redis(nil); got != nil {
		t.Fatalf("config = %#v, want nil", got)
	}
}
