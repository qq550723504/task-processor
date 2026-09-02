package listingkit

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	productasset "task-processor/internal/product/asset"
	"task-processor/internal/product/catalog"
	"task-processor/internal/product/catalog/canonical"
	sheinpub "task-processor/internal/publishing/shein"
)

func TestProductionResolutionSourcesContainNoOfflineFallbackCapability(t *testing.T) {
	t.Parallel()

	_, testFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve test source path")
	}
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(testFile), "..", ".."))
	files := []string{
		filepath.Join(repoRoot, "internal", "listingkit", "service_defaults.go"),
		filepath.Join(repoRoot, "internal", "publishing", "shein", "runtime_category_resolver.go"),
		filepath.Join(repoRoot, "internal", "publishing", "shein", "runtime_attribute_resolver.go"),
		filepath.Join(repoRoot, "internal", "publishing", "shein", "runtime_sale_attribute_resolver.go"),
	}
	for _, path := range files {
		parsed, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		ast.Inspect(parsed, func(node ast.Node) bool {
			switch typed := node.(type) {
			case *ast.Field:
				for _, name := range typed.Names {
					if name.Name == "fallback" {
						t.Errorf("%s contains forbidden offline fallback field", path)
					}
				}
			case *ast.Ident:
				lower := strings.ToLower(typed.Name)
				if strings.Contains(lower, "lexeme") || strings.Contains(lower, "morpholog") || lower == "defaultcategories" || lower == "categorydictionary" {
					t.Errorf("%s contains forbidden local category-enumeration identifier %q", path, typed.Name)
				}
			case *ast.CallExpr:
				if path != files[0] {
					break
				}
				selector, ok := typed.Fun.(*ast.SelectorExpr)
				if !ok {
					break
				}
				switch selector.Sel.Name {
				case "NewCategoryResolver", "NewAttributeResolver", "NewSaleAttributeResolver":
					t.Errorf("%s implicitly constructs forbidden resolver %s", path, selector.Sel.Name)
				}
			}
			return true
		})
	}
}

type failClosedCategoryResolver struct {
	resolution *sheinpub.CategoryResolution
}

func (r failClosedCategoryResolver) Resolve(*sheinpub.BuildRequest, *canonical.Product, *sheinpub.Package) *sheinpub.CategoryResolution {
	return r.resolution
}

type failClosedAttributeResolver struct {
	resolution *sheinpub.AttributeResolution
}

func (r failClosedAttributeResolver) Resolve(*sheinpub.BuildRequest, *canonical.Product, *sheinpub.Package) *sheinpub.AttributeResolution {
	return r.resolution
}

type failClosedSaleAttributeResolver struct {
	resolution *sheinpub.SaleAttributeResolution
}

func (r failClosedSaleAttributeResolver) Resolve(*sheinpub.BuildRequest, *canonical.Product, *sheinpub.Package) *sheinpub.SaleAttributeResolution {
	return r.resolution
}

func completeFailClosedAssemblerConfig() AssemblerConfig {
	return AssemblerConfig{
		AmazonBuilder: stubAmazonDraftBuilder{},
		SheinCategoryResolver: failClosedCategoryResolver{resolution: &sheinpub.CategoryResolution{
			Status: "resolved", Source: "remote_fixture", CategoryID: 42,
		}},
		SheinAttributeResolver: failClosedAttributeResolver{resolution: &sheinpub.AttributeResolution{
			Status: "resolved", Source: "remote_fixture", CategoryID: 42,
		}},
		SheinSaleAttributeResolver: failClosedSaleAttributeResolver{resolution: &sheinpub.SaleAttributeResolution{
			Status: "resolved", Source: "remote_fixture", CategoryID: 42,
		}},
	}
}

func TestPrepareServiceConfigDoesNotConstructDefaultSheinResolvers(t *testing.T) {
	t.Parallel()

	cfg := prepareServiceConfig(newTestServiceConfig(&stubSubmitRepo{}))
	if cfg.Shein.SheinCategoryResolver != nil {
		t.Fatal("category resolver was implicitly constructed")
	}
	if cfg.Shein.SheinAttributeResolver != nil {
		t.Fatal("attribute resolver was implicitly constructed")
	}
	if cfg.Shein.SheinSaleAttributeResolver != nil {
		t.Fatal("sale-attribute resolver was implicitly constructed")
	}
}

