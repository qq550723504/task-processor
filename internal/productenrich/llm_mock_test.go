package productenrich

import (
	"context"
	"fmt"
)

// mockLLMClient mock LLM 客户端
type mockLLMClient struct {
	response string
	err      error
}

func (m *mockLLMClient) Generate(_ context.Context, _ string) (string, error) {
	return m.response, m.err
}

func (m *mockLLMClient) AnalyzeImage(_ context.Context, _ string, _ string) (string, error) {
	return m.response, m.err
}

// mockLLMManager mock LLM 管理器
type mockLLMManager struct {
	clients map[string]*mockLLMClient
	def     *mockLLMClient
}

func newMockLLMManager(response string) *mockLLMManager {
	c := &mockLLMClient{response: response}
	return &mockLLMManager{
		clients: map[string]*mockLLMClient{"default": c, "fast": c, "vision": c},
		def:     c,
	}
}

func (m *mockLLMManager) GetClient(name string) (LLMClient, error) {
	c, ok := m.clients[name]
	if !ok {
		return nil, fmt.Errorf("client %q not found", name)
	}
	return c, nil
}

func (m *mockLLMManager) GetDefaultClient() LLMClient {
	return m.def
}
