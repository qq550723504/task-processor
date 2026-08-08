package shared

import "testing"

func TestNewCrawlerTaskRetainsSourceAccountID(t *testing.T) {
	task := NewCrawlerTask("https://detail.1688.com/offer/3001.html")
	task.SourceAccountID = 3001

	if task.SourceAccountID != 3001 {
		t.Fatalf("SourceAccountID = %d, want 3001", task.SourceAccountID)
	}
}

func TestCrawlerTaskRetainsTrustedTenantIDInWorkerPayload(t *testing.T) {
	task := NewCrawlerTask("https://detail.1688.com/offer/3001.html")
	task.TenantID = 101

	if task.TenantID != 101 {
		t.Fatalf("TenantID = %d, want 101", task.TenantID)
	}
}
