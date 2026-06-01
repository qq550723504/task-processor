package listingkit

import (
	"context"
	"encoding/json"
	"strings"
)

type StudioBatchGenerateExecutionInput struct {
	Request   *StudioDesignRequest
	SessionID string
	BatchID   string
	ItemID    string
	AttemptID string
}

type StudioBatchGenerateExecutionOutput struct {
	Response      *StudioDesignResponse
	SessionID     string
	BatchID       string
	ItemID        string
	AttemptID     string
	ResultPayload string
}

func ExecuteStudioDesignBatch(ctx context.Context, service StudioMediaService, input StudioBatchGenerateExecutionInput) (*StudioBatchGenerateExecutionOutput, error) {
	response, err := service.GenerateStudioDesigns(ctx, input.Request)
	if err != nil {
		return nil, err
	}
	payload, err := json.Marshal(response)
	if err != nil {
		return nil, err
	}
	return &StudioBatchGenerateExecutionOutput{
		Response:      response,
		SessionID:     strings.TrimSpace(input.SessionID),
		BatchID:       strings.TrimSpace(input.BatchID),
		ItemID:        strings.TrimSpace(input.ItemID),
		AttemptID:     strings.TrimSpace(input.AttemptID),
		ResultPayload: string(payload),
	}, nil
}
