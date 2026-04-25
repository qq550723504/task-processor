package listing

import "testing"

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
