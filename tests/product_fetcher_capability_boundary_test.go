package tests

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPlatformModulesUseFetcherCapabilityMethods(t *testing.T) {
	cases := []struct {
		path     string
		required []string
	}{
		{
			path: filepath.Join("..", "internal", "platforms", "shein", "module.go"),
			required: []string{
				"rt.ProductReader()",
				"rt.ProductCache()",
			},
		},
		{
			path: filepath.Join("..", "internal", "platforms", "temu", "module.go"),
			required: []string{
				"rt.ProductReader()",
				"rt.ProductCache()",
				"rt.ProductFetcherStats()",
			},
		},
	}

	for _, tc := range cases {
		content, err := os.ReadFile(tc.path)
		if err != nil {
			t.Fatalf("read %s: %v", tc.path, err)
		}
		source := string(content)
		if strings.Contains(source, "rt.ProductFetcher()") {
			t.Fatalf("%s uses rt.ProductFetcher(); platform modules must consume narrow fetcher capabilities", tc.path)
		}
		for _, required := range tc.required {
			if !strings.Contains(source, required) {
				t.Fatalf("%s is missing %s", tc.path, required)
			}
		}
	}
}
