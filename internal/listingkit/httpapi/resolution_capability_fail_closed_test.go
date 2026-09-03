package httpapi

import (
	"reflect"
	"strings"
	"testing"

	"task-processor/internal/core/config"
	"task-processor/internal/product/catalog/canonical"
	sheinpub "task-processor/internal/publishing/shein"
	"task-processor/internal/sheinlogin"
)

type cachedOnlyAttributeResolver struct{}

func (cachedOnlyAttributeResolver) Resolve(*sheinpub.BuildRequest, *canonical.Product, *sheinpub.Package) *sheinpub.AttributeResolution {
	return &sheinpub.AttributeResolution{Status: "resolved", Source: "stale-cache"}
}

func completeCapabilityHooks() BuildServiceHooks {
	return BuildRuntimeSupport(RuntimeSupportInput{SheinCookieStore: &sheinlogin.RedisStore{}}).Hooks
}

func completeSubmitCapabilityHooks() submitModuleHooks {
	hooks := completeCapabilityHooks()
	return newSubmitModuleHooks(hooks)
}

func clearFunctionField(target any, name string) {
	field := reflect.ValueOf(target).Elem().FieldByName(name)
	field.Set(reflect.Zero(field.Type()))
}

func replaceFunctionWithNilResult(target any, name string) {
	field := reflect.ValueOf(target).Elem().FieldByName(name)
	field.Set(reflect.MakeFunc(field.Type(), func([]reflect.Value) []reflect.Value {
		return []reflect.Value{reflect.Zero(field.Type().Out(0))}
	}))
}

func TestBuildServiceHooksRequireEverySheinCapabilityBuilder(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		field      string
		wantErrKey string
	}{
		{name: "category resolver", field: "SheinCategoryResolverBuilder", wantErrKey: "category resolver"},
		{name: "attribute resolver", field: "SheinAttributeResolverBuilder", wantErrKey: "attribute resolver"},
		{name: "sale-attribute resolver", field: "SheinSaleAttributeResolverBuilder", wantErrKey: "sale attribute resolver"},
		{name: "product API", field: "SheinProductAPIBuilderFactory", wantErrKey: "product api builder"},
		{name: "image API", field: "SheinImageAPIBuilderFactory", wantErrKey: "image api builder"},
		{name: "translate API", field: "SheinTranslateAPIBuilderFactory", wantErrKey: "translate api builder"},
		{name: "API client factory", field: "SheinAPIClientFactoryBuilder", wantErrKey: "api client factory"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hooks := completeCapabilityHooks()
			clearFunctionField(&hooks, tt.field)
			err := hooks.Validate()
			if err == nil || !strings.Contains(err.Error(), tt.wantErrKey) {
				t.Fatalf("Validate() error = %v, want explicit %s error", err, tt.wantErrKey)
			}
		})
	}
}

func TestBuildSubmitModuleRejectsNilSheinCapabilityResults(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		field      string
		wantErrKey string
	}{
		{name: "category resolver", field: "SheinCategoryResolverBuilder", wantErrKey: "category resolver"},
		{name: "attribute resolver", field: "SheinAttributeResolverBuilder", wantErrKey: "attribute resolver"},
		{name: "sale-attribute resolver", field: "SheinSaleAttributeResolverBuilder", wantErrKey: "sale-attribute resolver"},
		{name: "product API", field: "SheinProductAPIBuilderFactory", wantErrKey: "product API builder"},
		{name: "image API", field: "SheinImageAPIBuilderFactory", wantErrKey: "image API builder"},
		{name: "translate API", field: "SheinTranslateAPIBuilderFactory", wantErrKey: "translate API builder"},
		{name: "API client factory", field: "SheinAPIClientFactoryBuilder", wantErrKey: "API client factory"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hooks := completeSubmitCapabilityHooks()
			replaceFunctionWithNilResult(&hooks, tt.field)
			cfg := &config.Config{}
			cfg.ListingKit.ImageUpload.Provider = "local"
			cfg.ListingKit.ImageUpload.Local.RootDir = t.TempDir()
			module, err := buildSubmitModule(submitModuleInput{Config: cfg, Hooks: hooks})
			if err == nil || !strings.Contains(err.Error(), tt.wantErrKey) {
				t.Fatalf("buildSubmitModule() module/error = %#v/%v, want explicit %s error", module, err, tt.wantErrKey)
			}
		})
	}
}

func TestSubmitSheinDependenciesRejectAttributeResolverWithoutFreshCapability(t *testing.T) {
	t.Parallel()

	err := (submitSheinDependencies{
		categoryResolver:  buildListingKitSheinCategoryResolver(nil, nil, nil, nil),
		attributeResolver: cachedOnlyAttributeResolver{},
	}).validate()
	if err == nil || !strings.Contains(err.Error(), "fresh attribute resolution capability") {
		t.Fatalf("validate() error = %v, want explicit fresh capability error", err)
	}
}

func TestRuntimeListingKitAttributeResolverRetiresDerivedCacheBoundary(t *testing.T) {
	t.Parallel()

	resolver := buildListingKitSheinAttributeResolver(nil, nil, nil, nil)
	if _, ok := resolver.(sheinpub.FreshAttributeResolver); !ok {
		t.Fatal("runtime ListingKit attribute resolver must declare fresh resolution capability")
	}
	if _, ok := resolver.(sheinpub.AttributeResolutionCache); ok {
		t.Fatal("runtime ListingKit attribute resolver must not expose the retired derived cache boundary")
	}
}
