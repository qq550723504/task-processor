package sdslogin

import (
	"context"
	"encoding/json"
	"net"
	"strconv"
	"testing"

	miniredis "github.com/alicebob/miniredis/v2"

	"task-processor/internal/core/config"
)

func TestBuildHandlerReturnsNilWithoutConfig(t *testing.T) {
	result, err := BuildHandler(nil)
	if err != nil {
		t.Fatalf("BuildHandler() error = %v", err)
	}
	if result != nil {
		t.Fatalf("BuildHandler() = %v, want nil", result)
	}
}

func TestBuildHandlerBuildsServiceAndHandler(t *testing.T) {
	result, err := BuildHandler(&config.Config{})
	if err != nil {
		t.Fatalf("BuildHandler() error = %v", err)
	}
	if result == nil {
		t.Fatal("BuildHandler() returned nil result")
	}
	if result.Service == nil {
		t.Fatal("BuildHandler() returned nil service")
	}
	if result.Handler == nil {
		t.Fatal("BuildHandler() returned nil handler")
	}
}

func TestBuildHandlerPrefersExplicitSDSAuthRedis(t *testing.T) {
	t.Parallel()

	globalRedis := miniredis.RunT(t)
	sdsRedis := miniredis.RunT(t)

	globalHost, globalPortText, err := net.SplitHostPort(globalRedis.Addr())
	if err != nil {
		t.Fatalf("SplitHostPort(global) error = %v", err)
	}
	globalPort, err := strconv.Atoi(globalPortText)
	if err != nil {
		t.Fatalf("Atoi(global) error = %v", err)
	}

	sdsHost, sdsPortText, err := net.SplitHostPort(sdsRedis.Addr())
	if err != nil {
		t.Fatalf("SplitHostPort(sds) error = %v", err)
	}
	sdsPort, err := strconv.Atoi(sdsPortText)
	if err != nil {
		t.Fatalf("Atoi(sds) error = %v", err)
	}

	body, err := json.Marshal(map[string]any{
		"access_token": "token-from-sds-redis",
		"merchant_id":  36811,
		"user_id":      30098709,
	})
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	sdsRedis.Set(sdsSharedAuthStateKey, string(body))

	result, err := BuildHandler(&config.Config{
		Redis: &config.RedisConfig{
			Host: globalHost,
			Port: globalPort,
			DB:   0,
		},
		Platforms: config.PlatformsConfig{
			SDS: config.SDSPlatformConfig{
				LoginService: config.LoginServiceConfig{
					TenantID:   "1",
					Identifier: "869",
				},
				AuthRedis: config.RedisConfig{
					Host: sdsHost,
					Port: sdsPort,
					DB:   0,
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("BuildHandler() error = %v", err)
	}
	if result == nil || result.Service == nil {
		t.Fatalf("BuildHandler() result = %#v", result)
	}

	payload, err := result.Service.LoadAuthState(context.Background(), "1", "869")
	if err != nil {
		t.Fatalf("LoadAuthState() error = %v", err)
	}
	if payload == nil || payload.AccessToken != "token-from-sds-redis" || payload.MerchantID != 36811 {
		t.Fatalf("unexpected payload: %+v", payload)
	}
}
