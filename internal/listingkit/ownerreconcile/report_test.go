package ownerreconcile

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestReportFingerprintAndJSONAreDeterministicAndRedacted(t *testing.T) {
	report := NewReport("config-prod.yaml", "ruoyi-vue-pro", []Finding{
		{Table: "listing_store", TenantFingerprint: "sha256:tenant", OwnerFingerprint: "sha256:owner", Rows: 2, Reason: "no_candidate"},
		{Table: "listing_product_data", TenantFingerprint: "sha256:tenant2", OwnerFingerprint: "sha256:owner2", Rows: 1, Reason: "conflicting_candidates"},
	})
	first, err := report.Fingerprint()
	if err != nil {
		t.Fatal(err)
	}
	second, err := report.Fingerprint()
	if err != nil {
		t.Fatal(err)
	}
	if first != second || len(first) != 12 {
		t.Fatalf("fingerprints = %q and %q, want equal 12-hex values", first, second)
	}
	encoded, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	text := string(encoded)
	for _, forbidden := range []string{"legacy-user-42", "subject-42", "tenant-42", "email@example.com", "token-secret"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("report leaked %q: %s", forbidden, text)
		}
	}
	if !strings.Contains(text, "listing_store") || !strings.Contains(text, "no_candidate") {
		t.Fatalf("report omitted safe finding fields: %s", text)
	}
}

func TestReportSortsFindingsBeforeFingerprinting(t *testing.T) {
	left := NewReport("config.yaml", "db", []Finding{{Table: "b", TenantFingerprint: "t", OwnerFingerprint: "o", Rows: 1, Reason: "no_candidate"}, {Table: "a", TenantFingerprint: "t", OwnerFingerprint: "o", Rows: 1, Reason: "no_candidate"}})
	right := NewReport("config.yaml", "db", []Finding{{Table: "a", TenantFingerprint: "t", OwnerFingerprint: "o", Rows: 1, Reason: "no_candidate"}, {Table: "b", TenantFingerprint: "t", OwnerFingerprint: "o", Rows: 1, Reason: "no_candidate"}})
	leftFingerprint, err := left.Fingerprint()
	if err != nil {
		t.Fatal(err)
	}
	rightFingerprint, err := right.Fingerprint()
	if err != nil {
		t.Fatal(err)
	}
	if leftFingerprint != rightFingerprint {
		t.Fatalf("sorted fingerprints differ: %q != %q", leftFingerprint, rightFingerprint)
	}
}
