package identitypreflight

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"testing"
)

func TestOwnerTableInventoryMatchesOwnerScopedModels(t *testing.T) {
	t.Parallel()

	repositoryRoot := testRepositoryRoot(t)
	discovered, err := discoverOwnerTables(
		productionNonPersistentOwnerModelExclusions(repositoryRoot),
		filepath.Join(repositoryRoot, "internal", "listingkit"),
		filepath.Join(repositoryRoot, "internal", "listingadmin"),
		filepath.Join(repositoryRoot, "internal", "infra", "clients", "openai"),
	)
	if err != nil {
		t.Fatalf("discover owner-scoped models: %v", err)
	}

	// shein_studio_designs is deliberately absent: it has no independent user
	// column and inherits ownership from shein_studio_sessions.
	// Generic AI telemetry is deliberately absent because telemetry identities
	// are not used as owner-scope filters.
	// The invitation audit table is excluded below because UserID is the target
	// of an audited action, not the row owner.
	excluded := map[string]string{
		"listingkit_member_invitation_audits": "member invitation target identifier is not an owner scope",
	}
	for table := range excluded {
		delete(discovered, table)
	}

	inventory := make(map[string]OwnerTable, len(ownerTableInventory))
	var duplicates []string
	for _, table := range ownerTableInventory {
		if _, exists := inventory[table.Table]; exists {
			duplicates = append(duplicates, table.Table)
		}
		inventory[table.Table] = table
	}
	if len(duplicates) > 0 {
		sort.Strings(duplicates)
		t.Fatalf("duplicate inventory tables: %v", duplicates)
	}

	var missing, stale []string
	for name, model := range discovered {
		entry, ok := inventory[name]
		if !ok || entry.TenantColumn != model.TenantColumn || entry.UserColumn != model.UserColumn || entry.TenantDomain != model.TenantDomain {
			missing = append(missing, formatOwnerTable(model))
		}
	}
	for name, entry := range inventory {
		model, ok := discovered[name]
		if !ok || entry.TenantColumn != model.TenantColumn || entry.UserColumn != model.UserColumn || entry.TenantDomain != model.TenantDomain {
			stale = append(stale, formatOwnerTable(entry))
		}
	}
	sort.Strings(missing)
	sort.Strings(stale)
	if len(missing) > 0 || len(stale) > 0 {
		t.Fatalf("missing from inventory: %v; stale inventory: %v", missing, stale)
	}
}

func TestSystemOwnedNativeTablesIgnoreBlankOwnerRows(t *testing.T) {
	for _, table := range ownerTableInventory {
		switch table.Table {
		case "listing_kit_tasks", "listingkit_shein_pod_image_indexes", "listingkit_studio_async_jobs", "listingkit_studio_batches", "listingkit_studio_batch_items", "listingkit_studio_generation_attempts", "listingkit_studio_materialized_designs", "listingkit_studio_batch_task_links", "listingkit_studio_batch_runs", "listingkit_studio_batch_run_items", "shein_studio_sessions":
			if table.BlankUserPolicy != BlankUserPolicyIgnore {
				t.Fatalf("%s blank policy = %v, want system-owned rows ignored", table.Table, table.BlankUserPolicy)
			}
		}
	}
}

func TestLegacyOwnerTablesIgnoreBlankOwnerRows(t *testing.T) {
	for _, table := range ownerTableInventory {
		if table.TenantDomain == TenantDomainLegacyNumeric && table.BlankUserPolicy != BlankUserPolicyIgnore {
			t.Fatalf("%s blank policy = %v, want ownerless system rows ignored", table.Table, table.BlankUserPolicy)
		}
	}
}

func TestDiscoverOwnerTablesIncludesConventionBasedGORMColumns(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	source := `package sample

type ownerRecord struct {
	TenantID string
	UserID string
}

func (ownerRecord) TableName() string { return "sample_owner_records" }
`
	if err := os.WriteFile(filepath.Join(root, "model.go"), []byte(source), 0o600); err != nil {
		t.Fatalf("write model fixture: %v", err)
	}

	tables, err := discoverOwnerTables(nil, root)
	if err != nil {
		t.Fatalf("discover owner tables: %v", err)
	}
	want := OwnerTable{Table: "sample_owner_records", TenantColumn: "tenant_id", UserColumn: "user_id", TenantDomain: TenantDomainZITADELOrganization}
	if got, ok := tables[want.Table]; !ok || got != want {
		t.Fatalf("convention-based owner table = %#v, %v; want %#v", got, ok, want)
	}
}

