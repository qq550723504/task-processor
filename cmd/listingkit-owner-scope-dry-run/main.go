package main

import (
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"gorm.io/gorm"

	"task-processor/internal/core/config"
	"task-processor/internal/infra/database"
)

type report struct {
	GeneratedAt time.Time       `json:"generatedAt"`
	ConfigPath  string          `json:"configPath"`
	Database    databaseSummary `json:"database"`
	Summary     summary         `json:"summary"`
	Tables      []tableReport   `json:"tables"`
}

type databaseSummary struct {
	Host string `json:"host"`
	Port int    `json:"port"`
	Name string `json:"name"`
}

type summary struct {
	TotalTables            int `json:"totalTables"`
	TablesWithMissingCols  int `json:"tablesWithMissingColumns"`
	TablesNeedingBackfill  int `json:"tablesNeedingBackfill"`
	TotalRowsNeedingReview int `json:"totalRowsNeedingReview"`
}

type unresolvedSummary struct {
	TaskGroups   []taskGroupSummary   `json:"taskGroups,omitempty"`
	StudioGroups []studioGroupSummary `json:"studioGroups,omitempty"`
}

type taskGroupSummary struct {
	TenantID         string   `json:"tenantId,omitempty"`
	RequestTenantID  string   `json:"requestTenantId,omitempty"`
	SheinStoreID     string   `json:"sheinStoreId,omitempty"`
	Status           string   `json:"status,omitempty"`
	Count            int      `json:"count"`
	SampleTaskIDs    []string `json:"sampleTaskIds,omitempty"`
	SuggestedUserID  string   `json:"suggestedUserId,omitempty"`
	SuggestionReason string   `json:"suggestionReason,omitempty"`
}

type studioGroupSummary struct {
	TenantID         string   `json:"tenantId,omitempty"`
	Status           string   `json:"status,omitempty"`
	Count            int      `json:"count"`
	SampleSessionIDs []string `json:"sampleSessionIds,omitempty"`
	SuggestedUserID  string   `json:"suggestedUserId,omitempty"`
	SuggestionReason string   `json:"suggestionReason,omitempty"`
}

type tableReport struct {
	TableName            string           `json:"tableName"`
	Kind                 string           `json:"kind"`
	RowCount             int64            `json:"rowCount"`
	ExistingColumns      []string         `json:"existingColumns"`
	MissingColumns       []string         `json:"missingColumns,omitempty"`
	Metrics              map[string]int64 `json:"metrics,omitempty"`
	NeedsManualReview    bool             `json:"needsManualReview"`
	NeedsSchemaMigration bool             `json:"needsSchemaMigration"`
	DDLPreview           []string         `json:"ddlPreview,omitempty"`
	BackfillPreview      []string         `json:"backfillPreview,omitempty"`
	Notes                []string         `json:"notes,omitempty"`
}

type tableSpec struct {
	TableName       string
	Kind            string
	RequiredColumns []string
	Metrics         func(*gorm.DB, map[string]bool) (map[string]int64, []string, []string, error)
}

