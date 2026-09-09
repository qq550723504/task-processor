//go:build integration

package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"testing"
	"time"

	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestBinaryInitializesEmptyPostgresExactly(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	container, err := tcpostgres.Run(ctx, "postgres:16-alpine", tcpostgres.WithDatabase("sourceaccountregistry"), tcpostgres.WithUsername("sourceaccountregistry"), tcpostgres.WithPassword("sourceaccountregistry"), tcpostgres.BasicWaitStrategies())
	if err != nil {
		t.Fatalf("start PostgreSQL: %v", err)
	}
	t.Cleanup(func() { _ = container.Terminate(context.Background()) })
	host, err := container.Host(ctx)
	if err != nil {
		t.Fatal(err)
	}
	port, err := container.MappedPort(ctx, "5432/tcp")
	if err != nil {
		t.Fatal(err)
	}

	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "database-only.yaml")
	configContents := []byte(fmt.Sprintf("database:\n  host: %s\n  port: %s\n  user: sourceaccountregistry\n  password: sourceaccountregistry\n  database: sourceaccountregistry\n", host, port.Port()))
	if err := os.WriteFile(configPath, configContents, 0o600); err != nil {
		t.Fatal(err)
	}
	binaryName := "source-account-registry-schema-init"
	if runtime.GOOS == "windows" {
		binaryName += ".exe"
	}
	binaryPath := filepath.Join(tempDir, binaryName)
	build := exec.Command("go", "build", "-o", binaryPath, ".")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build schema initializer: %v\n%s", err, output)
	}
	for attempt := 1; attempt <= 2; attempt++ {
		run := exec.CommandContext(ctx, binaryPath, "-config", configPath)
		run.Dir = tempDir
		run.Env = databaseNeutralEnv()
		if output, err := run.CombinedOutput(); err != nil {
			t.Fatalf("schema initializer attempt %d: %v\n%s", attempt, err, output)
		}
	}

	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	var tables []string
	if err := db.Raw(`SELECT table_name FROM information_schema.tables WHERE table_schema = 'public' AND table_type = 'BASE TABLE' ORDER BY table_name`).Scan(&tables).Error; err != nil {
		t.Fatal(err)
	}
	want := []string{"goose_source_account_registry_version", "source_account_operations", "source_account_resources"}
	if !slices.Equal(tables, want) {
		t.Fatalf("public tables = %v, want %v", tables, want)
	}
}
