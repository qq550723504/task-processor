package httpimage

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	productimage "task-processor/internal/product/image"

	_ "golang.org/x/image/webp"
)

const maxProductImageHTTPResponseBytes = (productimage.MaxInlineArtifactAggregateBytes*4+2)/3 + (1 << 20)

type ProductImageEndpointConfig struct {
	Endpoint             string
	BearerToken          string
	Provider             string
	Model                string
	RouteReference       string
	CredentialReference  string
	ConfigurationVersion string
	PricingVersion       string
	CostMicrosPerOutput  int64
	CostUpperBoundKnown  bool
}

type ProductImageAdapterConfig struct {
	Subject         *ProductImageEndpointConfig
	WhiteBackground *ProductImageEndpointConfig
	Scene           *ProductImageEndpointConfig

	MaximumSceneOutputs int
	HTTPClient          *http.Client
	SourceFetcher       func(context.Context, string) ([]byte, error)
}

type ProductImageAdapter struct {
	subject             *ProductImageEndpointConfig
	whiteBackground     *ProductImageEndpointConfig
	scene               *ProductImageEndpointConfig
	maximumSceneOutputs int
	httpClient          *http.Client
	sourceFetcher       func(context.Context, string) ([]byte, error)
}

func NewProductImageAdapter(config ProductImageAdapterConfig) (*ProductImageAdapter, error) {
	if config.Subject == nil && config.WhiteBackground == nil && config.Scene == nil {
		return nil, fmt.Errorf("http product image adapter requires at least one endpoint")
	}
	for name, endpoint := range map[string]*ProductImageEndpointConfig{
		"subject": config.Subject, "white background": config.WhiteBackground, "scene": config.Scene,
	} {
		if endpoint == nil {
			continue
		}
		if err := validateProductImageEndpoint(*endpoint); err != nil {
			return nil, fmt.Errorf("invalid %s endpoint: %w", name, err)
		}
	}
	if config.Scene != nil && (config.MaximumSceneOutputs < 1 || config.MaximumSceneOutputs > 16) {
		return nil, fmt.Errorf("http product image maximum scene outputs is invalid")
	}
	client := config.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 60 * time.Second}
	}
	fetcher := config.SourceFetcher
	if fetcher == nil {
		fetcher = func(ctx context.Context, rawURL string) ([]byte, error) {
			return Download(ctx, NewPublicImageHTTPClient(), rawURL, productimage.MaxInlineArtifactBytes)
		}
	}
	return &ProductImageAdapter{
		subject: cloneProductImageEndpoint(config.Subject), whiteBackground: cloneProductImageEndpoint(config.WhiteBackground),
		scene: cloneProductImageEndpoint(config.Scene), maximumSceneOutputs: config.MaximumSceneOutputs,
		httpClient: client, sourceFetcher: fetcher,
	}, nil
}

func (a *ProductImageAdapter) Extract(ctx context.Context, request productimage.ExtractRequest) (productimage.Candidate, error) {
	if a == nil || a.subject == nil {
		return productimage.Candidate{}, productimage.ErrCapabilityUnsupported
	}
	response, err := a.invoke(ctx, *a.subject, productImageHTTPRequest{
		Task: "subject_extract", SourceURL: request.Source.URL, Product: newProductImageHTTPProduct(request.Product),
	}, request.Source)
	if err != nil {
		return productimage.Candidate{}, err
	}
	return singleProductImageCandidate(response, request.Source, productimage.RoleSubject, "extract_subject", *a.subject)
}

func (a *ProductImageAdapter) RenderWhiteBackground(ctx context.Context, request productimage.RenderRequest) (productimage.Candidate, error) {
	if a == nil || a.whiteBackground == nil {
		return productimage.Candidate{}, productimage.ErrCapabilityUnsupported
	}
	response, err := a.invoke(ctx, *a.whiteBackground, productImageHTTPRequest{
		Task: "white_background", SourceURL: request.Source.URL, Product: newProductImageHTTPProduct(request.Product),
	}, request.Source)
	if err != nil {
		return productimage.Candidate{}, err
	}
	return singleProductImageCandidate(response, request.Source, productimage.RoleWhiteBackground, "render_white_background", *a.whiteBackground)
}

