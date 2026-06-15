package httpapi

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRouteContractFamiliesOwnSeparatedSurfaces(t *testing.T) {
	t.Parallel()

	dir := "."

	assertRouteNoFile(t, filepath.Join(dir, "routes.go"))

	taskFile := readRouteFileContent(t, filepath.Join(dir, "routes_task.go"))
	settingsFile := readRouteFileContent(t, filepath.Join(dir, "routes_settings.go"))
	storeSubscriptionFile := readRouteFileContent(t, filepath.Join(dir, "routes_store_subscription.go"))
	adminFile := readRouteFileContent(t, filepath.Join(dir, "routes_admin.go"))
	sheinSyncFile := readRouteFileContent(t, filepath.Join(dir, "routes_shein_sync.go"))
	handlerFile := readRouteFileContent(t, filepath.Join(dir, "routes_handler.go"))
	descriptorsFile := readRouteFileContent(t, filepath.Join(dir, "routes_descriptors.go"))

	assertRouteContainsAll(t, taskFile,
		"type TaskActionRouteHandler interface {",
		"type TaskRouteHandler interface {",
		"type StudioGenerationRouteHandler interface {",
		"type studioBatchRunRouteHandler interface {",
	)
	assertRouteNotContainsAny(t, taskFile,
		"type SettingsRouteHandler interface {",
		"type StoreRouteHandler interface {",
		"type AdminRouteHandler interface {",
	)

	assertRouteContainsAll(t, settingsFile,
		"type SettingsStoreRouteHandler interface {",
		"type SettingsRouteHandler interface {",
	)
	assertRouteNotContainsAny(t, settingsFile,
		"type TaskRouteHandler interface {",
		"type SubscriptionRouteHandler interface {",
	)

	assertRouteContainsAll(t, storeSubscriptionFile,
		"type StoreRouteHandler interface {",
		"type SubscriptionRouteHandler interface {",
		"type PlatformAdminRouteHandler interface {",
	)
	assertRouteNotContainsAny(t, storeSubscriptionFile,
		"type TaskRouteHandler interface {",
		"type AdminRouteHandler interface {",
	)

	assertRouteContainsAll(t, adminFile,
		"type AdminRouteHandler interface {",
		"ListAdminStores(c *gin.Context)",
		"DeleteAdminProductData(c *gin.Context)",
	)
	assertRouteNotContainsAny(t, adminFile,
		"type TaskRouteHandler interface {",
		"type SettingsRouteHandler interface {",
	)

	assertRouteContainsAll(t, sheinSyncFile,
		"type sheinSyncRouteHandler interface {",
		"TriggerSheinStoreSync(c *gin.Context)",
	)
	assertRouteNotContainsAny(t, sheinSyncFile,
		"type TaskRouteHandler interface {",
		"type RouteHandler interface {",
	)

	assertRouteContainsAll(t, handlerFile,
		"type RouteHandler interface {",
		"TaskRouteHandler",
		"sheinSyncRouteHandler",
	)
	assertRouteNotContainsAny(t, handlerFile,
		"type TaskRouteHandler interface {",
		"type AdminRouteHandler interface {",
	)

	assertRouteContainsAll(t, descriptorsFile,
		"func AppendRouteDescriptors",
		"func AppendStudioSessionRouteDescriptors",
		"func appendTaskRouteDescriptors",
		"func appendAdminRouteDescriptors",
	)
	assertRouteNotContainsAny(t, descriptorsFile,
		"type TaskRouteHandler interface {",
		"type SettingsRouteHandler interface {",
	)
}

func assertRouteNoFile(t *testing.T, path string) {
	t.Helper()

	if _, err := os.Stat(path); err == nil {
		t.Fatalf("expected %s to be removed", path)
	}
}

func readRouteFileContent(t *testing.T, path string) string {
	t.Helper()

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(content)
}

func assertRouteContainsAll(t *testing.T, content string, snippets ...string) {
	t.Helper()

	for _, snippet := range snippets {
		if !strings.Contains(content, snippet) {
			t.Fatalf("expected content to contain %q", snippet)
		}
	}
}

func assertRouteNotContainsAny(t *testing.T, content string, snippets ...string) {
	t.Helper()

	for _, snippet := range snippets {
		if strings.Contains(content, snippet) {
			t.Fatalf("expected content to exclude %q", snippet)
		}
	}
}
