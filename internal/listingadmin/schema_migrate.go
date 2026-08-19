package listingadmin

import (
	"fmt"
	"strings"

	"gorm.io/gorm"
	"task-processor/internal/model"
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
	return ensureOwnerUserIDConstraint(db, table)
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

func ensureImportTaskActiveUniqueIndex(db *gorm.DB, table string) (bool, error) {
	if db == nil {
		return false, fmt.Errorf("database is not configured")
	}
	const indexName = "idx_listing_product_import_task_unique"
	for _, column := range []string{"tenant_id", "target_platform", "product_id", "region", "store_id", "deleted"} {
		if !db.Migrator().HasColumn(table, column) {
			return false, nil
		}
	}
	if db.Migrator().HasIndex(&listingProductImportTask{}, indexName) && importTaskUniqueIndexIsCanonicalActiveOnly(db, table, indexName) {
		return true, nil
	}
	duplicates, err := importTaskCanonicalDuplicatesExist(db, table)
	if err != nil {
		return false, err
	}
	if duplicates {
		return false, nil
	}
	if !db.Migrator().HasIndex(&listingProductImportTask{}, indexName) {
		if err := db.Exec(importTaskActiveUniqueIndexStatementForSchema(db, table, indexName)).Error; err != nil {
			return false, err
		}
		return true, nil
	}
	if err := replaceImportTaskActiveUniqueIndex(db, table, indexName); err != nil {
		return false, err
	}
	return true, nil
}

// EnsureImportTaskWriteReady verifies that every repository writer is protected
// by the canonical active-task uniqueness constraint. It fails closed while
// legacy duplicates prevent that constraint from being installed.
func EnsureImportTaskWriteReady(db *gorm.DB) error {
	ready, err := ensureImportTaskActiveUniqueIndex(db, (listingProductImportTask{}).TableName())
	if err != nil {
		return err
	}
	if !ready {
		return ErrImportTaskIntegrityUnavailable
	}
	return nil
}

func replaceImportTaskActiveUniqueIndex(db *gorm.DB, table, indexName string) error {
	replacementName := indexName + "_replacement"
	replacementReady := db.Migrator().HasIndex(&listingProductImportTask{}, replacementName) && importTaskUniqueIndexIsCanonicalActiveOnly(db, table, replacementName)
	if !replacementReady {
		if err := db.Exec(fmt.Sprintf(`DROP INDEX IF EXISTS "%s"`, replacementName)).Error; err != nil {
			return err
		}
		if err := db.Exec(importTaskActiveUniqueIndexStatementForSchema(db, table, replacementName)).Error; err != nil {
			return err
		}
	}
	err := db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec(fmt.Sprintf(`DROP INDEX IF EXISTS "%s"`, indexName)).Error; err != nil {
			return err
		}
		if tx.Dialector != nil && tx.Dialector.Name() == "postgres" {
			return tx.Exec(fmt.Sprintf(`ALTER INDEX "%s" RENAME TO "%s"`, replacementName, indexName)).Error
		}
		if err := tx.Exec(importTaskActiveUniqueIndexStatementForSchema(tx, table, indexName)).Error; err != nil {
			return err
		}
		return tx.Exec(fmt.Sprintf(`DROP INDEX IF EXISTS "%s"`, replacementName)).Error
	})
	if err != nil {
		return err
	}
	return nil
}

func importTaskCanonicalTargetPlatformExpression(targetColumn, platformColumn string) string {
	return fmt.Sprintf("LOWER(TRIM(COALESCE(NULLIF(TRIM(%s), ''), %s)))", targetColumn, platformColumn)
}