func main() {
	var (
		configPath          = flag.String("config", "config/config-dev.yaml", "path to config file")
		outputPath          = flag.String("output", ".local/tmp/listingkit-owner-scope-dry-run.json", "path to JSON report")
		sqlPath             = flag.String("sql-output", ".local/tmp/listingkit-owner-scope-dry-run.sql", "path to combined SQL preview output")
		schemaPath          = flag.String("schema-output", ".local/tmp/listingkit-owner-scope-schema.sql", "path to schema SQL preview output")
		backfillPath        = flag.String("backfill-output", ".local/tmp/listingkit-owner-scope-backfill.sql", "path to backfill SQL preview output")
		safeBackfillPath    = flag.String("safe-backfill-output", ".local/tmp/listingkit-owner-scope-safe-backfill.sql", "path to safe backfill SQL preview output")
		manualReviewSQLPath = flag.String("manual-review-output", ".local/tmp/listingkit-owner-scope-manual-review.sql", "path to manual review SQL preview output")
		tasksJSONPath       = flag.String("unresolved-tasks-json", ".local/tmp/listingkit-owner-scope-unresolved-tasks.json", "path to unresolved task rows JSON")
		tasksCSVPath        = flag.String("unresolved-tasks-csv", ".local/tmp/listingkit-owner-scope-unresolved-tasks.csv", "path to unresolved task rows CSV")
		studioJSONPath      = flag.String("unresolved-studio-json", ".local/tmp/listingkit-owner-scope-unresolved-studio-sessions.json", "path to unresolved studio session rows JSON")
		studioCSVPath       = flag.String("unresolved-studio-csv", ".local/tmp/listingkit-owner-scope-unresolved-studio-sessions.csv", "path to unresolved studio session rows CSV")
		summaryJSONPath     = flag.String("unresolved-summary-json", ".local/tmp/listingkit-owner-scope-unresolved-summary.json", "path to grouped unresolved summary JSON")
	)
	flag.Parse()

	cfg, err := config.LoadConfigFromFileWithoutValidation(*configPath)
	if err != nil {
		exitf("load config: %v", err)
	}
	if cfg.Database == nil {
		exitf("database config is empty")
	}

	db, err := database.NewDatabaseFromConfig(cfg.Database)
	if err != nil {
		exitf("connect database: %v", err)
	}
	defer func() { _ = database.CloseDatabase(db) }()

	specs := []tableSpec{
		newOwnerAuditSpec("listing_store"),
		newOwnerAuditSpec("listing_category"),
		newOwnerAuditSpec("listing_filter_rule"),
		newOwnerAuditSpec("listing_product_import_task"),
		newOwnerAuditSpec("listing_profit_rule"),
		newOwnerAuditSpec("listing_pricing_rule"),
		newOwnerAuditSpec("listing_operation_strategy"),
		newOwnerAuditSpec("listing_sensitive_word"),
		newOwnerAuditSpec("listing_product_import_mapping"),
		newOwnerAuditSpec("listing_product_data"),
		newTaskSpec(),
		newStudioSessionSpec(),
		newStudioDesignSpec(),
	}

	out := report{
		GeneratedAt: time.Now().UTC(),
		ConfigPath:  *configPath,
		Database: databaseSummary{
			Host: cfg.Database.Host,
			Port: cfg.Database.Port,
			Name: cfg.Database.Database,
		},
		Tables: make([]tableReport, 0, len(specs)),
	}

	for _, spec := range specs {
		table, err := buildTableReport(db, spec)
		if err != nil {
			exitf("inspect %s: %v", spec.TableName, err)
		}
		if len(table.MissingColumns) > 0 {
			out.Summary.TablesWithMissingCols++
		}
		if tableNeedsBackfill(table.Metrics) {
			out.Summary.TablesNeedingBackfill++
		}
		if table.NeedsManualReview {
			out.Summary.TotalRowsNeedingReview += int(metricValue(table.Metrics, "manual_review_rows"))
		}
		out.Tables = append(out.Tables, table)
	}
	out.Summary.TotalTables = len(out.Tables)

	if err := os.MkdirAll(filepath.Dir(*outputPath), 0o755); err != nil {
		exitf("create output dir: %v", err)
	}
	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		exitf("marshal report: %v", err)
	}
	if err := os.WriteFile(*outputPath, data, 0o644); err != nil {
		exitf("write report: %v", err)
	}
	if err := os.WriteFile(*sqlPath, []byte(renderSQLPreview(out)), 0o644); err != nil {
		exitf("write sql preview: %v", err)
	}
	if err := os.WriteFile(*schemaPath, []byte(renderSchemaSQL(out)), 0o644); err != nil {
		exitf("write schema preview: %v", err)
	}
	if err := os.WriteFile(*backfillPath, []byte(renderBackfillSQL(out)), 0o644); err != nil {
		exitf("write backfill preview: %v", err)
	}
	if err := os.WriteFile(*safeBackfillPath, []byte(renderSafeBackfillSQL(out)), 0o644); err != nil {
		exitf("write safe backfill preview: %v", err)
	}
	if err := os.WriteFile(*manualReviewSQLPath, []byte(renderManualReviewSQL(out)), 0o644); err != nil {
		exitf("write manual review preview: %v", err)
	}

	unresolvedTasks, err := loadUnresolvedTaskRows(db)
	if err != nil {
		exitf("load unresolved tasks: %v", err)
	}
	unresolvedStudio, err := loadUnresolvedStudioSessionRows(db)
	if err != nil {
		exitf("load unresolved studio sessions: %v", err)
	}
	hints, err := loadOwnerHints(db)
	if err != nil {
		exitf("load owner hints: %v", err)
	}
	applyOwnershipSuggestions(unresolvedTasks, unresolvedStudio, hints)
	if err := writeJSON(*tasksJSONPath, unresolvedTasks); err != nil {
		exitf("write unresolved tasks json: %v", err)
	}
	if err := writeJSON(*studioJSONPath, unresolvedStudio); err != nil {
		exitf("write unresolved studio json: %v", err)
	}
	if err := writeTaskCSV(*tasksCSVPath, unresolvedTasks); err != nil {
		exitf("write unresolved tasks csv: %v", err)
	}
	if err := writeStudioCSV(*studioCSVPath, unresolvedStudio); err != nil {
		exitf("write unresolved studio csv: %v", err)
	}
	groupedSummary := unresolvedSummary{
		TaskGroups:   summarizeUnresolvedTasks(unresolvedTasks),
		StudioGroups: summarizeUnresolvedStudio(unresolvedStudio),
	}
	if err := writeJSON(*summaryJSONPath, groupedSummary); err != nil {
		exitf("write unresolved summary json: %v", err)
	}

	fmt.Printf("wrote owner-scope dry-run report to %s\n", *outputPath)
	fmt.Printf("wrote owner-scope sql preview to %s\n", *sqlPath)
	fmt.Printf("wrote owner-scope schema preview to %s\n", *schemaPath)
	fmt.Printf("wrote owner-scope backfill preview to %s\n", *backfillPath)
	fmt.Printf("wrote owner-scope safe backfill preview to %s\n", *safeBackfillPath)
	fmt.Printf("wrote owner-scope manual review preview to %s\n", *manualReviewSQLPath)
	fmt.Printf("wrote unresolved task rows to %s and %s\n", *tasksJSONPath, *tasksCSVPath)
	fmt.Printf("wrote unresolved studio session rows to %s and %s\n", *studioJSONPath, *studioCSVPath)
	fmt.Printf("wrote unresolved grouped summary to %s\n", *summaryJSONPath)
}

