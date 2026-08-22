package main

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestBinaryAcceptsDatabaseOnlyConfig(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "database-only.yaml")
	configContents := []byte("database:\n  host: 127.0.0.1\n  port: 1\n  user: listingkit\n  password: test-only\n  database: listingkit\n")
	if err := os.WriteFile(configPath, configContents, 0o600); err != nil {
		t.Fatalf("write database-only config: %v", err)
	}

	binaryName := "product-listing-api-schema-migrate"
	if runtime.GOOS == "windows" {
		binaryName += ".exe"
	}
	binaryPath := filepath.Join(tempDir, binaryName)
	build := exec.Command("go", "build", "-o", binaryPath, ".")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build migration binary: %v\n%s", err, output)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	run := exec.CommandContext(ctx, binaryPath, "-config", configPath)
	run.Dir = tempDir
	run.Env = isolatedMigrationEnv()
	output, err := run.CombinedOutput()
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		t.Fatalf("migration did not stop after database connection timeout:\n%s", output)
	}
	if err == nil {
		t.Fatal("migration unexpectedly connected to the test database")
	}
	message := string(output)
	if strings.Contains(message, "config validation failed") {
		t.Fatalf("migration rejected database-only config:\n%s", message)
	}
	if !strings.Contains(message, "connect database") {
		t.Fatalf("migration did not reach database connection:\n%s", message)
	}
}

func isolatedMigrationEnv() []string {
	blockedPrefixes := []string{
		"TASK_PROCESSOR_OPENAI_",
		"OPENAI_",
		"TASK_PROCESSOR_DATABASE_",
		"DB_",
	}
	env := make([]string, 0, len(os.Environ())+5)
	for _, entry := range os.Environ() {
		key, _, found := strings.Cut(entry, "=")
		if !found {
			continue
		}
		upperKey := strings.ToUpper(key)
		blocked := false
		for _, prefix := range blockedPrefixes {
			if strings.HasPrefix(upperKey, prefix) {
				blocked = true
				break
			}
		}
		if !blocked {
			env = append(env, entry)
		}
	}
	return append(env,
		"TASK_PROCESSOR_DATABASE_HOST=127.0.0.1",
		"TASK_PROCESSOR_DATABASE_PORT=1",
		"TASK_PROCESSOR_DATABASE_USER=listingkit",
		"TASK_PROCESSOR_DATABASE_PASSWORD=test-only",
		"TASK_PROCESSOR_DATABASE_NAME=listingkit",
	)
}
