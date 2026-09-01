package httpapi

import (
	"os"
	"path/filepath"
	"testing"
)

type productImageRuntimePaths struct {
	workDir            string
	publisherOutputDir string
}

func configureProductImageRuntimePaths(t *testing.T) productImageRuntimePaths {
	t.Helper()
	runtimeRoot, err := filepath.Abs(t.TempDir())
	if err != nil {
		t.Fatalf("filepath.Abs(t.TempDir()) error = %v", err)
	}
	paths := productImageRuntimePaths{
		workDir:            filepath.Join(runtimeRoot, "productimage"),
		publisherOutputDir: filepath.Join(runtimeRoot, "published"),
	}
	t.Setenv("TASK_PROCESSOR_PRODUCTIMAGE_WORKDIR", paths.workDir)
	t.Setenv("TASK_PROCESSOR_PRODUCTIMAGE_PUBLISHER_OUTPUTDIR", paths.publisherOutputDir)
	assertNoPackageLocalRuntimeArtifacts(t)
	return paths
}

func assertNoPackageLocalRuntimeArtifacts(t *testing.T) {
	t.Helper()
	t.Cleanup(func() {
		if _, err := os.Stat(".local"); !os.IsNotExist(err) {
			t.Errorf("package-local runtime artifact .local exists or cannot be checked: %v", err)
		}
	})
}

func assertRuntimeDirectory(t *testing.T, path string) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("runtime directory %q stat error = %v", path, err)
	}
	if !info.IsDir() {
		t.Fatalf("runtime path %q is not a directory", path)
	}
}
