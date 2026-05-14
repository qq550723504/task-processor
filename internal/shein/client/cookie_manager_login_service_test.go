package client

import "testing"

func TestLoadSheinLoginAccountConfigPrefersConfiguredOverride(t *testing.T) {
	t.Setenv("TASK_PROCESSOR_SHEIN_LOGIN_SERVICE_TENANT_ID", "99")
	t.Setenv("TASK_PROCESSOR_SHEIN_LOGIN_SERVICE_IDENTIFIER", "999")
	ConfigureLoginAccount("1", "2")
	t.Cleanup(func() {
		ConfigureLoginAccount("", "")
	})

	cfg := loadSheinLoginAccountConfig()
	if cfg.tenantID != "1" {
		t.Fatalf("tenantID = %q, want configured value", cfg.tenantID)
	}
	if cfg.identifier != "2" {
		t.Fatalf("identifier = %q, want configured value", cfg.identifier)
	}
}

func TestLoadSheinLoginAccountConfigFallsBackToEnv(t *testing.T) {
	ConfigureLoginAccount("", "")
	t.Setenv("TASK_PROCESSOR_SHEIN_LOGIN_SERVICE_TENANT_ID", "99")
	t.Setenv("TASK_PROCESSOR_SHEIN_LOGIN_SERVICE_IDENTIFIER", "999")

	cfg := loadSheinLoginAccountConfig()
	if cfg.tenantID != "99" {
		t.Fatalf("tenantID = %q, want env value", cfg.tenantID)
	}
	if cfg.identifier != "999" {
		t.Fatalf("identifier = %q, want env value", cfg.identifier)
	}
}

func TestCookieManagerUsesConfiguredLoginServiceTenantAndIdentifier(t *testing.T) {
	ConfigureLoginAccount("1", "2")
	t.Cleanup(func() {
		ConfigureLoginAccount("", "")
	})

	cm := NewCookieManager(869, nil)
	if got := cm.forceLoginTenantID(); got != 1 {
		t.Fatalf("force login tenantID = %d, want configured tenant", got)
	}
	if got := cm.configuredLoginStoreID(); got != 2 {
		t.Fatalf("configured login storeID = %d, want configured identifier", got)
	}
}
