// Package productenrich 提供 LLM 基础设施适配器。
// 将 infra/clients/openai 的 Manager/Client 适配为本包定义的 LLMManager/LLMClient 接口。
package productenrich

import (
	"context"
	"fmt"

	"task-processor/internal/core/config"
	"task-processor/internal/infra/clients/openai"
)

// llmClientAdapter 将 openai.ChatCompleter 适配为 LLMClient 接口。
type llmClientAdapter struct {
	client openai.ChatCompleter
}

func (a *llmClientAdapter) Generate(ctx context.Context, prompt string) (string, error) {
	return a.client.Generate(ctx, prompt)
}

func (a *llmClientAdapter) AnalyzeImage(ctx context.Context, imageURL string, prompt string) (string, error) {
	return a.client.AnalyzeImage(ctx, imageURL, prompt)
}

// llmManagerAdapter 将 openai.Manager 适配为 LLMManager 接口。
type llmManagerAdapter struct {
	manager *openai.Manager
}

func (a *llmManagerAdapter) GetClient(clientName string) (LLMClient, error) {
	c, err := a.manager.GetClient(clientName)
	if err != nil {
		return nil, err
	}
	return &llmClientAdapter{client: c}, nil
}

func (a *llmManagerAdapter) GetDefaultClient() LLMClient {
	return &llmClientAdapter{client: a.manager.GetDefaultClient()}
}

// NewLLMManagerAdapter 根据 OpenAIConfig 创建满足 LLMManager 接口的适配器。
// 供 cmd 层在组装依赖时使用。
func NewLLMManagerAdapter(cfg config.OpenAIConfig) (LLMManager, error) {
	mgr, err := openai.NewManager(&openai.ManagerConfig{
		Clients:       cfg.ToClientConfigs(),
		DefaultClient: "default",
	})
	if err != nil {
		return nil, fmt.Errorf("创建 OpenAI Manager 失败: %w", err)
	}
	return &llmManagerAdapter{manager: mgr}, nil
}
