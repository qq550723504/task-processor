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
	descriptorEntryFile := readRouteFileContent(t, filepath.Join(dir, "routes_descriptor_entrypoints.go"))
	descriptorSettingsFile := readRouteFileContent(t, filepath.Join(dir, "routes_descriptor_settings.go"))
	descriptorStoreSubscriptionFile := readRouteFileContent(t, filepath.Join(dir, "routes_descriptor_store_subscription.go"))
	descriptorAdminFile := readRouteFileContent(t, filepath.Join(dir, "routes_descriptor_admin.go"))
	descriptorAdminStoreFile := readRouteFileContent(t, filepath.Join(dir, "routes_descriptor_admin_store.go"))
	descriptorAdminRulesFile := readRouteFileContent(t, filepath.Join(dir, "routes_descriptor_admin_rules.go"))
	descriptorAdminTopicsFile := readRouteFileContent(t, filepath.Join(dir, "routes_descriptor_admin_topics.go"))
	descriptorAdminCatalogFile := readRouteFileContent(t, filepath.Join(dir, "routes_descriptor_admin_catalog.go"))
	descriptorTaskFile := readRouteFileContent(t, filepath.Join(dir, "routes_descriptor_task.go"))
	descriptorSheinSyncFile := readRouteFileContent(t, filepath.Join(dir, "routes_descriptor_shein_sync.go"))

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

	assertRouteContainsAll(t, descriptorEntryFile,
		"func AppendRouteDescriptors",
		"func AppendStudioSessionRouteDescriptors",
		"appendSettingsRouteDescriptors",
		"appendSheinSyncRouteDescriptors",
	)
	assertRouteNotContainsAny(t, descriptorEntryFile,
		"func appendTaskRouteDescriptors",
		"func appendAdminRouteDescriptors",
		"type TaskRouteHandler interface {",
		"type SettingsRouteHandler interface {",
	)

	assertRouteContainsAll(t, descriptorSettingsFile,
		"func appendSettingsRouteDescriptors",
		"ListSettingsNamespaces",
		"UpdateAIClientSettings",
	)
	assertRouteNotContainsAny(t, descriptorSettingsFile,
		"func appendTaskRouteDescriptors",
		"func appendAdminRouteDescriptors",
	)

	assertRouteContainsAll(t, descriptorStoreSubscriptionFile,
		"func appendStoreRouteDescriptors",
		"func appendSubscriptionRouteDescriptors",
		"func appendPlatformAdminRouteDescriptors",
	)
	assertRouteNotContainsAny(t, descriptorStoreSubscriptionFile,
		"func appendAdminRouteDescriptors",
		"func appendTaskRouteDescriptors",
	)

	assertRouteContainsAll(t, descriptorAdminFile,
		"func appendAdminRouteDescriptors",
		"appendAdminStoreRouteDescriptors",
		"appendAdminCatalogDataRouteDescriptors",
	)
	assertRouteNotContainsAny(t, descriptorAdminFile,
		"ListAdminStores",
		"DeleteAdminProductData",
		"func appendTaskRouteDescriptors",
		"func appendSheinSyncRouteDescriptors",
	)

	assertRouteContainsAll(t, descriptorAdminStoreFile,
		"func appendAdminStoreRouteDescriptors",
		"ListAdminStores",
		"DeleteAdminImportTask",
	)
	assertRouteNotContainsAny(t, descriptorAdminStoreFile,
		"func appendAdminRuleRouteDescriptors",
		"DeleteAdminProductData",
	)

	assertRouteContainsAll(t, descriptorAdminRulesFile,
		"func appendAdminRuleRouteDescriptors",
		"ListAdminFilterRules",
		"DeleteAdminSensitiveWord",
	)
	assertRouteNotContainsAny(t, descriptorAdminRulesFile,
		"func appendAdminStoreRouteDescriptors",
		"DeleteAdminProductImportMapping",
	)

	assertRouteContainsAll(t, descriptorAdminTopicsFile,
		"func appendAdminTopicRouteDescriptors",
		"ListAdminGenerationTopicCatalog",
		"DeleteAdminGenerationTopicPolicy",
	)
	assertRouteNotContainsAny(t, descriptorAdminTopicsFile,
		"func appendAdminRuleRouteDescriptors",
		"DeleteAdminProductData",
	)

	assertRouteContainsAll(t, descriptorAdminCatalogFile,
		"func appendAdminCatalogDataRouteDescriptors",
		"ListAdminProductImportMappings",
		"DeleteAdminProductData",
	)
	assertRouteNotContainsAny(t, descriptorAdminCatalogFile,
		"func appendAdminStoreRouteDescriptors",
		"DeleteAdminSensitiveWord",
	)

	assertRouteContainsAll(t, descriptorTaskFile,
		"func appendStudioGenerationRouteDescriptors",
		"func appendTaskRouteDescriptors",
		"DispatchTaskGenerationNavigation",
	)
	assertRouteNotContainsAny(t, descriptorTaskFile,
		"func appendAdminRouteDescriptors",
		"func appendSheinSyncRouteDescriptors",
	)

	assertRouteContainsAll(t, descriptorSheinSyncFile,
		"func appendSheinSyncRouteDescriptors",
		"TriggerSheinStoreSync",
		"ListSheinActivityEnrollmentRuns",
	)
	assertRouteNotContainsAny(t, descriptorSheinSyncFile,
		"func appendTaskRouteDescriptors",
		"func appendAdminRouteDescriptors",
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
