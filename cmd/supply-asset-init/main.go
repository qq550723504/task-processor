// Explicit maintenance for the existing Asset fact owner. Never called by HTTP.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"github.com/jackc/pgx/v5"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"io"
	"os"
	"path/filepath"
	"strings"
	coreconfig "task-processor/internal/core/config"
	assetstore "task-processor/internal/integration/persistence/product/asset"
	"time"
)

func main() {
	path := flag.String("dsn-file", "", "absolute private Asset owner DSN file")
	database := flag.String("confirm-database", "", "exact dedicated Asset database")
	install := flag.Bool("install-empty-schema", false, "install current Asset schema only in an empty dedicated database")
	flag.Parse()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if flag.NArg() != 0 || initialize(ctx, *path, *database, *install) != nil {
		fmt.Fprintln(os.Stderr, "Asset initialization refused; require a private owner file, confirmed dedicated database and pre-provisioned supply runtime role")
		os.Exit(1)
	}
	fmt.Println("Current Asset schema and supply grants ready")
}
func initialize(ctx context.Context, path, confirmed string, install bool) error {
	refused := errors.New("Asset initialization refused")
	if ctx == nil || !filepath.IsAbs(path) || confirmed == "" || coreconfig.VerifyPrivateFiles(ctx, []string{path}) != nil {
		return refused
	}
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() {
		return refused
	}
	file, err := os.Open(path)
	if err != nil {
		return refused
	}
	defer file.Close()
	raw, err := io.ReadAll(io.LimitReader(file, 8193))
	if err != nil || len(raw) > 8192 {
		return refused
	}
	defer clear(raw)
	dsn := strings.TrimSpace(string(raw))
	parsed, err := pgx.ParseConfig(dsn)
	if err != nil || parsed.Host != "127.0.0.1" || parsed.Database != confirmed || parsed.Database == "postgres" || parsed.Database == "template0" || parsed.Database == "template1" || parsed.User == assetstore.SourceRuntimeRole {
		return refused
	}
	for _, fallback := range parsed.Fallbacks {
		if fallback.Host != "127.0.0.1" {
			return refused
		}
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{DisableAutomaticPing: true, Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		return refused
	}
	pool, err := db.DB()
	if err != nil {
		return refused
	}
	defer pool.Close()
	pool.SetMaxOpenConns(1)
	if pool.PingContext(ctx) != nil {
		return refused
	}
	err = db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if tx.Exec("SELECT pg_advisory_xact_lock(hashtextextended(current_database()||':supply-asset-init',0))").Error != nil {
			return refused
		}
		if install {
			var empty bool
			if tx.Raw(`SELECT current_schema()='public' AND NOT EXISTS(SELECT 1 FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname NOT LIKE 'pg_%' AND n.nspname<>'information_schema' AND c.relkind IN ('r','p','v','m','f','S'))`).Scan(&empty).Error != nil || !empty {
				return refused
			}
			if assetstore.InstallSourceApprovalSchema(tx) != nil {
				return refused
			}
		}
		return assetstore.GrantSourceRuntimePermissions(ctx, tx)
	})
	if err != nil {
		return refused
	}
	return nil
}
