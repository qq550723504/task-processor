package listingadmin

import (
	"fmt"
	"strings"

	"gorm.io/gorm"
)

type postgresColumnDefinition struct {
	DataType               string
	CharacterMaximumLength *int
}

type postgresColumnTypeMigration struct {
	TargetType                  string
	UsingExpression             string
	DropDefaultBeforeTypeChange bool
	DefaultExpression           string
}

func ensureOwnerAuditColumns(db *gorm.DB, table string) error {
	if db == nil {
		return fmt.Errorf("database is not configured")
	}
	columnDefinitions := map[string]string{
		"owner_user_id": "varchar(128)",
		"created_by":    "varchar(128)",
		"updated_by":    "varchar(128)",
	}
	for column, definition := range columnDefinitions {
		hasColumn := db.Migrator().HasColumn(table, column)
		if !hasColumn {
			statement := fmt.Sprintf(`ALTER TABLE "%s" ADD COLUMN "%s" %s`, table, column, definition)
			if err := db.Exec(statement).Error; err != nil {
				return err
			}
		}
	}
	statements := []string{
		fmt.Sprintf(`CREATE INDEX IF NOT EXISTS "idx_%s_owner_user_id" ON "%s" (owner_user_id)`, table, table),
	}
	for _, statement := range statements {
		if err := db.Exec(statement).Error; err != nil {
			return err
		}
	}
	return nil
}

func ensureTextColumn(db *gorm.DB, table, column, definition string) error {
	if db == nil {
		return fmt.Errorf("database is not configured")
	}
	if db.Migrator().HasColumn(table, column) {
		return nil
	}
	statement := fmt.Sprintf(`ALTER TABLE "%s" ADD COLUMN "%s" %s`, table, column, definition)
	return db.Exec(statement).Error
}

func ensureNullableImportTaskCategoryID(db *gorm.DB, table string) error {
	if db == nil {
		return fmt.Errorf("database is not configured")
	}
	if !db.Migrator().HasColumn(table, "category_id") {
		return nil
	}
	columnTypes, err := db.Migrator().ColumnTypes(table)
	if err != nil {
		return err
	}
	for _, columnType := range columnTypes {
		if columnType.Name() != "category_id" {
			continue
		}
		nullable, ok := columnType.Nullable()
		if !ok || nullable {
			return nil
		}
		if db.Dialector != nil && db.Dialector.Name() == "postgres" {
			return db.Exec(fmt.Sprintf(`ALTER TABLE "%s" ALTER COLUMN "category_id" DROP NOT NULL`, table)).Error
		}
		return db.Migrator().AlterColumn(&listingProductImportTask{}, "CategoryID")
	}
	return nil
}

func sensitiveWordLegacyColumnMigrations() map[string]postgresColumnTypeMigration {
	return map[string]postgresColumnTypeMigration{
		"status": {
			TargetType:                  "smallint",
			UsingExpression:             `CASE WHEN NULLIF(BTRIM(status), '') IS NULL THEN 0 ELSE status::smallint END`,
			DropDefaultBeforeTypeChange: true,
			DefaultExpression:           "0",
		},
		"creator": {
			TargetType:      "varchar(128)",
			UsingExpression: `CASE WHEN creator IS NULL THEN NULL ELSE creator::text END`,
		},
		"updater": {
			TargetType:      "varchar(128)",
			UsingExpression: `CASE WHEN updater IS NULL THEN NULL ELSE updater::text END`,
		},
	}
}

