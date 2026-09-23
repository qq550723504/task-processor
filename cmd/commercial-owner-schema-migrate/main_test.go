package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadSchemaOwnerConfigUsesOwnerCredentials(t *testing.T) {
	path := filepath.Join(t.TempDir(), "schema-owner.json")
	contents := `{"host":"127.0.0.1","port":5434,"user":"postgres","password":"private","database":"commercial","maxConnections":2,"maxIdleConnections":1}`
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	config, err := loadSchemaOwnerConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if config.User != "postgres" || config.Password != "private" || config.Database != "commercial" {
		t.Fatalf("loaded schema owner config = %#v", config)
	}
}

func TestLoadSchemaOwnerConfigRejectsRuntimeRole(t *testing.T) {
	path := filepath.Join(t.TempDir(), "runtime.json")
	contents := `{"host":"127.0.0.1","port":5434,"user":"commercial_owner_runtime","password":"private","database":"commercial","maxConnections":2,"maxIdleConnections":1}`
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := loadSchemaOwnerConfig(path); err == nil {
		t.Fatal("runtime role must not be accepted for schema migration")
	}
}
