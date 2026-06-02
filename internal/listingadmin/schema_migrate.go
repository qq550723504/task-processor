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
