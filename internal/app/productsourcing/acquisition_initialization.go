package productsourcing

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

	"gorm.io/gorm"
	sigjson "sigs.k8s.io/json"
	acquisitionstore "task-processor/internal/integration/persistence/product/acquisition"
	platformdatabase "task-processor/internal/platform/database"
)

var acquisitionInitName = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]{0,62}$`)

type acquisitionInitManifest struct {
	SchemaVersion int `json:"schemaVersion"`
	Database      struct {
		Host     string `json:"host"`
		Port     int    `json:"port"`
		User     string `json:"user"`
		Password string `json:"password"`
		Database string `json:"database"`
	} `json:"database"`
}

// InitializeAcquisitionDatabase is explicit maintenance, never ordinary startup.
func InitializeAcquisitionDatabase(ctx context.Context, manifest, confirmedDatabase string) error {
	unavailable := errors.New("acquisition initialization refused; require a private manifest, confirmed empty dedicated database and pre-provisioned runtime role")
	if ctx == nil || !filepath.IsAbs(manifest) {
		return unavailable
	}
	info, err := os.Lstat(manifest)
	if err != nil || !info.Mode().IsRegular() || (runtime.GOOS != "windows" && info.Mode().Perm()&0077 != 0) {
		return unavailable
	}
	file, err := os.Open(manifest)
	if err != nil {
		return unavailable
	}
	defer file.Close()
	raw, err := io.ReadAll(io.LimitReader(file, 8193))
	if err != nil || len(raw) > 8192 {
		return unavailable
	}
	var cfg acquisitionInitManifest
	strict, err := sigjson.UnmarshalStrict(raw, &cfg, sigjson.DisallowUnknownFields, sigjson.DisallowDuplicateFields)
	d := cfg.Database
	if err != nil || len(strict) > 0 || cfg.SchemaVersion != 1 || d.Host != "127.0.0.1" || d.Port < 1 || d.Port > 65535 || !acquisitionInitName.MatchString(d.User) || !acquisitionInitName.MatchString(d.Database) || confirmedDatabase != d.Database || d.User == acquisitionstore.RuntimeRole || d.Database == "postgres" || d.Database == "template0" || d.Database == "template1" || len(d.Password) < 1 || len(d.Password) > 1024 || strings.ContainsAny(d.Password, " ='\\\t\r\n\v\f\x00") {
		return unavailable
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	db, err := platformdatabase.OpenExistingWritableContext(ctx, &platformdatabase.Config{Host: d.Host, Port: d.Port, User: d.User, Password: d.Password, Database: d.Database, MaxConnections: 2, MaxIdleConnections: 2, ConnectionMaxLifetime: time.Minute})
	if err != nil {
		return unavailable
	}
	defer platformdatabase.Close(db)
	err = db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("SELECT pg_advisory_xact_lock(hashtextextended(current_database()||':source-acquisition-init',0))").Error; err != nil {
			return err
		}
		var empty bool
		if err := tx.Raw(`SELECT current_schema()='public' AND NOT EXISTS(SELECT 1 FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname NOT LIKE 'pg_%' AND n.nspname<>'information_schema' AND c.relkind IN ('r','p','v','m','f','S'))`).Scan(&empty).Error; err != nil || !empty {
			return unavailable
		}
		if err := InstallAcquisitionSchema(tx); err != nil {
			return err
		}
		return acquisitionstore.GrantRuntimePermissions(ctx, tx)
	})
	if err != nil {
		return unavailable
	}
	return nil
}