func buildTableReport(db *gorm.DB, spec tableSpec) (tableReport, error) {
	columnSet, err := loadColumnSet(db, spec.TableName)
	if err != nil {
		return tableReport{}, err
	}

	existing := make([]string, 0, len(columnSet))
	for column := range columnSet {
		existing = append(existing, column)
	}
	sort.Strings(existing)

	missing := make([]string, 0, len(spec.RequiredColumns))
	for _, column := range spec.RequiredColumns {
		if !columnSet[column] {
			missing = append(missing, column)
		}
	}

	rowCount, err := countRows(db, spec.TableName)
	if err != nil {
		return tableReport{}, err
	}

	metrics, ddl, backfill, err := spec.Metrics(db, columnSet)
	if err != nil {
		return tableReport{}, err
	}

	report := tableReport{
		TableName:            spec.TableName,
		Kind:                 spec.Kind,
		RowCount:             rowCount,
		ExistingColumns:      existing,
		MissingColumns:       missing,
		Metrics:              metrics,
		NeedsManualReview:    metricValue(metrics, "manual_review_rows") > 0,
		NeedsSchemaMigration: len(missing) > 0,
		DDLPreview:           ddl,
		BackfillPreview:      backfill,
	}
	if len(missing) > 0 {
		report.Notes = append(report.Notes, "run schema migration or application auto-migrate before enabling owner scope")
	}
	if report.NeedsManualReview {
		report.Notes = append(report.Notes, "some rows still cannot be assigned an owner/audit value from current fallback sources")
	}
	return report, nil
}

func newOwnerAuditSpec(tableName string) tableSpec {
	return tableSpec{
		TableName:       tableName,
		Kind:            "owner_audit",
		RequiredColumns: []string{"owner_user_id", "created_by", "updated_by"},
		Metrics: func(db *gorm.DB, columns map[string]bool) (map[string]int64, []string, []string, error) {
			metrics := map[string]int64{}
			ownerTargetMissing := blankCondition("owner_user_id", columns)
			createdTargetMissing := blankCondition("created_by", columns)
			updatedTargetMissing := blankCondition("updated_by", columns)

			ownerFillExpr := firstNonBlankExpr([]string{"owner_user_id", "created_by", "creator"}, columns)
			createdFillExpr := firstNonBlankExpr([]string{"created_by", "creator", "owner_user_id"}, columns)
			updatedFillExpr := firstNonBlankExpr([]string{"updated_by", "updater", "created_by", "creator", "owner_user_id"}, columns)

			values := map[string]*int64{
				"owner_missing_rows":           new(int64),
				"owner_backfillable_rows":      new(int64),
				"owner_unresolved_rows":        new(int64),
				"created_by_missing_rows":      new(int64),
				"created_by_backfillable_rows": new(int64),
				"created_by_unresolved_rows":   new(int64),
				"updated_by_missing_rows":      new(int64),
				"updated_by_backfillable_rows": new(int64),
				"updated_by_unresolved_rows":   new(int64),
			}

			for key, sql := range map[string]string{
				"owner_missing_rows":           fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE %s", quoteName(tableName), ownerTargetMissing),
				"owner_backfillable_rows":      fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE %s AND %s IS NOT NULL", quoteName(tableName), ownerTargetMissing, ownerFillExpr),
				"owner_unresolved_rows":        fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE %s AND %s IS NULL", quoteName(tableName), ownerTargetMissing, ownerFillExpr),
				"created_by_missing_rows":      fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE %s", quoteName(tableName), createdTargetMissing),
				"created_by_backfillable_rows": fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE %s AND %s IS NOT NULL", quoteName(tableName), createdTargetMissing, createdFillExpr),
				"created_by_unresolved_rows":   fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE %s AND %s IS NULL", quoteName(tableName), createdTargetMissing, createdFillExpr),
				"updated_by_missing_rows":      fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE %s", quoteName(tableName), updatedTargetMissing),
				"updated_by_backfillable_rows": fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE %s AND %s IS NOT NULL", quoteName(tableName), updatedTargetMissing, updatedFillExpr),
				"updated_by_unresolved_rows":   fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE %s AND %s IS NULL", quoteName(tableName), updatedTargetMissing, updatedFillExpr),
			} {
				if err := db.Raw(sql).Scan(values[key]).Error; err != nil {
					return nil, nil, nil, err
				}
			}

			for key, ptr := range values {
				metrics[key] = derefInt64(ptr)
			}
			metrics["manual_review_rows"] = maxInt64(metrics["owner_unresolved_rows"], metrics["created_by_unresolved_rows"], metrics["updated_by_unresolved_rows"])

			ddl := []string{
				fmt.Sprintf("ALTER TABLE %s ADD COLUMN IF NOT EXISTS owner_user_id varchar(128);", quoteName(tableName)),
				fmt.Sprintf("ALTER TABLE %s ADD COLUMN IF NOT EXISTS created_by varchar(128);", quoteName(tableName)),
				fmt.Sprintf("ALTER TABLE %s ADD COLUMN IF NOT EXISTS updated_by varchar(128);", quoteName(tableName)),
				fmt.Sprintf("CREATE INDEX IF NOT EXISTS %s ON %s (owner_user_id);", quoteName("idx_"+tableName+"_owner_user_id"), quoteName(tableName)),
			}

			backfill := []string{
				fmt.Sprintf(
					"UPDATE %s SET owner_user_id = COALESCE(%s), created_by = COALESCE(%s), updated_by = COALESCE(%s) WHERE %s OR %s OR %s;",
					quoteName(tableName),
					joinCoalesceArgs([]string{"owner_user_id", "created_by", "creator"}),
					joinCoalesceArgs([]string{"created_by", "creator", "owner_user_id"}),
					joinCoalesceArgs([]string{"updated_by", "updater", "created_by", "creator", "owner_user_id"}),
					sqlBlank("owner_user_id"),
					sqlBlank("created_by"),
					sqlBlank("updated_by"),
				),
			}

			return metrics, ddl, backfill, nil
		},
	}
}

