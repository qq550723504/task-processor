package tests

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"golang.org/x/mod/modfile"
)

type nestedModuleCompileResult struct {
	Directory    string
	ZeroPackages bool
}

func TestRepositoryNestedGoModulesCompile(t *testing.T) {
	repositoryRoot, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	moduleFilesBefore := readRepositoryModuleFiles(t, repositoryRoot)

	results, err := compileRepositoryNestedModules(repositoryRoot)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) == 0 {
		t.Fatal("repository module gate discovered no non-root modules")
	}
	for _, result := range results {
		classification := "compiled"
		if result.ZeroPackages {
			classification = "zero packages (N/A)"
		}
		t.Logf("nested module %s: %s", relativeModulePath(repositoryRoot, result.Directory), classification)
	}

	moduleFilesAfter := readRepositoryModuleFiles(t, repositoryRoot)
	if !reflect.DeepEqual(moduleFilesAfter, moduleFilesBefore) {
		t.Fatal("nested module compile gate modified a repository go.mod or go.sum")
	}
}

func TestRepositoryNestedModuleCompileGateFindsNewModuleAndRejectsFunctionAliasSignatureError(t *testing.T) {
	repositoryRoot := t.TempDir()
	writeModuleFixtureFile(t, repositoryRoot, "go.mod", "module example/root\n\ngo 1.26.0\n")
	writeModuleFixtureFile(t, repositoryRoot, "provider/provider.go", `package provider

func NewAdapter(config string, logger int) {}
`)
	writeModuleFixtureFile(t, repositoryRoot, "nested/go.mod", `module example/root/nested

go 1.26.0

require example/root v0.0.0

replace example/root => ..
`)
	writeModuleFixtureFile(t, repositoryRoot, "nested/main.go", `package nested

import . "example/root/provider"

var adapter = NewAdapter

func build() {
	adapter("config")
}
`)

	results, err := compileRepositoryNestedModules(repositoryRoot)
	if err == nil {
		t.Fatal("compile gate accepted a newly discovered nested module with a function-alias signature error")
	}
	if len(results) != 1 || filepath.Base(results[0].Directory) != "nested" {
		t.Fatalf("compile results = %+v, want exactly the dynamically discovered nested module", results)
	}
	if !strings.Contains(err.Error(), "not enough arguments") {
		t.Fatalf("compile error = %v, want the real Go type-checker signature failure", err)
	}
}

func writeModuleFixtureFile(t *testing.T, root, relativePath, contents string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(relativePath))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readRepositoryModuleFiles(t *testing.T, repositoryRoot string) map[string]string {
	t.Helper()
	modules, err := discoverRepositoryGoModules(repositoryRoot)
	if err != nil {
		t.Fatal(err)
	}
	contents := make(map[string]string)
	for _, goModPath := range modules {
		for _, path := range []string{goModPath, filepath.Join(filepath.Dir(goModPath), "go.sum")} {
			data, err := os.ReadFile(path)
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			if err != nil {
				t.Fatal(err)
			}
			relative, err := filepath.Rel(repositoryRoot, path)
			if err != nil {
				t.Fatal(err)
			}
			contents[filepath.ToSlash(relative)] = string(data)
		}
	}
	return contents
}

func compileRepositoryNestedModules(repositoryRoot string) ([]nestedModuleCompileResult, error) {
	modules, err := discoverRepositoryGoModules(repositoryRoot)
	if err != nil {
		return nil, err
	}
	rootMod := filepath.Clean(filepath.Join(repositoryRoot, "go.mod"))
	results := make([]nestedModuleCompileResult, 0, len(modules)-1)
	var compileErrors []error
	for _, goModPath := range modules {
		if filepath.Clean(goModPath) == rootMod {
			continue
		}
		result, err := compileNestedGoModule(repositoryRoot, goModPath)
		results = append(results, result)
		if err != nil {
			compileErrors = append(compileErrors, err)
		}
	}
	return results, errors.Join(compileErrors...)
}

func discoverRepositoryGoModules(repositoryRoot string) ([]string, error) {
	var modules []string
	err := filepath.WalkDir(repositoryRoot, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			switch entry.Name() {
			case ".git", ".worktrees", "node_modules", "vendor":
				if path != repositoryRoot {
					return filepath.SkipDir
				}
			}
			return nil
		}
		if entry.Name() == "go.mod" {
			modules = append(modules, path)
		}
		return nil
	})
	sort.Strings(modules)
	return modules, err
}

func compileNestedGoModule(repositoryRoot, goModPath string) (nestedModuleCompileResult, error) {
	moduleDir := filepath.Dir(goModPath)
	result := nestedModuleCompileResult{Directory: moduleDir}
	temporaryDir, err := os.MkdirTemp("", "task-processor-module-gate-")
	if err != nil {
		return result, err
	}
	defer os.RemoveAll(temporaryDir)

	temporaryModPath := filepath.Join(temporaryDir, "module.mod")
	if err := writeIsolatedModfile(goModPath, temporaryModPath); err != nil {
		return result, err
	}
	if err := copyFileIfPresent(filepath.Join(moduleDir, "go.sum"), filepath.Join(temporaryDir, "module.sum")); err != nil {
		return result, err
	}

	packages, output, err := runGoCommand(moduleDir, "list", "-mod=mod", "-modfile="+temporaryModPath, "./...")
	if err != nil {
		return result, fmt.Errorf("list nested module %s: %w\n%s", relativeModulePath(repositoryRoot, moduleDir), err, output)
	}
	if strings.TrimSpace(packages) == "" {
		result.ZeroPackages = true
		return result, nil
	}
	_, output, err = runGoCommand(moduleDir, "test", "-mod=mod", "-modfile="+temporaryModPath, "-run", "^$", "./...")
	if err != nil {
		return result, fmt.Errorf("compile nested module %s: %w\n%s", relativeModulePath(repositoryRoot, moduleDir), err, output)
	}
	return result, nil
}

func writeIsolatedModfile(sourcePath, destinationPath string) error {
	contents, err := os.ReadFile(sourcePath)
	if err != nil {
		return err
	}
	parsed, err := modfile.Parse(sourcePath, contents, nil)
	if err != nil {
		return err
	}
	moduleDir := filepath.Dir(sourcePath)
	for _, replacement := range append([]*modfile.Replace(nil), parsed.Replace...) {
		if replacement.New.Version != "" || filepath.IsAbs(replacement.New.Path) {
			continue
		}
		absoluteTarget, err := filepath.Abs(filepath.Join(moduleDir, filepath.FromSlash(replacement.New.Path)))
		if err != nil {
			return err
		}
		if err := parsed.AddReplace(replacement.Old.Path, replacement.Old.Version, filepath.ToSlash(absoluteTarget), ""); err != nil {
			return err
		}
	}
	formatted, err := parsed.Format()
	if err != nil {
		return err
	}
	return os.WriteFile(destinationPath, formatted, 0o644)
}

func copyFileIfPresent(sourcePath, destinationPath string) error {
	contents, err := os.ReadFile(sourcePath)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	return os.WriteFile(destinationPath, contents, 0o644)
}

func runGoCommand(directory string, arguments ...string) (string, string, error) {
	command := exec.Command("go", arguments...)
	command.Dir = directory
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	err := command.Run()
	return stdout.String(), stderr.String(), err
}

func relativeModulePath(repositoryRoot, moduleDir string) string {
	relative, err := filepath.Rel(repositoryRoot, moduleDir)
	if err != nil {
		return moduleDir
	}
	return filepath.ToSlash(relative)
}
