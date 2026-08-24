package schema

import (
	"context"
	"strings"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type schemaSQLRecorder struct {
	logger.Interface
	statements []string
}

func (r *schemaSQLRecorder) Trace(ctx context.Context, begin time.Time, fc func() (string, int64), err error) {
	sql, _ := fc()
	r.statements = append(r.statements, sql)
}

func TestAutoMigrateSystemOwnedExceptionRegistrySkipsExistingIndexDDL(t *testing.T) {
	recorder := &schemaSQLRecorder{Interface: logger.Default}
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: recorder})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`CREATE TABLE listingkit_owner_scope_system_owned_exceptions (
        id INTEGER PRIMARY KEY,
        table_name TEXT NOT NULL,
        tenant_fingerprint TEXT NOT NULL,
        candidate_fingerprint TEXT NOT NULL,
        report_fingerprint TEXT NOT NULL,
        reason TEXT NOT NULL,
        row_count BIGINT NOT NULL DEFAULT 0,
        active BOOLEAN NOT NULL DEFAULT TRUE,
        created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
    )`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`CREATE INDEX listingkit_owner_scope_system_owned_exceptions_active_idx
        ON listingkit_owner_scope_system_owned_exceptions (active, table_name, tenant_fingerprint)`).Error; err != nil {
		t.Fatal(err)
	}
	recorder.statements = nil

	if err := autoMigrateSystemOwnedExceptionRegistry(db); err != nil {
		t.Fatal(err)
	}
	for _, statement := range recorder.statements {
		if strings.Contains(strings.ToUpper(statement), "CREATE INDEX") {
			t.Fatalf("existing registry index must not trigger DDL: %s", statement)
		}
	}
}
