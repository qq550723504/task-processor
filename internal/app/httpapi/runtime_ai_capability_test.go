package httpapi

import (
	"errors"
	"strings"
	"testing"

	"github.com/sirupsen/logrus"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	_ "modernc.org/sqlite"

	"task-processor/internal/aicapability"
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
	if deps.asyncJobStore != nil {
		t.Fatal("expected legacy mode to omit async job binding store")
	}
	if len(deps.closers) != 0 {
		t.Fatalf("closers = %d, want 0", len(deps.closers))
	}
}

func TestBuildAICapabilityRuntimeDepsRequiresDatabaseOutsideLegacy(t *testing.T) {
	for _, mode := range []string{"shadow", "active", "SHADOW", "Active"} {
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
			if strings.Contains(err.Error(), "parse AI capability") {
				t.Fatalf("error = %q, case-insensitive validated mode must reach runtime dependency checks", err)
			}
		})
	}
}

func TestBuildAICapabilityRuntimeDepsRequiresDatabaseWhenProductImageSceneEnabled(t *testing.T) {
	_, err := buildAICapabilityRuntimeDeps(&config.Config{
		AICapability: config.AICapabilityConfig{
			StudioImageRoutingMode:   "legacy",
			ProductImageSceneEnabled: true,
		},
	}, logrus.New())
	if err == nil {
		t.Fatal("expected missing database error")
	}
	if !strings.Contains(err.Error(), "AI capability") {
		t.Fatalf("error = %q, want AI capability resource context", err)
	}
}

func TestBuildAICapabilityRuntimeDepsRequiresDatabaseWhenProductEnrichGovernanceEnabled(t *testing.T) {
	_, err := buildAICapabilityRuntimeDeps(&config.Config{
		AICapability: config.AICapabilityConfig{
			StudioImageRoutingMode:   "legacy",
			ProductEnrichTextEnabled: true,
		},
	}, logrus.New())
	if err == nil {
		t.Fatal("expected missing database error")
	}
	if !strings.Contains(err.Error(), "AI capability") {
		t.Fatalf("error = %q, want AI capability resource context", err)
	}
}

func TestProductEnrichInvocationErrorHandlerLogsLedgerFailure(t *testing.T) {
	logger := logrus.New()
	hook := &captureLogHook{}
	logger.AddHook(hook)

	productEnrichInvocationErrorHandler(logger)(aicapability.InvocationRecord{
		InvocationID: "invocation-1",
		Capability:   aicapability.CapabilityProductEnrichListing,
		Operation:    aicapability.OperationProductEnrichJSONGenerate,
	}, errors.New("ledger unavailable"))

	if len(hook.entries) != 1 {
		t.Fatalf("logged entries = %d, want 1", len(hook.entries))
	}
	entry := hook.entries[0]
	if entry.Message != "ai invocation ledger write failed" {
		t.Fatalf("message = %q", entry.Message)
	}
	if entry.Data["invocation_id"] != "invocation-1" || entry.Data["operation"] != string(aicapability.OperationProductEnrichJSONGenerate) {
		t.Fatalf("fields = %#v", entry.Data)
	}
}

func TestUniqueProductEnrichClientNamesPreservesOrderedRuntimeCandidates(t *testing.T) {
	tests := []struct {
		name string
		in   []string
		want []string
	}{
		{name: "text understanding", in: []string{"fast", "default"}, want: []string{"fast", "default"}},
		{name: "vision understanding", in: []string{"vision", "default"}, want: []string{"vision", "default"}},
		{name: "listing and fusion", in: []string{"default"}, want: []string{"default"}},
		{name: "quality scorer", in: []string{"scorer", "default"}, want: []string{"scorer", "default"}},
		{name: "quality default is not duplicated", in: []string{" default ", "default", ""}, want: []string{"default"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := uniqueProductEnrichClientNames(tt.in...)
			if len(got) != len(tt.want) {
				t.Fatalf("clients = %#v, want %#v", got, tt.want)
			}
			for index := range tt.want {
				if got[index] != tt.want[index] {
					t.Fatalf("clients = %#v, want %#v", got, tt.want)
				}
			}
		})
	}
}

type captureLogHook struct {
	entries []*logrus.Entry
}

func (h *captureLogHook) Levels() []logrus.Level { return logrus.AllLevels }

func (h *captureLogHook) Fire(entry *logrus.Entry) error {
	h.entries = append(h.entries, entry)
	return nil
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
	if !db.Migrator().HasTable("ai_async_jobs") {
		t.Fatal("expected ai_async_jobs table to be created")
	}
}
