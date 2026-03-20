package productenrich

import (
	"context"
	"errors"
	"testing"
	"time"
)

// --- mock 组件 ---

type mockInputParser struct {
	result *ParsedInput
	err    error
}

func (m *mockInputParser) ParseInput(_ context.Context, _ *GenerateRequest) (*ParsedInput, error) {
	return m.result, m.err
}
func (m *mockInputParser) CollectImages(_ context.Context, _ []string) ([]string, error) {
	return nil, nil
}
func (m *mockInputParser) CleanText(text string) string { return text }
func (m *mockInputParser) Scrape1688(_ context.Context, _ string) (*ScrapedData, error) {
	return nil, nil
}

type mockProductUnderstanding struct {
	result *ProductAnalysis
	err    error
}

func (m *mockProductUnderstanding) AnalyzeProduct(_ context.Context, _ *ParsedInput) (*ProductAnalysis, error) {
	return m.result, m.err
}
func (m *mockProductUnderstanding) AnalyzeImage(_ context.Context, _ string) (*ImageAttributes, error) {
	return nil, nil
}
func (m *mockProductUnderstanding) ExtractTextAttributes(_ context.Context, _ string) (*TextAttributes, error) {
	return nil, nil
}
func (m *mockProductUnderstanding) FuseMultimodal(_ context.Context, _ *ImageAttributes, _ *TextAttributes) (*ProductRepresentation, error) {
	return nil, nil
}

type mockJSONGenerator struct {
	result *ProductJSON
	err    error
}

func (m *mockJSONGenerator) GenerateJSON(_ context.Context, _ *ProductAnalysis, _ VariantGenerator, _ bool) (*ProductJSON, error) {
	return m.result, m.err
}

type mockQualityScorer struct {
	score float64
	err   error
}

func (m *mockQualityScorer) CalculateScore(_ context.Context, _ *ValidationResult) (float64, error) {
	return m.score, m.err
}

type mockStrategySelector struct {
	strategy ProcessingStrategy
	err      error
}

func (m *mockStrategySelector) SelectStrategy(_ context.Context, _ float64) (ProcessingStrategy, error) {
	return m.strategy, m.err
}

func (m *mockStrategySelector) GetStrategyDetails(_ ProcessingStrategy) (*StrategyDetails, error) {
	return &StrategyDetails{}, nil
}

type mockInputValidator struct {
	result *ValidationResult
	err    error
}

func (m *mockInputValidator) Validate(_ context.Context, _ *ParsedInput) (*ValidationResult, error) {
	return m.result, m.err
}
func (m *mockInputValidator) ValidateImages(_ context.Context, _ []string) (*ImageValidation, error) {
	return nil, nil
}
func (m *mockInputValidator) ValidateText(_ context.Context, _ string) (*TextValidation, error) {
	return nil, nil
}
func (m *mockInputValidator) ValidateScrapedData(_ context.Context, _ *ScrapedData) (*ScrapedDataValidation, error) {
	return nil, nil
}

type mockEnhancementSuggester struct {
	result *EnhancementSuggestion
	err    error
}

func (m *mockEnhancementSuggester) GenerateSuggestions(_ context.Context, _ *ValidationResult) (*EnhancementSuggestion, error) {
	return m.result, m.err
}

type mockResultValidator struct {
	result *ResultValidation
	err    error
}

func (m *mockResultValidator) ValidateResult(_ context.Context, _ *ParsedInput, _ *ProductJSON) (*ResultValidation, error) {
	return m.result, m.err
}
func (m *mockResultValidator) CheckImageConsistency(_ []string, _ []string) bool { return true }
func (m *mockResultValidator) CheckKeywordMatch(_ string, _ string, _ string) (float64, error) {
	return 1.0, nil
}
func (m *mockResultValidator) CheckCompleteness(_ *ProductJSON) (*CompletenessReport, error) {
	return &CompletenessReport{}, nil
}

// mockRedisClient 最小化 Redis mock（满足 RedisClient 接口）
type mockRedisClient struct{}

func (m *mockRedisClient) Push(_ context.Context, _ string, _ string) error {
	return nil
}
func (m *mockRedisClient) Get(_ context.Context, _ string) (string, error) {
	return "", nil
}
func (m *mockRedisClient) Set(_ context.Context, _ string, _ string, _ time.Duration) error {
	return nil
}
func (m *mockRedisClient) Delete(_ context.Context, _ string) error {
	return nil
}

// --- ProcessProduct 测试 ---

