package httpapi

import (
	"os"
	"strings"
	"testing"
)

func TestBootstrapKeepsLLMScorerAssemblyInDedicatedFile(t *testing.T) {
	bootstrapSource := readProductEnrichHTTPAPIBoundaryFile(t, "bootstrap.go")
	for _, marker := range []string{
		"const productScorerClientName",
		"func buildLLMScorerWithCache(",
		"productenrich.LLMScorerConfig{",
		"scorerCfg.TextClient = productScorerClientName",
	} {
		if strings.Contains(bootstrapSource, marker) {
			t.Fatalf("bootstrap.go should delegate ProductEnrich LLM scorer assembly to scorer_builder.go; found %s", marker)
		}
	}

	builderSource := readProductEnrichHTTPAPIBoundaryFile(t, "scorer_builder.go")
	for _, marker := range []string{
		"const productScorerClientName",
		"func buildLLMScorerWithCache(",
		"productenrich.LLMScorerConfig{",
		"scorerCfg.TextClient = productScorerClientName",
	} {
		if !strings.Contains(builderSource, marker) {
			t.Fatalf("scorer_builder.go missing %s", marker)
		}
	}
}

func readProductEnrichHTTPAPIBoundaryFile(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