func (a *ProductImageAdapter) RenderScene(ctx context.Context, request productimage.SceneRequest) ([]productimage.Candidate, error) {
	if a == nil || a.scene == nil {
		return nil, productimage.ErrCapabilityUnsupported
	}
	if request.MaximumOutputs < 1 || request.MaximumOutputs > a.maximumSceneOutputs {
		return nil, productimage.ErrCapabilityUnsupported
	}
	styleURLs := make([]string, len(request.StyleReferences))
	for index, style := range request.StyleReferences {
		if style.URL == "" {
			return nil, productimage.ErrCapabilityUnsupported
		}
		styleURLs[index] = style.URL
	}
	response, err := a.invoke(ctx, *a.scene, productImageHTTPRequest{
		Task: "scene", SourceURL: request.Source.URL, Product: newProductImageHTTPProduct(request.Product),
		SceneOptions: newProductImageHTTPSceneOptions(request.Options), ProfileName: request.ProfileName,
		StyleReferenceURLs: styleURLs, MaximumOutputs: request.MaximumOutputs,
	}, request.Source)
	if err != nil {
		return nil, err
	}
	items := response.Images
	if len(items) == 0 && response.ImageBase64 != "" {
		items = []productImageHTTPResponseImage{{ImageBase64: response.ImageBase64, Format: response.Format}}
	}
	if len(items) == 0 || len(items) > request.MaximumOutputs {
		return nil, productimage.ErrOutputValidation
	}
	candidates := make([]productimage.Candidate, len(items))
	used := 0
	for index, item := range items {
		content, mediaType, width, height, err := decodeProductImageHTTPItem(item, &used)
		if err != nil {
			return nil, err
		}
		candidates[index] = productImageHTTPCandidate(content, mediaType, width, height, request.Source, productimage.RoleScene, "render_scene", *a.scene)
	}
	return candidates, nil
}

func (a *ProductImageAdapter) QuoteUsage(_ context.Context, request productimage.UsageQuoteRequest) (productimage.UsageQuote, error) {
	if a == nil || !canonicalProductImageHTTPString(request.InputFingerprint) {
		return productimage.UsageQuote{}, productimage.ErrInputInvalid
	}
	var endpoint *ProductImageEndpointConfig
	maximumOutputs := request.MaximumOutputs
	switch request.Operation {
	case "extract_subject":
		endpoint, maximumOutputs = a.subject, 1
	case "render_white_background":
		endpoint, maximumOutputs = a.whiteBackground, 1
	case "render_scene":
		endpoint = a.scene
		if maximumOutputs < 1 || maximumOutputs > int64(a.maximumSceneOutputs) {
			return productimage.UsageQuote{}, productimage.ErrCapabilityUnsupported
		}
	default:
		return productimage.UsageQuote{}, productimage.ErrCapabilityUnsupported
	}
	if endpoint == nil {
		return productimage.UsageQuote{}, productimage.ErrCapabilityUnsupported
	}
	if endpoint.CostMicrosPerOutput != 0 && maximumOutputs > (1<<63-1)/endpoint.CostMicrosPerOutput {
		return productimage.UsageQuote{}, productimage.ErrInputInvalid
	}
	maximumCost := endpoint.CostMicrosPerOutput * maximumOutputs
	encoded, err := json.Marshal(struct {
		Operation, InputFingerprint, Provider, Model, RouteReference, CredentialReference, ConfigurationVersion, PricingVersion string
		MaximumOutputs, MaximumCost                                                                                             int64
	}{
		request.Operation, request.InputFingerprint, endpoint.Provider, endpoint.Model, endpoint.RouteReference,
		endpoint.CredentialReference, endpoint.ConfigurationVersion, endpoint.PricingVersion, maximumOutputs, maximumCost,
	})
	if err != nil {
		return productimage.UsageQuote{}, productimage.ErrInputInvalid
	}
	digest := sha256.Sum256(encoded)
	return productimage.UsageQuote{
		Operation: request.Operation, RouteReference: endpoint.RouteReference, Model: endpoint.Model,
		CredentialReference: endpoint.CredentialReference, ConfigurationVersion: endpoint.ConfigurationVersion,
		PricingVersion: endpoint.PricingVersion, Fingerprint: hex.EncodeToString(digest[:]),
		MaximumOutputs: maximumOutputs, MaximumModelCalls: 1, MaximumCostMicros: maximumCost,
		CostUpperBoundKnown: endpoint.CostUpperBoundKnown,
	}, nil
}

