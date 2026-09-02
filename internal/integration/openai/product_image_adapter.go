package openai

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"strings"

	"task-processor/internal/ai"
	"task-processor/internal/integration/httpimage"
	productimage "task-processor/internal/product/image"
)

const maxProductImagePromptBytes = 64 << 10

type ProductImagePrompts struct {
	Subject         string
	WhiteBackground string
	Scene           string
	Review          string
	Version         string
}

type ProductImageAdapterConfig struct {
	ImageClient  ai.ImageGenerator
	ReviewClient ai.ChatCompleter
	Prompts      ProductImagePrompts

	Provider             string
	ImageModel           string
	ReviewModel          string
	RouteReference       string
	CredentialReference  string
	ConfigurationVersion string
	PricingVersion       string

	MaximumSceneOutputs      int
	ImageCostMicrosPerOutput int64
	ReviewCostMicros         int64
	CostUpperBoundKnown      bool

	GeneratedImageFetcher func(context.Context, string) ([]byte, error)
}

type ProductImageAdapter struct {
	config ProductImageAdapterConfig
}

type routeBoundProductImageGenerator interface {
	EditImageWithRoute(context.Context, *ai.ImageEditRequest, ImageRouteSelection) (*ai.ImageResponse, error)
}

func NewProductImageAdapter(config ProductImageAdapterConfig) (*ProductImageAdapter, error) {
	if nilInterface(config.ImageClient) || nilInterface(config.ReviewClient) {
		return nil, fmt.Errorf("openai product image clients are required")
	}
	if _, ok := config.ImageClient.(routeBoundProductImageGenerator); !ok {
		return nil, fmt.Errorf("openai product image client must support exact route pinning")
	}
	for name, value := range map[string]string{
		"provider": config.Provider, "image model": config.ImageModel, "review model": config.ReviewModel,
		"route reference": config.RouteReference, "credential reference": config.CredentialReference,
		"configuration version": config.ConfigurationVersion, "pricing version": config.PricingVersion,
		"subject prompt": config.Prompts.Subject, "white background prompt": config.Prompts.WhiteBackground,
		"scene prompt": config.Prompts.Scene, "review prompt": config.Prompts.Review, "prompt version": config.Prompts.Version,
	} {
		if !canonicalProductImageString(value) {
			return nil, fmt.Errorf("openai product image %s is required and must be canonical", name)
		}
	}
	if config.MaximumSceneOutputs < 1 || config.MaximumSceneOutputs > 16 || config.ImageCostMicrosPerOutput < 0 || config.ReviewCostMicros < 0 {
		return nil, fmt.Errorf("openai product image limits are invalid")
	}
	if config.GeneratedImageFetcher == nil {
		config.GeneratedImageFetcher = func(ctx context.Context, rawURL string) ([]byte, error) {
			return httpimage.Download(ctx, httpimage.NewPublicImageHTTPClient(), rawURL, productimage.MaxInlineArtifactBytes)
		}
	}
	return &ProductImageAdapter{config: config}, nil
}

func (a *ProductImageAdapter) Extract(ctx context.Context, request productimage.ExtractRequest) (productimage.Candidate, error) {
	return a.editOne(ctx, request.Source, request.Source, request.Product, productimage.RoleSubject, "extract_subject", a.config.Prompts.Subject)
}

func (a *ProductImageAdapter) RenderWhiteBackground(ctx context.Context, request productimage.RenderRequest) (productimage.Candidate, error) {
	return a.editOne(ctx, request.Subject.Asset, request.Source, request.Product, productimage.RoleWhiteBackground, "render_white_background", a.config.Prompts.WhiteBackground)
}

