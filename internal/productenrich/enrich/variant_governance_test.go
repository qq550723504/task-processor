package enrich_test

import (
	"context"
	"testing"

	"task-processor/internal/productenrich"
	productenrichenrich "task-processor/internal/productenrich/enrich"
)

func TestVariantGeneratorUsesGovernedSpecsGeneratorWithoutLegacyClientLookup(t *testing.T) {
	specsGenerator := &recordingContentGenerator{response: `{"technical":{"material":"wood"}}`}
	generator, err := productenrichenrich.NewVariantGeneratorWithSpecsGenerator(&defaultClientMustNotBeCalled{}, specsGenerator)
	if err != nil {
		t.Fatalf("NewVariantGeneratorWithSpecsGenerator: %v", err)
	}
	specs, err := generator.GenerateSpecs(context.Background(), &productenrich.ProductAnalysis{})
	if err != nil {
		t.Fatalf("GenerateSpecs: %v", err)
	}
	if specs.Technical["material"] != "wood" || !specsGenerator.called {
		t.Fatalf("specs/generator = %+v/%v", specs, specsGenerator.called)
	}
}