func newTaskSpec() tableSpec {
	return tableSpec{
		TableName:       "listing_kit_tasks",
		Kind:            "task_scope",
		RequiredColumns: []string{"tenant_id", "user_id"},
		Metrics: func(db *gorm.DB, columns map[string]bool) (map[string]int64, []string, []string, error) {
			metrics := map[string]int64{}
			for key, sql := range map[string]string{
				"tenant_id_missing_rows":         fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE %s", quoteName("listing_kit_tasks"), blankCondition("tenant_id", columns)),
				"user_id_missing_rows":           fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE %s", quoteName("listing_kit_tasks"), blankCondition("user_id", columns)),
				"tenant_id_backfillable_rows":    `SELECT COUNT(*) FROM "listing_kit_tasks" WHERE (tenant_id IS NULL OR BTRIM(tenant_id) = '') AND NULLIF(BTRIM((request::jsonb ->> 'tenant_id')), '') IS NOT NULL`,
				"user_id_backfillable_rows":      `SELECT COUNT(*) FROM "listing_kit_tasks" WHERE (user_id IS NULL OR BTRIM(user_id) = '') AND NULLIF(BTRIM((request::jsonb ->> 'user_id')), '') IS NOT NULL`,
				"tenant_id_unresolved_rows":      `SELECT COUNT(*) FROM "listing_kit_tasks" WHERE (tenant_id IS NULL OR BTRIM(tenant_id) = '') AND NULLIF(BTRIM((request::jsonb ->> 'tenant_id')), '') IS NULL`,
				"user_id_unresolved_rows":        `SELECT COUNT(*) FROM "listing_kit_tasks" WHERE (user_id IS NULL OR BTRIM(user_id) = '') AND NULLIF(BTRIM((request::jsonb ->> 'user_id')), '') IS NULL`,
				"request_missing_user_id_rows":   `SELECT COUNT(*) FROM "listing_kit_tasks" WHERE NULLIF(BTRIM((request::jsonb ->> 'user_id')), '') IS NULL`,
				"request_missing_tenant_id_rows": `SELECT COUNT(*) FROM "listing_kit_tasks" WHERE NULLIF(BTRIM((request::jsonb ->> 'tenant_id')), '') IS NULL`,
				"request_present_user_id_rows":   `SELECT COUNT(*) FROM "listing_kit_tasks" WHERE NULLIF(BTRIM((request::jsonb ->> 'user_id')), '') IS NOT NULL`,
				"request_present_tenant_id_rows": `SELECT COUNT(*) FROM "listing_kit_tasks" WHERE NULLIF(BTRIM((request::jsonb ->> 'tenant_id')), '') IS NOT NULL`,
			} {
				var count int64
				if err := db.Raw(sql).Scan(&count).Error; err != nil {
					return nil, nil, nil, err
				}
				metrics[key] = count
			}
			metrics["manual_review_rows"] = maxInt64(metrics["tenant_id_unresolved_rows"], metrics["user_id_unresolved_rows"])
			notesSQL := []string{
				`UPDATE "listing_kit_tasks" SET tenant_id = COALESCE(NULLIF(BTRIM(tenant_id), ''), NULLIF(BTRIM((request::jsonb ->> 'tenant_id')), '')) WHERE tenant_id IS NULL OR BTRIM(tenant_id) = '';`,
				`UPDATE "listing_kit_tasks" SET user_id = COALESCE(NULLIF(BTRIM(user_id), ''), NULLIF(BTRIM((request::jsonb ->> 'user_id')), '')) WHERE user_id IS NULL OR BTRIM(user_id) = '';`,
			}
			return metrics, nil, notesSQL, nil
		},
	}
}

func newStudioSessionSpec() tableSpec {
	return tableSpec{
		TableName:       "shein_studio_sessions",
		Kind:            "studio_session_scope",
		RequiredColumns: []string{"tenant_id", "user_id"},
		Metrics: func(db *gorm.DB, columns map[string]bool) (map[string]int64, []string, []string, error) {
			metrics := map[string]int64{}
			for key, sql := range map[string]string{
				"tenant_id_missing_rows": fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE %s", quoteName("shein_studio_sessions"), blankCondition("tenant_id", columns)),
				"user_id_missing_rows":   fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE %s", quoteName("shein_studio_sessions"), blankCondition("user_id", columns)),
			} {
				var count int64
				if err := db.Raw(sql).Scan(&count).Error; err != nil {
					return nil, nil, nil, err
				}
				metrics[key] = count
			}
			metrics["tenant_id_backfillable_rows"] = 0
			metrics["user_id_backfillable_rows"] = 0
			metrics["tenant_id_unresolved_rows"] = metrics["tenant_id_missing_rows"]
			metrics["user_id_unresolved_rows"] = metrics["user_id_missing_rows"]
			metrics["manual_review_rows"] = maxInt64(metrics["tenant_id_missing_rows"], metrics["user_id_missing_rows"])
			return metrics, nil, nil, nil
		},
	}
}