func ensurePostgresColumnTypeMigrations(db *gorm.DB, table string, migrations map[string]postgresColumnTypeMigration) error {
	if db == nil || db.Dialector == nil || db.Dialector.Name() != "postgres" {
		return nil
	}
	for column, migration := range migrations {
		definition, exists, err := lookupPostgresColumnDefinition(db, table, column)
		if err != nil {
			return err
		}
		if !exists || !postgresColumnDefinitionNeedsTypeMigration(definition, migration.TargetType) {
			continue
		}
		if migration.DropDefaultBeforeTypeChange {
			statement := fmt.Sprintf(`ALTER TABLE "%s" ALTER COLUMN "%s" DROP DEFAULT`, table, column)
			if err := db.Exec(statement).Error; err != nil {
				return err
			}
		}
		statement := fmt.Sprintf(
			`ALTER TABLE "%s" ALTER COLUMN "%s" TYPE %s USING %s`,
			table,
			column,
			migration.TargetType,
			migration.UsingExpression,
		)
		if err := db.Exec(statement).Error; err != nil {
			return err
		}
		if strings.TrimSpace(migration.DefaultExpression) != "" {
			statement := fmt.Sprintf(
				`ALTER TABLE "%s" ALTER COLUMN "%s" SET DEFAULT %s`,
				table,
				column,
				migration.DefaultExpression,
			)
			if err := db.Exec(statement).Error; err != nil {
				return err
			}
		}
	}
	return nil
}

func lookupPostgresColumnDefinition(db *gorm.DB, table, column string) (postgresColumnDefinition, bool, error) {
	type row struct {
		DataType               string
		CharacterMaximumLength *int
	}

	var result row
	query := `
SELECT data_type, character_maximum_length
FROM information_schema.columns
WHERE table_schema = current_schema()
  AND table_name = ?
  AND column_name = ?
LIMIT 1
`
	err := db.Raw(query, table, column).Scan(&result).Error
	if err != nil {
		return postgresColumnDefinition{}, false, err
	}
	if strings.TrimSpace(result.DataType) == "" {
		return postgresColumnDefinition{}, false, nil
	}
	return postgresColumnDefinition{
		DataType:               result.DataType,
		CharacterMaximumLength: result.CharacterMaximumLength,
	}, true, nil
}

func postgresColumnDefinitionNeedsTypeMigration(actual postgresColumnDefinition, expected string) bool {
	return normalizePostgresColumnTypeName(actual) != normalizeExpectedPostgresType(expected)
}

func normalizePostgresColumnTypeName(actual postgresColumnDefinition) string {
	dataType := strings.TrimSpace(strings.ToLower(actual.DataType))
	switch dataType {
	case "character varying", "varchar":
		if actual.CharacterMaximumLength != nil && *actual.CharacterMaximumLength > 0 {
			return fmt.Sprintf("varchar(%d)", *actual.CharacterMaximumLength)
		}
		return "varchar"
	case "smallint", "int2":
		return "smallint"
	case "integer", "int", "int4":
		return "integer"
	case "bigint", "int8":
		return "bigint"
	default:
		return dataType
	}
}

func normalizeExpectedPostgresType(expected string) string {
	return strings.TrimSpace(strings.ToLower(expected))
}

func ensureUniqueIndex(db *gorm.DB, table, indexName string, columns ...string) error {
	if db == nil {
		return fmt.Errorf("database is not configured")
	}
	if strings.TrimSpace(table) == "" || strings.TrimSpace(indexName) == "" || len(columns) == 0 {
		return fmt.Errorf("table, index name, and columns are required")
	}
	quotedColumns := make([]string, 0, len(columns))
	for _, column := range columns {
		trimmedColumn := strings.TrimSpace(column)
		if trimmedColumn == "" {
			return fmt.Errorf("index column is required")
		}
		quotedColumns = append(quotedColumns, fmt.Sprintf(`"%s"`, trimmedColumn))
	}
	statement := fmt.Sprintf(
		`CREATE UNIQUE INDEX IF NOT EXISTS "%s" ON "%s" (%s)`,
		indexName,
		table,
		strings.Join(quotedColumns, ", "),
	)
	return db.Exec(statement).Error
}

func ensureImportTaskPlatformIntegrity(db *gorm.DB, table string) error {
	if db == nil {
		return fmt.Errorf("database is not configured")
	}
	for _, column := range []string{"platform", "source_platform", "target_platform", "deleted"} {
		if !db.Migrator().HasColumn(table, column) {
			return nil
		}
	}

	if db.Dialector == nil {
		return nil
	}
	if db.Dialector.Name() == "postgres" {
		if err := ensureNoImportTaskPlatformViolations(db, table); err != nil {
			return err
		}
		if err := ensureNoImportTaskCaseFoldDuplicates(db, table); err != nil {
			return err
		}
		if err := ensureImportTaskPlatformCheckConstraint(db, table); err != nil {
			return err
		}
	}

	return ensureImportTaskActiveUniqueIndex(db, table)
}

