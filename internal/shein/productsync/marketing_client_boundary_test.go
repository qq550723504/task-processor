package productsync

import (
	"os"
	"strings"
	"testing"
)

func TestPackageDoesNotOwnMarketingAPIClient(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("ReadDir(.): %v", err)
	}

	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}

		content, err := os.ReadFile(name)
		if err != nil {
			t.Fatalf("ReadFile(%s): %v", name, err)
		}
		for _, forbidden := range []string{
			`"task-processor/internal/shein/api/marketing"`,
			"type MarketingAPI struct",
			"func NewMarketingAPI(",
		} {
			if strings.Contains(string(content), forbidden) {
				t.Fatalf("%s must not contain %q; marketing transport belongs to internal/shein/api/marketing", name, forbidden)
			}
		}
	}
}
