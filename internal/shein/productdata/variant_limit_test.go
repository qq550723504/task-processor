package productdata

import "testing"

func TestValidateVariantASINCount(t *testing.T) {
	tests := []struct {
		name      string
		count     int
		wantError bool
	}{
		{name: "allows exactly limit", count: 1000, wantError: false},
		{name: "rejects above limit", count: 1001, wantError: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			variantAsins := make([]string, tc.count)
			err := validateVariantASINCount(variantAsins)
			if (err != nil) != tc.wantError {
				t.Fatalf("validateVariantASINCount(%d) error = %v, wantError %v", tc.count, err, tc.wantError)
			}
		})
	}
}