func TestDiscoverOwnerTablesRequiresLiteralTableNameForConventionOnlyCandidate(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	source := `package sample

type conventionOnlyOwnerRecord struct {
	TenantID string
	OwnerUserID string
}
`
	if err := os.WriteFile(filepath.Join(root, "model.go"), []byte(source), 0o600); err != nil {
		t.Fatalf("write model fixture: %v", err)
	}

	_, err := discoverOwnerTables(nil, root)
	if err == nil {
		t.Fatal("discover owner tables error = nil, want missing literal TableName failure")
	}
	if !strings.Contains(err.Error(), "conventionOnlyOwnerRecord") ||
		!strings.Contains(err.Error(), "add an explicit production TableName method") {
		t.Fatalf("error = %q, want candidate name and explicit TableName instruction", err)
	}
}

func testRepositoryRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve inventory test path")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", ".."))
}

type ownerModel struct {
	TypeName     string
	TenantColumn string
	UserColumn   string
	TenantDomain TenantDomain
	Table        string
	Source       string
}

type ownerModelKey struct {
	Directory string
	Package   string
	TypeName  string
}

func productionNonPersistentOwnerModelExclusions(repositoryRoot string) map[ownerModelKey]string {
	listingKitDirectory := filepath.Join(repositoryRoot, "internal", "listingkit")
	listingAdminDirectory := filepath.Join(repositoryRoot, "internal", "listingadmin")
	identityPreflightDirectory := filepath.Join(listingKitDirectory, "identitypreflight")
	memberInviteDirectory := filepath.Join(listingKitDirectory, "memberinvite")
	storeDirectory := filepath.Join(listingKitDirectory, "store")
	openAIDirectory := filepath.Join(repositoryRoot, "internal", "infra", "clients", "openai")

	return map[ownerModelKey]string{
		{Directory: identityPreflightDirectory, Package: "identitypreflight", TypeName: "PersistedOwner"}:      "read-only aggregate result returned by the preflight repository",
		{Directory: identityPreflightDirectory, Package: "identitypreflight", TypeName: "unknownOwnerFinding"}: "in-memory comparison finding, never passed to GORM",

		{Directory: listingAdminDirectory, Package: "listingadmin", TypeName: "CategoryQuery"}:                "repository filter input; Category is represented by a separate persistence row",
		{Directory: listingAdminDirectory, Package: "listingadmin", TypeName: "FilterRuleQuery"}:              "repository filter input; FilterRule is represented by a separate persistence row",
		{Directory: listingAdminDirectory, Package: "listingadmin", TypeName: "GenerationTopicOverrideQuery"}: "repository filter input; GenerationTopicOverride is represented by a separate persistence row",
		{Directory: listingAdminDirectory, Package: "listingadmin", TypeName: "GenerationTopicPolicyQuery"}:   "repository filter input; GenerationTopicPolicy is represented by a separate persistence row",
		{Directory: listingAdminDirectory, Package: "listingadmin", TypeName: "ImportTaskQuery"}:              "repository filter input; ImportTask is represented by a separate persistence row",
		{Directory: listingAdminDirectory, Package: "listingadmin", TypeName: "OperationStrategyQuery"}:       "repository filter input; OperationStrategy is represented by a separate persistence row",
		{Directory: listingAdminDirectory, Package: "listingadmin", TypeName: "PricingRuleQuery"}:             "repository filter input; PricingRule is represented by a separate persistence row",
		{Directory: listingAdminDirectory, Package: "listingadmin", TypeName: "ProductDataQuery"}:             "repository filter input; ProductData is represented by a separate persistence row",
		{Directory: listingAdminDirectory, Package: "listingadmin", TypeName: "ProductImportMappingQuery"}:    "repository filter input; ProductImportMapping is represented by a separate persistence row",
		{Directory: listingAdminDirectory, Package: "listingadmin", TypeName: "ProfitRuleQuery"}:              "repository filter input; ProfitRule is represented by a separate persistence row",
		{Directory: listingAdminDirectory, Package: "listingadmin", TypeName: "ScheduledTaskConfigQuery"}:     "repository filter input; ScheduledTaskConfig is represented by a separate persistence row",
		{Directory: listingAdminDirectory, Package: "listingadmin", TypeName: "SensitiveWordQuery"}:           "repository filter input; SensitiveWord is represented by a separate persistence row",
		{Directory: listingAdminDirectory, Package: "listingadmin", TypeName: "StoreQuery"}:                   "repository filter input; Store is represented by listingStore for persistence",
		{Directory: listingAdminDirectory, Package: "listingadmin", TypeName: "StoreStatisticsQuery"}:         "repository aggregate filter input, never passed to GORM as a model",
		{Directory: listingAdminDirectory, Package: "listingadmin", TypeName: "listQueryScope"}:               "HTTP handler pagination and authorization input, never passed to GORM as a model",
		{Directory: listingAdminDirectory, Package: "listingadmin", TypeName: "Store"}:                        "API and domain view mapped to listingStore before persistence",
		{Directory: listingAdminDirectory, Package: "listingadmin", TypeName: "ProductImportMapping"}:         "API and domain view mapped to listingProductImportMapping before persistence",
		{Directory: listingAdminDirectory, Package: "listingadmin", TypeName: "StoreRespDTO"}:                 "management API transport DTO, never passed to GORM as a model",

		{Directory: listingAdminDirectory, Package: "listingadmin", TypeName: "DailyListingCountGetReqDTO"}:       "management API transport DTO, never passed to GORM as a model",
		{Directory: listingAdminDirectory, Package: "listingadmin", TypeName: "DailyListingCountRespDTO"}:         "management API transport DTO, never passed to GORM as a model",
		{Directory: listingAdminDirectory, Package: "listingadmin", TypeName: "DailyListingCountSetReqDTO"}:       "management API transport DTO, never passed to GORM as a model",
		{Directory: listingAdminDirectory, Package: "listingadmin", TypeName: "RollbackDailyQuotaReqDTO"}:         "management API transport DTO, never passed to GORM as a model",
		{Directory: listingAdminDirectory, Package: "listingadmin", TypeName: "TryConsumeDailyQuotaReqDTO"}:       "management API transport DTO, never passed to GORM as a model",
		{Directory: listingAdminDirectory, Package: "listingadmin", TypeName: "ProductImportMappingCreateReqDTO"}: "management API transport DTO, never passed to GORM as a model",

		{Directory: listingKitDirectory, Package: "listingkit", TypeName: "AIAsyncImageQueryContext"}:        "request context value, never passed to GORM as a model",
		{Directory: listingKitDirectory, Package: "listingkit", TypeName: "AIClientCredential"}:              "service contract mapped to the infrastructure credential store",
		{Directory: listingKitDirectory, Package: "listingkit", TypeName: "AuthenticatedIdentity"}:           "verified request context value, never passed to GORM as a model",
		{Directory: listingKitDirectory, Package: "listingkit", TypeName: "GenerateRequest"}:                 "orchestration request input, never passed to GORM as a model",
		{Directory: listingKitDirectory, Package: "listingkit", TypeName: "RequestIdentity"}:                 "request context value, never passed to GORM as a model",
		{Directory: listingKitDirectory, Package: "listingkit", TypeName: "SourceFactsGenerateRequestInput"}: "source-facts bridge input, never passed to GORM as a model",
		{Directory: memberInviteDirectory, Package: "memberinvite", TypeName: "AuditRecord"}:                 "repository command mapped to memberInvitationAuditRow before persistence",
		{Directory: memberInviteDirectory, Package: "memberinvite", TypeName: "Invitation"}:                  "identity-provider response value, never passed to GORM as a model",
		{Directory: storeDirectory, Package: "store", TypeName: "sheinPODImageLookupBackfillTaskRow"}:        "read projection selected through the explicit listingkit.Task GORM model",
		{Directory: openAIDirectory, Package: "openai", TypeName: "Identity"}:                                "request identity context, never persisted as a GORM model",
	}
}