func (a *ProductImageAdapter) RenderScene(ctx context.Context, request productimage.SceneRequest) ([]productimage.Candidate, error) {
	if request.MaximumOutputs < 1 || request.MaximumOutputs > a.config.MaximumSceneOutputs {
		return nil, productimage.ErrCapabilityUnsupported
	}
	styleURLs := make([]string, len(request.StyleReferences))
	for index, style := range request.StyleReferences {
		if style.URL == "" {
			return nil, productimage.ErrCapabilityUnsupported
		}
		styleURLs[index] = style.URL
	}
	prompt, err := productImagePrompt(a.config.Prompts.Scene, request.Product, struct {
		ProfileName string                    `json:"profile_name,omitempty"`
		Options     productimage.SceneOptions `json:"options"`
	}{ProfileName: request.ProfileName, Options: request.Options})
	if err != nil {
		return nil, err
	}
	response, err := a.editImage(ctx, &ai.ImageEditRequest{
		Model: a.config.ImageModel, Prompt: prompt, ImageURL: request.Source.URL, ImageContentType: request.Source.MediaType,
		ImageURLs: styleURLs, ResponseFormat: "b64_json", N: request.MaximumOutputs, Size: "auto",
	})
	if err != nil {
		return nil, err
	}
	return a.imageCandidates(ctx, response, request.Source, productimage.RoleScene, "render_scene", request.MaximumOutputs)
}

func (a *ProductImageAdapter) Review(ctx context.Context, request productimage.ReviewRequest) (productimage.Review, error) {
	summary, err := json.Marshal(struct {
		Product        productImagePromptContext `json:"product"`
		SourceCount    int                       `json:"source_count"`
		CandidateCount int                       `json:"candidate_count"`
	}{Product: newProductImagePromptContext(request.Product), SourceCount: len(request.Sources), CandidateCount: len(request.Candidates)})
	if err != nil {
		return productimage.Review{}, productimage.ErrInputInvalid
	}
	parts := []ai.ChatCompletionContentPart{{Type: "text", Text: a.config.Prompts.Review + "\nContext: " + string(summary) + `\nReturn only {"score":0.0,"needs_human_review":false,"reasons":[]}.`}}
	for _, source := range request.Sources {
		parts = append(parts, imageContentPart(source))
	}
	for _, candidate := range request.Candidates {
		parts = append(parts, imageContentPart(candidate.Asset))
	}
	temperature := float32(0)
	response, err := a.config.ReviewClient.CreateChatCompletion(ctx, &ai.ChatCompletionRequest{
		Model: a.config.ReviewModel, Temperature: &temperature, ResponseFormat: "json_object",
		Messages: []ai.ChatCompletionMessage{{Role: "user", MultiContent: parts}},
	})
	if err != nil {
		return productimage.Review{}, err
	}
	if response == nil || len(response.Choices) != 1 {
		return productimage.Review{}, productimage.ErrOutputValidation
	}
	var payload struct {
		Score            float64  `json:"score"`
		NeedsHumanReview bool     `json:"needs_human_review"`
		Reasons          []string `json:"reasons"`
	}
	decoder := json.NewDecoder(strings.NewReader(response.Choices[0].Message.Content))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&payload); err != nil {
		return productimage.Review{}, productimage.ErrOutputValidation
	}
	var additional any
	if err := decoder.Decode(&additional); err != io.EOF {
		return productimage.Review{}, productimage.ErrOutputValidation
	}
	return productimage.Review{Score: payload.Score, NeedsHumanReview: payload.NeedsHumanReview, Reasons: payload.Reasons}, nil
}

