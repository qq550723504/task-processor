package listingadmin

import (
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	_ "modernc.org/sqlite"
)

func TestSensitiveWordLegacyColumnMigrationsCoverLegacyStatusAndAuditColumns(t *testing.T) {
	t.Parallel()

	migrations := sensitiveWordLegacyColumnMigrations()

	status, ok := migrations["status"]
	if !ok {
		t.Fatal("expected status migration to exist")
	}
	if status.TargetType != "smallint" {
		t.Fatalf("status target type = %q, want smallint", status.TargetType)
	}
	if status.UsingExpression == "" {
		t.Fatal("expected status migration to include USING expression")
	}
	if !status.DropDefaultBeforeTypeChange {
		t.Fatal("expected status migration to drop legacy default before type change")
	}
	if status.DefaultExpression != "0" {
		t.Fatalf("status default expression = %q, want 0", status.DefaultExpression)
	}

	for _, column := range []string{"creator", "updater"} {
		migration, ok := migrations[column]
		if !ok {
			t.Fatalf("expected %s migration to exist", column)
		}
		if migration.TargetType != "varchar(128)" {
			t.Fatalf("%s target type = %q, want varchar(128)", column, migration.TargetType)
		}
		if migration.UsingExpression == "" {
			t.Fatalf("expected %s migration to include USING expression", column)
		}
	}
}

func TestPostgresColumnDefinitionNeedsTypeMigration(t *testing.T) {
	t.Parallel()

	if !postgresColumnDefinitionNeedsTypeMigration(
		postgresColumnDefinition{DataType: "character varying", CharacterMaximumLength: intPtr(20)},
		"smallint",
	) {
		t.Fatal("expected varchar(20) to require migration to smallint")
	}

	if !postgresColumnDefinitionNeedsTypeMigration(
		postgresColumnDefinition{DataType: "bigint"},
		"varchar(128)",
	) {
		t.Fatal("expected bigint to require migration to varchar(128)")
	}

	if postgresColumnDefinitionNeedsTypeMigration(
		postgresColumnDefinition{DataType: "character varying", CharacterMaximumLength: intPtr(128)},
		"varchar(128)",
	) {
		t.Fatal("expected varchar(128) to already match target type")
	}
}

func TestAutoMigrateImportTaskRepositoryMakesCategoryIDNullable(t *testing.T) {
	t.Parallel()

	db, err := gorm.Open(sqlite.Dialector{DriverName: "sqlite", DSN: ":memory:"}, &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&legacyImportTaskWithRequiredCategory{}); err != nil {
		t.Fatalf("migrate legacy import task: %v", err)
	}
	if nullable := importTaskCategoryNullable(t, db); nullable {
		t.Fatal("legacy category_id unexpectedly nullable")
	}

	if err := AutoMigrateImportTaskRepository(db); err != nil {
		t.Fatalf("AutoMigrateImportTaskRepository() error = %v", err)
	}
	if nullable := importTaskCategoryNullable(t, db); !nullable {
		t.Fatal("category_id remains NOT NULL after migration")
	}
}

type legacyImportTaskWithRequiredCategory struct {
	ID         int64 `gorm:"column:id;primaryKey;autoIncrement"`
	CategoryID int64 `gorm:"column:category_id;not null"`
}

func (legacyImportTaskWithRequiredCategory) TableName() string {
	return "listing_product_import_task"
}

func importTaskCategoryNullable(t *testing.T, db *gorm.DB) bool {
	t.Helper()
	columns, err := db.Migrator().ColumnTypes("listing_product_import_task")
	if err != nil {
		t.Fatalf("load import task columns: %v", err)
	}
	for _, column := range columns {
		if column.Name() != "category_id" {
			continue
		}
		nullable, ok := column.Nullable()
		if !ok {
			t.Fatal("category_id nullability metadata unavailable")
		}
		return nullable
	}
	t.Fatal("category_id column missing")
	return false
}

func intPtr(value int) *int {
	return &value
}