func TestProcessProduct_NilTask(t *testing.T) {
	repo := newMockTaskRepo()
	svc, _ := NewProductService(&ProductServiceConfig{
		QueueName:   "test",
		TaskRepo:    repo,
		RedisClient: &mockRedisClient{},
	})
	_, err := svc.ProcessProduct(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error for nil task")
	}
}

func TestProcessProduct_NoInputParser_UsesSimpleParsing(t *testing.T) {
	// 无 InputParser 时应直接用 task.Request 的字段构建 ParsedInput
	task := &Task{
		ID: "t1",
		Request: &GenerateRequest{
			ImageURLs: []string{"http://img.example.com/1.jpg"},
			Text:      "a product",
		},
		Status: TaskStatusPending,
	}
	repo := newMockTaskRepo(task)

	svc, _ := NewProductService(&ProductServiceConfig{
		QueueName:   "test",
		TaskRepo:    repo,
		RedisClient: &mockRedisClient{},
		JSONGenerator: &mockJSONGenerator{
			result: &ProductJSON{Title: "Simple Product"},
		},
	})

	result, err := svc.ProcessProduct(context.Background(), task)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Title != "Simple Product" {
		t.Errorf("Title = %q, want %q", result.Title, "Simple Product")
	}
	// 图片列表应来自 parsedInput.Images（即 task.Request.ImageURLs）
	if len(result.Images) != 1 {
		t.Errorf("Images len = %d, want 1", len(result.Images))
	}
}

func TestProcessProduct_NoValidationComponents_UsesFullStrategy(t *testing.T) {
	// 无 inputValidator/qualityScorer/strategySelector 时应默认走 FullStrategy，任务应成功完成
	task := &Task{ID: "t2", Request: &GenerateRequest{}, Status: TaskStatusPending}
	repo := newMockTaskRepo(task)

	svc, _ := NewProductService(&ProductServiceConfig{
		QueueName:   "test",
		TaskRepo:    repo,
		RedisClient: &mockRedisClient{},
		JSONGenerator: &mockJSONGenerator{
			result: &ProductJSON{Title: "Full"},
		},
	})

	result, err := svc.ProcessProduct(context.Background(), task)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Title != "Full" {
		t.Errorf("Title = %q, want %q", result.Title, "Full")
	}
	// 无验证组件时任务应正常完成（full 策略）
	if task.Status != TaskStatusCompleted {
		t.Errorf("Status = %q, want completed", task.Status)
	}
}

func TestProcessProduct_StrategyBasic_SkipsVariants(t *testing.T) {
	task := &Task{ID: "t3", Request: &GenerateRequest{}, Status: TaskStatusPending}
	repo := newMockTaskRepo(task)

	validationResult := &ValidationResult{QualityScore: 65}
	variantCalled := false

	svc, _ := NewProductService(&ProductServiceConfig{
		QueueName:        "test",
		TaskRepo:         repo,
		RedisClient:      &mockRedisClient{},
		InputValidator:   &mockInputValidator{result: validationResult},
		QualityScorer:    &mockQualityScorer{score: 65},
		StrategySelector: &mockStrategySelector{strategy: StrategyBasic},
		JSONGenerator:    &mockJSONGenerator{result: &ProductJSON{Title: "Basic"}},
		VariantGenerator: &mockVariantGeneratorCapture{
			onGenerateVariants: func() { variantCalled = true },
		},
	})

	_, err := svc.ProcessProduct(context.Background(), task)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if variantCalled {
		t.Error("GenerateVariants should not be called for StrategyBasic (skipVariants=true)")
	}
}

func TestProcessProduct_StrategyMinimal_SkipsVariantGen(t *testing.T) {
	task := &Task{ID: "t4", Request: &GenerateRequest{}, Status: TaskStatusPending}
	repo := newMockTaskRepo(task)

	validationResult := &ValidationResult{QualityScore: 40}
	specsCalled := false

	svc, _ := NewProductService(&ProductServiceConfig{
		QueueName:        "test",
		TaskRepo:         repo,
		RedisClient:      &mockRedisClient{},
		InputValidator:   &mockInputValidator{result: validationResult},
		QualityScorer:    &mockQualityScorer{score: 40},
		StrategySelector: &mockStrategySelector{strategy: StrategyMinimal},
		JSONGenerator:    &mockJSONGenerator{result: &ProductJSON{Title: "Minimal"}},
		VariantGenerator: &mockVariantGeneratorCapture{
			onGenerateSpecs: func() { specsCalled = true },
		},
	})

	_, err := svc.ProcessProduct(context.Background(), task)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if specsCalled {
		t.Error("GenerateSpecs should not be called for StrategyMinimal (variantGen=nil)")
	}
}

