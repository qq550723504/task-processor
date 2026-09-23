package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"

	orgresourceadapter "task-processor/internal/integration/orgresource"
	commercialstore "task-processor/internal/integration/persistence/commercialbilling"
	moneystore "task-processor/internal/integration/persistence/money"
	platformdatabase "task-processor/internal/platform/database"
)

func main() {
	manifest := flag.String("config", "", "absolute path to the private commercial schema-owner database JSON config")
	flag.Parse()
	if *manifest == "" {
		fmt.Fprintln(os.Stderr, "-config is required")
		os.Exit(2)
	}
	if err := run(*manifest); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(manifestPath string) error {
	dbConfig, err := loadSchemaOwnerConfig(manifestPath)
	if err != nil {
		return err
	}
	db, err := platformdatabase.OpenExistingWritableContext(context.Background(), dbConfig)
	if err != nil {
		return fmt.Errorf("open commercial owner database: %w", err)
	}
	defer platformdatabase.Close(db)
	var canCreate bool
	if err := db.Raw("SELECT has_schema_privilege(current_user, current_schema(), 'CREATE')").Scan(&canCreate).Error; err != nil {
		return fmt.Errorf("verify commercial schema-owner privileges: %w", err)
	}
	if !canCreate {
		return fmt.Errorf("commercial schema migration role requires CREATE on the target schema")
	}
	if err := moneystore.AutoMigrate(db); err != nil {
		return fmt.Errorf("migrate money owner schema: %w", err)
	}
	if err := commercialstore.AutoMigrate(db); err != nil {
		return fmt.Errorf("migrate commercial billing schema: %w", err)
	}
	if err := orgresourceadapter.AutoMigrate(db); err != nil {
		return fmt.Errorf("migrate organization resource schema: %w", err)
	}
	return nil
}

type schemaOwnerManifest struct {
	Host               string `json:"host"`
	Port               int    `json:"port"`
	User               string `json:"user"`
	Password           string `json:"password"`
	Database           string `json:"database"`
	MaxConnections     int    `json:"maxConnections"`
	MaxIdleConnections int    `json:"maxIdleConnections"`
}

func loadSchemaOwnerConfig(path string) (*platformdatabase.Config, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open private commercial schema-owner config: %w", err)
	}
	defer file.Close()
	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	var manifest schemaOwnerManifest
	if err := decoder.Decode(&manifest); err != nil {
		return nil, fmt.Errorf("decode commercial schema-owner config: %w", err)
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return nil, fmt.Errorf("commercial schema-owner config must contain one JSON object")
	}
	if manifest.Host == "" || manifest.Port < 1 || manifest.Port > 65535 || manifest.User == "" || manifest.Password == "" || manifest.Database == "" || manifest.MaxConnections < 1 || manifest.MaxIdleConnections < 0 || manifest.MaxIdleConnections > manifest.MaxConnections {
		return nil, fmt.Errorf("commercial schema-owner config has invalid database settings")
	}
	if manifest.User == "commercial_runtime" || manifest.User == "commercial_owner_runtime" {
		return nil, fmt.Errorf("commercial runtime database roles cannot run schema migrations")
	}
	return &platformdatabase.Config{Host: manifest.Host, Port: manifest.Port, User: manifest.User, Password: manifest.Password, Database: manifest.Database, MaxConnections: manifest.MaxConnections, MaxIdleConnections: manifest.MaxIdleConnections}, nil
}
