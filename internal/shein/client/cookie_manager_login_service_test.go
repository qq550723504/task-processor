package client

import "testing"

func TestLoadSheinLoginServiceConfigPrefersConfiguredOverride(t *testing.T) {
	t.Setenv("TASK_PROCESSOR_SHEIN_LOGIN_SERVICE_BASE_URL", "http://env-login:8000")
	t.Setenv("TASK_PROCESSOR_SHEIN_LOGIN_SERVICE_SHARED_KEY", "env-key")
	t.Setenv("TASK_PROCESSOR_SHEIN_LOGIN_SERVICE_TENANT_ID", "99")
	t.Setenv("TASK_PROCESSOR_SHEIN_LOGIN_SERVICE_IDENTIFIER", "999")
	ConfigureLoginService("http://config-login:8000", "config-key", "1", "2")
	t.Cleanup(func() {
		ConfigureLoginService("", "", "", "")
	})

	cfg := loadSheinLoginServiceConfig()
	if cfg.baseURL != "http://config-login:8000" {
		t.Fatalf("baseURL = %q, want configured value", cfg.baseURL)
	}
	if cfg.sharedKey != "config-key" {
		t.Fatalf("sharedKey = %q, want configured value", cfg.sharedKey)
	}
	if cfg.tenantID != "1" {
		t.Fatalf("tenantID = %q, want configured value", cfg.tenantID)
	}
	if cfg.identifier != "2" {
		t.Fatalf("identifier = %q, want configured value", cfg.identifier)
	}
}

func TestLoadSheinLoginServiceConfigFallsBackToEnv(t *testing.T) {
	ConfigureLoginService("", "", "", "")
	t.Setenv("TASK_PROCESSOR_SHEIN_LOGIN_SERVICE_BASE_URL", "http://env-login:8000")
	t.Setenv("TASK_PROCESSOR_SHEIN_LOGIN_SERVICE_SHARED_KEY", "env-key")
	t.Setenv("TASK_PROCESSOR_SHEIN_LOGIN_SERVICE_TENANT_ID", "99")
	t.Setenv("TASK_PROCESSOR_SHEIN_LOGIN_SERVICE_IDENTIFIER", "999")

	cfg := loadSheinLoginServiceConfig()
	if cfg.baseURL != "http://env-login:8000" {
		t.Fatalf("baseURL = %q, want env value", cfg.baseURL)
	}
	if cfg.sharedKey != "env-key" {
		t.Fatalf("sharedKey = %q, want env value", cfg.sharedKey)
	}
	if cfg.tenantID != "99" {
		t.Fatalf("tenantID = %q, want env value", cfg.tenantID)
	}
	if cfg.identifier != "999" {
		t.Fatalf("identifier = %q, want env value", cfg.identifier)
	}
}

func TestCookieManagerUsesConfiguredLoginServiceTenantAndIdentifier(t *testing.T) {
	ConfigureLoginService("", "", "1", "2")
	t.Cleanup(func() {
		ConfigureLoginService("", "", "", "")
	})

	cm := NewCookieManager(869, nil)
	if got := cm.forceLoginTenantID(); got != 1 {
		t.Fatalf("force login tenantID = %d, want configured tenant", got)
	}
	if got := cm.forceLoginIdentifier(); got != "2" {
		t.Fatalf("force login identifier = %q, want configured identifier", got)
	}
}