func TestProcessProduct_StrategyReject_ReturnsNoRetryError(t *testing.T) {
	task := &Task{ID: "t5", Request: &GenerateRequest{}, Status: TaskStatusPending}
	repo := newMockTaskRepo(task)

	validationResult := &ValidationResult{QualityScore: 10}

	svc, _ := NewProductService(&ProductServiceConfig{
		QueueName:        "test",
		TaskRepo:         repo,
		RedisClient:      &mockRedisClient{},
		InputValidator:   &mockInputValidator{result: validationResult},
		QualityScorer:    &mockQualityScorer{score: 10},
		StrategySelector: &mockStrategySelector{strategy: StrategyReject},
	})

	_, err := svc.ProcessProduct(context.Background(), task)
	if err == nil {
		t.Fatal("expected error for StrategyReject")
	}
	if !isNoRetryError(err) {
		t.Errorf("expected errNoRetry, got %T: %v", err, err)
	}
}

func TestProcessProduct_NoResultValidator_SkipsValidation(t *testing.T) {
	// 无 ResultValidator 时不应报错，直接跳过验证步骤
	task := &Task{ID: "t6", Request: &GenerateRequest{}, Status: TaskStatusPending}
	repo := newMockTaskRepo(task)

	svc, _ := NewProductService(&ProductServiceConfig{
		QueueName:     "test",
		TaskRepo:      repo,
		RedisClient:   &mockRedisClient{},
		JSONGenerator: &mockJSONGenerator{result: &ProductJSON{Title: "NoValidator"}},
		// ResultValidator 故意不设置
	})

	result, err := svc.ProcessProduct(context.Background(), task)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Title != "NoValidator" {
		t.Errorf("Title = %q, want %q", result.Title, "NoValidator")
	}
}

func TestProcessProduct_JSONGeneratorFail_ReturnsError(t *testing.T) {
	task := &Task{ID: "t7", Request: &GenerateRequest{}, Status: TaskStatusPending}
	repo := newMockTaskRepo(task)

	svc, _ := NewProductService(&ProductServiceConfig{
		QueueName:     "test",
		TaskRepo:      repo,
		RedisClient:   &mockRedisClient{},
		JSONGenerator: &mockJSONGenerator{err: errors.New("generation failed")},
	})

	_, err := svc.ProcessProduct(context.Background(), task)
	if err == nil {
		t.Fatal("expected error when JSONGenerator fails")
	}
}

func TestProcessProduct_SaveResultFail_ReturnsError(t *testing.T) {
	task := &Task{ID: "t8", Request: &GenerateRequest{}, Status: TaskStatusPending}
	repo := &failingSaveRepo{mockTaskRepo: newMockTaskRepo(task)}

	svc, _ := NewProductService(&ProductServiceConfig{
		QueueName:     "test",
		TaskRepo:      repo,
		RedisClient:   &mockRedisClient{},
		JSONGenerator: &mockJSONGenerator{result: &ProductJSON{Title: "ok"}},
	})

	_, err := svc.ProcessProduct(context.Background(), task)
	if err == nil {
		t.Fatal("expected error when SaveTaskResult fails")
	}
}

// --- 辅助 mock ---

// mockVariantGeneratorCapture 用于捕获 GenerateSpecs/GenerateVariants 是否被调用
type mockVariantGeneratorCapture struct {
	onGenerateSpecs    func()
	onGenerateVariants func()
}

func (m *mockVariantGeneratorCapture) GenerateSpecs(_ context.Context, _ *ProductAnalysis) (*ProductSpecs, error) {
	if m.onGenerateSpecs != nil {
		m.onGenerateSpecs()
	}
	return nil, nil
}
func (m *mockVariantGeneratorCapture) GenerateVariants(_ context.Context, _ *ProductAnalysis) ([]ProductVariant, error) {
	if m.onGenerateVariants != nil {
		m.onGenerateVariants()
	}
	return nil, nil
}
func (m *mockVariantGeneratorCapture) ExtractDimensions(_ context.Context, _ string) (*Dimensions, error) {
	return nil, nil
}
func (m *mockVariantGeneratorCapture) ExtractWeight(_ context.Context, _ string) (*Weight, error) {
	return nil, nil
}

// failingSaveRepo 让 SaveTaskResult 返回错误
type failingSaveRepo struct {
	*mockTaskRepo
}

func (r *failingSaveRepo) SaveTaskResult(_ context.Context, _ string, _ *ProductJSON) error {
	return errors.New("db write failed")
}
