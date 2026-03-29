package productenrich

import (
	"testing"
)

func TestGetImageFormat(t *testing.T) {
	v := &inputValidator{}
	cases := []struct {
		url  string
		want string
	}{
		{"https://example.com/photo.jpg", "jpg"},
		{"https://example.com/photo.jpeg", "jpg"},
		{"https://example.com/photo.PNG", "png"},
		{"https://example.com/photo.webp", "webp"},
		{"https://example.com/photo.gif", "unknown"},
		{"https://example.com/photo", "unknown"},
	}
	for _, tc := range cases {
		t.Run(tc.url, func(t *testing.T) {
			got := v.getImageFormat(tc.url)
			if got != tc.want {
				t.Errorf("getImageFormat(%q) = %q, want %q", tc.url, got, tc.want)
			}
		})
	}
}

func TestIsTrustedCDN(t *testing.T) {
	cases := []struct {
		host string
		want bool
	}{
		{"cbu01.alicdn.com", true},
		{"img.alicdn.com", true},
		{"img.1688.com", true},
		{"sc02.alicdn.com", true},
		{"example.com", false},
		{"evil.cbu01.alicdn.com.attacker.com", false},
		{"notalicdn.com", false},
	}
	for _, tc := range cases {
		t.Run(tc.host, func(t *testing.T) {
			got := isTrustedCDN(tc.host)
			if got != tc.want {
				t.Errorf("isTrustedCDN(%q) = %v, want %v", tc.host, got, tc.want)
			}
		})
	}
}

func TestValidateSingleImage_InvalidURL(t *testing.T) {
	v := &inputValidator{
		httpClient: nil, // 不会走到 HTTP 请求
		maxWorkers: 1,
	}

	cases := []struct {
		name   string
		url    string
		wantOK bool
	}{
		{"invalid scheme", "ftp://example.com/img.jpg", false},
		{"malformed url", "://bad", false},
		{"unsupported format", "https://example.com/photo.gif", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			info := v.validateSingleImage(nil, tc.url) //nolint:staticcheck
			if info.IsValid != tc.wantOK {
				t.Errorf("IsValid = %v, want %v (error: %s)", info.IsValid, tc.wantOK, info.Error)
			}
		})
	}
}
