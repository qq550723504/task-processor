package tests

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAccountComposeCommercialSchemaUsesGORMUniqueConstraintNames(t *testing.T) {
	path := filepath.Join("..", "deployments", "docker", "referrals-compose", "terraform", "commercial-schema.sql")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read commercial schema: %v", err)
	}

	schema := strings.Join(strings.Fields(string(content)), " ")
	for _, want := range []string{
		"CONSTRAINT uni_saas_tenant_subscriptions_tenant_id UNIQUE (tenant_id)",
		"CONSTRAINT idx_saas_tenant_module UNIQUE (tenant_id, module_code)",
		"CONSTRAINT uni_saas_usage_events_reversal_of UNIQUE (reversal_of)",
		"CONSTRAINT idx_saas_usage_event_tenant_idempotency_key UNIQUE (tenant_id, idempotency_key)",
		"CONSTRAINT uni_saas_usage_event_outbox_event_id UNIQUE (event_id)",
	} {
		if !strings.Contains(schema, want) {
			t.Fatalf("commercial schema must declare the GORM unique constraint as %q; schema contains no matching contract", want)
		}
	}
}
