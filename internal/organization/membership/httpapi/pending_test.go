package httpapi

import (
	"net/url"
	"testing"
)

func TestPendingQueryBounds(t *testing.T) {
	for _, raw := range []string{"?", "?limit=0", "?limit=101", "?limit=-1", "?limit=+1", "?limit=1&limit=2", "?actor=other", "?after=bad", "?after=00000000-0000-0000-0000-000000000000", "?after=EA0390E6-6FD0-4834-8E9C-277CAF59C122", "?limit=%zz", "?limit="} {
		u, _ := url.Parse("/" + raw)
		if _, err := parsePendingPage(u); err == nil {
			t.Errorf("query admitted: %s", raw)
		}
	}
	for _, raw := range []string{"", "?limit=1", "?limit=100&after=ea0390e6-6fd0-4834-8e9c-277caf59c122"} {
		u, _ := url.Parse("/" + raw)
		if _, err := parsePendingPage(u); err != nil {
			t.Errorf("query rejected: %s", raw)
		}
	}
}
