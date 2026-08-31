package httpapi

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"

	"task-processor/internal/core/config"
	kernelmodule "task-processor/internal/kernel/module"
	"task-processor/internal/listingsubscription"
	"task-processor/internal/storecenter"
	storecenterhttpapi "task-processor/internal/storecenter/httpapi"
	workbenchcontexthttpapi "task-processor/internal/workbenchcontext/httpapi"
)

func TestStoreCenterBuildIsInertWhenWorkbenchIsDisabled(t *testing.T) {
	for _, cfg := range []*config.Config{nil, {Workbench: config.WorkbenchConfig{Enabled: false}}} {
		calls := 0
		result, err := buildStoreCenterModule(cfg, logrus.New(), storeCenterFactories{
			openDatabase: func(*config.DatabaseConfig) (*gorm.DB, error) {
				calls++
				return nil, errors.New("must not be called")
			},
		})
		if err != nil {
			t.Fatalf("buildStoreCenterModule() error = %v", err)
		}
		if result.module != nil || result.closer != nil {
			t.Fatalf("disabled build returned runtime resources: %+v", result)
		}
		if calls != 0 {
			t.Fatalf("disabled build made %d factory calls", calls)
		}
	}
}

func TestStoreCenterBuildFailsClosedWithoutDatabase(t *testing.T) {
	calls := 0
	result, err := buildStoreCenterModule(&config.Config{Workbench: config.WorkbenchConfig{Enabled: true}}, logrus.New(), storeCenterFactories{
		openDatabase: func(*config.DatabaseConfig) (*gorm.DB, error) {
			calls++
			return nil, errors.New("must not be called")
		},
	})
	if err == nil {
		t.Fatal("enabled Workbench without database was accepted")
	}
	if result.module != nil || result.closer != nil {
		t.Fatalf("failed build returned runtime resources: %+v", result)
	}
	if calls != 0 {
		t.Fatalf("missing database made %d factory calls", calls)
	}
}

func TestStoreCenterBuildSharesOneDatabaseAcrossPersistenceAndReturnsOneCloser(t *testing.T) {
	db := &gorm.DB{}
	factories := defaultStoreCenterFactories()
	openCalls, closeCalls := 0, 0
	var storeDB, subscriptionDB, auditDB *gorm.DB
	factories.openDatabase = func(*config.DatabaseConfig) (*gorm.DB, error) {
		openCalls++
		return db, nil
	}
	factories.closeDatabase = func(_ *config.DatabaseConfig, got *gorm.DB) error {
		if got != db {
			t.Fatal("closer received a different database")
		}
		closeCalls++
		return nil
	}
	factories.newStoreRepository = func(got *gorm.DB) (storecenter.Repository, error) {
		storeDB = got
		return storecenter.NewGormStoreRepository(got)
	}
	factories.newSubscriptionRepository = func(got *gorm.DB) *listingsubscription.GormRepository {
		subscriptionDB = got
		return listingsubscription.NewGormRepository(got)
	}
	factories.newAuditRepository = func(got *gorm.DB) (storecenter.AuditRepository, error) {
		auditDB = got
		return storecenter.NewGormAuditRepository(got)
	}

	result, err := buildStoreCenterModule(enabledStoreCenterConfig(), logrus.New(), factories)
	if err != nil {
		t.Fatalf("buildStoreCenterModule() error = %v", err)
	}
	if result.module == nil || result.closer == nil {
		t.Fatalf("enabled build result = %+v", result)
	}
	if openCalls != 1 {
		t.Fatalf("database open calls = %d, want 1", openCalls)
	}
	if storeDB != db || subscriptionDB != db || auditDB != db {
		t.Fatalf("persistence DBs differ: store=%p subscription=%p audit=%p want=%p", storeDB, subscriptionDB, auditDB, db)
	}
	if closeCalls != 0 {
		t.Fatalf("database closed before runtime shutdown: %d", closeCalls)
	}
	if err := result.closer(); err != nil {
		t.Fatalf("close Store Center database: %v", err)
	}
	if closeCalls != 1 {
		t.Fatalf("database close calls = %d, want 1", closeCalls)
	}
}

func TestStoreCenterBuildCleansUpAndReturnsSafeErrorsAfterOpen(t *testing.T) {
	wantErr := errors.New("postgres://operator:super-secret@database.internal/workbench")
	for _, test := range []struct {
		name      string
		configure func(*storeCenterFactories)
	}{
		{
			name: "repository constructor",
			configure: func(factories *storeCenterFactories) {
				factories.newAuditRepository = func(*gorm.DB) (storecenter.AuditRepository, error) { return nil, wantErr }
			},
		},
		{
			name: "module constructor",
			configure: func(factories *storeCenterFactories) {
				factories.newModule = func(*storecenterhttpapi.Handler) (kernelmodule.Module, error) { return nil, wantErr }
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			db := &gorm.DB{}
			factories := defaultStoreCenterFactories()
			closeCalls := 0
			factories.openDatabase = func(*config.DatabaseConfig) (*gorm.DB, error) { return db, nil }
			factories.closeDatabase = func(*config.DatabaseConfig, *gorm.DB) error {
				closeCalls++
				return nil
			}
			test.configure(&factories)

			result, err := buildStoreCenterModule(enabledStoreCenterConfig(), logrus.New(), factories)
			if !errors.Is(err, wantErr) {
				t.Fatalf("buildStoreCenterModule() error = %v, want wrapped constructor error", err)
			}
			if strings.Contains(err.Error(), "super-secret") {
				t.Fatalf("startup error leaked credential text: %v", err)
			}
			if result.module != nil || result.closer != nil {
				t.Fatalf("failed build returned runtime resources: %+v", result)
			}
			if closeCalls != 1 {
				t.Fatalf("cleanup close calls = %d, want 1", closeCalls)
			}
		})
	}
}

