package browser

import "testing"

func TestParseCurrencyCodeFromText(t *testing.T) {
	cases := []struct {
		text string
		want string
	}{
		{text: "£GBP - Pounds", want: "GBP"},
		{text: "JP¥JPY - Japanese Yen", want: "JPY"},
		{text: "US$ - USD - US Dollar", want: "USD"},
		{text: "CN¥ - CNY - Chinese Yuan", want: "CNY"},
		{text: "no currency here", want: ""},
	}

	for _, tc := range cases {
		if got := parseCurrencyCodeFromText(tc.text); got != tc.want {
			t.Fatalf("parseCurrencyCodeFromText(%q)=%q want %q", tc.text, got, tc.want)
		}
	}
}

func TestExtractCurrencyFromURL(t *testing.T) {
	cases := []struct {
		rawURL string
		want   string
	}{
		{rawURL: "https://www.amazon.co.uk/?currency=GBP", want: "GBP"},
		{rawURL: "https://www.amazon.com/dp/B000?currency=USD&psc=1", want: "USD"},
		{rawURL: "https://www.amazon.co.uk/", want: ""},
		{rawURL: "://bad-url", want: ""},
	}

	for _, tc := range cases {
		if got := extractCurrencyFromURL(tc.rawURL); got != tc.want {
			t.Fatalf("extractCurrencyFromURL(%q)=%q want %q", tc.rawURL, got, tc.want)
		}
	}
}
