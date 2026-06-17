package product

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFetchersUseVariantFetchRequestAdapter(t *testing.T) {
	t.Parallel()

	files := []string{
		filepath.Join("product_fetcher.go"),
		filepath.Join("..", "crawler", "fetcher", "remote_fetcher.go"),
		filepath.Join("..", "crawler", "fetcher", "distributed_fetcher.go"),
	}
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("read %s: %v", file, err)
		}
		source := string(data)
		if !strings.Contains(source, "VariantFetchRequest(") {
			t.Fatalf("%s should use VariantFetchRequest for variant request adaptation", file)
		}
		if strings.Contains(source, "VariantSourceRequest(SourceRequestFromFetch(") ||
			strings.Contains(source, "VariantSourceRequest(domainProduct.SourceRequestFromFetch(") {
			t.Fatalf("%s still hand-composes source variant request adaptation", file)
		}
	}
}
