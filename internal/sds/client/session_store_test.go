package client

import "testing"

func TestParseCookieHeader(t *testing.T) {
	t.Parallel()

	cookies := ParseCookieHeader("sid=abc; token=xyz", ".sdsdiy.com")
	if len(cookies) != 2 {
		t.Fatalf("expected 2 cookies, got %d", len(cookies))
	}

	if cookies[0].Name != "sid" || cookies[0].Value != "abc" {
		t.Fatalf("unexpected first cookie: %+v", cookies[0])
	}

	if cookies[1].Domain != ".sdsdiy.com" {
		t.Fatalf("unexpected cookie domain: %+v", cookies[1])
	}
}
