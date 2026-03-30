package browser

import "testing"

func TestInferCountryFromTargetURL(t *testing.T) {
	cases := []struct {
		url  string
		want string
	}{
		{url: "https://www.amazon.com/dp/B001234567", want: "United States"},
		{url: "https://www.amazon.co.uk/dp/B001234567", want: "United Kingdom"},
		{url: "https://www.amazon.ca/dp/B001234567", want: "Canada"},
		{url: "https://www.amazon.co.jp/dp/B001234567", want: "Japan"},
		{url: "https://example.com/item", want: ""},
	}

	for _, tc := range cases {
		if got := inferCountryFromTargetURL(tc.url); got != tc.want {
			t.Fatalf("inferCountryFromTargetURL(%q)=%q want %q", tc.url, got, tc.want)
		}
	}
}

func TestInferCountryFromZipcode(t *testing.T) {
	cases := []struct {
		zipcode string
		want    string
	}{
		{zipcode: "10001", want: "United States"},
		{zipcode: "M5V2T6", want: "Canada"},
		{zipcode: "100-0001", want: "Japan"},
		{zipcode: "SW1A 1AA", want: "United Kingdom"},
		{zipcode: "UNKNOWN", want: ""},
	}

	for _, tc := range cases {
		if got := inferCountryFromZipcode(tc.zipcode); got != tc.want {
			t.Fatalf("inferCountryFromZipcode(%q)=%q want %q", tc.zipcode, got, tc.want)
		}
	}
}

func TestInferDeliveryCountryPrefersTargetURL(t *testing.T) {
	got := inferDeliveryCountry("https://www.amazon.co.uk/dp/B001234567", "10001")
	if got != "United Kingdom" {
		t.Fatalf("inferDeliveryCountry returned %q", got)
	}
}

func TestBuildCountrySelectionQueries(t *testing.T) {
	got := buildCountrySelectionQueries("United States")
	if got != nil {
		t.Fatalf("buildCountrySelectionQueries returned %v", got)
	}
}
