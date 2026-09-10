package sourceaccountregistryschemainit

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"gorm.io/gorm"

	"task-processor/internal/core/config"
)

func TestRunUsesDatabaseOnlyConfigAndClosesOwnedHandle(t *testing.T) {
	var order []string
	db := &gorm.DB{}
	err := runWithDependencies(context.Background(), "task-owned-config.yaml", Dependencies{
		LoadConfig: func(path string) (*config.Config, error) {
			order = append(order, "config:"+path)
			return &config.Config{Database: &config.DatabaseConfig{Host: "127.0.0.1"}}, nil
		},
		OpenDB:  func(*config.DatabaseConfig) (*gorm.DB, error) { order = append(order, "open"); return db, nil },
		Migrate: func(context.Context, *gorm.DB) error { order = append(order, "migrate"); return nil },
		CloseDB: func(*gorm.DB) error { order = append(order, "close"); return nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"config:task-owned-config.yaml", "open", "migrate", "close"}
	if !reflect.DeepEqual(order, want) {
		t.Fatalf("order=%v, want %v", order, want)
	}
}

func TestRunClosesAfterMigrationFailure(t *testing.T) {
	want := errors.New("migration failed")
	closed := false
	err := runWithDependencies(context.Background(), "config.yaml", Dependencies{
		LoadConfig: func(string) (*config.Config, error) {
			return &config.Config{Database: &config.DatabaseConfig{Host: "127.0.0.1"}}, nil
		},
		OpenDB:  func(*config.DatabaseConfig) (*gorm.DB, error) { return &gorm.DB{}, nil },
		Migrate: func(context.Context, *gorm.DB) error { return want },
		CloseDB: func(*gorm.DB) error { closed = true; return nil },
	})
	if !errors.Is(err, want) || !closed {
		t.Fatalf("runWithDependencies() error=%v closed=%t", err, closed)
	}
}
