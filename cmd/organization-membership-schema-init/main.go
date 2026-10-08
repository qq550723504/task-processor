package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"task-processor/internal/authidentity"
	"task-processor/internal/authz"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	memberstore "task-processor/internal/integration/persistence/organization/membership"
)

type slotInventory struct {
	ProjectID     string `json:"projectId"`
	Organizations []struct {
		OrganizationID string   `json:"organizationId"`
		RoleKeys       []string `json:"roleKeys"`
	} `json:"organizations"`
}

func readInventory(path string) (*slotInventory, error) {
	if path == "" {
		return nil, nil
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, 65537))
	if err != nil || len(data) > 65536 {
		return nil, errors.New("invalid slot inventory")
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var inventory slotInventory
	if decoder.Decode(&inventory) != nil || decoder.Decode(new(any)) != io.EOF || !authidentity.IsBoundedIdentifier(inventory.ProjectID) || len(inventory.Organizations) < 1 || len(inventory.Organizations) > 8 {
		return nil, errors.New("invalid slot inventory")
	}
	seen := map[string]bool{}
	for _, org := range inventory.Organizations {
		if !authidentity.IsBoundedIdentifier(org.OrganizationID) || seen[org.OrganizationID] || len(org.RoleKeys) != authz.EnterpriseRoleCapacity {
			return nil, errors.New("invalid slot organization")
		}
		seen[org.OrganizationID] = true
		keys := map[string]bool{}
		for _, key := range org.RoleKeys {
			if keys[key] {
				return nil, errors.New("duplicate slot")
			}
			keys[key] = true
		}
		for slot := 1; slot <= authz.EnterpriseRoleCapacity; slot++ {
			if !keys[authz.EnterpriseRoleKey(org.OrganizationID, slot)] {
				return nil, errors.New("slot scope mismatch")
			}
		}
	}
	return &inventory, nil
}

func install(ctx context.Context, dsn string, inventory *slotInventory) error {
	if ctx == nil || strings.TrimSpace(dsn) == "" || !strings.HasPrefix(dsn, "postgresql://") && !strings.HasPrefix(dsn, "postgres://") {
		return fmt.Errorf("invalid membership database DSN")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{DisableAutomaticPing: true, Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		return fmt.Errorf("open membership database: %w", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("open membership database pool: %w", err)
	}
	defer sqlDB.Close()
	sqlDB.SetMaxOpenConns(1)
	sqlDB.SetMaxIdleConns(1)
	if err := sqlDB.PingContext(ctx); err != nil {
		return fmt.Errorf("ping membership database: %w", err)
	}
	tx, err := sqlDB.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin membership schema transaction: %w", err)
	}
	if err := memberstore.InstallSchemaTx(ctx, tx); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("install membership schema: %w", err)
	}
	if inventory != nil {
		for _, org := range inventory.Organizations {
			if err := memberstore.InstallRoleSlotsTx(ctx, tx, inventory.ProjectID, org.OrganizationID); err != nil {
				_ = tx.Rollback()
				return fmt.Errorf("install role slots: %w", err)
			}
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit membership schema: %w", err)
	}
	return nil
}

func run(args []string, output io.Writer, installFn func(context.Context, string, *slotInventory) error) int {
	fail := func() int { _, _ = fmt.Fprintln(output, "membership schema initialization failed"); return 1 }
	flags := flag.NewFlagSet("organization-membership-schema-init", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	path := flags.String("dsn-file", "", "explicit connection file")
	slotsPath := flags.String("role-slots-file", "", "verified fresh native bootstrap inventory")
	timeout := flags.Duration("timeout", 30*time.Second, "bounded installation timeout")
	if flags.Parse(args) != nil || flags.NArg() != 0 || *path == "" || *timeout <= 0 || *timeout > time.Minute || installFn == nil {
		return fail()
	}
	f, err := os.Open(*path)
	if err != nil {
		return fail()
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() > 65536 {
		return fail()
	}
	data, err := io.ReadAll(io.LimitReader(f, 65537))
	if err != nil || len(data) > 65536 || strings.TrimSpace(string(data)) == "" {
		return fail()
	}
	inventory, err := readInventory(*slotsPath)
	if err != nil {
		return fail()
	}
	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()
	if installFn(ctx, strings.TrimSpace(string(data)), inventory) != nil {
		return fail()
	}
	return 0
}

func main() { os.Exit(run(os.Args[1:], os.Stderr, install)) }
