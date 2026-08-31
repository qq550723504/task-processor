package httpapi

import (
	"reflect"
	"testing"

	"task-processor/internal/core/config"
	platformredis "task-processor/internal/platform/redis"
)

func TestPlatformRedisConfigPreservesAllFields(t *testing.T) {
	in := &config.RedisConfig{Host: "redis", Port: 6380, Password: "secret", DB: 4, PoolSize: 23}
	want := &platformredis.Config{Host: "redis", Port: 6380, Password: "secret", DB: 4, PoolSize: 23}
	if got := platformRedisConfig(in); !reflect.DeepEqual(got, want) {
		t.Fatalf("config = %#v, want %#v", got, want)
	}
}
