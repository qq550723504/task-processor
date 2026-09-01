package config

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestFileSourceReadsAndNamesConfiguredFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("enabled: true\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	source := NewFileSource(path)
	data, err := source.Read()
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "enabled: true\n" {
		t.Fatalf("data = %q", data)
	}
	if source.Name() != "file:"+path {
		t.Fatalf("name = %q", source.Name())
	}
}

func TestFileSourceWithoutPathUsesDefaultConfiguration(t *testing.T) {
	source := NewFileSource("")
	data, err := source.Read()
	if err != nil {
		t.Fatal(err)
	}
	if len(data) != 0 {
		t.Fatalf("data = %q", data)
	}
	if source.Name() != "default" {
		t.Fatalf("name = %q", source.Name())
	}
	if err := source.Watch(context.Background(), func([]byte) {
		t.Fatal("empty file source invoked callback")
	}); err != nil {
		t.Fatal(err)
	}
}

func TestFileSourceWatchReportsWrittenContents(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("enabled: false\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	updates := make(chan []byte, 1)
	if err := NewFileSource(path).Watch(ctx, func(data []byte) {
		updates <- data
	}); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(path, []byte("enabled: true\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	select {
	case data := <-updates:
		if string(data) != "enabled: true\n" {
			t.Fatalf("data = %q", data)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for configuration update")
	}
}

func TestMemorySourceReturnsAndNamesInputWithoutFilesystemAccess(t *testing.T) {
	source := NewMemorySource("unit", []byte("answer: 42"))
	data, err := source.Read()
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "answer: 42" {
		t.Fatalf("data = %q", data)
	}
	if source.Name() != "memory:unit" {
		t.Fatalf("name = %q", source.Name())
	}
	if err := source.Watch(context.Background(), func([]byte) {
		t.Fatal("memory source invoked callback")
	}); err != nil {
		t.Fatal(err)
	}
}

func TestLoadJSONAndYAMLDecodeFiles(t *testing.T) {
	tests := []struct {
		name    string
		content string
		load    func(string, any) error
	}{
		{name: "JSON", content: "{\"answer\":42}", load: LoadJSON},
		{name: "YAML", content: "answer: 42\n", load: LoadYAML},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config")
			if err := os.WriteFile(path, []byte(tt.content), 0o600); err != nil {
				t.Fatal(err)
			}
			var got struct {
				Answer int `json:"answer" yaml:"answer"`
			}
			if err := tt.load(path, &got); err != nil {
				t.Fatal(err)
			}
			if got.Answer != 42 {
				t.Fatalf("answer = %d", got.Answer)
			}
		})
	}
}

func TestSaveJSONAndYAMLCreateParentDirectories(t *testing.T) {
	tests := []struct {
		name string
		save func(string, any) error
		load func(string, any) error
	}{
		{name: "JSON", save: SaveJSON, load: LoadJSON},
		{name: "YAML", save: SaveYAML, load: LoadYAML},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "nested", "config")
			if err := tt.save(path, struct {
				Enabled bool `json:"enabled" yaml:"enabled"`
			}{Enabled: true}); err != nil {
				t.Fatal(err)
			}
			var got struct {
				Enabled bool `json:"enabled" yaml:"enabled"`
			}
			if err := tt.load(path, &got); err != nil {
				t.Fatal(err)
			}
			if !got.Enabled {
				t.Fatal("enabled = false")
			}
		})
	}
}

func TestResolvePathPreservesAbsolutePathAndJoinsRelativePath(t *testing.T) {
	absolute := filepath.Join(t.TempDir(), "config.yaml")
	if got := ResolvePath("ignored", absolute); got != absolute {
		t.Fatalf("absolute path = %q", got)
	}

	base := filepath.Join("opt", "service")
	if got := ResolvePath(base, filepath.Join("config", "app.yaml")); got != filepath.Join("opt", "service", "config", "app.yaml") {
		t.Fatalf("relative path = %q", got)
	}
}

func TestExecutableBasePathReturnsExecutableDirectory(t *testing.T) {
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Dir(executable)

	got, err := ExecutableBasePath()
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("base path = %q, want %q", got, want)
	}
}

type rejectingValidator struct {
	err error
}

func (v rejectingValidator) Validate(any) error {
	return v.err
}

func TestBaseConfigLoaderExposesPathAndValidatorResult(t *testing.T) {
	wantErr := errors.New("invalid configuration")
	loader := NewBaseConfigLoader("config/app.yaml", rejectingValidator{err: wantErr})
	if got := loader.GetConfigPath(); got != "config/app.yaml" {
		t.Fatalf("path = %q", got)
	}
	if err := loader.ValidateConfig(struct{}{}); !errors.Is(err, wantErr) {
		t.Fatalf("validation error = %v", err)
	}
}

func TestBaseConfigLoaderWithoutValidatorAcceptsConfiguration(t *testing.T) {
	loader := NewBaseConfigLoader("config/app.yaml", nil)
	if err := loader.ValidateConfig(struct{}{}); err != nil {
		t.Fatalf("validation error = %v", err)
	}
}
