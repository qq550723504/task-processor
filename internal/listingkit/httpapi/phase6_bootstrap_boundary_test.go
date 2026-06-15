package httpapi

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBootstrapFamiliesOwnSeparatedResponsibilities(t *testing.T) {
	t.Parallel()

	dir := "."

	entryFile := readBootstrapFileContent(t, filepath.Join(dir, "bootstrap.go"))
	contractsFile := readBootstrapFileContent(t, filepath.Join(dir, "bootstrap_contracts.go"))
	validationFile := readBootstrapFileContent(t, filepath.Join(dir, "bootstrap_validation.go"))
	closersFile := readBootstrapFileContent(t, filepath.Join(dir, "bootstrap_closers.go"))
	moduleServiceFile := readBootstrapFileContent(t, filepath.Join(dir, "bootstrap_module_service.go"))

	assertBootstrapContainsAll(t, entryFile,
		"func BuildModule(",
		"func BuildService(",
		"buildRepositories(input, closers)",
		"buildServiceRuntime(input, repositories, closers)",
	)
	assertBootstrapNotContainsAny(t, entryFile,
		"type Module struct",
		"type BuildServiceInput struct",
		"func buildModuleService(",
		"func (s *closerStack) Add(",
	)

	assertBootstrapContainsAll(t, contractsFile,
		"type Module struct {",
		"type BuildModuleInput struct {",
		"type BuildServiceInput struct {",
		"type BuildServiceHooks struct {",
	)
	assertBootstrapNotContainsAny(t, contractsFile,
		"func BuildService(",
		"func (s *closerStack) Add(",
		"func buildModuleService(",
	)

	assertBootstrapContainsAll(t, validationFile,
		"func (b CoreRepositoryBuilders) Validate() error {",
		"func (b AdminRepositoryBuilders) Validate() error {",
		"func (h BuildServiceHooks) Validate() error {",
		"func (in BuildServiceInput) Validate() error {",
	)
	assertBootstrapNotContainsAny(t, validationFile,
		"type Module struct",
		"func BuildService(",
		"func buildModuleService(",
	)

	assertBootstrapContainsAll(t, closersFile,
		"type closerStack struct {",
		"func (s *closerStack) Add(",
		"func buildNamedWithClosers",
	)
	assertBootstrapNotContainsAny(t, closersFile,
		"func BuildService(",
		"type BuildServiceInput struct",
		"func buildModuleService(",
	)

	assertBootstrapContainsAll(t, moduleServiceFile,
		"func prepareModuleServiceEnvironment(",
		"func buildModuleService(",
		"func wireTemporalWorkflowClients(",
		"func assembleServiceBundle(",
	)
	assertBootstrapNotContainsAny(t, moduleServiceFile,
		"func BuildService(",
		"type BuildServiceInput struct",
		"func (s *closerStack) Add(",
	)
}

func readBootstrapFileContent(t *testing.T, path string) string {
	t.Helper()

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(content)
}

func assertBootstrapContainsAll(t *testing.T, content string, snippets ...string) {
	t.Helper()

	for _, snippet := range snippets {
		if !strings.Contains(content, snippet) {
			t.Fatalf("expected content to contain %q", snippet)
		}
	}
}

func assertBootstrapNotContainsAny(t *testing.T, content string, snippets ...string) {
	t.Helper()

	for _, snippet := range snippets {
		if strings.Contains(content, snippet) {
			t.Fatalf("expected content to exclude %q", snippet)
		}
	}
}
