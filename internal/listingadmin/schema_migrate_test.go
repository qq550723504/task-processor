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

func TestAutoMigrateImportTaskRepositoryRepairsHistoricalUniqueIndex(t *testing.T) {
	db, err := gorm.Open(sqlite.Dialector{DriverName: "sqlite", DSN: ":memory:"}, &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&importTaskPlatformIntegrityRow{}); err != nil {
		t.Fatalf("migrate legacy import task row: %v", err)
	}
	if err := db.Create(&importTaskPlatformIntegrityRow{
		TenantID: 101, Platform: "Amazon", SourcePlatform: "Amazon", TargetPlatform: "SHEIN",
		ProductID: "P1", Region: "US", StoreID: 986, Deleted: 0,
	}).Error; err != nil {
		t.Fatalf("seed historical row: %v", err)
	}
	if err := db.Exec(`CREATE UNIQUE INDEX idx_listing_product_import_task_unique ON listing_product_import_task (target_platform, product_id, region, store_id)`).Error; err != nil {
		t.Fatalf("create existing unique index: %v", err)
	}

	if err := AutoMigrateImportTaskRepository(db); err != nil {
		t.Fatalf("AutoMigrateImportTaskRepository() error = %v", err)
	}

	var historical importTaskPlatformIntegrityRow
	if err := db.Where("id = ?", 1).Take(&historical).Error; err != nil {
		t.Fatalf("load historical row: %v", err)
	}
	if historical.Platform != "Amazon" || historical.SourcePlatform != "Amazon" || historical.TargetPlatform != "SHEIN" {
		t.Fatalf("historical platforms = %q/%q/%q, want unchanged", historical.Platform, historical.SourcePlatform, historical.TargetPlatform)
	}
	if err := db.Create(&importTaskPlatformIntegrityRow{
		TenantID: 101, Platform: "shein", TargetPlatform: "SHEIN", ProductID: "P1", Region: "US", StoreID: 986, Deleted: 1,
	}).Error; err != nil {
		t.Fatalf("reimport after soft delete = %v, want historical full index repaired to active-only index", err)
	}
}

func TestAutoMigrateImportTaskRepositoryEnforcesCanonicalTargetPlatform(t *testing.T) {
	db, err := gorm.Open(sqlite.Dialector{DriverName: "sqlite", DSN: ":memory:"}, &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&importTaskPlatformIntegrityRow{}); err != nil {
		t.Fatalf("migrate import task row: %v", err)
	}
	if err := AutoMigrateImportTaskRepository(db); err != nil {
		t.Fatalf("AutoMigrateImportTaskRepository() error = %v", err)
	}
	if err := db.Create(&importTaskPlatformIntegrityRow{
		TenantID: 101, Platform: "amazon", TargetPlatform: "SHEIN", ProductID: "P2", Region: "US", StoreID: 987, Deleted: 0,
	}).Error; err != nil {
		t.Fatalf("seed canonical target row: %v", err)
	}
	if err := db.Create(&importTaskPlatformIntegrityRow{
		TenantID: 101, Platform: "amazon", TargetPlatform: "shein", ProductID: "P2", Region: "US", StoreID: 987, Deleted: 0,
	}).Error; err == nil {
		t.Fatal("mixed-case canonical duplicate = nil, want unique index violation")
	}
	if err := db.Create(&importTaskPlatformIntegrityRow{
		TenantID: 202, Platform: "amazon", TargetPlatform: "shein", ProductID: "P2", Region: "US", StoreID: 987, Deleted: 0,
	}).Error; err != nil {
		t.Fatalf("same canonical tuple in another tenant = %v, want allowed", err)
	}
}

func TestAutoMigrateImportTaskRepositoryDefersIndexWhenCanonicalDuplicatesExist(t *testing.T) {
	db, err := gorm.Open(sqlite.Dialector{DriverName: "sqlite", DSN: ":memory:"}, &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&importTaskPlatformIntegrityRow{}); err != nil {
		t.Fatalf("migrate import task row: %v", err)
	}
	for _, target := range []string{"SHEIN", "shein"} {
		if err := db.Create(&importTaskPlatformIntegrityRow{
			TenantID: 303, Platform: "amazon", TargetPlatform: target, ProductID: "P3", Region: "US", StoreID: 988, Deleted: 0,
		}).Error; err != nil {
			t.Fatalf("seed duplicate target %q: %v", target, err)
		}
	}
	if err := AutoMigrateImportTaskRepository(db); err != nil {
		t.Fatalf("AutoMigrateImportTaskRepository() with canonical duplicates = %v", err)
	}
	if db.Migrator().HasIndex(&listingProductImportTask{}, "idx_listing_product_import_task_unique") {
		t.Fatal("unique index installed despite existing canonical duplicates")
	}
}

type importTaskPlatformIntegrityRow struct {
	ID             int64  `gorm:"column:id;primaryKey;autoIncrement"`
	TenantID       int64  `gorm:"column:tenant_id;not null"`
	Platform       string `gorm:"column:platform;not null"`
	SourcePlatform string `gorm:"column:source_platform"`
	TargetPlatform string `gorm:"column:target_platform"`
	ProductID      string `gorm:"column:product_id;not null"`
	Region         string `gorm:"column:region;not null"`
	StoreID        int64  `gorm:"column:store_id;not null"`
	Deleted        int16  `gorm:"column:deleted;not null"`
}

func (importTaskPlatformIntegrityRow) TableName() string {
	return "listing_product_import_task"
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
