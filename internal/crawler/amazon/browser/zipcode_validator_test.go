package browser

import "testing"

func TestInferTargetCountry(t *testing.T) {
	got := inferTargetCountry("https://www.amazon.co.jp/dp/B001234567", "153-0064")
	if got != "Japan" {
		t.Fatalf("inferTargetCountry returned %q", got)
	}
}

func TestLocationMatchesTargetCountry(t *testing.T) {
	cases := []struct {
		name          string
		currentText   string
		targetCountry string
		want          bool
	}{
		{name: "jp-country", currentText: "日本 〒153-0064", targetCountry: "Japan", want: true},
		{name: "jp-city", currentText: "東京都目黒区", targetCountry: "Japan", want: true},
		{name: "jp-foreign", currentText: "シンガポール", targetCountry: "Japan", want: false},
		{name: "uk-country", currentText: "United Kingdom SW1A 1AA", targetCountry: "United Kingdom", want: true},
		{name: "ca-country", currentText: "Delivering to Toronto, Canada", targetCountry: "Canada", want: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := locationMatchesTargetCountry(tc.currentText, tc.targetCountry); got != tc.want {
				t.Fatalf("locationMatchesTargetCountry(%q, %q)=%v want %v", tc.currentText, tc.targetCountry, got, tc.want)
			}
		})
	}
}

func TestTextMatchesTargetContext(t *testing.T) {
	cases := []struct {
		name          string
		currentText   string
		targetCountry string
		want          bool
	}{
		{name: "us-zip", currentText: "San Franc... 94107", targetCountry: "United States", want: true},
		{name: "us-foreign", currentText: "Hong Kong 999077", targetCountry: "United States", want: false},
		{name: "ca-postcode", currentText: "Toronto M5V 2T6", targetCountry: "Canada", want: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := textMatchesTargetContext(tc.currentText, tc.targetCountry); got != tc.want {
				t.Fatalf("textMatchesTargetContext(%q, %q)=%v want %v", tc.currentText, tc.targetCountry, got, tc.want)
			}
		})
	}
}
