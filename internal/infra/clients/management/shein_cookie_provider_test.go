package management

import "testing"

func TestExtractTenantIDFromSheinCookieKey(t *testing.T) {
	tenantID, ok := extractTenantIDFromSheinCookieKey("shein:cookie:227:869", 869)
	if !ok {
		t.Fatalf("expected key to match")
	}
	if tenantID != 227 {
		t.Fatalf("tenantID = %d, want 227", tenantID)
	}
}

func TestNormalizeSheinCookiePayloadWrapper(t *testing.T) {
	raw := `{"cookies":[{"name":"sid","value":"abc","domain":".shein.com","path":"/"}]}`
	got, err := normalizeSheinCookiePayload(raw)
	if err != nil {
		t.Fatalf("normalizeSheinCookiePayload() error = %v", err)
	}
	want := `[{"name":"sid","value":"abc","domain":".shein.com","path":"/"}]`
	if got != want {
		t.Fatalf("normalizeSheinCookiePayload() = %s, want %s", got, want)
	}
}
