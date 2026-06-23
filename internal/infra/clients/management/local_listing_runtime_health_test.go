package management

import (
	"strings"
	"testing"

	"task-processor/internal/core/config"

	miniredis "github.com/alicebob/miniredis/v2"
	goredis "github.com/redis/go-redis/v9"
)

func TestValidateLocalListingRuntimeRequiresLocalProvider(t *testing.T) {
	manager := NewClientManager(&config.ManagementConfig{BaseURL: "http://127.0.0.1:1"})

	report, err := manager.ValidateLocalListingRuntime()
	if err == nil {
		t.Fatal("ValidateLocalListingRuntime() error = nil, want missing local provider error")
	}
	if report.Ready {
		t.Fatalf("report.Ready = true, want false")
	}
	if !strings.Contains(err.Error(), "local management data provider is not configured") {
		t.Fatalf("error = %v, want local provider message", err)
	}
}

func TestValidateLocalListingRuntimeRequiresLocalRedis(t *testing.T) {
	provider := newSQLiteProvider(t)
	manager := NewClientManager(&config.ManagementConfig{BaseURL: "http://127.0.0.1:1"})
	manager.SetLocalDataProvider(provider)

	report, err := manager.ValidateLocalListingRuntime()
	if err == nil {
		t.Fatal("ValidateLocalListingRuntime() error = nil, want local redis error")
	}
	if report.DB != true || report.Redis {
		t.Fatalf("report db/redis = %v/%v, want true/false", report.DB, report.Redis)
	}
	if !strings.Contains(err.Error(), "local redis is not configured") {
		t.Fatalf("error = %v, want local redis message", err)
	}
}

func TestValidateLocalListingRuntimeReportsReady(t *testing.T) {
	provider := newSQLiteProvider(t)
	redisServer := miniredis.RunT(t)
	provider.redis = goredis.NewClient(&goredis.Options{Addr: redisServer.Addr()})
	t.Cleanup(func() { _ = provider.redis.Close() })

	manager := NewClientManager(&config.ManagementConfig{BaseURL: "http://127.0.0.1:1"})
	manager.SetLocalDataProvider(provider)

	report, err := manager.ValidateLocalListingRuntime()
	if err != nil {
		t.Fatalf("ValidateLocalListingRuntime() error = %v", err)
	}
	if !report.Ready {
		t.Fatalf("report.Ready = false, report = %+v", report)
	}
	if !report.ImportTask || !report.Store || !report.ProductImportMapping || !report.DailyQuota {
		t.Fatalf("report missing required capability: %+v", report)
	}
}
