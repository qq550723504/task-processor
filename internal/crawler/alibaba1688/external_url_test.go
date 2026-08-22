package alibaba1688

import (
	"strings"
	"testing"
)

const (
	testURLScheme   = "https"
	testURLUserName = "fixture-user"
	testURLPassword = "fixture-password"
	testURLHost     = "cdn.example"
	testURLPath     = "/image.jpg"
)

func credentialedExternalURLForTest() string {
	return strings.Join([]string{
		testURLScheme + "://",
		testURLUserName,
		":",
		testURLPassword,
		"@",
		testURLHost,
		testURLPath,
	}, "")
}

func credentialedUserInfoForTest() string {
	return testURLUserName + ":" + testURLPassword
}

func TestIsValidMediaURL(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want bool
	}{
		{
			name: "rejects user info",
			raw:  credentialedExternalURLForTest(),
			want: false,
		},
		{
			name: "accepts signed https url with query",
			raw:  "https://cdn.example/image.jpg?Expires=123&Signature=abc",
			want: true,
		},
		{
			name: "rejects hostless url",
			raw:  "https:///image.jpg",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isValidMediaURL(tt.raw); got != tt.want {
				t.Fatalf("unexpected media URL admission result: got %t, want %t", got, tt.want)
			}
		})
	}
}

func TestIsValidExternalURLRejectsMalformedRequestURI(t *testing.T) {
	policy := externalURLPolicy{allowQuery: true}
	tests := []struct {
		name string
		raw  string
	}{
		{
			name: "contains a space",
			raw:  "https://cdn.example/image with space.jpg",
		},
		{
			name: "contains leading whitespace",
			raw:  " https://cdn.example/image.jpg",
		},
		{
			name: "contains an invalid percent escape",
			raw:  "https://cdn.example/%zz",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if isValidExternalURL(tt.raw, policy) {
				t.Fatal("malformed request URI was admitted")
			}
		})
	}
}

func TestIsValidSupplierShopURL(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want bool
	}{
		{
			name: "rejects user info",
			raw:  credentialedExternalURLForTest(),
			want: false,
		},
		{
			name: "rejects http",
			raw:  "http://shop.example/store",
			want: false,
		},
		{
			name: "rejects query",
			raw:  "https://shop.example/store?from=ads",
			want: false,
		},
		{
			name: "rejects fragment",
			raw:  "https://shop.example/store#intro",
			want: false,
		},
		{
			name: "accepts clean https shop url",
			raw:  "https://shop.example/store",
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isValidSupplierShopURL(tt.raw); got != tt.want {
				t.Fatalf("unexpected supplier shop URL admission result: got %t, want %t", got, tt.want)
			}
		})
	}
}
