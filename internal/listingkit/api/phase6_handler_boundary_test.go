package api

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHandlerFilesOwnSeparatedFamilies(t *testing.T) {
	t.Parallel()

	dir := "."

	rootFile := readHandlerFileContent(t, filepath.Join(dir, "handler.go"))
	optionsFile := readHandlerFileContent(t, filepath.Join(dir, "handler_options.go"))
	tasksFile := readHandlerFileContent(t, filepath.Join(dir, "handler_tasks.go"))
	subscriptionFile := readHandlerFileContent(t, filepath.Join(dir, "subscription_handler.go"))
	subscriptionPlatformFile := readHandlerFileContent(t, filepath.Join(dir, "subscription_handler_platform.go"))
	subscriptionGuardFile := readHandlerFileContent(t, filepath.Join(dir, "subscription_guard.go"))

	assertHandlerContainsAll(t, rootFile,
		"type handler struct {",
		"type HandlerService interface {",
		"func WithDependencies(deps HandlerDependencies) HandlerOption {",
		"func WithSheinSyncServices(",
	)
	assertHandlerNotContainsAny(t, rootFile,
		"func NewHandler(",
		"func (h *handler) GenerateListingKit(",
		"func (h *handler) requireSubscription(",
	)

	assertHandlerContainsAll(t, optionsFile,
		"func newHandlerWithDefaults(",
		"func (h *handler) attachCoreServices(",
		"func (h *handler) finalize(",
		"func NewHandler(",
	)
	assertHandlerNotContainsAny(t, optionsFile,
		"func (h *handler) GenerateListingKit(",
		"func (h *handler) requireSubscription(",
	)

	assertHandlerContainsAll(t, tasksFile,
		"func (h *handler) GenerateListingKit(",
		"func (h *handler) ListTasks(",
		"func (h *handler) GetTaskResult(",
	)
	assertHandlerNotContainsAny(t, tasksFile,
		"func NewHandler(",
		"func (h *handler) requireSubscription(",
	)

	assertHandlerContainsAll(t, subscriptionFile,
		"func (h *handler) GetCurrentSubscription(",
		"func (h *handler) ListSubscriptionModules(",
		"func (h *handler) writeSubscriptionPlanError(",
	)
	assertHandlerNotContainsAny(t, subscriptionFile,
		"func (h *handler) UpsertPlatformSubscriptionPlan(",
		"func (h *handler) requireSubscription(",
	)

	assertHandlerContainsAll(t, subscriptionPlatformFile,
		"func (h *handler) UpsertSubscriptionEntitlement(",
		"func (h *handler) UpsertPlatformSubscriptionPlan(",
		"func (h *handler) GetPlatformTenantSubscription(",
		"func (h *handler) ListPlatformTenantSubscriptionAuditLogs(",
	)
	assertHandlerNotContainsAny(t, subscriptionPlatformFile,
		"func (h *handler) GetCurrentSubscription(",
		"func (h *handler) requireSubscription(",
	)

	assertHandlerContainsAll(t, subscriptionGuardFile,
		"func (h *handler) requireSubscription(",
		"func (h *handler) requireSubscriptionUsage(",
		"func (h *handler) requirePlatformSubscriptionAccess(",
		"func splitCSVHeaders(",
	)
	assertHandlerNotContainsAny(t, subscriptionGuardFile,
		"func (h *handler) GetCurrentSubscription(",
		"func (h *handler) UpsertPlatformSubscriptionPlan(",
	)
}

func readHandlerFileContent(t *testing.T, path string) string {
	t.Helper()

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(content)
}

func assertHandlerContainsAll(t *testing.T, content string, snippets ...string) {
	t.Helper()

	for _, snippet := range snippets {
		if !strings.Contains(content, snippet) {
			t.Fatalf("expected content to contain %q", snippet)
		}
	}
}

func assertHandlerNotContainsAny(t *testing.T, content string, snippets ...string) {
	t.Helper()

	for _, snippet := range snippets {
		if strings.Contains(content, snippet) {
			t.Fatalf("expected content to exclude %q", snippet)
		}
	}
}
