package listing

import (
	"os"
	"testing"
)

func TestResolveConfigPath(t *testing.T) {
	tests := []struct {
		name      string
		config    string
		appConfig string
		want      string
	}{
		{
			name:      "config wins",
			config:    "config/config-dev.yaml",
			appConfig: "config/legacy.yaml",
			want:      "config/config-dev.yaml",
		},
		{
			name:      "legacy app config fallback",
			appConfig: "config/legacy.yaml",
			want:      "config/legacy.yaml",
		},
		{
			name: "default config",
			want: defaultConfigPath,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ResolveConfigPath(tt.config, tt.appConfig); got != tt.want {
				t.Fatalf("ResolveConfigPath() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestResolveDebugTaskID(t *testing.T) {
	const envKey = "TASK_PROCESSOR_DEBUG_TASK_ID"
	original := os.Getenv(envKey)
	t.Cleanup(func() {
		if original == "" {
			_ = os.Unsetenv(envKey)
			return
		}
		_ = os.Setenv(envKey, original)
	})

	t.Run("reads env value", func(t *testing.T) {
		if err := os.Setenv(envKey, " 8189311 "); err != nil {
			t.Fatalf("Setenv() error = %v", err)
		}
		if got := ResolveDebugTaskID(); got != 8189311 {
			t.Fatalf("ResolveDebugTaskID() = %d, want %d", got, 8189311)
		}
	})

	t.Run("invalid env value falls back to zero", func(t *testing.T) {
		if err := os.Setenv(envKey, "abc"); err != nil {
			t.Fatalf("Setenv() error = %v", err)
		}
		if got := ResolveDebugTaskID(); got != 0 {
			t.Fatalf("ResolveDebugTaskID() = %d, want 0", got)
		}
	})

	t.Run("missing env value falls back to zero", func(t *testing.T) {
		if err := os.Unsetenv(envKey); err != nil {
			t.Fatalf("Unsetenv() error = %v", err)
		}
		if got := ResolveDebugTaskID(); got != 0 {
			t.Fatalf("ResolveDebugTaskID() = %d, want 0", got)
		}
	})
}