func discoverOwnerTables(exclusions map[ownerModelKey]string, roots ...string) (map[string]OwnerTable, error) {
	models := make(map[ownerModelKey]*ownerModel)
	tableNames := make(map[ownerModelKey]string)
	fset := token.NewFileSet()
	for key, rationale := range exclusions {
		if strings.TrimSpace(rationale) == "" {
			return nil, fmt.Errorf("owner-scoped candidate exclusion %s.%s has no rationale", key.Package, key.TypeName)
		}
	}

	for _, root := range roots {
		err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
				return nil
			}
			file, err := parser.ParseFile(fset, path, nil, 0)
			if err != nil {
				return fmt.Errorf("parse %s: %w", path, err)
			}
			directory := filepath.Clean(filepath.Dir(path))
			for _, declaration := range file.Decls {
				switch typed := declaration.(type) {
				case *ast.GenDecl:
					collectOwnerStructs(models, directory, file.Name.Name, path, typed)
				case *ast.FuncDecl:
					typeName, tableName, ok := literalTableName(typed)
					if ok {
						tableNames[ownerModelKey{Directory: directory, Package: file.Name.Name, TypeName: typeName}] = tableName
					}
				}
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}

	result := make(map[string]OwnerTable)
	usedExclusions := make(map[ownerModelKey]struct{}, len(exclusions))
	var missingTableNames []string
	for key, model := range models {
		if model.TenantColumn == "" || model.UserColumn == "" {
			continue
		}
		tableName, ok := tableNames[key]
		if !ok {
			if _, excluded := exclusions[key]; excluded {
				usedExclusions[key] = struct{}{}
				continue
			}
			missingTableNames = append(missingTableNames, fmt.Sprintf("%s.%s in %s", key.Package, model.TypeName, model.Source))
			continue
		}
		if _, exists := result[tableName]; exists {
			return nil, fmt.Errorf("multiple owner-scoped models resolve to table %s", tableName)
		}
		result[tableName] = OwnerTable{Table: tableName, TenantColumn: model.TenantColumn, UserColumn: model.UserColumn, TenantDomain: model.TenantDomain}
	}
	if len(missingTableNames) > 0 {
		sort.Strings(missingTableNames)
		return nil, fmt.Errorf("owner-scoped candidates have no literal TableName method: %v; add an explicit production TableName method or a narrowly named exclusion with rationale", missingTableNames)
	}
	var staleExclusions []string
	for key := range exclusions {
		if _, used := usedExclusions[key]; !used {
			staleExclusions = append(staleExclusions, fmt.Sprintf("%s.%s", key.Package, key.TypeName))
		}
	}
	if len(staleExclusions) > 0 {
		sort.Strings(staleExclusions)
		return nil, fmt.Errorf("stale non-persistent owner-model exclusions: %v", staleExclusions)
	}
	return result, nil
}

