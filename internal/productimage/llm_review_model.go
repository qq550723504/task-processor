package productimage

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"task-processor/internal/pkg/jsonx"
	productenrich "task-processor/internal/productenrich"
	"task-processor/internal/prompt"
)

type llmReviewModel struct {
	llmManager productenrich.LLMManager
}

type llmReviewDecisionPayload struct {
	NeedsReview bool     `json:"needs_review"`
	Reasons     []string `json:"reasons,omitempty"`
	Confidence  float64  `json:"confidence,omitempty"`
}

func NewLLMReviewModel(llmManager productenrich.LLMManager) (ImageReviewModel, error) {
	if llmManager == nil {
		return nil, fmt.Errorf("llm manager cannot be nil")
	}
	return &llmReviewModel{llmManager: llmManager}, nil
}

func (m *llmReviewModel) Review(ctx context.Context, req *ReviewModelRequest) (*ReviewModelResult, error) {
	if req == nil {
		return nil, fmt.Errorf("review model request cannot be nil")
	}
	payload, err := json.Marshal(buildReviewSummary(req))
	if err != nil {
		return nil, fmt.Errorf("marshal review summary: %w", err)
	}
	resolvedPrompt := buildReviewResolvedPrompt(req, string(payload))
	promptText := resolvedPrompt.Text

	response, err := m.generateReviewResponse(ctx, req, promptText)
	if err != nil {
		return nil, err
	}

	var parsed llmReviewDecisionPayload
	if err := json.Unmarshal([]byte(jsonx.CleanLLMResponse(response)), &parsed); err != nil {
		return &ReviewModelResult{
			Decision:   &ReviewDecision{NeedsReview: false},
			Confidence: 0,
		}, nil
	}

	return &ReviewModelResult{
		Decision: &ReviewDecision{
			NeedsReview: parsed.NeedsReview,
			Reasons:     uniqueStrings(parsed.Reasons),
		},
		Confidence: parsed.Confidence,
	}, nil
}

func buildReviewResolvedPrompt(req *ReviewModelRequest, summaryJSON string) resolvedProductImagePrompt {
	fallback := "Review this product image processing result and return JSON only: " +
		`{"needs_review":true|false,"reasons":["..."],"confidence":0.0}` +
		"\n\nSummary:\n" + summaryJSON
	resolved := resolveProductImagePrompt("", prompt.KProductImageReviewDefault, map[string]any{
		"product_type": productTypeFromReviewRequest(req),
		"title":        titleFromReviewRequest(req),
		"summary_json": summaryJSON,
	}, fallback)
	if strings.TrimSpace(resolved.Text) == "" {
		resolved.Text = fallback
	}
	return resolved
}

func productTypeFromReviewRequest(req *ReviewModelRequest) string {
	if req == nil || req.Context == nil {
		return ""
	}
	return strings.TrimSpace(req.Context.ProductType)
}

func titleFromReviewRequest(req *ReviewModelRequest) string {
	if req == nil || req.Context == nil {
		return ""
	}
	return strings.TrimSpace(req.Context.Title)
}

func (m *llmReviewModel) generateReviewResponse(ctx context.Context, req *ReviewModelRequest, prompt string) (string, error) {
	if mainURL := reviewableAssetURL(req); mainURL != "" {
		visionClient, err := m.llmManager.GetClient("vision")
		if err == nil && visionClient != nil {
			return visionClient.AnalyzeImage(ctx, mainURL, prompt)
		}
	}
	defaultClient, err := m.llmManager.GetClient("default")
	if err != nil || defaultClient == nil {
		return "", fmt.Errorf("failed to get default review client: %w", err)
	}
	return defaultClient.Generate(ctx, prompt)
}

func buildReviewSummary(req *ReviewModelRequest) map[string]any {
	summary := map[string]any{}
	if req.Context != nil {
		summary["product_type"] = req.Context.ProductType
		summary["title"] = req.Context.Title
	}
	if req.Result != nil {
		if req.Result.Quality != nil {
			summary["quality"] = req.Result.Quality
		}
		if req.Result.Compliance != nil {
			summary["compliance"] = req.Result.Compliance
		}
		if req.Result.IPRisk != nil {
			summary["ip_risk"] = req.Result.IPRisk
		}
		if req.Result.MainImage != nil {
			summary["main_image"] = map[string]any{
				"url":        req.Result.MainImage.URL,
				"source_url": req.Result.MainImage.SourceURL,
				"metadata":   req.Result.MainImage.Metadata,
			}
		}
		if req.Result.WhiteBgImage != nil {
			summary["white_bg_image"] = map[string]any{
				"url":      req.Result.WhiteBgImage.URL,
				"metadata": req.Result.WhiteBgImage.Metadata,
			}
		}
		summary["gallery_count"] = len(req.Result.GalleryImages)
	}
	return summary
}

func reviewableAssetURL(req *ReviewModelRequest) string {
	if req == nil || req.Result == nil || req.Result.MainImage == nil {
		return ""
	}
	url := strings.TrimSpace(req.Result.MainImage.URL)
	if strings.HasPrefix(strings.ToLower(url), "http://") || strings.HasPrefix(strings.ToLower(url), "https://") {
		return url
	}
	return ""
}
