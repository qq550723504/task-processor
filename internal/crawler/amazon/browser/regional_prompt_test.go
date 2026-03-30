package browser

import "testing"

func TestContainsRegionalPromptText(t *testing.T) {
	cases := []struct {
		text string
		want bool
	}{
		{text: "Visiting from Singapore?", want: true},
		{text: "Choosing your Amazon website", want: true},
		{text: "Visit Amazon.sg for the best experience", want: true},
		{text: "Stay on Amazon.sg", want: true},
		{text: "Go to Amazon.com", want: true},
		{text: "Delivering to Singapore 490000", want: false},
		{text: "Update location", want: false},
	}

	for _, tc := range cases {
		if got := ContainsRegionalPromptText(tc.text); got != tc.want {
			t.Fatalf("ContainsRegionalPromptText(%q)=%v want %v", tc.text, got, tc.want)
		}
	}
}

func TestNormalizedAmazonHost(t *testing.T) {
	if got := normalizedAmazonHost("https://www.amazon.com/dp/B001234567?mr_donotredirect=1"); got != "www.amazon.com" {
		t.Fatalf("normalizedAmazonHost returned %q", got)
	}
}
