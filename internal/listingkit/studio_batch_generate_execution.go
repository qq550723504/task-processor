package listingkit

import (
	"context"
	"strings"
)

type StudioBatchGenerateExecutionInput struct {
	Request   *StudioDesignRequest
	SessionID string
}

type StudioBatchGenerateExecutionOutput struct {
	Response  *StudioDesignResponse
	SessionID string
}

func ExecuteStudioDesignBatch(ctx context.Context, service StudioMediaService, input StudioBatchGenerateExecutionInput) (*StudioBatchGenerateExecutionOutput, error) {
	response, err := service.GenerateStudioDesigns(ctx, input.Request)
	if err != nil {
		return nil, err
	}
	return &StudioBatchGenerateExecutionOutput{
		Response:  response,
		SessionID: strings.TrimSpace(input.SessionID),
	}, nil
}