func newStudioDesignSpec() tableSpec {
	return tableSpec{
		TableName:       "shein_studio_designs",
		Kind:            "studio_design_scope",
		RequiredColumns: []string{"tenant_id", "session_id"},
		Metrics: func(db *gorm.DB, columns map[string]bool) (map[string]int64, []string, []string, error) {
			metrics := map[string]int64{}
			var tenantMissing int64
			if err := db.Raw(
				`SELECT COUNT(*) FROM "shein_studio_designs" WHERE tenant_id IS NULL OR BTRIM(tenant_id) = ''`,
			).Scan(&tenantMissing).Error; err != nil {
				return nil, nil, nil, err
			}
			var tenantBackfillable int64
			if err := db.Raw(
				`SELECT COUNT(*) FROM "shein_studio_designs" d JOIN "shein_studio_sessions" s ON s.id = d.session_id WHERE (d.tenant_id IS NULL OR BTRIM(d.tenant_id) = '') AND s.tenant_id IS NOT NULL AND BTRIM(s.tenant_id) <> ''`,
			).Scan(&tenantBackfillable).Error; err != nil {
				return nil, nil, nil, err
			}
			metrics["tenant_id_missing_rows"] = tenantMissing
			metrics["tenant_id_backfillable_rows"] = tenantBackfillable
			metrics["manual_review_rows"] = tenantMissing - tenantBackfillable
			backfill := []string{
				`UPDATE "shein_studio_designs" d SET tenant_id = s.tenant_id FROM "shein_studio_sessions" s WHERE s.id = d.session_id AND (d.tenant_id IS NULL OR BTRIM(d.tenant_id) = '') AND s.tenant_id IS NOT NULL AND BTRIM(s.tenant_id) <> '';`,
			}
			return metrics, nil, backfill, nil
		},
	}
}

func loadColumnSet(db *gorm.DB, tableName string) (map[string]bool, error) {
	var rows []struct {
		ColumnName string `gorm:"column:column_name"`
	}
	if err := db.Raw(`
SELECT column_name
FROM information_schema.columns
WHERE table_schema = current_schema() AND table_name = ?
ORDER BY ordinal_position
`, tableName).Scan(&rows).Error; err != nil {
		return nil, err
	}
	result := make(map[string]bool, len(rows))
	for _, row := range rows {
		result[row.ColumnName] = true
	}
	return result, nil
}

