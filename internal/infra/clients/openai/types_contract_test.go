package openai

import (
	"testing"

	"task-processor/internal/ai"
)

func TestOpenAIClientImplementsProviderNeutralContracts(t *testing.T) {
	var _ ai.ChatCompleter = (*Client)(nil)
	var _ ai.ImageGenerator = (*Client)(nil)
	if ErrAsyncImageGenerationNotSupported != ai.ErrAsyncImageGenerationNotSupported {
		t.Fatal("OpenAI compatibility sentinel must alias the provider-neutral sentinel")
	}
}
