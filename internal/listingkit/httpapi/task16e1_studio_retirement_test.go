package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	kernelmodule "task-processor/internal/kernel/module"
	"task-processor/internal/listingkit"
	listingkitapi "task-processor/internal/listingkit/api"
)

func TestListingKitProductionRoutesRetireStudioAndKeepUploads(t *testing.T) {
	t.Parallel()

	registry := kernelmodule.NewRegistry()
	requireNoError(t, NewRuntimeModule(&Module{Handler: stubRouteHandler{}}).Register(registry))

	keys := routeKeys(registry.Routes())
	for _, key := range keys {
		if strings.Contains(key, "/api/v1/listing-kits/studio/") {
			t.Fatalf("production route %q still exposes retired ListingKit Studio", key)
		}
	}
	for _, want := range []string{
		"POST /api/v1/listing-kits/uploads/images",
		"GET /api/v1/listing-kits/uploads/files/*key",
		"DELETE /api/v1/listing-kits/uploads/files/*key",
	} {
		if !containsRouteKey(keys, want) {
			t.Fatalf("production routes = %v, want retained upload route %q", keys, want)
		}
	}
}

func TestListingKitProductionContractsDoNotRequestStudioRuntimeDependencies(t *testing.T) {
	t.Parallel()

	assertNoFields(t, reflect.TypeOf(BuildServiceHooks{}),
		"StudioImageGeneratorBuilder",
		"StudioAICapabilityRouterBuilder",
		"StudioBackgroundRemoverBuilder",
	)
	assertNoFields(t, reflect.TypeOf(CoreRepositories{}),
		"StudioAsyncJob",
		"StudioBatch",
		"StudioBatchRun",
		"StudioSession",
	)
	assertNoFields(t, reflect.TypeOf(Module{}), "StudioSessionHandler")
	assertNoFields(t, reflect.TypeOf(listingkit.ServiceCoreDependencies{}),
		"StudioSessionRepository",
		"StudioBatchRepository",
		"StudioBatchRunRepository",
	)
	assertNoFields(t, reflect.TypeOf(listingkit.ServiceSheinDependencies{}),
		"StudioPromptDiversifier",
		"StudioImageGenerator",
		"StudioBackgroundRemover",
	)
}

func TestListingKitProductionBootstrapWiresUploadedImageDeletion(t *testing.T) {
	t.Parallel()

	const uploadKey = "upload-17"
	service := &productionUploadedImageDeletionService{deleted: &listingkit.DeletedUploadedImage{
		Key:  uploadKey,
		Size: 17,
	}}
	handler, err := listingkitapi.NewHandler(nil, buildHandlerOptions(serviceBundleRuntime{service: service})...)
	requireNoError(t, err)

	registry := kernelmodule.NewRegistry()
	requireNoError(t, NewRuntimeModule(&Module{Handler: handler}).Register(registry))
	router := gin.New()
	for _, route := range registry.Routes() {
		if route.Method == http.MethodDelete && route.Path == "/api/v1/listing-kits/uploads/files/*key" {
			router.Handle(route.Method, route.Path, route.Handler)
		}
	}

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodDelete, "/api/v1/listing-kits/uploads/files/"+uploadKey, nil)
	router.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("DELETE production upload route status = %d, body = %s, want 200", response.Code, response.Body.String())
	}
	if !strings.Contains(response.Body.String(), `"key":"`+uploadKey+`"`) {
		t.Fatalf("DELETE production upload route body = %s, want deleted key %q", response.Body.String(), uploadKey)
	}
}

type productionUploadedImageDeletionService struct {
	moduleService
	deleted *listingkit.DeletedUploadedImage
}

func (s *productionUploadedImageDeletionService) DeleteUploadedImage(context.Context, string) (*listingkit.DeletedUploadedImage, error) {
	return s.deleted, nil
}

func assertNoFields(t *testing.T, typ reflect.Type, names ...string) {
	t.Helper()
	for _, name := range names {
		if _, ok := typ.FieldByName(name); ok {
			t.Fatalf("%s still exposes retired production dependency %s", typ.Name(), name)
		}
	}
}

func containsRouteKey(keys []string, want string) bool {
	for _, key := range keys {
		if key == want {
			return true
		}
	}
	return false
}

func requireNoError(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