func ensureNoImportTaskPlatformViolations(db *gorm.DB, table string) error {
	var count int64
	query := fmt.Sprintf(`
		SELECT COUNT(*)
		FROM "%s"
		WHERE deleted = 0
		  AND (platform IS DISTINCT FROM lower(trim(platform))
		   OR (source_platform IS NOT NULL AND source_platform IS DISTINCT FROM lower(trim(source_platform)))
		   OR (target_platform IS NOT NULL AND target_platform IS DISTINCT FROM lower(trim(target_platform))))`, table)
	if err := db.Raw(query).Scan(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return fmt.Errorf("listing_product_import_task contains %d non-canonical platform rows; normalize them before startup", count)
	}
	return nil
}

func ensureNoImportTaskCaseFoldDuplicates(db *gorm.DB, table string) error {
	var count int64
	query := fmt.Sprintf(`
		SELECT COUNT(*)
		FROM (
			SELECT lower(btrim(target_platform)), product_id, region, store_id
			FROM "%s"
			WHERE deleted = 0 AND target_platform IS NOT NULL
			GROUP BY lower(btrim(target_platform)), product_id, region, store_id
			HAVING COUNT(*) > 1
		) duplicates`, table)
	if err := db.Raw(query).Scan(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return fmt.Errorf("listing_product_import_task contains %d active case-insensitive duplicate product keys; resolve them before startup", count)
	}
	return nil
}

func ensureImportTaskPlatformCheckConstraint(db *gorm.DB, table string) error {
	const constraintName = "listing_product_import_task_platform_case_chk"
	var name string
	if err := db.Raw(`
		SELECT conname
		FROM pg_constraint
		WHERE conrelid = ?::regclass AND conname = ?`, table, constraintName).Scan(&name).Error; err != nil {
		return err
	}
	if strings.TrimSpace(name) != "" {
		return nil
	}
	statement := fmt.Sprintf(`
		ALTER TABLE "%s"
		ADD CONSTRAINT "%s"
		CHECK (
			platform = lower(btrim(platform))
			AND (source_platform IS NULL OR source_platform = lower(btrim(source_platform)))
			AND (target_platform IS NULL OR target_platform = lower(btrim(target_platform)))
		)`, table, constraintName)
	return db.Exec(statement).Error
}

func ensureImportTaskActiveUniqueIndex(db *gorm.DB, table string) error {
	const indexName = "idx_listing_product_import_task_unique"
	if db.Migrator().HasIndex(&listingProductImportTask{}, indexName) {
		if importTaskUniqueIndexIsActiveOnly(db, table, indexName) {
			return nil
		}
		if err := db.Exec(fmt.Sprintf(`DROP INDEX IF EXISTS "%s"`, indexName)).Error; err != nil {
			return err
		}
	}
	return db.Exec(importTaskActiveUniqueIndexStatement(table)).Error
}

func importTaskActiveUniqueIndexStatement(table string) string {
	return fmt.Sprintf(
		`CREATE UNIQUE INDEX IF NOT EXISTS "idx_listing_product_import_task_unique" ON "%s" (target_platform, product_id, region, store_id) WHERE deleted = 0`,
		table,
	)
}

func importTaskUniqueIndexIsActiveOnly(db *gorm.DB, table, indexName string) bool {
	if db == nil || db.Dialector == nil {
		return false
	}
	var definition string
	switch db.Dialector.Name() {
	case "postgres":
		if err := db.Raw(`SELECT indexdef FROM pg_indexes WHERE tablename = ? AND indexname = ?`, table, indexName).Scan(&definition).Error; err != nil {
			return false
		}
	case "sqlite":
		if err := db.Raw(`SELECT sql FROM sqlite_master WHERE type = 'index' AND name = ?`, indexName).Scan(&definition).Error; err != nil {
			return false
		}
	default:
		return false
	}
	definition = strings.ToLower(strings.Join(strings.Fields(definition), " "))
	return strings.Contains(definition, "where deleted = 0")
}
