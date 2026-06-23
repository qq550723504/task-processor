package listingcontrol

import (
	"context"
	"errors"
	"flag"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	controllib "task-processor/internal/listingcontrol"

	"github.com/sirupsen/logrus"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestResolveConfigPathAndParseFlags(t *testing.T) {
	if got := ResolveConfigPath("", ""); got != "config/config-prod.yaml" {
		t.Fatalf("default config path = %q", got)
	}
	if got := ResolveConfigPath("config/custom.yaml", "legacy.yaml"); got != "config/custom.yaml" {
		t.Fatalf("config path precedence = %q", got)
	}
	if got := ResolveConfigPath("", "legacy.yaml"); got != "legacy.yaml" {
		t.Fatalf("legacy config path = %q", got)
	}

	fs := flag.NewFlagSet("listing-control-plane", flag.ContinueOnError)
	opts := ParseFlagsFrom(fs,
		"--config", "config/runtime.yaml",
		"--app-config", "legacy.yaml",
		"--log-level", "debug",
		"--force",
	)
	if opts.Config != "config/runtime.yaml" || opts.AppConfig != "legacy.yaml" || opts.LogLevel != "debug" || !opts.Force {
		t.Fatalf("unexpected parsed options: %+v", opts)
	}
}

func TestRunReturnsNilWhenDisabledWithoutInitializingDependencies(t *testing.T) {
	configPath := writeRuntimeConfig(t, `
openai:
  apiKey: "test-key"
management:
  clientSecret: "test-secret"
listingControlPlane:
  enabled: false
`)
	deps := newFakeRuntimeDeps()

	if err := runWithDependencies(context.Background(), Options{Config: configPath, LogLevel: "error"}, deps.runtimeDependencies); err != nil {
		t.Fatalf("runWithDependencies returned error: %v", err)
	}
	if deps.dbOpened || deps.redisOpened || deps.rabbitConnected {
		t.Fatalf("dependencies initialized for disabled control plane: %+v", deps)
	}
}

func TestRunErrorsWhenEnabledAndRequiredConfigsMissing(t *testing.T) {
	configPath := writeRuntimeConfig(t, `
openai:
  apiKey: "test-key"
management:
  clientSecret: "test-secret"
listingControlPlane:
  enabled: true
`)
	deps := newFakeRuntimeDeps()

	err := runWithDependencies(context.Background(), Options{Config: configPath, LogLevel: "error"}, deps.runtimeDependencies)
	if err == nil {
		t.Fatal("expected missing dependency config error")
	}
	if !strings.Contains(err.Error(), "RabbitMQ") {
		t.Fatalf("expected RabbitMQ config error first, got %v", err)
	}
	if deps.dbOpened || deps.redisOpened || deps.rabbitConnected {
		t.Fatalf("dependencies initialized before config validation: %+v", deps)
	}
}

func TestDirectStoreSourceMapsListingStoreRowsWithoutOwnerScope(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.Exec(`CREATE TABLE listing_store (
		id integer primary key,
		tenant_id integer not null,
		owner_user_id text,
		name text,
		platform text not null,
		status integer not null,
		enable_auto_listing boolean,
		deleted integer not null default 0
	)`).Error; err != nil {
		t.Fatalf("create listing_store: %v", err)
	}
	if err := db.Exec(`INSERT INTO listing_store (id, tenant_id, owner_user_id, name, platform, status, enable_auto_listing, deleted) VALUES
		(100, 10, 'owner-a', 'ready', 'SHEIN', 0, true, 0),
		(101, 20, 'owner-b', 'disabled', 'shein', 1, false, 0),
		(102, 30, 'owner-c', 'deleted', 'shein', 0, true, 1),
		(103, 40, 'owner-d', 'other', 'temu', 0, true, 0)`).Error; err != nil {
		t.Fatalf("seed listing_store: %v", err)
	}

	source := NewDirectStoreSource(db)
	stores, err := source.ListEnabledAutoListingStores(context.Background(), "shein")
	if err != nil {
		t.Fatalf("ListEnabledAutoListingStores returned error: %v", err)
	}

	if len(stores) != 2 {
		t.Fatalf("expected 2 non-deleted shein rows, got %d: %+v", len(stores), stores)
	}
	if stores[0].TenantID != 10 || stores[0].StoreID != 100 || stores[0].Name != "ready" || stores[0].Platform != "SHEIN" {
		t.Fatalf("unexpected first store mapping: %+v", stores[0])
	}
	if stores[0].EnableAutoListing == nil || !*stores[0].EnableAutoListing {
		t.Fatalf("expected first auto-listing flag true: %+v", stores[0])
	}
	if stores[1].TenantID != 20 || stores[1].StoreID != 101 || stores[1].Status != 1 {
		t.Fatalf("expected disabled row to be included for readiness reasoning: %+v", stores[1])
	}
	if stores[1].EnableAutoListing == nil || *stores[1].EnableAutoListing {
		t.Fatalf("expected second auto-listing flag false: %+v", stores[1])
	}
}

func TestControlPlaneServiceRunsRecoveryBeforeDispatchAndStopsOnContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	recovery := &fakeOnceRunner{name: "recovery"}
	var order []string
	service := controlPlaneService{
		Recovery: recovery.run(&order),
		Dispatch: func(ctx context.Context) (controllib.DispatchSummary, error) {
			order = append(order, "dispatch")
			cancel()
			return controllib.DispatchSummary{}, nil
		},
		ScanInterval: time.Hour,
	}

	if err := service.Run(ctx); err != nil {
		t.Fatalf("service Run returned error: %v", err)
	}
	if !reflect.DeepEqual(order, []string{"recovery", "dispatch"}) {
		t.Fatalf("execution order = %v", order)
	}
}

