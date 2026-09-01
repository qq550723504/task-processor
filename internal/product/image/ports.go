package image

import (
	"context"
	"errors"
	"net"
	"net/url"
	"path"
	"reflect"
	"sort"
	"strings"
)

const (
	maxProductAttributes = 256
	maxStyleReferences   = 16
	maxSceneCandidates   = 16
	maxReviewCandidates  = 64
	maxOperations        = 32
	maxMetadataValues    = 128
	maxImageStringBytes  = 8 << 10
	maxImageInputBytes   = 64 << 10
)

type SubjectExtractor interface {
	Extract(context.Context, ExtractRequest) (Candidate, error)
}

type WhiteBackgroundRenderer interface {
	RenderWhiteBackground(context.Context, RenderRequest) (Candidate, error)
}

type SceneRenderer interface {
	RenderScene(context.Context, SceneRequest) ([]Candidate, error)
}

type Reviewer interface {
	Review(context.Context, ReviewRequest) (Review, error)
}

type UsageQuoter interface {
	QuoteUsage(context.Context, UsageQuoteRequest) (UsageQuote, error)
}

type subjectCapability struct{ backend SubjectExtractor }

func NewSubjectCapability(backend SubjectExtractor) (SubjectExtractor, error) {
	if isNilCapability(backend) {
		return nil, ErrInputInvalid
	}
	return &subjectCapability{backend: backend}, nil
}

func (c *subjectCapability) Extract(ctx context.Context, request ExtractRequest) (Candidate, error) {
	if err := contextError(ctx); err != nil {
		return Candidate{}, err
	}
	cloned, err := cloneExtractRequest(request)
	if err != nil {
		return Candidate{}, err
	}
	if err := contextError(ctx); err != nil {
		return Candidate{}, err
	}
	candidate, err := c.backend.Extract(ctx, cloned)
	if err != nil {
		return Candidate{}, capabilityError(err)
	}
	if err := contextError(ctx); err != nil {
		return Candidate{}, err
	}
	return validateCandidate(candidate, cloned.Source, RoleSubject, "extract_subject")
}

type whiteBackgroundCapability struct{ backend WhiteBackgroundRenderer }

func NewWhiteBackgroundCapability(backend WhiteBackgroundRenderer) (WhiteBackgroundRenderer, error) {
	if isNilCapability(backend) {
		return nil, ErrInputInvalid
	}
	return &whiteBackgroundCapability{backend: backend}, nil
}

func (c *whiteBackgroundCapability) RenderWhiteBackground(ctx context.Context, request RenderRequest) (Candidate, error) {
	if err := contextError(ctx); err != nil {
		return Candidate{}, err
	}
	cloned, err := cloneRenderRequest(request)
	if err != nil {
		return Candidate{}, err
	}
	if err := contextError(ctx); err != nil {
		return Candidate{}, err
	}
	candidate, err := c.backend.RenderWhiteBackground(ctx, cloned)
	if err != nil {
		return Candidate{}, capabilityError(err)
	}
	if err := contextError(ctx); err != nil {
		return Candidate{}, err
	}
	return validateCandidate(candidate, cloned.Source, RoleWhiteBackground, "render_white_background")
}

type sceneCapability struct{ backend SceneRenderer }

func NewSceneCapability(backend SceneRenderer) (SceneRenderer, error) {
	if isNilCapability(backend) {
		return nil, ErrInputInvalid
	}
	return &sceneCapability{backend: backend}, nil
}

func (c *sceneCapability) RenderScene(ctx context.Context, request SceneRequest) ([]Candidate, error) {
	if err := contextError(ctx); err != nil {
		return nil, err
	}
	cloned, limit, err := cloneSceneRequest(request)
	if err != nil {
		return nil, err
	}
	if err := contextError(ctx); err != nil {
		return nil, err
	}
	candidates, err := c.backend.RenderScene(ctx, cloned)
	if err != nil {
		return nil, capabilityError(err)
	}
	if err := contextError(ctx); err != nil {
		return nil, err
	}
	if len(candidates) == 0 || len(candidates) > limit {
		return nil, ErrOutputValidation
	}
	validated := make([]Candidate, 0, len(candidates))
	seenURLs := make(map[string]struct{}, len(candidates))
	for _, candidate := range candidates {
		item, err := validateCandidate(candidate, cloned.Source, RoleScene, "render_scene")
		if err != nil {
			return nil, err
		}
		identity := canonicalHTTPURL(item.Asset.URL)
		if _, exists := seenURLs[identity]; exists {
			return nil, ErrOutputValidation
		}
		seenURLs[identity] = struct{}{}
		validated = append(validated, item)
	}
	return validated, nil
}

