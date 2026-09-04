package storecenter

import (
	"errors"
	"strings"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	_ "modernc.org/sqlite"
)

func TestStoreServiceConstraintDefinitionsMirrorDomainInvariants(t *testing.T) {
	definitions := storeServiceConstraintDefinitions()
	if len(definitions) != 5 {
		t.Fatalf("constraint count = %d, want 5", len(definitions))
	}
	wantFragments := map[string][]string{
		"workbench_stores_record_status_nn": {
			"record_status IS NOT NULL",
		},
		"workbench_stores_record_status_enum": {
			"record_status IN ('provisioning', 'active', 'deleting', 'deleted')",
		},
		"workbench_stores_service_status_enum": {
			"service_status IS NULL", "'pending_activation'", "'active'", "'expired'", "'suspended'",
		},
		"workbench_stores_record_service_shape": {
			"record_status = 'active'", "service_status IS NOT NULL", "record_status IN ('provisioning', 'deleting', 'deleted')", "service_status IS NULL",
		},
		"workbench_stores_service_timestamp_shape": {
			"service_status = 'pending_activation'", "service_status IN ('active', 'expired')", "service_status = 'suspended'", "service_expires_at > service_started_at",
		},
	}
	for _, definition := range definitions {
		fragments, ok := wantFragments[definition.Name]
		if !ok {
			t.Fatalf("unexpected constraint %q", definition.Name)
		}
		if definition.Marker == "" {
			t.Fatalf("constraint %q has no ownership marker", definition.Name)
		}
		for _, fragment := range fragments {
			if !strings.Contains(definition.Expression, fragment) {
				t.Fatalf("constraint %q expression missing %q: %s", definition.Name, fragment, definition.Expression)
			}
		}
		delete(wantFragments, definition.Name)
	}
	if len(wantFragments) != 0 {
		t.Fatalf("missing constraints: %v", wantFragments)
	}
}

func TestStoreServiceConstraintsRejectNonPostgresDatabase(t *testing.T) {
	db, err := gorm.Open(sqlite.Dialector{DriverName: "sqlite", DSN: ":memory:"}, &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	migrator, err := NewGormStoreHistoryMigrator(db, NoAuthoritativeHistorySourceManifest{
		SchemaVersion:     NoAuthoritativeHistorySourceManifestV1,
		DecisionReference: "product-decision:store-service-history:phase1",
		ApprovedBy:        "repository-owner",
		ApprovedAt:        time.Date(2026, 9, 4, 14, 0, 0, 0, time.UTC),
	}, "store-history-migration", time.Now)
	if err != nil {
		t.Fatal(err)
	}
	_, err = migrator.ApplyConstraints(t.Context(), StoreServiceConstraintOptions{LockTimeout: time.Second, StatementTimeout: time.Minute})
	if !errors.Is(err, ErrStoreServiceConstraintsUnsupported) {
		t.Fatalf("ApplyConstraints() error = %v, want PostgreSQL-only rejection", err)
	}
}

func TestPostgresStringLiteralEscapesQuotes(t *testing.T) {
	if got, want := postgresStringLiteral("owner's-marker"), "'owner''s-marker'"; got != want {
		t.Fatalf("postgresStringLiteral() = %q, want %q", got, want)
	}
}

func TestStoreServiceConstraintOptionsRequireBoundedTimeouts(t *testing.T) {
	for _, options := range []StoreServiceConstraintOptions{
		{},
		{LockTimeout: time.Second},
		{StatementTimeout: time.Second},
		{LockTimeout: -time.Second, StatementTimeout: time.Second},
		{LockTimeout: 31 * time.Second, StatementTimeout: time.Second},
		{LockTimeout: time.Second, StatementTimeout: 31 * time.Minute},
	} {
		if err := options.Validate(); err == nil {
			t.Fatalf("options %+v accepted", options)
		}
	}
	if err := (StoreServiceConstraintOptions{LockTimeout: time.Second, StatementTimeout: time.Minute}).Validate(); err != nil {
		t.Fatalf("valid options rejected: %v", err)
	}
}
