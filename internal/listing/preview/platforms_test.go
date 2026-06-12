package preview

import (
	"slices"
	"testing"
)

func TestResolvePlatforms(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		resultPlatforms  []string
		requestPlatforms []string
		want             []string
	}{
		{
			name:             "prefers result platforms",
			resultPlatforms:  []string{"shein", "amazon"},
			requestPlatforms: []string{"temu"},
			want:             []string{"shein", "amazon"},
		},
		{
			name:             "falls back to request platforms",
			requestPlatforms: []string{"temu", "walmart"},
			want:             []string{"temu", "walmart"},
		},
		{
			name: "returns nil when both are empty",
			want: nil,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := ResolvePlatforms(tt.resultPlatforms, tt.requestPlatforms)
			if !slices.Equal(got, tt.want) {
				t.Fatalf("ResolvePlatforms(%#v, %#v) = %#v, want %#v", tt.resultPlatforms, tt.requestPlatforms, got, tt.want)
			}
		})
	}
}