func importTaskCanonicalDuplicatesExist(db *gorm.DB, table string) (bool, error) {
	expression := importTaskCanonicalTargetPlatformExpression("target_platform", "platform")
	activePredicate := "deleted = 0"
	if db.Migrator().HasColumn(table, "status") {
		activePredicate = importTaskCanonicalActivePredicate()
	}
	query := fmt.Sprintf(`
		SELECT COUNT(*)
		FROM (
			SELECT tenant_id, %s AS canonical_target_platform, product_id, region, store_id
			FROM "%s"
			WHERE %s AND %s IS NOT NULL
			GROUP BY tenant_id, %s, product_id, region, store_id
			HAVING COUNT(*) > 1
			LIMIT 1
		) duplicates`, expression, table, activePredicate, expression, expression)
	var count int64
	if err := db.Raw(query).Scan(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func importTaskActiveUniqueIndexStatement(table, indexName string) string {
	return importTaskUniqueIndexStatement(table, indexName, "deleted = 0")
}

func importTaskActiveUniqueIndexStatementForSchema(db *gorm.DB, table, indexName string) string {
	predicate := "deleted = 0"
	if db != nil && db.Migrator().HasColumn(table, "status") {
		predicate = importTaskCanonicalActivePredicate()
	}
	return importTaskUniqueIndexStatement(table, indexName, predicate)
}

func importTaskUniqueIndexStatement(table, indexName, predicate string) string {
	expression := importTaskCanonicalTargetPlatformExpression("target_platform", "platform")
	return fmt.Sprintf(
		`CREATE UNIQUE INDEX IF NOT EXISTS "%s" ON "%s" ((%s), tenant_id, product_id, region, store_id) WHERE %s`,
		indexName,
		table,
		expression,
		predicate,
	)
}

func importTaskCanonicalActivePredicate() string {
	return fmt.Sprintf("deleted = 0 AND status NOT IN (%d, %d)", model.TaskStatusPublished.Int16(), model.TaskStatusDraft.Int16())
}

func importTaskUniqueIndexIsCanonicalActiveOnly(db *gorm.DB, table, indexName string) bool {
	if db == nil || db.Dialector == nil {
		return false
	}
	var definition string
	switch db.Dialector.Name() {
	case "postgres":
		if err := db.Raw(postgresImportTaskIndexDefinitionQuery(), table, indexName).Scan(&definition).Error; err != nil {
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
	definition = strings.ReplaceAll(definition, `"`, "")
	if !strings.Contains(definition, "create unique index") {
		return false
	}
	keyColumns, predicate, ok := parseImportTaskIndexDefinition(definition)
	if !ok {
		return false
	}
	normalizedPredicate := normalizeImportTaskIndexDefinition(predicate)
	if db.Migrator().HasColumn(table, "status") {
		if !importTaskCanonicalActivePredicateMatches(normalizedPredicate) {
			return false
		}
	} else if normalizedPredicate != "wheredeleted=0" {
		return false
	}
	expected := []string{
		normalizeImportTaskIndexDefinition(importTaskCanonicalTargetPlatformExpression("target_platform", "platform")),
		"tenant_id",
		"product_id",
		"region",
		"store_id",
	}
	if len(keyColumns) != len(expected) {
		return false
	}
	for i, column := range keyColumns {
		if normalizeImportTaskIndexDefinition(column) != expected[i] {
			return false
		}
	}
	return true
}

func importTaskCanonicalActivePredicateMatches(predicate string) bool {
	if !strings.Contains(predicate, "deleted=0") {
		return false
	}
	return strings.Contains(predicate, "statusnotin(6,8)") ||
		(strings.Contains(predicate, "status<>6") && strings.Contains(predicate, "status<>8"))
}

func postgresImportTaskIndexDefinitionQuery() string {
	return `
SELECT pg_get_indexdef(index_rel.oid)
FROM pg_index AS index_meta
JOIN pg_class AS index_rel ON index_rel.oid = index_meta.indexrelid
JOIN pg_class AS table_rel ON table_rel.oid = index_meta.indrelid
JOIN pg_namespace AS table_schema ON table_schema.oid = table_rel.relnamespace
WHERE table_schema.nspname = current_schema()
  AND table_rel.relname = ?
  AND index_rel.relname = ?
  AND index_meta.indisvalid = TRUE
  AND index_meta.indisready = TRUE
LIMIT 1`
}

func parseImportTaskIndexDefinition(definition string) ([]string, string, bool) {
	onPosition := strings.Index(definition, " on ")
	if onPosition < 0 {
		return nil, "", false
	}
	openPosition := strings.Index(definition[onPosition+4:], "(")
	if openPosition < 0 {
		return nil, "", false
	}
	openPosition += onPosition + 4
	depth := 0
	closePosition := -1
	for i := openPosition; i < len(definition); i++ {
		switch definition[i] {
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				closePosition = i
			}
		}
		if closePosition >= 0 {
			break
		}
	}
	if closePosition < 0 {
		return nil, "", false
	}
	keyColumns := splitImportTaskIndexColumns(definition[openPosition+1 : closePosition])
	predicate := strings.TrimSpace(definition[closePosition+1:])
	if !strings.HasPrefix(predicate, "where ") {
		return nil, "", false
	}
	return keyColumns, predicate, true
}

func splitImportTaskIndexColumns(columns string) []string {
	result := make([]string, 0, 5)
	start := 0
	depth := 0
	for i := 0; i < len(columns); i++ {
		switch columns[i] {
		case '(':
			depth++
		case ')':
			depth--
		case ',':
			if depth == 0 {
				result = append(result, strings.TrimSpace(columns[start:i]))
				start = i + 1
			}
		}
	}
	result = append(result, strings.TrimSpace(columns[start:]))
	return result
}

func normalizeImportTaskIndexDefinition(definition string) string {
	definition = strings.ToLower(strings.Join(strings.Fields(definition), ""))
	definition = strings.ReplaceAll(definition, `"`, "")
	definition = strings.ReplaceAll(definition, "::text", "")
	definition = strings.ReplaceAll(definition, "::character varying", "")
	definition = strings.ReplaceAll(definition, "btrim", "trim")
	// PostgreSQL pg_get_indexdef renders TRIM(value) as TRIM(BOTH FROM value).
	// Whitespace is removed above, so normalize the deparser's BOTH FROM token
	// before comparing the canonical expression.
	definition = strings.ReplaceAll(definition, "bothfrom", "")
	definition = strings.ReplaceAll(definition, "(", "")
	definition = strings.ReplaceAll(definition, ")", "")
	return definition
}
