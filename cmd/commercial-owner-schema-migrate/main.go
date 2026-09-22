package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	currentapplication "task-processor/internal/app/runtime/currentapplication"
	orgresourceadapter "task-processor/internal/integration/orgresource"
	commercialstore "task-processor/internal/integration/persistence/commercialbilling"
	moneystore "task-processor/internal/integration/persistence/money"
	platformdatabase "task-processor/internal/platform/database"
)

func main() {
	manifest := flag.String("config", "", "absolute path to the private current-application JSON manifest")
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
	cfg, err := currentapplication.LoadConfig(manifestPath)
	if err != nil {
		return err
	}
	if cfg.CommercialOwnerDatabase == nil {
		return fmt.Errorf("commercialOwnerDatabase is required")
	}
	dbConfig := &platformdatabase.Config{Host: cfg.CommercialOwnerDatabase.Host, Port: cfg.CommercialOwnerDatabase.Port, User: cfg.CommercialOwnerDatabase.User, Password: cfg.CommercialOwnerDatabase.Password, Database: cfg.CommercialOwnerDatabase.Database, MaxConnections: cfg.CommercialOwnerDatabase.MaxConnections, MaxIdleConnections: cfg.CommercialOwnerDatabase.MaxConnections}
	db, err := platformdatabase.OpenExistingWritableContext(context.Background(), dbConfig)
	if err != nil {
		return fmt.Errorf("open commercial owner database: %w", err)
	}
	defer platformdatabase.Close(db)
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
