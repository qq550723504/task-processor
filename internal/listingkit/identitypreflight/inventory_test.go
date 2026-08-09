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
		filepath.Join(repositoryRoot, "internal", "listingkit"),
		filepath.Join(repositoryRoot, "internal", "listingadmin"),
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
		if !ok || entry.TenantColumn != model.TenantColumn || entry.UserColumn != model.UserColumn {
			missing = append(missing, formatOwnerTable(model))
		}
	}
	for name, entry := range inventory {
		model, ok := discovered[name]
		if !ok || entry.TenantColumn != model.TenantColumn || entry.UserColumn != model.UserColumn {
			stale = append(stale, formatOwnerTable(entry))
		}
	}
	sort.Strings(missing)
	sort.Strings(stale)
	if len(missing) > 0 || len(stale) > 0 {
		t.Fatalf("missing from inventory: %v; stale inventory: %v", missing, stale)
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

	tables, err := discoverOwnerTables(root)
	if err != nil {
		t.Fatalf("discover owner tables: %v", err)
	}
	want := OwnerTable{Table: "sample_owner_records", TenantColumn: "tenant_id", UserColumn: "user_id"}
	if got, ok := tables[want.Table]; !ok || got != want {
		t.Fatalf("convention-based owner table = %#v, %v; want %#v", got, ok, want)
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
	TypeName        string
	TenantColumn    string
	UserColumn      string
	Table           string
	Source          string
	HasGORMMetadata bool
}

type ownerModelKey struct {
	Directory string
	Package   string
	TypeName  string
}

func discoverOwnerTables(roots ...string) (map[string]OwnerTable, error) {
	models := make(map[ownerModelKey]*ownerModel)
	tableNames := make(map[ownerModelKey]string)
	fset := token.NewFileSet()

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
	for key, model := range models {
		if model.TenantColumn == "" || model.UserColumn == "" {
			continue
		}
		tableName, ok := tableNames[key]
		if !model.HasGORMMetadata && !ok {
			continue
		}
		if !ok {
			return nil, fmt.Errorf("owner-scoped model %s in %s has no literal TableName method", model.TypeName, model.Source)
		}
		if _, exists := result[tableName]; exists {
			return nil, fmt.Errorf("multiple owner-scoped models resolve to table %s", tableName)
		}
		result[tableName] = OwnerTable{Table: tableName, TenantColumn: model.TenantColumn, UserColumn: model.UserColumn}
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
		model := &ownerModel{TypeName: typeSpec.Name.Name, Source: path}
		for _, field := range structure.Fields.List {
			column, hasGORMMetadata := gormColumn(field)
			model.HasGORMMetadata = model.HasGORMMetadata || hasGORMMetadata
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

func gormColumn(field *ast.Field) (string, bool) {
	if len(field.Names) != 1 {
		return "", false
	}
	hasGORMMetadata := false
	if field.Tag != nil {
		tag, err := strconv.Unquote(field.Tag.Value)
		if err != nil {
			return "", false
		}
		gormTag, hasGORMTag := reflect.StructTag(tag).Lookup("gorm")
		hasGORMMetadata = hasGORMMetadata || hasGORMTag
		for _, option := range strings.Split(gormTag, ";") {
			if option == "-" || strings.HasPrefix(option, "-:") {
				return "", hasGORMMetadata
			}
			if column, ok := strings.CutPrefix(option, "column:"); ok {
				return column, hasGORMMetadata
			}
		}
	}
	switch field.Names[0].Name {
	case "TenantID":
		return "tenant_id", hasGORMMetadata
	case "UserID":
		return "user_id", hasGORMMetadata
	case "OwnerUserID":
		return "owner_user_id", hasGORMMetadata
	default:
		return "", hasGORMMetadata
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
