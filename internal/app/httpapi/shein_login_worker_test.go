package httpapi

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadSheinLoginWorkerConfigAcceptsDatabaseAndRedisOnly(t *testing.T) {
	path := filepath.Join(t.TempDir(), "worker.yaml")
	contents := []byte(`database:
  host: database.internal
  port: 5432
  user: listingkit
  password: test-only
  database: listingkit
platforms:
  shein:
    loginService:
      baseURL: https://login.example.invalid
      sharedKey: must-not-reach-worker
      tenantID: legacy-tenant
      identifier: legacy-store
      merchantName: legacy-merchant
      username: legacy-user
      password: legacy-password
      maxConcurrentLogins: 3
    cookieRedis:
      host: redis.internal
      port: 6379
      password: test-only
      db: 9
`)
	if err := os.WriteFile(path, contents, 0o600); err != nil {
		t.Fatalf("write worker config: %v", err)
	}

	cfg, err := loadSheinLoginWorkerConfig(path)
	if err != nil {
		t.Fatalf("load worker config: %v", err)
	}
	if cfg.Database == nil || cfg.Database.Host != "database.internal" {
		t.Fatalf("database config = %#v", cfg.Database)
	}
	redis := cfg.EffectiveSheinCookieRedis()
	if redis.Host != "redis.internal" || redis.DB != 9 {
		t.Fatalf("cookie redis config = %#v", redis)
	}
	login := cfg.Platforms.Shein.LoginService
	if login.BaseURL != "" || login.SharedKey != "" || login.TenantID != "" || login.Identifier != "" || login.MerchantName != "" || login.Username != "" || login.Password != "" {
		t.Fatalf("worker retained unused login client/account values: %#v", login)
	}
	if login.MaxConcurrentLogins != 3 {
		t.Fatalf("worker max concurrent logins = %d, want 3", login.MaxConcurrentLogins)
	}
}