func (a *ProductImageAdapter) QuoteUsage(_ context.Context, request productimage.UsageQuoteRequest) (productimage.UsageQuote, error) {
	if !canonicalProductImageString(request.Operation) || !canonicalProductImageString(request.InputFingerprint) {
		return productimage.UsageQuote{}, productimage.ErrInputInvalid
	}
	maximumOutputs := request.MaximumOutputs
	model := a.config.ImageModel
	costPerOutput := a.config.ImageCostMicrosPerOutput
	switch request.Operation {
	case "extract_subject", "render_white_background":
		maximumOutputs = 1
	case "render_scene":
		if maximumOutputs < 1 || maximumOutputs > int64(a.config.MaximumSceneOutputs) {
			return productimage.UsageQuote{}, productimage.ErrCapabilityUnsupported
		}
	case "review":
		maximumOutputs = 1
		model = a.config.ReviewModel
		costPerOutput = a.config.ReviewCostMicros
	default:
		return productimage.UsageQuote{}, productimage.ErrCapabilityUnsupported
	}
	if costPerOutput != 0 && maximumOutputs > (1<<63-1)/costPerOutput {
		return productimage.UsageQuote{}, productimage.ErrInputInvalid
	}
	maximumCost := costPerOutput * maximumOutputs
	fingerprintPayload := struct {
		Operation, InputFingerprint, Provider, Model, RouteReference, CredentialReference, ConfigurationVersion, PricingVersion string
		MaximumOutputs, MaximumCost                                                                                             int64
	}{
		request.Operation, request.InputFingerprint, a.config.Provider, model, a.config.RouteReference,
		a.config.CredentialReference, a.config.ConfigurationVersion, a.config.PricingVersion, maximumOutputs, maximumCost,
	}
	encoded, err := json.Marshal(fingerprintPayload)
	if err != nil {
		return productimage.UsageQuote{}, productimage.ErrInputInvalid
	}
	digest := sha256.Sum256(encoded)
	return productimage.UsageQuote{
		Operation: request.Operation, Provider: a.config.Provider, RouteReference: a.config.RouteReference, Model: model,
		CredentialReference: a.config.CredentialReference, ConfigurationVersion: a.config.ConfigurationVersion,
		PricingVersion: a.config.PricingVersion, Fingerprint: hex.EncodeToString(digest[:]),
		MaximumOutputs: maximumOutputs, MaximumModelCalls: 1, MaximumCostMicros: maximumCost,
		CostUpperBoundKnown: a.config.CostUpperBoundKnown,
	}, nil
}

func (a *ProductImageAdapter) editOne(ctx context.Context, input, source productimage.Asset, product productimage.ProductContext, role productimage.Role, operation, basePrompt string) (productimage.Candidate, error) {
	prompt, err := productImagePrompt(basePrompt, product, struct {
		Operation string `json:"operation"`
	}{Operation: operation})
	if err != nil {
		return productimage.Candidate{}, err
	}
	response, err := a.editImage(ctx, &ai.ImageEditRequest{
		Model: a.config.ImageModel, Prompt: prompt, Image: append([]byte(nil), input.Bytes...),
		ImageURL: input.URL, ImageContentType: input.MediaType,
		ResponseFormat: "b64_json", N: 1, Size: "auto",
	})
	if err != nil {
		return productimage.Candidate{}, err
	}
	candidates, err := a.imageCandidates(ctx, response, source, role, operation, 1)
	if err != nil {
		return productimage.Candidate{}, err
	}
	if len(candidates) != 1 {
		return productimage.Candidate{}, productimage.ErrOutputValidation
	}
	return candidates[0], nil
}

func (a *ProductImageAdapter) editImage(ctx context.Context, request *ai.ImageEditRequest) (*ai.ImageResponse, error) {
	client := a.config.ImageClient.(routeBoundProductImageGenerator)
	return client.EditImageWithRoute(ctx, request, ImageRouteSelection{
		CredentialReference: a.config.CredentialReference, ConfigurationVersion: a.config.ConfigurationVersion,
	})
}

func (a *ProductImageAdapter) imageCandidates(ctx context.Context, response *ai.ImageResponse, source productimage.Asset, role productimage.Role, operation string, maximum int) ([]productimage.Candidate, error) {
	if response == nil || len(response.Data) == 0 || len(response.Data) > maximum {
		return nil, productimage.ErrOutputValidation
	}
	candidates := make([]productimage.Candidate, len(response.Data))
	usedArtifactBytes := 0
	for index, item := range response.Data {
		content, err := a.generatedImageBytes(ctx, item)
		if err != nil {
			return nil, err
		}
		usedArtifactBytes, err = consumeProductImageArtifactBudget(usedArtifactBytes, len(content))
		if err != nil {
			return nil, err
		}
		mediaType, width, height, err := httpimage.InspectGeneratedArtifact(content)
		if err != nil {
			return nil, err
		}
		candidates[index] = productimage.Candidate{
			Asset: productimage.Asset{
				Bytes: content, MediaType: mediaType, SourceURL: source.SourceURL, SourceAssetID: source.SourceAssetID,
				Role: role, Width: width, Height: height, Operations: []string{operation},
			},
			Metadata: productimage.GenerationMetadata{
				Capability: operation, ModelFamily: a.config.ImageModel, InvocationID: response.RequestID,
				PromptReference: operation, PromptVersion: a.config.Prompts.Version,
				Values: map[string]string{"provider": a.config.Provider, "configuration_version": a.config.ConfigurationVersion},
			},
		}
	}
	return candidates, nil
}

