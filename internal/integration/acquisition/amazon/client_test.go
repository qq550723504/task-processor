package amazon

import (
	"errors"
	"io"
	"net/http"
	"net/netip"
	"strings"
	"task-processor/internal/product/dataacquisition"
	"testing"
)

func TestBoundedResponseDetectsChallengeBeforeNonOKRejection(t *testing.T) {
	for _, tc := range []struct {
		name              string
		status            int
		contentType, body string
		want              error
	}{
		{"forbidden captcha", 403, "text/html", `<form action="/errors/validateCaptcha"><input id="captchacharacters"></form>`, ErrChallenge},
		{"unavailable robot check", 503, "text/html; charset=utf-8", `<html><title>Robot Check</title></html>`, ErrChallenge},
		{"ordinary maintenance", 503, "text/html", `<html><title>Maintenance</title></html>`, ErrRejected},
		{"normal document", 200, "text/html", `<html><title>Amazon</title></html>`, nil},
		{"non HTML", 403, "application/json", `{"message":"Robot Check"}`, ErrRejected},
	} {
		t.Run(tc.name, func(t *testing.T) {
			resp := &http.Response{StatusCode: tc.status, Header: http.Header{"Content-Type": []string{tc.contentType}}, ContentLength: int64(len(tc.body)), Body: io.NopCloser(strings.NewReader(tc.body))}
			body, err := readDocumentResponse(resp)
			if !errors.Is(err, tc.want) {
				t.Fatalf("expected %v, got %v", tc.want, err)
			}
			if err == nil && string(body) != tc.body {
				t.Fatal("successful document changed")
			}
		})
	}
	for _, size := range []int64{maxHTMLBytes + 1, -1} {
		resp := &http.Response{StatusCode: 503, Header: http.Header{"Content-Type": []string{"text/html"}}, ContentLength: size, Body: io.NopCloser(strings.NewReader(`<title>Robot Check</title>` + strings.Repeat(" ", maxHTMLBytes)))}
		if _, err := readDocumentResponse(resp); !errors.Is(err, ErrRejected) {
			t.Fatalf("oversized challenge must remain rejected: %v", err)
		}
	}
}

func TestEgressRejectsNonPublicAddressesAndNonCanonicalNavigation(t *testing.T) {
	for _, address := range []string{"127.0.0.1", "::1", "10.0.0.1", "169.254.169.254", "100.64.0.1", "192.0.0.1", "224.0.0.1", "fd00::1", "::ffff:127.0.0.1"} {
		if PublicAddress(netip.MustParseAddr(address)) {
			t.Fatalf("private/special address admitted: %s", address)
		}
	}
	site, _ := dataacquisition.ResolveSite("us")
	for _, address := range []string{"http://www.amazon.com/dp/B000123456", "https://www.amazon.com/cart/add", "https://www.amazon.com.evil.test/s?k=a", "https://user@www.amazon.com/s?k=a"} {
		if allowedNavigation(site, address) {
			t.Fatalf("unsafe navigation: %s", address)
		}
	}
}
