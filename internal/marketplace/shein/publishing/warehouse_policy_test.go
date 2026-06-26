package publishing

import "testing"

func TestSubmitPreferredWarehouseCodeUsesFirstConfiguredWarehouse(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		warehouseCode string
		want          string
	}{
		"first csv item": {
			warehouseCode: " WH-CA-1, WH-US-1 ",
			want:          "WH-CA-1",
		},
		"skips blanks": {
			warehouseCode: " , WH-US-1 ",
			want:          "WH-US-1",
		},
		"default sentinel": {
			warehouseCode: " , ",
			want:          "DEFAULT",
		},
	}

	for name, tt := range tests {
		tt := tt
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if got := SubmitPreferredWarehouseCode(tt.warehouseCode); got != tt.want {
				t.Fatalf("SubmitPreferredWarehouseCode(%q) = %q, want %q", tt.warehouseCode, got, tt.want)
			}
		})
	}
}
