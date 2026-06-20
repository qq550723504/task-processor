package listingkit

import (
	"context"
	"errors"
	"time"

	openaiclient "task-processor/internal/infra/clients/openai"
)

type AIChatCompleter interface {
	CreateChatCompletion(ctx context.Context, req *openaiclient.ChatCompletionRequest) (*openaiclient.ChatCompletionResponse, error)
	Generate(ctx context.Context, prompt string) (string, error)
	AnalyzeImage(ctx context.Context, imageURL string, prompt string) (string, error)
	GetDefaultModel() string
}

type AIImageGenerator interface {
	GenerateImage(ctx context.Context, req *AIImageGenerateRequest) (*AIImageResponse, error)
	EditImage(ctx context.Context, req *AIImageEditRequest) (*AIImageResponse, error)
	GetDefaultModel() string
}

type AIAsyncImageGenerator interface {
	SupportsAsyncImageGeneration() bool
	SubmitImageGeneration(ctx context.Context, req *AIImageGenerateRequest) (*AIImageAsyncSubmit, error)
	QueryImageGeneration(ctx context.Context, jobID string) (*AIImageAsyncResult, error)
}

type AIImageGenerateRequest struct {
	Model          string
	Prompt         string
	Size           string
	ResponseFormat string
	N              int
}

type AIImageEditRequest struct {
	Model          string
	Prompt         string
	ImageURL       string
	ImageURLs      []string
	Size           string
	ResponseFormat string
	N              int
}

type AIImageResponse struct {
	Data          []AIImageData
	Usage         AIUsage
	RequestID     string
	UpstreamJobID string
	RawResponse   string
}

type AIImageData struct {
	URL           string
	B64JSON       string
	RevisedPrompt string
}

type AIUsage struct {
	PromptTokens     int `json:"prompt_tokens,omitempty"`
	CompletionTokens int `json:"completion_tokens,omitempty"`
	TotalTokens      int `json:"total_tokens,omitempty"`
}

type StudioAIUsage = AIUsage

var ErrAsyncImageGenerationNotSupported = errors.New("async image generation is not supported")

type AIImageAsyncSubmit struct {
	JobID             string    `json:"job_id,omitempty"`
	RequestID         string    `json:"request_id,omitempty"`
	Provider          string    `json:"provider,omitempty"`
	RawSubmitResponse string    `json:"raw_submit_response,omitempty"`
	AcceptedAt        time.Time `json:"accepted_at,omitempty"`
}

type AIImageAsyncResultStatus string

const (
	AIImageAsyncResultQueued    AIImageAsyncResultStatus = "queued"
	AIImageAsyncResultRunning   AIImageAsyncResultStatus = "running"
	AIImageAsyncResultSucceeded AIImageAsyncResultStatus = "succeeded"
	AIImageAsyncResultFailed    AIImageAsyncResultStatus = "failed"
)

type AIImageAsyncResult struct {
	JobID             string                   `json:"job_id,omitempty"`
	RequestID         string                   `json:"request_id,omitempty"`
	Provider          string                   `json:"provider,omitempty"`
	Status            AIImageAsyncResultStatus `json:"status,omitempty"`
	RawResultResponse string                   `json:"raw_result_response,omitempty"`
	Error             string                   `json:"error,omitempty"`
	Usage             AIUsage                  `json:"usage,omitempty"`
	Response          *AIImageResponse         `json:"response,omitempty"`
}
