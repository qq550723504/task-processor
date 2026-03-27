package enrich_test

import (
	"context"
	"fmt"

	productenrich "task-processor/internal/productenrich"
)

type mockLLMClient struct {
	response          string
	err               error
	lastAnalyzePrompt string
}

func (m *mockLLMClient) Generate(_ context.Context, _ string) (string, error) {
	return m.response, m.err
}

func (m *mockLLMClient) AnalyzeImage(_ context.Context, _ string, prompt string) (string, error) {
	m.lastAnalyzePrompt = prompt
	return m.response, m.err
}

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

func (m *mockLLMManager) GetClient(name string) (productenrich.LLMClient, error) {
	c, ok := m.clients[name]
	if !ok {
		return nil, fmt.Errorf("client %q not found", name)
	}
	return c, nil
}

func (m *mockLLMManager) GetDefaultClient() productenrich.LLMClient {
	return m.def
}