type reviewCapability struct{ backend Reviewer }

func NewReviewCapability(backend Reviewer) (Reviewer, error) {
	if isNilCapability(backend) {
		return nil, ErrInputInvalid
	}
	return &reviewCapability{backend: backend}, nil
}

func (c *reviewCapability) Review(ctx context.Context, request ReviewRequest) (Review, error) {
	if err := contextError(ctx); err != nil {
		return Review{}, err
	}
	cloned, err := cloneReviewRequest(request)
	if err != nil {
		return Review{}, err
	}
	if err := contextError(ctx); err != nil {
		return Review{}, err
	}
	review, err := c.backend.Review(ctx, cloned)
	if err != nil {
		return Review{}, capabilityError(err)
	}
	if err := contextError(ctx); err != nil {
		return Review{}, err
	}
	if review.Score < 0 || review.Score > 1 {
		return Review{}, ErrOutputValidation
	}
	reasons, err := normalizedStrings(review.Reasons, maxMetadataValues)
	if err != nil || review.NeedsHumanReview && len(reasons) == 0 {
		return Review{}, ErrOutputValidation
	}
	sort.Strings(reasons)
	review.Reasons = reasons
	return review, nil
}

func cloneExtractRequest(request ExtractRequest) (ExtractRequest, error) {
	source, product, err := validatedInput(request.Source, request.Product)
	if err != nil {
		return ExtractRequest{}, err
	}
	return ExtractRequest{Source: source, Product: product}, nil
}

func cloneRenderRequest(request RenderRequest) (RenderRequest, error) {
	source, product, err := validatedInput(request.Source, request.Product)
	if err != nil {
		return RenderRequest{}, err
	}
	return RenderRequest{Source: source, Product: product}, nil
}

func cloneSceneRequest(request SceneRequest) (SceneRequest, int, error) {
	source, product, err := validatedInput(request.Source, request.Product)
	if err != nil {
		return SceneRequest{}, 0, err
	}
	options, err := MergeSceneOptions(nil, &request.Options)
	if err != nil {
		return SceneRequest{}, 0, err
	}
	if options == nil {
		options = &SceneOptions{}
	}
	if !isCanonicalOptional(request.ProfileName) {
		return SceneRequest{}, 0, ErrInputInvalid
	}
	if len(request.StyleReferences) > maxStyleReferences {
		return SceneRequest{}, 0, ErrInputInvalid
	}
	styleReferences := make([]Asset, len(request.StyleReferences))
	seen := make(map[string]struct{}, len(request.StyleReferences))
	for index, styleReference := range request.StyleReferences {
		cloned, err := validateSourceAsset(styleReference)
		if err != nil {
			return SceneRequest{}, 0, err
		}
		if _, exists := seen[cloned.SourceAssetID]; exists {
			return SceneRequest{}, 0, ErrInputInvalid
		}
		seen[cloned.SourceAssetID] = struct{}{}
		styleReferences[index] = cloned
	}
	limit := request.MaximumOutputs
	if limit == 0 {
		limit = maxSceneCandidates
	}
	if limit < 1 || limit > maxSceneCandidates {
		return SceneRequest{}, 0, ErrInputInvalid
	}
	return SceneRequest{
		Source: source, Product: product, Options: *options,
		ProfileName: request.ProfileName, StyleReferences: styleReferences, MaximumOutputs: limit,
	}, limit, nil
}

