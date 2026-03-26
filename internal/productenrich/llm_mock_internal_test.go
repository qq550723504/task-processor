package productenrich

import (
	"context"
	"fmt"
)

type internalMockLLMClient struct {
	response string
	err      error
}

func (m *internalMockLLMClient) Generate(_ context.Context, _ string) (string, error) {
	return m.response, m.err
}

func (m *internalMockLLMClient) AnalyzeImage(_ context.Context, _ string, _ string) (string, error) {
	return m.response, m.err
}

type internalMockLLMManager struct {
	clients map[string]*internalMockLLMClient
	def     *internalMockLLMClient
}

func newMockLLMManager(response string) *internalMockLLMManager {
	c := &internalMockLLMClient{response: response}
	return &internalMockLLMManager{
		clients: map[string]*internalMockLLMClient{"default": c, "fast": c, "vision": c},
		def:     c,
	}
}

func (m *internalMockLLMManager) GetClient(name string) (LLMClient, error) {
	c, ok := m.clients[name]
	if !ok {
		return nil, fmt.Errorf("client %q not found", name)
	}
	return c, nil
}

func (m *internalMockLLMManager) GetDefaultClient() LLMClient {
	return m.def
}
