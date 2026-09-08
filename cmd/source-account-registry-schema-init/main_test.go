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
	contents := []byte("database:\n  host: 127.0.0.1\n  port: 1\n  user: source-account\n  password: test-only\n  database: source-account\n")
	if err := os.WriteFile(configPath, contents, 0o600); err != nil {
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
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	run := exec.CommandContext(ctx, binaryPath, "-config", configPath)
	run.Dir = tempDir
	run.Env = isolatedSchemaInitEnv()
	output, err := run.CombinedOutput()
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		t.Fatalf("initializer did not respect connection timeout:\n%s", output)
	}
	if err == nil {
		t.Fatal("initializer unexpectedly connected to port 1")
	}
	message := string(output)
	if !strings.Contains(message, "connect database") || strings.Contains(message, "config validation failed") {
		t.Fatalf("initializer did not reach bounded database connection:\n%s", message)
	}
}

func isolatedSchemaInitEnv() []string {
	return append(databaseNeutralEnv(),
		"TASK_PROCESSOR_DATABASE_HOST=127.0.0.1",
		"TASK_PROCESSOR_DATABASE_PORT=1",
		"TASK_PROCESSOR_DATABASE_USER=source-account",
		"TASK_PROCESSOR_DATABASE_PASSWORD=test-only",
		"TASK_PROCESSOR_DATABASE_NAME=source-account",
	)
}

func databaseNeutralEnv() []string {
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
	return env
}
