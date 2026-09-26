package grsai

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"task-processor/internal/ai"
)

// GenerationObservation is a provider effect fact, not an image-validity or
// approval decision. It deliberately carries no provider URL or raw response.
type GenerationObservation struct {
	ResponseID   string
	RequestID    string
	ResultDigest string
	UsageKnown   bool
	Usage        ai.Usage
}

type GenerationObserver func(context.Context, GenerationObservation) error

// NewSynchronousProductImageAdapter is the only assembly for the fixed
// source-only GPT-image consumer. Unlike the ordinary adapter, its pinned
// client cannot fall back to the polling/retrying EditImage implementation.
func NewSynchronousProductImageAdapter(config ProductImageAdapterConfig, observe GenerationObserver) (ProductImageProvider, error) {
	client, ok := config.Client.(*Client)
	if !ok || client == nil || observe == nil || config.ImageModel != "gpt-image-2.5" {
		return nil, errors.New("synchronous product image dependencies unavailable")
	}
	config.Client = &synchronousProductImageClient{ImageGenerator: client, client: client, observe: observe}
	return NewProductImageAdapter(config)
}

type synchronousProductImageClient struct {
	ai.ImageGenerator
	client  *Client
	observe GenerationObserver
}

func (c *synchronousProductImageClient) EditImage(ctx context.Context, request *ai.ImageEditRequest) (*ai.ImageResponse, error) {
	return c.client.EditImageOnce(ctx, request, c.observe)
}

// EditImageOnce is the explicit synchronous GPT-image protocol. Its observer
// must persist the provider effect before any output materialization.
func (c *Client) EditImageOnce(ctx context.Context, req *ai.ImageEditRequest, observe GenerationObserver) (*ai.ImageResponse, error) {
	if c == nil || ctx == nil || req == nil || observe == nil || req.Model != "gpt-image-2.5" ||
		c.cfg.Model != req.Model || strings.TrimSpace(req.Prompt) == "" || len(req.Image) == 0 ||
		int64(len(req.Image)) > maxImageReferenceBytes || req.ImageURL != "" || len(req.ImageURLs) != 0 ||
		len(req.Mask) != 0 || (req.N != 0 && req.N != 1) ||
		(req.Size != "" && req.Size != "auto" && req.Size != "1024x1024") ||
		(req.Quality != "" && req.Quality != "auto") {
		return nil, errors.New("synchronous image edit input is unavailable")
	}
	if c.cfg.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.cfg.Timeout)
		defer cancel()
	}
	// Reuse bounded exact-byte materialization; remote URL inputs were rejected
	// above, so this path never re-fetches a mutable source reference.
	releaseBudget, err := c.referenceMaterialization.acquire(ctx, int64(base64EncodedSize(len(req.Image))))
	if err != nil {
		return nil, errors.New("synchronous image reference budget unavailable")
	}
	defer releaseBudget()
	images, release, err := c.imageInputsForRequest(ctx, req)
	if err != nil {
		return nil, errors.New("synchronous image reference unavailable")
	}
	defer release()
	submitURL, err := buildSubmitURL(c.cfg.SubmitURL, req.Model)
	if err != nil {
		return nil, errors.New("synchronous image route unavailable")
	}
	local := *c
	local.disableSubmitReplay = true
	payload, err := local.submitGenerationRequestNoPoll(ctx, submitURL, submitRequest{
		Model: req.Model, Prompt: req.Prompt, Images: images, AspectRatio: "1024x1024",
		Quality: "auto", ReplyType: "json", UnifiedSync: true,
	})
	if err != nil {
		return nil, errors.New("synchronous image generation outcome unknown")
	}
	// Running is deliberately not polled: this fixed owner has no accepted-job
	// recovery protocol. A job id alone never authorizes another submission.
	if payload.Status != "succeeded" || payload.ID == "" || len(payload.ID) > 192 || len(payload.RequestID) > 192 {
		return nil, errors.New("synchronous image generation outcome unknown")
	}
	encoded, err := json.Marshal(payload.Results)
	if err != nil {
		return nil, errors.New("synchronous image result unavailable")
	}
	digest := sha256.Sum256(encoded)
	observation := GenerationObservation{ResponseID: payload.ID, RequestID: payload.RequestID, ResultDigest: hex.EncodeToString(digest[:])}
	// The generation may be complete even if the caller's request was cancelled
	// just after the response. Keep only this bounded finalization alive.
	finalization, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancel()
	if err := observe(finalization, observation); err != nil {
		return nil, errors.New("synchronous image outcome persistence unavailable")
	}
	response := &ai.ImageResponse{RequestID: payload.RequestID, UpstreamJobID: payload.ID}
	for _, item := range payload.Results {
		response.Data = append(response.Data, ai.ImageData{URL: item.URL})
	}
	// No download here. The existing ProductImageAdapter owns public-URL SSRF,
	// bounded fetching, decoding and candidate validation after this callback.
	return response, nil
}

func base64EncodedSize(size int) int { return (size + 2) / 3 * 4 }
