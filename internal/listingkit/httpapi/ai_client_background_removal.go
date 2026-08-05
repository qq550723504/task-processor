package httpapi

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"strings"

	openaiclient "task-processor/internal/infra/clients/openai"
	"task-processor/internal/listingkit"
)

const studioBackgroundRemovalPrompt = "Remove the image background precisely. Preserve the complete foreground artwork, text, thin lines, holes, and internal white areas. Return a PNG with a real alpha channel and no checkerboard, white, or colored replacement background."

type listingKitBackgroundRemover struct {
	client openaiclient.ImageGenerator
}

func adaptListingKitBackgroundRemover(client openaiclient.ImageGenerator) listingkit.StudioBackgroundRemover {
	if client == nil {
		return nil
	}
	return listingKitBackgroundRemover{client: client}
}

func (r listingKitBackgroundRemover) Remove(ctx context.Context, input []byte, contentType string) (*listingkit.StudioBackgroundRemovalResult, error) {
	if r.client == nil {
		return nil, fmt.Errorf("background removal client is not configured")
	}
	if len(input) == 0 {
		return nil, fmt.Errorf("background removal input is empty")
	}
	response, err := r.client.EditImage(ctx, &openaiclient.ImageEditRequest{
		Prompt:           studioBackgroundRemovalPrompt,
		Image:            input,
		ImageContentType: strings.TrimSpace(contentType),
		ResponseFormat:   "b64_json",
		N:                1,
	})
	if err != nil {
		return nil, err
	}
	if response == nil || len(response.Data) == 0 {
		return nil, fmt.Errorf("background removal returned no image data")
	}
	first := response.Data[0]
	data, outputType, err := loadBackgroundRemovalImage(ctx, first)
	if err != nil {
		return nil, err
	}
	return &listingkit.StudioBackgroundRemovalResult{
		Data:        data,
		ContentType: outputType,
		Model:       strings.TrimSpace(r.client.GetDefaultModel()),
		RequestID:   strings.TrimSpace(response.RequestID),
		RawResponse: strings.TrimSpace(response.RawResponse),
		Usage:       listingkit.AIUsage(response.Usage),
	}, nil
}

func loadBackgroundRemovalImage(ctx context.Context, image openaiclient.ImageData) ([]byte, string, error) {
	if strings.TrimSpace(image.B64JSON) != "" {
		data, err := base64.StdEncoding.DecodeString(image.B64JSON)
		if err != nil {
			return nil, "", fmt.Errorf("decode background removal image: %w", err)
		}
		return data, "image/png", nil
	}
	if strings.TrimSpace(image.URL) == "" {
		return nil, "", fmt.Errorf("background removal image contains neither b64_json nor url")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, image.URL, nil)
	if err != nil {
		return nil, "", err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("download background removal image: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, "", fmt.Errorf("download background removal image returned status %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if err != nil {
		return nil, "", fmt.Errorf("read background removal image: %w", err)
	}
	contentType := strings.TrimSpace(resp.Header.Get("Content-Type"))
	if contentType == "" {
		contentType = "image/png"
	}
	return data, contentType, nil
}
