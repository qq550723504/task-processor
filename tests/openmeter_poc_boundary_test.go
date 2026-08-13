package tests

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

const openMeterSDKImportPrefix = "github.com/openmeterio/openmeter/api/v3/client"

func TestOpenMeterImportsStayInsideIsolatedAdapter(t *testing.T) {
	for _, path := range trackedOpenMeterGoFiles(t) {
		file, err := parser.ParseFile(token.NewFileSet(), filepath.Join("..", filepath.FromSlash(path)), nil, parser.ImportsOnly)
		if err != nil {
			t.Fatalf("parse imports in %s: %v", path, err)
		}
		for _, imported := range file.Imports {
			if imported.Path.Value != `"`+openMeterSDKImportPrefix+`"` {
				continue
			}
			if !strings.HasPrefix(path, "internal/integration/openmeter/") {
				t.Errorf("%s imports %s; keep the OpenMeter SDK inside internal/integration/openmeter", path, openMeterSDKImportPrefix)
			}
		}
	}
}

func TestOpenMeterPoCDoesNotEnterRuntimeConfigurationOrDeployments(t *testing.T) {
	servicePattern := regexp.MustCompile(`(?mi)^\s*openmeter\s*:`)
	imagePattern := regexp.MustCompile(`(?mi)^\s*image\s*:\s*.*\bopenmeter\b`)

	for _, path := range trackedOpenMeterRuntimeFiles(t) {
		content, err := os.ReadFile(filepath.Join("..", filepath.FromSlash(path)))
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		source := string(content)
		if strings.Contains(source, "OPENMETER_") {
			t.Errorf("%s adds an OPENMETER_ runtime configuration", path)
		}
		if strings.Contains(source, "github.com/openmeterio/openmeter") {
			t.Errorf("%s imports an OpenMeter package in a runtime/configuration path", path)
		}
		if servicePattern.MatchString(source) || imagePattern.MatchString(source) {
			t.Errorf("%s adds an OpenMeter service or image to a runtime/configuration path", path)
		}
	}
}

func trackedOpenMeterGoFiles(t *testing.T) []string {
	t.Helper()
	var goFiles []string
	for _, path := range trackedFiles(t, ".") {
		if strings.HasSuffix(path, ".go") {
			goFiles = append(goFiles, path)
		}
	}
	return goFiles
}

func trackedOpenMeterRuntimeFiles(t *testing.T) []string {
	t.Helper()
	var runtimeFiles []string
	for _, root := range []string{"cmd", "config", "deployments"} {
		runtimeFiles = append(runtimeFiles, trackedFiles(t, root)...)
	}
	return runtimeFiles
}
