package httpapi

import (
	"reflect"
	"strings"
	"testing"

	"github.com/sirupsen/logrus"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"task-processor/internal/core/config"
)

func TestBuildServiceFailsClosedBeforeFallbackWithoutTaskRepository(t *testing.T) {
	t.Parallel()

	support := BuildRuntimeSupport(RuntimeSupportInput{})
	bundle, err := BuildService(BuildServiceInput{
		Config:       &config.Config{},
		Logger:       logrus.New(),
		Repositories: support.Repositories,
		Hooks:        support.Hooks,
	})

	if err == nil || !strings.Contains(err.Error(), "core.task") || !strings.Contains(err.Error(), "required") {
		t.Fatalf("BuildService() bundle/error = %v/%v, want explicit core.task repository error", bundle, err)
	}
	if bundle != nil {
		t.Fatalf("BuildService() bundle = %#v, want nil before module or pool construction", bundle)
	}
}

func TestBuildServiceRepositoryContractCarriesTypedValuesNotFactories(t *testing.T) {
	t.Parallel()

	repositoriesType := reflect.TypeOf(BuildServiceRepositories{})
	for i := 0; i < repositoriesType.NumField(); i++ {
		groupField := repositoriesType.Field(i)
		if groupField.Type.Kind() != reflect.Struct {
			t.Fatalf("BuildServiceRepositories.%s kind = %s, want repository group struct", groupField.Name, groupField.Type.Kind())
		}
		for j := 0; j < groupField.Type.NumField(); j++ {
			repositoryField := groupField.Type.Field(j)
			if repositoryField.Type.Kind() == reflect.Func {
				t.Errorf("BuildServiceRepositories.%s.%s kind = func, want typed repository interface value", groupField.Name, repositoryField.Name)
			}
		}
	}
}

func TestBuildPersistentRepositoriesOwnsOneSharedConnection(t *testing.T) {
	t.Parallel()

	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	openCount := 0
	closeCount := 0
	open := func(*config.DatabaseConfig, *logrus.Logger) (*gorm.DB, func() error, error) {
		openCount++
		return db, func() error {
			closeCount++
			return nil
		}, nil
	}

	repositories, closer, err := buildPersistentRepositoriesWithOpener(&config.DatabaseConfig{Host: "persistent"}, logrus.New(), open)
	if err != nil {
		t.Fatalf("buildPersistentRepositoriesWithOpener() error = %v", err)
	}
	if repositories.Core.Task == nil || repositories.Admin.Store == nil {
		t.Fatalf("repositories = %#v, want complete persistent set", repositories)
	}
	if openCount != 1 {
		t.Fatalf("database opens = %d, want 1 for the complete set", openCount)
	}
	if closer == nil {
		t.Fatal("repository set closer is nil")
	}
	if err := closer(); err != nil {
		t.Fatalf("close repository set: %v", err)
	}
	if closeCount != 1 {
		t.Fatalf("database closes = %d, want 1 for the complete set", closeCount)
	}
}