func cloneReviewRequest(request ReviewRequest) (ReviewRequest, error) {
	product, err := validateProductContext(request.Product)
	if err != nil || len(request.Sources) == 0 || len(request.Candidates) == 0 || len(request.Candidates) > maxReviewCandidates {
		return ReviewRequest{}, ErrInputInvalid
	}
	sources := make([]Asset, len(request.Sources))
	byID := make(map[string]Asset, len(request.Sources))
	for index, source := range request.Sources {
		cloned, err := validateSourceAsset(source)
		if err != nil {
			return ReviewRequest{}, err
		}
		if _, exists := byID[cloned.SourceAssetID]; exists {
			return ReviewRequest{}, ErrInputInvalid
		}
		byID[cloned.SourceAssetID] = cloned
		sources[index] = cloned
	}
	candidates := make([]Candidate, len(request.Candidates))
	for index, candidate := range request.Candidates {
		source, exists := byID[candidate.Asset.SourceAssetID]
		if !exists || !isGeneratedRole(candidate.Asset.Role) {
			return ReviewRequest{}, ErrInputInvalid
		}
		cloned, err := validateCandidate(candidate, source, candidate.Asset.Role, "")
		if err != nil {
			return ReviewRequest{}, ErrInputInvalid
		}
		candidates[index] = cloned
	}
	return ReviewRequest{Product: product, Sources: sources, Candidates: candidates}, nil
}

func validatedInput(source Asset, product ProductContext) (Asset, ProductContext, error) {
	validatedSource, err := validateSourceAsset(source)
	if err != nil {
		return Asset{}, ProductContext{}, err
	}
	validatedProduct, err := validateProductContext(product)
	if err != nil {
		return Asset{}, ProductContext{}, err
	}
	return validatedSource, validatedProduct, nil
}

func validateSourceAsset(asset Asset) (Asset, error) {
	if asset.Role != RoleSource || !isCanonicalRequired(asset.SourceAssetID) || !isCanonicalHTTPURL(asset.URL) || !isCanonicalHTTPURL(asset.SourceURL) {
		return Asset{}, ErrInputInvalid
	}
	if !validDimensions(asset.Width, asset.Height) {
		return Asset{}, ErrInputInvalid
	}
	operations, err := validateOperations(asset.Operations, false)
	if err != nil {
		return Asset{}, ErrInputInvalid
	}
	asset.Operations = operations
	return asset, nil
}

func validateCandidate(candidate Candidate, source Asset, role Role, requiredOperation string) (Candidate, error) {
	asset := candidate.Asset
	if asset.Role != role || role == RoleSource || !isCanonicalRequired(asset.SourceAssetID) || asset.SourceAssetID != source.SourceAssetID {
		return Candidate{}, ErrOutputValidation
	}
	if !isCanonicalHTTPURL(asset.URL) || !isCanonicalHTTPURL(asset.SourceURL) || !validDimensions(asset.Width, asset.Height) || asset.Width == 0 {
		return Candidate{}, ErrOutputValidation
	}
	if sourceURLEquivalent(asset.URL, source.URL) || sourceURLEquivalent(asset.URL, source.SourceURL) {
		return Candidate{}, ErrOutputValidation
	}
	if !sourceURLEquivalent(asset.SourceURL, source.URL) && !sourceURLEquivalent(asset.SourceURL, source.SourceURL) {
		return Candidate{}, ErrOutputValidation
	}
	operations, err := validateOperations(asset.Operations, true)
	if err != nil || requiredOperation != "" && !containsString(operations, requiredOperation) {
		return Candidate{}, ErrOutputValidation
	}
	metadata, err := NormalizeGenerationMetadata(candidate.Metadata)
	if err != nil {
		return Candidate{}, ErrOutputValidation
	}
	asset.Operations = operations
	return Candidate{Asset: asset, Metadata: metadata}, nil
}