func countRows(db *gorm.DB, tableName string) (int64, error) {
	var count int64
	if err := db.Raw("SELECT COUNT(*) FROM " + quoteName(tableName)).Scan(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

func blankCondition(column string, columns map[string]bool) string {
	if !columns[column] {
		return "TRUE"
	}
	return sqlBlank(column)
}

func sqlBlank(column string) string {
	return fmt.Sprintf(`%s IS NULL OR BTRIM(%s) = ''`, quoteName(column), quoteName(column))
}

func firstNonBlankExpr(candidates []string, columns map[string]bool) string {
	args := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		if columns[candidate] {
			args = append(args, nullIfBlank(candidate))
		}
	}
	if len(args) == 0 {
		return "NULL"
	}
	if len(args) == 1 {
		return args[0]
	}
	return "COALESCE(" + strings.Join(args, ", ") + ")"
}

func joinCoalesceArgs(columns []string) string {
	args := make([]string, 0, len(columns))
	for _, column := range columns {
		args = append(args, nullIfBlank(column))
	}
	return strings.Join(args, ", ")
}

func nullIfBlank(column string) string {
	return fmt.Sprintf(`NULLIF(BTRIM(%s), '')`, quoteName(column))
}

func quoteName(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}

func metricValue(metrics map[string]int64, key string) int64 {
	if metrics == nil {
		return 0
	}
	return metrics[key]
}

func tableNeedsBackfill(metrics map[string]int64) bool {
	for key, value := range metrics {
		if strings.HasSuffix(key, "_missing_rows") && value > 0 {
			return true
		}
	}
	return false
}

func derefInt64(value *int64) int64 {
	if value == nil {
		return 0
	}
	return *value
}

func maxInt64(values ...int64) int64 {
	var max int64
	for _, value := range values {
		if value > max {
			max = value
		}
	}
	return max
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func exitf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}

type unresolvedTaskRow struct {
	ID               string    `json:"id"`
	TenantID         string    `json:"tenantId,omitempty"`
	UserID           string    `json:"userId,omitempty"`
	Status           string    `json:"status,omitempty"`
	CreatedAt        time.Time `json:"createdAt"`
	RequestUserID    string    `json:"requestUserId,omitempty"`
	RequestTenantID  string    `json:"requestTenantId,omitempty"`
	SheinStoreID     string    `json:"sheinStoreId,omitempty"`
	SuggestedUserID  string    `json:"suggestedUserId,omitempty"`
	SuggestionReason string    `json:"suggestionReason,omitempty"`
	RequestRaw       string    `json:"-"`
	RequestPreview   string    `json:"requestPreview,omitempty"`
}

type unresolvedStudioSessionRow struct {
	ID               string    `json:"id"`
	TenantID         string    `json:"tenantId,omitempty"`
	UserID           string    `json:"userId,omitempty"`
	SelectionKey     string    `json:"selectionKey,omitempty"`
	Status           string    `json:"status,omitempty"`
	CreatedAt        time.Time `json:"createdAt"`
	UpdatedAt        time.Time `json:"updatedAt"`
	SuggestedUserID  string    `json:"suggestedUserId,omitempty"`
	SuggestionReason string    `json:"suggestionReason,omitempty"`
}

func loadUnresolvedTaskRows(db *gorm.DB) ([]unresolvedTaskRow, error) {
	rows := make([]unresolvedTaskRow, 0)
	err := db.Raw(`
SELECT
  id,
  tenant_id,
  user_id,
  status,
  created_at,
  NULLIF(BTRIM((request::jsonb ->> 'user_id')), '') AS request_user_id,
  NULLIF(BTRIM((request::jsonb ->> 'tenant_id')), '') AS request_tenant_id,
  NULLIF(BTRIM(COALESCE(request::jsonb ->> 'shein_store_id', request::jsonb #>> '{options,sds,shein_store_id}', request::jsonb #>> '{options,shein_store_id}')), '') AS shein_store_id,
  COALESCE(request, '') AS request_raw,
  LEFT(COALESCE(request, ''), 2000) AS request_preview
FROM "listing_kit_tasks"
WHERE (user_id IS NULL OR BTRIM(user_id) = '')
  AND NULLIF(BTRIM((request::jsonb ->> 'user_id')), '') IS NULL
ORDER BY created_at DESC
`).Scan(&rows).Error
	return rows, err
}

func loadUnresolvedStudioSessionRows(db *gorm.DB) ([]unresolvedStudioSessionRow, error) {
	rows := make([]unresolvedStudioSessionRow, 0)
	err := db.Raw(`
SELECT
  id,
  tenant_id,
  user_id,
  selection_key,
  status,
  created_at,
  updated_at
FROM "shein_studio_sessions"
WHERE user_id IS NULL OR BTRIM(user_id) = ''
ORDER BY updated_at DESC
`).Scan(&rows).Error
	return rows, err
}

type ownerHints struct {
	StoreOwners    map[string]string
	UniqueByTenant map[string]string
}

func loadOwnerHints(db *gorm.DB) (*ownerHints, error) {
	type storeOwnerRow struct {
		TenantID    string `gorm:"column:tenant_id"`
		OwnerUserID string `gorm:"column:owner_user_id"`
		StoreID     string `gorm:"column:store_id"`
	}

	var rows []storeOwnerRow
	if err := db.Raw(`
SELECT
  BTRIM(COALESCE(tenant_id::text, '')) AS tenant_id,
  BTRIM(COALESCE(owner_user_id, '')) AS owner_user_id,
  BTRIM(COALESCE(store_id, '')) AS store_id
FROM "listing_store"
WHERE deleted = 0
`).Scan(&rows).Error; err != nil {
		return nil, err
	}

	hints := &ownerHints{
		StoreOwners:    map[string]string{},
		UniqueByTenant: map[string]string{},
	}
	tenantOwners := map[string]map[string]struct{}{}
	for _, row := range rows {
		tenantID := strings.TrimSpace(row.TenantID)
		ownerUserID := strings.TrimSpace(row.OwnerUserID)
		storeID := strings.TrimSpace(row.StoreID)
		if tenantID == "" || ownerUserID == "" {
			continue
		}
		if storeID != "" {
			hints.StoreOwners[tenantID+"|"+storeID] = ownerUserID
		}
		if tenantOwners[tenantID] == nil {
			tenantOwners[tenantID] = map[string]struct{}{}
		}
		tenantOwners[tenantID][ownerUserID] = struct{}{}
	}
	for tenantID, owners := range tenantOwners {
		if len(owners) != 1 {
			continue
		}
		for owner := range owners {
			hints.UniqueByTenant[tenantID] = owner
		}
	}
	return hints, nil
}

func applyOwnershipSuggestions(tasks []unresolvedTaskRow, studio []unresolvedStudioSessionRow, hints *ownerHints) {
	if hints == nil {
		return
	}
	for i := range tasks {
		tenantID := firstNonEmpty(tasks[i].TenantID, tasks[i].RequestTenantID)
		if tenantID != "" && tasks[i].SheinStoreID != "" {
			if owner := hints.StoreOwners[tenantID+"|"+tasks[i].SheinStoreID]; owner != "" {
				tasks[i].SuggestedUserID = owner
				tasks[i].SuggestionReason = "matched_listing_store_by_tenant_and_shein_store_id"
				continue
			}
		}
		if tenantID != "" {
			if owner := hints.UniqueByTenant[tenantID]; owner != "" {
				tasks[i].SuggestedUserID = owner
				tasks[i].SuggestionReason = "single_owner_in_tenant"
			}
		}
	}
	for i := range studio {
		if owner := hints.UniqueByTenant[strings.TrimSpace(studio[i].TenantID)]; owner != "" {
			studio[i].SuggestedUserID = owner
			studio[i].SuggestionReason = "single_owner_in_tenant"
		}
	}
}

func writeJSON(path string, value any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func writeTaskCSV(path string, rows []unresolvedTaskRow) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()
	writer := csv.NewWriter(file)
	defer writer.Flush()
	if err := writer.Write([]string{"id", "tenant_id", "user_id", "status", "created_at", "request_user_id", "request_tenant_id", "shein_store_id", "suggested_user_id", "suggestion_reason", "request_preview"}); err != nil {
		return err
	}
	for _, row := range rows {
		record := []string{
			row.ID,
			row.TenantID,
			row.UserID,
			row.Status,
			row.CreatedAt.Format(time.RFC3339),
			row.RequestUserID,
			row.RequestTenantID,
			row.SheinStoreID,
			row.SuggestedUserID,
			row.SuggestionReason,
			row.RequestPreview,
		}
		if err := writer.Write(record); err != nil {
			return err
		}
	}
	return writer.Error()
}

func writeStudioCSV(path string, rows []unresolvedStudioSessionRow) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()
	writer := csv.NewWriter(file)
	defer writer.Flush()
	if err := writer.Write([]string{"id", "tenant_id", "user_id", "selection_key", "status", "created_at", "updated_at", "suggested_user_id", "suggestion_reason"}); err != nil {
		return err
	}
	for _, row := range rows {
		record := []string{
			row.ID,
			row.TenantID,
			row.UserID,
			row.SelectionKey,
			row.Status,
			row.CreatedAt.Format(time.RFC3339),
			row.UpdatedAt.Format(time.RFC3339),
			row.SuggestedUserID,
			row.SuggestionReason,
		}
		if err := writer.Write(record); err != nil {
			return err
		}
	}
	return writer.Error()
}