func TestStoreCenterRuntimeConnectionProviderReportsUnavailable(t *testing.T) {
	status, err := (unavailableConnectionStatusProvider{}).Status(context.Background(), storecenter.ConnectionStatusInput{
		OrganizationID: "opaque-organization-id",
		StoreID:        "4c6741a2-b62a-4ca2-abcd-da2fcfc2be48",
		Platform:       storecenter.PlatformShein,
		ConnectionRef:  "opaque-connection-reference",
	})
	if err != nil {
		t.Fatalf("Status() error = %v", err)
	}
	if status != storecenter.ConnectionStatusUnavailable {
		t.Fatalf("Status() = %q, want unavailable", status)
	}
}

func TestStoreCenterCompositionBuildsAfterContextAndRegistersOneCloser(t *testing.T) {
	order := make([]string, 0, 2)
	contextModule := workbenchcontexthttpapi.NewModule(workbenchcontexthttpapi.NewHandler())
	storeModule := storeCenterHTTPModuleForTest(t)
	authDependencies := newRouteAuthDependencies()
	closed := 0
	builder := httpFeatureCompositionBuilder{
		buildWorkbenchContext: func(*config.Config, *logrus.Logger) (workbenchContextBuildResult, error) {
			order = append(order, "context")
			return workbenchContextBuildResult{module: contextModule, authDependencies: &authDependencies}, nil
		},
		buildStoreCenter: func(*config.Config, *logrus.Logger) (storeCenterBuildResult, error) {
			order = append(order, "store-center")
			return storeCenterBuildResult{module: storeModule, closer: func() error { closed++; return nil }}, nil
		},
	}
	deps := &runtimeDeps{shared: &sharedRuntimeDeps{cfg: enabledStoreCenterConfig()}}
	var composition httpFeatureComposition
	if err := builder.buildWorkbenchModules(logrus.New(), deps, &composition); err != nil {
		t.Fatalf("buildWorkbenchModules() error = %v", err)
	}
	if strings.Join(order, ",") != "context,store-center" {
		t.Fatalf("Workbench build order = %v", order)
	}
	if composition.workbenchContextModule == nil || composition.storeCenterModule == nil || composition.workbenchAuthDependencies == nil {
		t.Fatalf("incomplete Workbench composition: %+v", composition)
	}
	if len(deps.shared.closers) != 1 {
		t.Fatalf("registered closers = %d, want 1", len(deps.shared.closers))
	}
	if err := deps.shared.closers[0](); err != nil {
		t.Fatal(err)
	}
	if closed != 1 {
		t.Fatalf("Store Center closer calls = %d, want 1", closed)
	}
}

func TestStoreCenterRoutesRegisterOnceAfterWorkbenchContextRoutes(t *testing.T) {
	cfg := enabledStoreCenterConfig()
	composition := httpFeatureComposition{
		workbenchContextModule: workbenchcontexthttpapi.NewModule(workbenchcontexthttpapi.NewHandler()),
		storeCenterModule:      storeCenterHTTPModuleForTest(t),
	}
	bundle, err := buildRuntimeBundleFromModules(cfg, composition.runtimeModules())
	if err != nil {
		t.Fatalf("buildRuntimeBundleFromModules() error = %v", err)
	}
	lastContextIndex, firstStoreIndex, storeRoutes := -1, -1, 0
	for index, route := range bundle.routes {
		switch route.Module {
		case workbenchcontexthttpapi.ModuleName:
			lastContextIndex = index
		case storecenterhttpapi.ModuleName:
			if firstStoreIndex == -1 {
				firstStoreIndex = index
			}
			storeRoutes++
		}
	}
	if lastContextIndex < 0 || firstStoreIndex <= lastContextIndex {
		t.Fatalf("route order context=%d store=%d", lastContextIndex, firstStoreIndex)
	}
	if storeRoutes != 7 {
		t.Fatalf("Store Center route count = %d, want 7", storeRoutes)
	}
}

func enabledStoreCenterConfig() *config.Config {
	return &config.Config{Workbench: config.WorkbenchConfig{Enabled: true}, Database: &config.DatabaseConfig{}}
}

type inertStoreService struct {
	storecenterhttpapi.StoreService
}

func storeCenterHTTPModuleForTest(t *testing.T) kernelmodule.Module {
	t.Helper()
	handler, err := storecenterhttpapi.NewHandler(inertStoreService{})
	if err != nil {
		t.Fatalf("NewHandler() error = %v", err)
	}
	return storecenterhttpapi.NewModule(handler)
}