func validateProductContext(product ProductContext) (ProductContext, error) {
	if !isCanonicalRequired(product.ProductKey) || !isCanonicalOptional(product.Title) || !isCanonicalOptional(product.ProductType) || len(product.Attributes) > maxProductAttributes {
		return ProductContext{}, ErrInputInvalid
	}
	used := len(product.ProductKey) + len(product.Title) + len(product.ProductType)
	attributes := make(map[string]string, len(product.Attributes))
	for key, value := range product.Attributes {
		if !isCanonicalRequired(key) || !isCanonicalRequired(value) {
			return ProductContext{}, ErrInputInvalid
		}
		used += len(key) + len(value)
		if used > maxImageInputBytes {
			return ProductContext{}, ErrInputInvalid
		}
		attributes[key] = value
	}
	product.Attributes = attributes
	return product, nil
}

func validateOperations(operations []string, generated bool) ([]string, error) {
	if len(operations) == 0 || len(operations) > maxOperations {
		return nil, ErrInputInvalid
	}
	seen := make(map[string]struct{}, len(operations))
	cloned := make([]string, 0, len(operations))
	for _, operation := range operations {
		if !isCanonicalToken(operation) {
			return nil, ErrInputInvalid
		}
		if generated && (strings.Contains(operation, "pass_through") || strings.Contains(operation, "placeholder") || operation == "source") {
			return nil, ErrInputInvalid
		}
		if _, exists := seen[operation]; exists {
			return nil, ErrInputInvalid
		}
		seen[operation] = struct{}{}
		cloned = append(cloned, operation)
	}
	return cloned, nil
}

func isCanonicalToken(value string) bool {
	if !isCanonicalRequired(value) {
		return false
	}
	for index, char := range value {
		if char >= 'a' && char <= 'z' || char >= '0' && char <= '9' && index > 0 || char == '_' && index > 0 {
			continue
		}
		return false
	}
	return true
}

func isCanonicalRequired(value string) bool {
	return value != "" && len(value) <= maxImageStringBytes && strings.TrimSpace(value) == value
}

func isCanonicalOptional(value string) bool {
	return value == "" || isCanonicalRequired(value)
}

func isCanonicalHTTPURL(value string) bool {
	return strings.TrimSpace(value) == value && canonicalHTTPURL(value) != "" && len(value) <= maxImageStringBytes
}

func sourceURLEquivalent(left, right string) bool {
	leftCanonical, rightCanonical := canonicalHTTPURL(left), canonicalHTTPURL(right)
	return leftCanonical != "" && leftCanonical == rightCanonical
}

func canonicalHTTPURL(value string) string {
	parsed, err := url.Parse(strings.TrimSpace(value))
	if err != nil || parsed.Host == "" || parsed.User != nil || parsed.Fragment != "" {
		return ""
	}
	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" {
		return ""
	}
	host := strings.ToLower(parsed.Hostname())
	if host == "" {
		return ""
	}
	port := parsed.Port()
	if port != "" && !((scheme == "http" && port == "80") || (scheme == "https" && port == "443")) {
		host = net.JoinHostPort(host, port)
	}
	cleanPath := path.Clean(parsed.Path)
	if cleanPath == "." || cleanPath == "/" {
		cleanPath = ""
	} else if !strings.HasPrefix(cleanPath, "/") {
		cleanPath = "/" + cleanPath
	}
	return (&url.URL{Scheme: scheme, Host: host, Path: cleanPath, RawQuery: parsed.Query().Encode()}).String()
}

func validDimensions(width, height int) bool {
	return width >= 0 && height >= 0 && (width == 0) == (height == 0)
}

func normalizedStrings(values []string, limit int) ([]string, error) {
	if len(values) > limit {
		return nil, ErrInputInvalid
	}
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if len(value) > maxImageStringBytes {
			return nil, ErrInputInvalid
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out, nil
}

func isGeneratedRole(role Role) bool {
	return role == RoleSubject || role == RoleWhiteBackground || role == RoleScene
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func contextError(ctx context.Context) error {
	if ctx == nil {
		return ErrInputInvalid
	}
	return ctx.Err()
}

func capabilityError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	for _, stable := range []error{ErrInputInvalid, ErrCapabilityUnsupported, ErrExternalCapabilityUnavailable, ErrOutputValidation, ErrPolicyRejected} {
		if errors.Is(err, stable) {
			return stable
		}
	}
	return ErrExternalCapabilityUnavailable
}

func isNilCapability(value any) bool {
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
