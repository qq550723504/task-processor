package listingkit

import (
	"os"
	"strings"
	"testing"
)

func TestSubmitStoreContextFileKeepsRemoteClientBootstrapOutOfSettingsHydration(t *testing.T) {
	t.Parallel()

	facadeSrc, err := os.ReadFile("service_submit_store_context_facade.go")
	if err != nil {
		t.Fatalf("ReadFile(service_submit_store_context_facade.go) error = %v", err)
	}
	facadeContent := string(facadeSrc)

	if !strings.Contains(facadeContent, "buildSubmitRuntimeContextResolver(s).resolveSubmitSettings(ctx, task)") {
		t.Fatal("service_submit_store_context_facade.go should delegate submit settings resolution through the resolver seam")
	}

	src, err := os.ReadFile("service_submit_store_context.go")
	if err != nil {
		t.Fatalf("ReadFile(service_submit_store_context.go) error = %v", err)
	}
	content := string(src)

	for _, needle := range []string{
		"NewBaseAPIClient(",
		"ForceRefreshCookies(",
		"GetWarehouses(",
	} {
		if strings.Contains(content, needle) {
			t.Fatalf("service_submit_store_context.go should not contain %q", needle)
		}
	}
	if strings.Contains(content, "buildSubmitRuntimeContextResolver(s).resolveSubmitSettings(ctx, task)") {
		t.Fatal("service_submit_store_context.go should not keep submit settings delegation after facade split")
	}
}

func TestSheinStoreClientFileKeepsSettingsHydrationOutOfRemoteLookup(t *testing.T) {
	t.Parallel()

	storeInfoSource := readExactMethodSource(t, "service_submit_context_facade.go", "func (s *service) resolveSheinStoreInfo(")
	apiClientSource := readExactMethodSource(t, "service_submit_context_facade.go", "func (s *service) newSheinAPIClient(")

	for _, needle := range []string{
		"applySubmitSettingsProfile(",
		"applySubmitSettingsTaskRequest(",
		"applySubmitWarehouseOverride(",
		"currentSheinSubmitSettings(",
	} {
		if strings.Contains(storeInfoSource, needle) {
			t.Fatalf("service_submit_context_facade.go resolveSheinStoreInfo should not contain %q", needle)
		}
		if strings.Contains(apiClientSource, needle) {
			t.Fatalf("service_submit_context_facade.go newSheinAPIClient should not contain %q", needle)
		}
	}
	if !strings.Contains(storeInfoSource, "buildSubmitRuntimeContextResolver(s).resolveStoreInfo(ctx, task)") {
		t.Fatal("service_submit_context_facade.go resolveSheinStoreInfo should delegate remote store lookup through the resolver seam")
	}
	if !strings.Contains(apiClientSource, "buildSubmitRuntimeContextResolver(s).newAPIClient(ctx, task)") {
		t.Fatal("service_submit_context_facade.go newSheinAPIClient should delegate client bootstrap through the resolver seam")
	}
}

func TestSubmitContextResolverFileOwnsCrossCuttingSubmitRuntimeResolution(t *testing.T) {
	t.Parallel()

	src, err := os.ReadFile("service_submit_context_resolver.go")
	if err != nil {
		t.Fatalf("ReadFile(service_submit_context_resolver.go) error = %v", err)
	}
	content := string(src)

	for _, needle := range []string{
		"func buildSubmitRuntimeContextResolver(s *service) *submitRuntimeContextResolver {",
		"func (r *submitRuntimeContextResolver) resolveSubmitSettings(ctx context.Context, task *Task) SheinSettings {",
		"func (r *submitRuntimeContextResolver) resolveStoreInfo(ctx context.Context, task *Task) (*SheinStoreInfo, error) {",
		"func (r *submitRuntimeContextResolver) newAPIClient(ctx context.Context, task *Task) (*SheinRuntimeAPIClient, int64, error) {",
	} {
		if !strings.Contains(content, needle) {
			t.Fatalf("service_submit_context_resolver.go should contain %q", needle)
		}
	}
}