func summarizeUnresolvedTasks(rows []unresolvedTaskRow) []taskGroupSummary {
	type aggregate struct {
		taskGroupSummary
	}
	groups := map[string]*aggregate{}
	for _, row := range rows {
		key := strings.Join([]string{
			firstNonEmpty(row.TenantID, row.RequestTenantID),
			row.RequestTenantID,
			row.SheinStoreID,
			row.Status,
			row.SuggestedUserID,
			row.SuggestionReason,
		}, "|")
		group := groups[key]
		if group == nil {
			group = &aggregate{
				taskGroupSummary: taskGroupSummary{
					TenantID:         firstNonEmpty(row.TenantID, row.RequestTenantID),
					RequestTenantID:  row.RequestTenantID,
					SheinStoreID:     row.SheinStoreID,
					Status:           row.Status,
					SuggestedUserID:  row.SuggestedUserID,
					SuggestionReason: row.SuggestionReason,
				},
			}
			groups[key] = group
		}
		group.Count++
		if len(group.SampleTaskIDs) < 5 {
			group.SampleTaskIDs = append(group.SampleTaskIDs, row.ID)
		}
	}
	result := make([]taskGroupSummary, 0, len(groups))
	for _, group := range groups {
		result = append(result, group.taskGroupSummary)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Count != result[j].Count {
			return result[i].Count > result[j].Count
		}
		if result[i].TenantID != result[j].TenantID {
			return result[i].TenantID < result[j].TenantID
		}
		if result[i].SheinStoreID != result[j].SheinStoreID {
			return result[i].SheinStoreID < result[j].SheinStoreID
		}
		return result[i].Status < result[j].Status
	})
	return result
}

func summarizeUnresolvedStudio(rows []unresolvedStudioSessionRow) []studioGroupSummary {
	type aggregate struct {
		studioGroupSummary
	}
	groups := map[string]*aggregate{}
	for _, row := range rows {
		key := strings.Join([]string{
			row.TenantID,
			row.Status,
			row.SuggestedUserID,
			row.SuggestionReason,
		}, "|")
		group := groups[key]
		if group == nil {
			group = &aggregate{
				studioGroupSummary: studioGroupSummary{
					TenantID:         row.TenantID,
					Status:           row.Status,
					SuggestedUserID:  row.SuggestedUserID,
					SuggestionReason: row.SuggestionReason,
				},
			}
			groups[key] = group
		}
		group.Count++
		if len(group.SampleSessionIDs) < 5 {
			group.SampleSessionIDs = append(group.SampleSessionIDs, row.ID)
		}
	}
	result := make([]studioGroupSummary, 0, len(groups))
	for _, group := range groups {
		result = append(result, group.studioGroupSummary)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Count != result[j].Count {
			return result[i].Count > result[j].Count
		}
		if result[i].TenantID != result[j].TenantID {
			return result[i].TenantID < result[j].TenantID
		}
		return result[i].Status < result[j].Status
	})
	return result
}

func renderSQLPreview(in report) string {
	var b strings.Builder
	b.WriteString("-- ListingKit owner-scope dry-run SQL preview\n")
	b.WriteString("-- Generated at: " + in.GeneratedAt.Format(time.RFC3339) + "\n")
	b.WriteString("-- Database: " + in.Database.Name + "\n\n")
	for _, table := range in.Tables {
		b.WriteString("-- " + table.TableName + " (" + table.Kind + ")\n")
		for _, note := range table.Notes {
			b.WriteString("-- note: " + note + "\n")
		}
		for _, stmt := range table.DDLPreview {
			b.WriteString(stmt)
			if !strings.HasSuffix(stmt, "\n") {
				b.WriteString("\n")
			}
		}
		if len(table.DDLPreview) > 0 && len(table.BackfillPreview) > 0 {
			b.WriteString("\n")
		}
		for _, stmt := range table.BackfillPreview {
			b.WriteString(stmt)
			if !strings.HasSuffix(stmt, "\n") {
				b.WriteString("\n")
			}
		}
		b.WriteString("\n")
	}
	return b.String()
}