func (a *ProductImageAdapter) invoke(ctx context.Context, endpoint ProductImageEndpointConfig, payload productImageHTTPRequest, source productimage.Asset) (productImageHTTPResponse, error) {
	content, err := a.sourceFetcher(ctx, source.URL)
	if err != nil {
		return productImageHTTPResponse{}, err
	}
	if len(content) == 0 || len(content) > productimage.MaxInlineArtifactBytes {
		return productImageHTTPResponse{}, productimage.ErrInputInvalid
	}
	payload.ImageBase64 = base64.StdEncoding.EncodeToString(content)
	body, err := json.Marshal(payload)
	if err != nil {
		return productImageHTTPResponse{}, productimage.ErrInputInvalid
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint.Endpoint, bytes.NewReader(body))
	if err != nil {
		return productImageHTTPResponse{}, err
	}
	request.Header.Set("Content-Type", "application/json")
	if endpoint.BearerToken != "" {
		request.Header.Set("Authorization", "Bearer "+endpoint.BearerToken)
	}
	response, err := a.httpClient.Do(request)
	if err != nil {
		return productImageHTTPResponse{}, err
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return productImageHTTPResponse{}, fmt.Errorf("http product image service returned status %d", response.StatusCode)
	}
	if response.ContentLength > maxProductImageHTTPResponseBytes {
		return productImageHTTPResponse{}, productimage.ErrOutputValidation
	}
	raw, err := io.ReadAll(io.LimitReader(response.Body, maxProductImageHTTPResponseBytes+1))
	if err != nil || len(raw) == 0 || len(raw) > maxProductImageHTTPResponseBytes {
		return productImageHTTPResponse{}, productimage.ErrOutputValidation
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var parsed productImageHTTPResponse
	if err := decoder.Decode(&parsed); err != nil {
		return productImageHTTPResponse{}, productimage.ErrOutputValidation
	}
	var additional any
	if err := decoder.Decode(&additional); err != io.EOF {
		return productImageHTTPResponse{}, productimage.ErrOutputValidation
	}
	if len(parsed.Error) > 8<<10 {
		return productImageHTTPResponse{}, productimage.ErrOutputValidation
	}
	if parsed.Error != "" {
		return productImageHTTPResponse{}, fmt.Errorf("http product image service error: %s", parsed.Error)
	}
	return parsed, nil
}

type productImageHTTPRequest struct {
	ImageBase64        string                       `json:"image_base64"`
	SourceURL          string                       `json:"source_url"`
	Task               string                       `json:"task"`
	Product            productImageHTTPProduct      `json:"product"`
	ProfileName        string                       `json:"profile_name,omitempty"`
	SceneOptions       productImageHTTPSceneOptions `json:"scene_options,omitempty"`
	StyleReferenceURLs []string                     `json:"style_reference_urls,omitempty"`
	MaximumOutputs     int                          `json:"maximum_outputs,omitempty"`
}

type productImageHTTPProduct struct {
	ProductKey  string            `json:"product_key"`
	Title       string            `json:"title,omitempty"`
	ProductType string            `json:"product_type,omitempty"`
	Attributes  map[string]string `json:"attributes,omitempty"`
}

type productImageHTTPSceneOptions struct {
	SceneCategory     string   `json:"scene_category,omitempty"`
	SceneStyle        string   `json:"scene_style,omitempty"`
	BackgroundTone    string   `json:"background_tone,omitempty"`
	Composition       string   `json:"composition,omitempty"`
	PropsLevel        string   `json:"props_level,omitempty"`
	AudienceHint      string   `json:"audience_hint,omitempty"`
	CustomSceneHint   string   `json:"custom_scene_hint,omitempty"`
	SlotRole          string   `json:"slot_role,omitempty"`
	SlotBrief         string   `json:"slot_brief,omitempty"`
	StyleReferenceIDs []string `json:"style_reference_ids,omitempty"`
}

type productImageHTTPResponse struct {
	ImageBase64 string                          `json:"image_base64,omitempty"`
	Format      string                          `json:"format,omitempty"`
	Images      []productImageHTTPResponseImage `json:"images,omitempty"`
	BBox        string                          `json:"bbox,omitempty"`
	Metadata    map[string]string               `json:"metadata,omitempty"`
	Error       string                          `json:"error,omitempty"`
}

type productImageHTTPResponseImage struct {
	ImageBase64 string            `json:"image_base64"`
	Format      string            `json:"format,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

func singleProductImageCandidate(response productImageHTTPResponse, source productimage.Asset, role productimage.Role, operation string, endpoint ProductImageEndpointConfig) (productimage.Candidate, error) {
	if len(response.Images) > 0 {
		if len(response.Images) != 1 || response.ImageBase64 != "" {
			return productimage.Candidate{}, productimage.ErrOutputValidation
		}
		response.ImageBase64, response.Format = response.Images[0].ImageBase64, response.Images[0].Format
	}
	used := 0
	content, mediaType, width, height, err := decodeProductImageHTTPItem(productImageHTTPResponseImage{ImageBase64: response.ImageBase64, Format: response.Format}, &used)
	if err != nil {
		return productimage.Candidate{}, err
	}
	return productImageHTTPCandidate(content, mediaType, width, height, source, role, operation, endpoint), nil
}

func productImageHTTPCandidate(content []byte, mediaType string, width, height int, source productimage.Asset, role productimage.Role, operation string, endpoint ProductImageEndpointConfig) productimage.Candidate {
	return productimage.Candidate{
		Asset: productimage.Asset{
			Bytes: content, MediaType: mediaType, SourceURL: source.SourceURL, SourceAssetID: source.SourceAssetID,
			Role: role, Width: width, Height: height, Operations: []string{operation},
		},
		Metadata: productimage.GenerationMetadata{
			Capability: operation, ModelFamily: endpoint.Model, PromptReference: operation,
			PromptVersion: endpoint.ConfigurationVersion,
			Values:        map[string]string{"provider": endpoint.Provider, "configuration_version": endpoint.ConfigurationVersion},
		},
	}
}

func decodeProductImageHTTPItem(item productImageHTTPResponseImage, used *int) ([]byte, string, int, int, error) {
	encoded := strings.TrimSpace(item.ImageBase64)
	if encoded == "" || base64.StdEncoding.DecodedLen(len(encoded)) > productimage.MaxInlineArtifactAggregateBytes-*used {
		return nil, "", 0, 0, productimage.ErrOutputValidation
	}
	content, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil || len(content) == 0 || len(content) > productimage.MaxInlineArtifactBytes || len(content) > productimage.MaxInlineArtifactAggregateBytes-*used {
		return nil, "", 0, 0, productimage.ErrOutputValidation
	}
	*used += len(content)
	mediaType, width, height, err := InspectGeneratedArtifact(content)
	return content, mediaType, width, height, err
}

func InspectGeneratedArtifact(content []byte) (string, int, int, error) {
	config, format, err := image.DecodeConfig(bytes.NewReader(content))
	if err != nil || config.Width <= 0 || config.Height <= 0 {
		return "", 0, 0, productimage.ErrOutputValidation
	}
	mediaType := map[string]string{"jpeg": "image/jpeg", "png": "image/png", "webp": "image/webp"}[format]
	if mediaType == "" {
		return "", 0, 0, productimage.ErrOutputValidation
	}
	return mediaType, config.Width, config.Height, nil
}

func validateProductImageEndpoint(config ProductImageEndpointConfig) error {
	parsed, err := url.Parse(config.Endpoint)
	if err != nil || strings.TrimSpace(config.Endpoint) != config.Endpoint || parsed.Host == "" || parsed.User != nil || parsed.Fragment != "" || parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("endpoint URL must be canonical HTTP(S)")
	}
	for _, value := range []string{config.Provider, config.Model, config.RouteReference, config.CredentialReference, config.ConfigurationVersion, config.PricingVersion} {
		if !canonicalProductImageHTTPString(value) {
			return fmt.Errorf("endpoint identity is incomplete")
		}
	}
	if config.CostMicrosPerOutput < 0 {
		return fmt.Errorf("endpoint cost is invalid")
	}
	if strings.ContainsAny(config.BearerToken, "\r\n") {
		return fmt.Errorf("endpoint bearer token is invalid")
	}
	return nil
}

func cloneProductImageEndpoint(config *ProductImageEndpointConfig) *ProductImageEndpointConfig {
	if config == nil {
		return nil
	}
	cloned := *config
	return &cloned
}

func newProductImageHTTPProduct(product productimage.ProductContext) productImageHTTPProduct {
	return productImageHTTPProduct{ProductKey: product.ProductKey, Title: product.Title, ProductType: product.ProductType, Attributes: product.Attributes}
}

func newProductImageHTTPSceneOptions(options productimage.SceneOptions) productImageHTTPSceneOptions {
	return productImageHTTPSceneOptions{
		SceneCategory: options.SceneCategory, SceneStyle: options.SceneStyle, BackgroundTone: options.BackgroundTone,
		Composition: options.Composition, PropsLevel: options.PropsLevel, AudienceHint: options.AudienceHint,
		CustomSceneHint: options.CustomSceneHint, SlotRole: options.SlotRole, SlotBrief: options.SlotBrief,
		StyleReferenceIDs: append([]string(nil), options.StyleReferenceIDs...),
	}
}

func canonicalProductImageHTTPString(value string) bool {
	return value != "" && strings.TrimSpace(value) == value && len(value) <= 64<<10
}

var (
	_ productimage.SubjectExtractor        = (*ProductImageAdapter)(nil)
	_ productimage.WhiteBackgroundRenderer = (*ProductImageAdapter)(nil)
	_ productimage.SceneRenderer           = (*ProductImageAdapter)(nil)
	_ productimage.UsageQuoter             = (*ProductImageAdapter)(nil)
)