func consumeProductImageArtifactBudget(used, size int) (int, error) {
	if used < 0 || size <= 0 || size > productimage.MaxInlineArtifactBytes || size > productimage.MaxInlineArtifactAggregateBytes-used {
		return 0, productimage.ErrOutputValidation
	}
	return used + size, nil
}

func (a *ProductImageAdapter) generatedImageBytes(ctx context.Context, item ai.ImageData) ([]byte, error) {
	encoded := strings.TrimSpace(item.B64JSON)
	if encoded != "" {
		if base64.StdEncoding.DecodedLen(len(encoded)) > productimage.MaxInlineArtifactBytes {
			return nil, productimage.ErrOutputValidation
		}
		decoded, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil || len(decoded) == 0 || len(decoded) > productimage.MaxInlineArtifactBytes {
			return nil, productimage.ErrOutputValidation
		}
		return decoded, nil
	}
	if item.URL == "" || strings.TrimSpace(item.URL) != item.URL {
		return nil, productimage.ErrOutputValidation
	}
	downloaded, err := a.config.GeneratedImageFetcher(ctx, item.URL)
	if err != nil {
		return nil, err
	}
	if len(downloaded) == 0 || len(downloaded) > productimage.MaxInlineArtifactBytes {
		return nil, productimage.ErrOutputValidation
	}
	return append([]byte(nil), downloaded...), nil
}

func productImagePrompt(base string, product productimage.ProductContext, details any) (string, error) {
	payload, err := json.Marshal(struct {
		Product productImagePromptContext `json:"product"`
		Details any                       `json:"details"`
	}{Product: newProductImagePromptContext(product), Details: details})
	if err != nil {
		return "", productimage.ErrInputInvalid
	}
	prompt := base + "\nControlled context: " + string(payload)
	if len(prompt) > maxProductImagePromptBytes {
		return "", productimage.ErrInputInvalid
	}
	return prompt, nil
}

type productImagePromptContext struct {
	ProductKey  string            `json:"product_key"`
	Title       string            `json:"title,omitempty"`
	ProductType string            `json:"product_type,omitempty"`
	Attributes  map[string]string `json:"attributes,omitempty"`
}

func newProductImagePromptContext(product productimage.ProductContext) productImagePromptContext {
	return productImagePromptContext{
		ProductKey: product.ProductKey, Title: product.Title, ProductType: product.ProductType, Attributes: product.Attributes,
	}
}

func imageContentPart(asset productimage.Asset) ai.ChatCompletionContentPart {
	imageURL := asset.URL
	if len(asset.Bytes) > 0 {
		imageURL = "data:" + asset.MediaType + ";base64," + base64.StdEncoding.EncodeToString(asset.Bytes)
	}
	return ai.ChatCompletionContentPart{Type: "image_url", ImageURL: &ai.ChatCompletionContentPartImage{URL: imageURL, Detail: "high"}}
}

func canonicalProductImageString(value string) bool {
	return value != "" && strings.TrimSpace(value) == value && len(value) <= maxProductImagePromptBytes
}

func nilInterface(value any) bool {
	if value == nil {
		return true
	}
	reflected := reflect.ValueOf(value)
	switch reflected.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return reflected.IsNil()
	default:
		return false
	}
}

var (
	_ productimage.SubjectExtractor        = (*ProductImageAdapter)(nil)
	_ productimage.WhiteBackgroundRenderer = (*ProductImageAdapter)(nil)
	_ productimage.SceneRenderer           = (*ProductImageAdapter)(nil)
	_ productimage.Reviewer                = (*ProductImageAdapter)(nil)
	_ productimage.UsageQuoter             = (*ProductImageAdapter)(nil)
)
