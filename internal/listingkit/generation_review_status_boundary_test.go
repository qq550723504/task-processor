package listingkit

import (
	"os"
	"strings"
	"testing"
)

func TestGenerationReviewStatusUsesDomainDirectly(t *testing.T) {
	source, err := os.ReadFile("generation_review_persistence.go")
	if err != nil {
		t.Fatalf("read generation_review_persistence.go: %v", err)
	}
	content := string(source)
	if strings.Contains(content, "func generationReviewStatusFromDecision(") {
		t.Fatal("generation_review_persistence.go should not keep the forwarding wrapper")
	}
	if !strings.Contains(content, "listinggeneration.ReviewStatusFromDecision(") {
		t.Fatal("generation_review_persistence.go should call the generation domain directly")
	}
}
