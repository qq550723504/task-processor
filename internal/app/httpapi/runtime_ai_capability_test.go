package httpapi

import (
	"strings"
	"testing"

	"github.com/sirupsen/logrus"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	_ "modernc.org/sqlite"

	"task-processor/internal/core/config"
)

func TestBuildAICapabilityRuntimeDepsKeepsLegacyModeDependencyFree(t *testing.T) {
	deps, err := buildAICapabilityRuntimeDeps(&config.Config{
		AICapability: config.AICapabilityConfig{StudioImageRoutingMode: "legacy"},
	}, logrus.New())
	if err != nil {
		t.Fatalf("buildAICapabilityRuntimeDeps() error = %v", err)
	}
	if deps.invocationRecorder != nil {
		t.Fatal("expected legacy mode to omit invocation recorder")
	}
	if len(deps.closers) != 0 {
		t.Fatalf("closers = %d, want 0", len(deps.closers))
	}
}

func TestBuildAICapabilityRuntimeDepsRequiresDatabaseOutsideLegacy(t *testing.T) {
	for _, mode := range []string{"shadow", "active"} {
		t.Run(mode, func(t *testing.T) {
			_, err := buildAICapabilityRuntimeDeps(&config.Config{
				AICapability: config.AICapabilityConfig{StudioImageRoutingMode: mode},
			}, logrus.New())
			if err == nil {
				t.Fatal("expected missing database error")
			}
			if !strings.Contains(err.Error(), "AI capability") {
				t.Fatalf("error = %q, want AI capability resource context", err)
			}
		})
	}
}

func TestAutoMigrateProductListingAPIRuntimeSchemaCreatesAIInvocationsTable(t *testing.T) {
	db, err := gorm.Open(sqlite.Dialector{DriverName: "sqlite", DSN: ":memory:"}, &gorm.Config{})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}

	if err := AutoMigrateProductListingAPIRuntimeSchema(db); err != nil {
		t.Fatalf("AutoMigrateProductListingAPIRuntimeSchema() error = %v", err)
	}
	if !db.Migrator().HasTable("ai_invocations") {
		t.Fatal("expected ai_invocations table to be created")
	}
}
