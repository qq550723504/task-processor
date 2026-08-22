package enrich_test

import (
	"context"
	"errors"
	"testing"

	"github.com/sirupsen/logrus"

	"task-processor/internal/productenrich"
	productenrichenrich "task-processor/internal/productenrich/enrich"
)

func TestJSONGeneratorUsesGovernedContentGeneratorWithoutLegacyDefaultClient(t *testing.T) {
	content := &recordingContentGenerator{response: `{"title":"Desk","category":["Furniture"],"attributes":{"material":"wood"}}`}
	generator, err := productenrichenrich.NewJSONGeneratorWithTextGenerator(logrus.New(), &defaultClientMustNotBeCalled{}, content)
	if err != nil {
		t.Fatalf("NewJSONGeneratorWithTextGenerator: %v", err)
	}
	result, err := generator.GenerateJSON(context.Background(), &productenrich.ProductAnalysis{}, nil, true)
	if err != nil {
		t.Fatalf("GenerateJSON: %v", err)
	}
	if result.Title != "Desk" || !content.called {
		t.Fatalf("result/content = %+v/%v", result, content.called)
	}
}

type recordingContentGenerator struct {
	response string
	called   bool
}

func (g *recordingContentGenerator) Generate(context.Context, string) (string, error) {
	g.called = true
	return g.response, nil
}

type defaultClientMustNotBeCalled struct{}

func (*defaultClientMustNotBeCalled) GetClient(string) (productenrich.LLMClient, error) {
	return nil, errors.New("legacy client must not be called")
}
func (*defaultClientMustNotBeCalled) GetDefaultClient() productenrich.LLMClient {
	return &panicLLMClient{}
}

type panicLLMClient struct{}

func (*panicLLMClient) Generate(context.Context, string) (string, error) {
	panic("legacy default client was called")
}
func (*panicLLMClient) AnalyzeImage(context.Context, string, string) (string, error) { return "", nil }
