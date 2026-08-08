package httpapi

import (
	"fmt"
	"os"
	"strings"
	"sync"

	"gorm.io/gorm"

	"task-processor/internal/core/config"
	listingkitschema "task-processor/internal/listingkit/schema"
)

type repositorySchemaBootstrapper struct {
	mu      sync.Mutex
	entries map[string]*repositorySchemaBootstrapEntry
}

type repositorySchemaBootstrapEntry struct {
	once sync.Once
	err  error
}

func newRepositorySchemaBootstrapper() *repositorySchemaBootstrapper {
	return &repositorySchemaBootstrapper{
		entries: make(map[string]*repositorySchemaBootstrapEntry),
	}
}

func (b *repositorySchemaBootstrapper) ensure(cfg *config.DatabaseConfig, run func() error) error {
	if b == nil || run == nil {
		return nil
	}

	key := repositorySchemaKey(cfg)

	b.mu.Lock()
	entry := b.entries[key]
	if entry == nil {
		entry = &repositorySchemaBootstrapEntry{}
		b.entries[key] = entry
	}
	b.mu.Unlock()

	entry.once.Do(func() {
		entry.err = run()
	})
	return entry.err
}

func repositorySchemaKey(cfg *config.DatabaseConfig) string {
	if cfg == nil {
		return ""
	}
	return fmt.Sprintf("%s:%d:%s:%s", cfg.Host, cfg.Port, cfg.User, cfg.Database)
}

// listingKitRepositorySchemaBootstrapper is the default bootstrapper for schema migrations.
// It can be overridden in tests for isolation.
var listingKitRepositorySchemaBootstrapper = newRepositorySchemaBootstrapper()

// SetRepositorySchemaBootstrapper allows tests to override the global bootstrapper.
// This should only be used in test code.
func SetRepositorySchemaBootstrapper(b *repositorySchemaBootstrapper) {
	listingKitRepositorySchemaBootstrapper = b
}

func ensureListingKitRepositorySchema(cfg *config.DatabaseConfig, db *gorm.DB) error {
	if !shouldAutoMigrateListingKitRuntime() {
		return nil
	}
	return listingKitRepositorySchemaBootstrapper.ensure(cfg, func() error {
		return runListingKitRepositoryAutoMigrations(db)
	})
}

func shouldAutoMigrateListingKitRuntime() bool {
	raw := strings.TrimSpace(os.Getenv("TASK_PROCESSOR_LISTINGKIT_RUNTIME_AUTOMIGRATE"))
	if raw == "" {
		return true
	}
	switch strings.ToLower(raw) {
	case "0", "false", "no", "n", "off", "disabled":
		return false
	default:
		return true
	}
}

func AutoMigrateListingKitRuntimeSchema(db *gorm.DB) error {
	return runListingKitRepositoryAutoMigrations(db)
}

func runListingKitRepositoryAutoMigrations(db *gorm.DB) error {
	return listingkitschema.AutoMigrateRuntime(db)
}
