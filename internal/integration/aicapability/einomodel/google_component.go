package einomodel

import (
	"context"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"task-processor/internal/integration/googleinteractions"
)

// The official SDK owns Interactions serialization. This component is only
// the bounded Eino message seam and has no tool, stream or retry authority.
type googleComponent struct {
	config ComponentConfig
	guard  *GuardedClient
}

func (g *googleComponent) Generate(ctx context.Context, messages []*schema.Message, options ...model.Option) (*schema.Message, error) {
	if len(options) != 0 || len(messages) != 2 || messages[0] == nil || messages[1] == nil ||
		messages[0].Role != schema.System || messages[1].Role != schema.User {
		return nil, ErrInvalid
	}
	for _, m := range messages {
		if m.Content == "" || m.ToolCallID != "" || len(m.ToolCalls) != 0 || len(m.MultiContent) != 0 || len(m.UserInputMultiContent) != 0 ||
			len(m.AssistantGenMultiContent) != 0 || m.ReasoningContent != "" || len(m.Extra) != 0 {
			return nil, ErrInvalid
		}
	}
	result, err := googleinteractions.Generate(ctx, g.guard.Client, g.config.APIKey, messages[0].Content, messages[1].Content, g.config.MaximumOutputTokens)
	if err != nil {
		return nil, ErrOutcomeUnknown
	}
	return &schema.Message{Role: schema.Assistant, Content: result.Content, ResponseMeta: &schema.ResponseMeta{
		FinishReason: result.FinishReason, Usage: &schema.TokenUsage{PromptTokens: result.PromptTokens, CompletionTokens: result.CompletionTokens, TotalTokens: result.TotalTokens}}}, nil
}

func (*googleComponent) Stream(context.Context, []*schema.Message, ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	return nil, ErrInvalid
}

var _ model.BaseChatModel = (*googleComponent)(nil)
