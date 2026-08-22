package enrich

import (
	"context"
	"testing"

	"github.com/sirupsen/logrus"

	"task-processor/internal/aicapability"
	productenrich "task-processor/internal/productenrich"
)

type identityFailTextGenerator struct{}

type identityTestLLMManager struct{}

func (identityTestLLMManager) GetClient(string) (productenrich.LLMClient, error) { return nil, nil }
func (identityTestLLMManager) GetDefaultClient() productenrich.LLMClient         { return nil }

func (identityFailTextGenerator) Generate(context.Context, string) (string, error) {
	return "", aicapability.NewError(aicapability.ErrorIdentityIntegrity, string(aicapability.OperationProductEnrichTextExtract), nil)
}

func TestAnalyzeProductDoesNotSwallowIdentityIntegrity(t *testing.T) {
	understanding, err := NewProductUnderstandingWithTextGenerator(identityTestLLMManager{}, identityFailTextGenerator{})
	if err != nil {
		t.Fatalf("NewProductUnderstandingWithTextGenerator: %v", err)
	}

	_, err = understanding.AnalyzeProduct(context.Background(), &productenrich.ParsedInput{Text: "product"})
	if aicapability.CategoryOf(err) != aicapability.ErrorIdentityIntegrity {
		t.Fatalf("error category = %q, want %q", aicapability.CategoryOf(err), aicapability.ErrorIdentityIntegrity)
	}
}

func TestGenerateJSONDoesNotFallbackOnIdentityIntegrity(t *testing.T) {
	generator, err := NewJSONGeneratorWithTextGenerator(logrus.New(), identityTestLLMManager{}, identityFailTextGenerator{})
	if err != nil {
		t.Fatalf("NewJSONGeneratorWithTextGenerator: %v", err)
	}

	_, err = generator.GenerateJSON(context.Background(), &productenrich.ProductAnalysis{Representation: &productenrich.ProductRepresentation{ProductType: "product"}}, nil, true)
	if aicapability.CategoryOf(err) != aicapability.ErrorIdentityIntegrity {
		t.Fatalf("error category = %q, want %q", aicapability.CategoryOf(err), aicapability.ErrorIdentityIntegrity)
	}
}