func TestAssemblerRequiresEveryResolverOnlyForSheinTargets(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		remove     func(*AssemblerConfig)
		wantErrKey string
	}{
		{name: "category", remove: func(cfg *AssemblerConfig) { cfg.SheinCategoryResolver = nil }, wantErrKey: "category resolver"},
		{name: "attribute", remove: func(cfg *AssemblerConfig) { cfg.SheinAttributeResolver = nil }, wantErrKey: "attribute resolver"},
		{name: "sale attribute", remove: func(cfg *AssemblerConfig) { cfg.SheinSaleAttributeResolver = nil }, wantErrKey: "sale-attribute resolver"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := completeFailClosedAssemblerConfig()
			tt.remove(&cfg)
			result, err := NewAssemblerWithConfig(cfg).Assemble(
				&Task{Request: &GenerateRequest{Platforms: []string{"shein"}}},
				&catalog.ProductSnapshot{Title: "Remote product"},
				&productasset.ApprovedAssetInventory{},
			)
			if err == nil || !strings.Contains(err.Error(), tt.wantErrKey) {
				t.Fatalf("Assemble() result/error = %#v/%v, want explicit %s error", result, err, tt.wantErrKey)
			}
			if result != nil {
				t.Fatalf("Assemble() result = %#v, want no partial SHEIN result", result)
			}
		})
	}

	result, err := NewAssemblerWithConfig(AssemblerConfig{AmazonBuilder: stubAmazonDraftBuilder{}}).Assemble(
		&Task{Request: &GenerateRequest{Platforms: []string{"amazon"}}},
		&catalog.ProductSnapshot{Title: "Amazon product"},
		&productasset.ApprovedAssetInventory{},
	)
	if err != nil {
		t.Fatalf("Amazon-only Assemble() error = %v, want no SHEIN dependency error", err)
	}
	if result == nil || result.Amazon == nil {
		t.Fatalf("Amazon-only Assemble() result = %#v, want Amazon draft", result)
	}
}

func TestAssemblerRejectsMissingRuntimeResolutionWithoutConvertingItToPartial(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		remove     func(*AssemblerConfig)
		wantErrKey string
	}{
		{name: "category", remove: func(cfg *AssemblerConfig) { cfg.SheinCategoryResolver = failClosedCategoryResolver{} }, wantErrKey: "category resolution"},
		{name: "attribute", remove: func(cfg *AssemblerConfig) { cfg.SheinAttributeResolver = failClosedAttributeResolver{} }, wantErrKey: "attribute resolution"},
		{name: "sale attribute", remove: func(cfg *AssemblerConfig) { cfg.SheinSaleAttributeResolver = failClosedSaleAttributeResolver{} }, wantErrKey: "sale-attribute resolution"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := completeFailClosedAssemblerConfig()
			tt.remove(&cfg)
			result, err := NewAssemblerWithConfig(cfg).Assemble(
				&Task{Request: &GenerateRequest{Platforms: []string{"shein"}}},
				&catalog.ProductSnapshot{Title: "Remote product"},
				&productasset.ApprovedAssetInventory{},
			)
			if err == nil || !strings.Contains(err.Error(), tt.wantErrKey) {
				t.Fatalf("Assemble() result/error = %#v/%v, want explicit %s error", result, err, tt.wantErrKey)
			}
			if result != nil {
				t.Fatalf("Assemble() result = %#v, want no fallback/partial result", result)
			}
		})
	}
}

func TestAssemblerPreservesExplicitBusinessPartialResolutions(t *testing.T) {
	t.Parallel()

	cfg := completeFailClosedAssemblerConfig()
	cfg.SheinCategoryResolver = failClosedCategoryResolver{resolution: &sheinpub.CategoryResolution{
		Status: "partial", Source: "remote_api", CategoryID: 42, ReviewNotes: []string{"remote category ambiguity"},
	}}
	cfg.SheinAttributeResolver = failClosedAttributeResolver{resolution: &sheinpub.AttributeResolution{
		Status: "partial", Source: "remote_api", CategoryID: 42, ReviewNotes: []string{"remote attribute ambiguity"},
	}}
	cfg.SheinSaleAttributeResolver = failClosedSaleAttributeResolver{resolution: &sheinpub.SaleAttributeResolution{
		Status: "partial", Source: "remote_api", CategoryID: 42, ReviewNotes: []string{"remote sale-attribute ambiguity"},
	}}

	result, err := NewAssemblerWithConfig(cfg).Assemble(
		&Task{Request: &GenerateRequest{Platforms: []string{"shein"}}},
		&catalog.ProductSnapshot{Title: "Remote product"},
		&productasset.ApprovedAssetInventory{},
	)
	if err != nil {
		t.Fatalf("Assemble() error = %v, want explicit business partial to remain valid", err)
	}
	if result == nil || result.Shein == nil || result.Shein.CategoryResolution == nil || result.Shein.AttributeResolution == nil || result.Shein.SaleAttributeResolution == nil {
		t.Fatalf("Assemble() result = %#v, want all explicit partial resolutions", result)
	}
	if result.Shein.CategoryResolution.Status != "partial" || result.Shein.AttributeResolution.Status != "partial" || result.Shein.SaleAttributeResolution.Status != "partial" {
		t.Fatalf("resolution statuses = %q/%q/%q, want explicit partial values preserved", result.Shein.CategoryResolution.Status, result.Shein.AttributeResolution.Status, result.Shein.SaleAttributeResolution.Status)
	}
}