func collectOwnerStructs(models map[ownerModelKey]*ownerModel, directory, packageName, path string, declaration *ast.GenDecl) {
	if declaration.Tok != token.TYPE {
		return
	}
	for _, spec := range declaration.Specs {
		typeSpec, ok := spec.(*ast.TypeSpec)
		if !ok {
			continue
		}
		structure, ok := typeSpec.Type.(*ast.StructType)
		if !ok {
			continue
		}
		tenantDomain := TenantDomainZITADELOrganization
		if packageName == "listingadmin" {
			tenantDomain = TenantDomainLegacyNumeric
		}
		model := &ownerModel{TypeName: typeSpec.Name.Name, TenantDomain: tenantDomain, Source: path}
		for _, field := range structure.Fields.List {
			column := gormColumn(field)
			switch column {
			case "tenant_id":
				model.TenantColumn = column
			case "user_id", "owner_user_id":
				model.UserColumn = column
			}
		}
		if model.TenantColumn != "" || model.UserColumn != "" {
			models[ownerModelKey{Directory: directory, Package: packageName, TypeName: typeSpec.Name.Name}] = model
		}
	}
}

func gormColumn(field *ast.Field) string {
	if len(field.Names) != 1 {
		return ""
	}
	if field.Tag != nil {
		tag, err := strconv.Unquote(field.Tag.Value)
		if err != nil {
			return ""
		}
		gormTag := reflect.StructTag(tag).Get("gorm")
		for _, option := range strings.Split(gormTag, ";") {
			if option == "-" || strings.HasPrefix(option, "-:") {
				return ""
			}
			if column, ok := strings.CutPrefix(option, "column:"); ok {
				return column
			}
		}
	}
	switch field.Names[0].Name {
	case "TenantID":
		return "tenant_id"
	case "UserID":
		return "user_id"
	case "OwnerUserID":
		return "owner_user_id"
	default:
		return ""
	}
}

func literalTableName(function *ast.FuncDecl) (string, string, bool) {
	if function.Name.Name != "TableName" || function.Recv == nil || len(function.Recv.List) != 1 || function.Body == nil || len(function.Body.List) != 1 {
		return "", "", false
	}
	typeName, ok := receiverTypeName(function.Recv.List[0].Type)
	if !ok {
		return "", "", false
	}
	statement, ok := function.Body.List[0].(*ast.ReturnStmt)
	if !ok || len(statement.Results) != 1 {
		return "", "", false
	}
	literal, ok := statement.Results[0].(*ast.BasicLit)
	if !ok || literal.Kind != token.STRING {
		return "", "", false
	}
	tableName, err := strconv.Unquote(literal.Value)
	if err != nil {
		return "", "", false
	}
	return typeName, tableName, true
}

func receiverTypeName(expression ast.Expr) (string, bool) {
	switch typed := expression.(type) {
	case *ast.Ident:
		return typed.Name, true
	case *ast.StarExpr:
		identifier, ok := typed.X.(*ast.Ident)
		if !ok {
			return "", false
		}
		return identifier.Name, true
	default:
		return "", false
	}
}

func formatOwnerTable(table OwnerTable) string {
	return fmt.Sprintf("%s(%s,%s)", table.Table, table.TenantColumn, table.UserColumn)
}
