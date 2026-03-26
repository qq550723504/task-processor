package productenrich

import (
	"context"
	"testing"
)

func TestRunPipeline_PopulatesPipelineState(t *testing.T) {
	task := &Task{
		ID:      "pipe-1",
		Request: &GenerateRequest{Text: "a product"},
		Status:  TaskStatusPending,
	}
	repo := newMockTaskRepo(task)

	svc, _ := NewProductService(&ProductServiceConfig{
		QueueName:        "test",
		TaskRepo:         repo,
		RedisClient:      &mockRedisClient{},
		InputParser:      &mockInputParser{result: &ParsedInput{Text: "parsed"}},
		InputValidator:   &mockInputValidator{result: &ValidationResult{}},
		QualityScorer:    &mockQualityScorer{score: 65},
		StrategySelector: &mockStrategySelector{strategy: StrategyBasic},
		ProductUnderstanding: &mockProductUnderstanding{
			result: &ProductAnalysis{Representation: &ProductRepresentation{ProductType: "Widget"}},
		},
		JSONGenerator: &mockJSONGenerator{result: &ProductJSON{Title: "Generated"}},
	})

	state := &PipelineState{Task: task}
	if err := svc.(*productService).runPipeline(context.Background(), state); err != nil {
		t.Fatalf("runPipeline error: %v", err)
	}
	if state.ParsedInput == nil || state.ParsedInput.Text != "parsed" {
		t.Fatalf("unexpected ParsedInput: %+v", state.ParsedInput)
	}
	if state.Strategy != StrategyBasic {
		t.Fatalf("Strategy = %q, want basic", state.Strategy)
	}
	if state.Analysis == nil || state.Analysis.Representation == nil {
		t.Fatalf("unexpected Analysis: %+v", state.Analysis)
	}
	if state.ProductJSON == nil || state.ProductJSON.Title != "Generated" {
		t.Fatalf("unexpected ProductJSON: %+v", state.ProductJSON)
	}
}