func renderSchemaSQL(in report) string {
	var b strings.Builder
	b.WriteString("-- ListingKit owner-scope schema SQL preview\n")
	b.WriteString("-- Generated at: " + in.GeneratedAt.Format(time.RFC3339) + "\n")
	b.WriteString("-- Database: " + in.Database.Name + "\n\n")
	for _, table := range in.Tables {
		if len(table.DDLPreview) == 0 {
			continue
		}
		b.WriteString("-- " + table.TableName + "\n")
		for _, stmt := range table.DDLPreview {
			b.WriteString(stmt)
			if !strings.HasSuffix(stmt, "\n") {
				b.WriteString("\n")
			}
		}
		b.WriteString("\n")
	}
	return b.String()
}

func renderBackfillSQL(in report) string {
	var b strings.Builder
	b.WriteString("-- ListingKit owner-scope backfill SQL preview\n")
	b.WriteString("-- Generated at: " + in.GeneratedAt.Format(time.RFC3339) + "\n")
	b.WriteString("-- Database: " + in.Database.Name + "\n\n")
	for _, table := range in.Tables {
		if len(table.BackfillPreview) == 0 {
			continue
		}
		b.WriteString("-- " + table.TableName + "\n")
		for _, note := range table.Notes {
			b.WriteString("-- note: " + note + "\n")
		}
		for _, stmt := range table.BackfillPreview {
			b.WriteString(stmt)
			if !strings.HasSuffix(stmt, "\n") {
				b.WriteString("\n")
			}
		}
		b.WriteString("\n")
	}
	return b.String()
}

func renderSafeBackfillSQL(in report) string {
	var b strings.Builder
	b.WriteString("-- ListingKit owner-scope safe backfill SQL preview\n")
	b.WriteString("-- Generated at: " + in.GeneratedAt.Format(time.RFC3339) + "\n")
	b.WriteString("-- Database: " + in.Database.Name + "\n")
	b.WriteString("-- Only includes tables whose fallback sources are already explicit in current schema.\n\n")
	for _, table := range in.Tables {
		if !isSafeBackfillTable(table) || len(table.BackfillPreview) == 0 {
			continue
		}
		b.WriteString("-- " + table.TableName + "\n")
		for _, stmt := range table.BackfillPreview {
			b.WriteString(stmt)
			if !strings.HasSuffix(stmt, "\n") {
				b.WriteString("\n")
			}
		}
		b.WriteString("\n")
	}
	return b.String()
}

func renderManualReviewSQL(in report) string {
	var b strings.Builder
	b.WriteString("-- ListingKit owner-scope manual review SQL preview\n")
	b.WriteString("-- Generated at: " + in.GeneratedAt.Format(time.RFC3339) + "\n")
	b.WriteString("-- Database: " + in.Database.Name + "\n")
	b.WriteString("-- These statements are for inspection first, not blind execution.\n\n")
	for _, table := range in.Tables {
		if !table.NeedsManualReview {
			continue
		}
		b.WriteString("-- " + table.TableName + "\n")
		for _, note := range table.Notes {
			b.WriteString("-- note: " + note + "\n")
		}
		for _, stmt := range manualReviewStatements(table.TableName) {
			b.WriteString(stmt)
			if !strings.HasSuffix(stmt, "\n") {
				b.WriteString("\n")
			}
		}
		b.WriteString("\n")
	}
	return b.String()
}

func isSafeBackfillTable(table tableReport) bool {
	switch table.TableName {
	case "listing_store",
		"listing_category",
		"listing_filter_rule",
		"listing_product_import_task",
		"listing_profit_rule",
		"listing_pricing_rule",
		"listing_operation_strategy",
		"listing_sensitive_word",
		"listing_product_import_mapping",
		"listing_product_data",
		"shein_studio_designs":
		return true
	default:
		return false
	}
}

func manualReviewStatements(tableName string) []string {
	switch tableName {
	case "listing_kit_tasks":
		return []string{
			`SELECT id, tenant_id, user_id, status, created_at, LEFT(COALESCE(request, ''), 500) AS request_preview FROM "listing_kit_tasks" WHERE (user_id IS NULL OR BTRIM(user_id) = '') AND NULLIF(BTRIM((request::jsonb ->> 'user_id')), '') IS NULL ORDER BY created_at DESC LIMIT 200;`,
			`SELECT COUNT(*) AS unresolved_rows FROM "listing_kit_tasks" WHERE (user_id IS NULL OR BTRIM(user_id) = '') AND NULLIF(BTRIM((request::jsonb ->> 'user_id')), '') IS NULL;`,
		}
	case "shein_studio_sessions":
		return []string{
			`SELECT id, tenant_id, user_id, selection_key, status, created_at, updated_at FROM "shein_studio_sessions" WHERE user_id IS NULL OR BTRIM(user_id) = '' ORDER BY updated_at DESC LIMIT 200;`,
			`SELECT COUNT(*) AS unresolved_rows FROM "shein_studio_sessions" WHERE user_id IS NULL OR BTRIM(user_id) = '';`,
		}
	default:
		return []string{
			`-- no table-specific review query registered`,
		}
	}
}
