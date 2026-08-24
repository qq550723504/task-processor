package shared

import (
	"encoding/json"
	"testing"
)

func TestCrawlerResultRoundTripsSourceAccessMetadata(t *testing.T) {
	result := NewCrawlerResult("task-1")
	result.SourceAccessMode = "account_assisted"
	result.SourceFallbackReason = "public_challenge"

	payload, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	var decoded CrawlerResult
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if decoded.SourceAccessMode != "account_assisted" || decoded.SourceFallbackReason != "public_challenge" {
		t.Fatalf("metadata = (%q, %q), want account-assisted metadata", decoded.SourceAccessMode, decoded.SourceFallbackReason)
	}
}
