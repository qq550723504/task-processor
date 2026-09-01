package tests

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

type maintainedLoggingFileConfig struct {
	Logging struct {
		File         string `yaml:"file"`
		SplitByLevel []struct {
			File string `yaml:"file"`
		} `yaml:"split_by_level"`
	} `yaml:"logging"`
}

func TestMaintainedLoggingFilesStayUnderLocalRuntimeRoot(t *testing.T) {
	for _, name := range []string{"config-dev.yaml", "config-test.yaml", "config-prod.yaml"} {
		t.Run(name, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join("..", "config", name))
			if err != nil {
				t.Fatal(err)
			}
			var cfg maintainedLoggingFileConfig
			if err := yaml.Unmarshal(data, &cfg); err != nil {
				t.Fatal(err)
			}
			paths := []string{cfg.Logging.File}
			for _, split := range cfg.Logging.SplitByLevel {
				paths = append(paths, split.File)
			}
			for _, path := range paths {
				if path == "" {
					continue
				}
				clean := filepath.ToSlash(filepath.Clean(path))
				if !strings.HasPrefix(clean, ".local/logs/") {
					t.Errorf("logging path %q must stay under .local/logs", path)
				}
			}
		})
	}
}
