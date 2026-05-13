package productenrich

import (
	"context"
	"fmt"
	"strings"
)

const ProductEnrichMockLLMEnv = "TASK_PROCESSOR_PRODUCTENRICH_MOCK_LLM"

type localMockLLMManager struct {
	client LLMClient
}

type localMockLLMClient struct{}

func NewLocalMockLLMManager() LLMManager {
	return &localMockLLMManager{client: &localMockLLMClient{}}
}

func (m *localMockLLMManager) GetClient(_ string) (LLMClient, error) {
	return m.client, nil
}

func (m *localMockLLMManager) GetDefaultClient() LLMClient {
	return m.client
}

func (c *localMockLLMClient) Generate(_ context.Context, prompt string) (string, error) {
	switch {
	case strings.Contains(prompt, "请以 JSON 格式返回评分结果"):
		return `{"score": 86, "reason": "mock scoring", "strengths": ["content_complete"], "weaknesses": []}`, nil
	case strings.Contains(prompt, "Return product JSON with fields") || strings.Contains(prompt, "Generate a complete product JSON"):
		return `{"title":"Mock Product","category":["Home","General"],"attributes":{"brand":"MockBrand","color":"Black","material":"Plastic"},"selling_points":["基础信息完整","图片可识别","适合本地联调"],"seo_keywords":["mock product","local validation"],"description":"Mock product generated for local validation flows."}`, nil
	case strings.Contains(prompt, `"dimensions"`) && strings.Contains(prompt, `"technical"`):
		return `{"dimensions":{"length":12,"width":8,"height":4,"unit":"cm"},"weight":{"value":0.3,"unit":"kg"},"package":{"dimensions":{"length":14,"width":10,"height":6,"unit":"cm"},"weight":{"value":0.4,"unit":"kg"},"quantity":1},"technical":{"material":"plastic"}}`, nil
	case strings.Contains(prompt, "Return a JSON array:"):
		return `[{"sku":"MOCK-001","attributes":{"color":"Black"},"price":{"currency":"CNY","amount":29.9,"compare_at":39.9,"cost_price":19.9},"stock":100,"images":[],"is_default":true}]`, nil
	case strings.Contains(prompt, `"product_type"`) && strings.Contains(prompt, `"features"`):
		return `{"product_type":"Mock Product","attributes":{"color":"Black","material":"Plastic","usage":"general"},"features":["基础识别完成","支持本地验证","无需真实 OpenAI"]}`, nil
	case strings.Contains(prompt, `"title": "a concise product title"`):
		return `{"title":"Mock Product","attributes":{"brand":"MockBrand","color":"Black"},"selling_points":["基础识别完成","支持本地验证"]}`, nil
	default:
		return `{"score": 85, "reason": "mock default", "strengths": [], "weaknesses": []}`, nil
	}
}

func (c *localMockLLMClient) AnalyzeImage(_ context.Context, _ string, prompt string) (string, error) {
	if strings.Contains(prompt, "产品图片质量评估专家") {
		return `{"score": 88, "reason": "mock image scoring", "strengths": ["clear"], "weaknesses": []}`, nil
	}
	return `{"color":"black","material":"plastic","scene":"studio","usage":"general"}`, nil
}

func IsMockLLMEnabled(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func ValidateMockLLMManager(mgr LLMManager) error {
	if mgr == nil {
		return fmt.Errorf("llm manager cannot be nil")
	}
	if _, err := mgr.GetClient("default"); err != nil {
		return err
	}
	return nil
}
