package size

import "testing"

func TestParseShoeSize(t *testing.T) {
	tests := []struct {
		name       string
		raw        string
		wantBase   string
		wantSystem System
		wantWidth  Width
		wantShoe   bool
	}{
		{name: "plain_numeric", raw: "7", wantBase: "7", wantSystem: SystemUnknown, wantWidth: WidthRegular, wantShoe: true},
		{name: "us_wide_suffix", raw: "US7W", wantBase: "7", wantSystem: SystemUS, wantWidth: WidthWide, wantShoe: true},
		{name: "us_xwide_words", raw: "US 7 X-Wide", wantBase: "7", wantSystem: SystemUS, wantWidth: WidthXWide, wantShoe: true},
		{name: "brand_us_style", raw: "7.5 B(M) US", wantBase: "7.5", wantSystem: SystemUS, wantWidth: WidthRegular, wantShoe: true},
		{name: "eu_value", raw: "EU 40", wantBase: "40", wantSystem: SystemEU, wantWidth: WidthRegular, wantShoe: true},
		{name: "range_value_is_not_single_size", raw: "US7-8", wantBase: "", wantSystem: SystemUnknown, wantWidth: WidthUnknown, wantShoe: false},
		{name: "non_size_text", raw: "Light Pink", wantBase: "", wantSystem: SystemUnknown, wantWidth: WidthUnknown, wantShoe: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseShoeSize(tt.raw)
			if got.BaseSize != tt.wantBase {
				t.Fatalf("BaseSize = %q, want %q", got.BaseSize, tt.wantBase)
			}
			if got.System != tt.wantSystem {
				t.Fatalf("System = %q, want %q", got.System, tt.wantSystem)
			}
			if got.Width != tt.wantWidth {
				t.Fatalf("Width = %q, want %q", got.Width, tt.wantWidth)
			}
			if got.IsShoeSize != tt.wantShoe {
				t.Fatalf("IsShoeSize = %v, want %v", got.IsShoeSize, tt.wantShoe)
			}
		})
	}
}

func TestAreShoeSizesFuzzyCompatible(t *testing.T) {
	tests := []struct {
		name string
		a    string
		b    string
		want bool
	}{
		{name: "same_base_wide_xwide", a: "7 Wide", b: "7 X-Wide", want: true},
		{name: "same_base_regular", a: "10.5", b: "10.5", want: true},
		{name: "same_base_regular_vs_wide", a: "10", b: "10 Wide", want: false},
		{name: "different_base", a: "5 Wide", b: "8.5 Wide", want: false},
		{name: "different_decimal", a: "10.5", b: "10", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := AreShoeSizesFuzzyCompatible(tt.a, tt.b); got != tt.want {
				t.Fatalf("AreShoeSizesFuzzyCompatible(%q, %q) = %v, want %v", tt.a, tt.b, got, tt.want)
			}
		})
	}
}