func TestControlPlaneServiceDoesNotDispatchAfterRecoveryError(t *testing.T) {
	recoveryErr := errors.New("recovery failed")
	var order []string
	service := controlPlaneService{
		Recovery: func(ctx context.Context) (controllib.RecoverySummary, error) {
			order = append(order, "recovery")
			return controllib.RecoverySummary{}, recoveryErr
		},
		Dispatch: func(ctx context.Context) (controllib.DispatchSummary, error) {
			order = append(order, "dispatch")
			return controllib.DispatchSummary{}, nil
		},
		ScanInterval: time.Hour,
	}

	err := service.Run(context.Background())
	if !errors.Is(err, recoveryErr) {
		t.Fatalf("expected recovery error, got %v", err)
	}
	if !reflect.DeepEqual(order, []string{"recovery"}) {
		t.Fatalf("execution order = %v", order)
	}
}

type fakeOnceRunner struct {
	name string
}

func (f *fakeOnceRunner) run(order *[]string) func(context.Context) (controllib.RecoverySummary, error) {
	return func(ctx context.Context) (controllib.RecoverySummary, error) {
		*order = append(*order, f.name)
		return controllib.RecoverySummary{}, nil
	}
}

type fakeRuntimeDeps struct {
	runtimeDependencies
	dbOpened        bool
	redisOpened     bool
	rabbitConnected bool
}

func newFakeRuntimeDeps() *fakeRuntimeDeps {
	deps := &fakeRuntimeDeps{}
	deps.runtimeDependencies = defaultRuntimeDependencies()
	deps.OpenDB = func(ctx context.Context, cfg databaseConfig) (dbHandle, error) {
		deps.dbOpened = true
		return nil, nil
	}
	deps.OpenRedis = func(ctx context.Context, cfg redisConfig) (redisRuntime, error) {
		deps.redisOpened = true
		return nil, nil
	}
	deps.OpenRabbitMQ = func(ctx context.Context, cfg rabbitConfig, logger *logrus.Logger) (rabbitRuntime, error) {
		deps.rabbitConnected = true
		return nil, nil
	}
	return deps
}

func writeRuntimeConfig(t *testing.T, body string) string {
	t.Helper()

	path := t.TempDir() + string(os.PathSeparator) + "config.yaml"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}
