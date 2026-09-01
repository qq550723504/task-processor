package image

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"math"
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
	if contextErr := contextError(ctx); contextErr != nil {
		return Candidate{}, contextErr
	}
	if err != nil {
		return Candidate{}, capabilityError(err)
	}
	return validateCandidate(candidate, cloned.Source, RoleSubject, "extract_subject", forbiddenArtifactURLs(cloned.Source))
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
	if contextErr := contextError(ctx); contextErr != nil {
		return Candidate{}, contextErr
	}
	if err != nil {
		return Candidate{}, capabilityError(err)
	}
	return validateCandidate(candidate, cloned.Source, RoleWhiteBackground, "render_white_background", forbiddenArtifactURLs(cloned.Source))
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
	if contextErr := contextError(ctx); contextErr != nil {
		return nil, contextErr
	}
	if err != nil {
		return nil, capabilityError(err)
	}
	if len(candidates) == 0 || len(candidates) > limit {
		return nil, ErrOutputValidation
	}
	if err := validateGeneratedArtifactAggregate(candidates, MaxInlineArtifactAggregateBytes); err != nil {
		return nil, ErrOutputValidation
	}
	validated := make([]Candidate, 0, len(candidates))
	seenArtifacts := make(map[string]struct{}, len(candidates))
	forbidden := forbiddenArtifactURLs(append([]Asset{cloned.Source}, cloned.StyleReferences...)...)
	for _, candidate := range candidates {
		identity := artifactIdentity(candidate.Asset)
		if _, exists := seenArtifacts[identity]; exists {
			return nil, ErrOutputValidation
		}
		item, err := validateCandidate(candidate, cloned.Source, RoleScene, "render_scene", forbidden)
		if err != nil {
			return nil, err
		}
		seenArtifacts[identity] = struct{}{}
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
	if contextErr := contextError(ctx); contextErr != nil {
		return Review{}, contextErr
	}
	if err != nil {
		return Review{}, capabilityError(err)
	}
	if math.IsNaN(review.Score) || math.IsInf(review.Score, 0) || review.Score < 0 || review.Score > 1 {
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
	seen := map[string]struct{}{source.SourceAssetID: {}}
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
	if len(request.Sources) == 0 || len(request.Sources) > maxReviewCandidates || len(request.Candidates) == 0 || len(request.Candidates) > maxReviewCandidates {
		return ReviewRequest{}, ErrInputInvalid
	}
	if err := validateGeneratedArtifactAggregate(request.Candidates, MaxInlineArtifactAggregateBytes); err != nil {
		return ReviewRequest{}, ErrInputInvalid
	}
	product, err := validateProductContext(request.Product)
	if err != nil {
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
	allForbidden := forbiddenArtifactURLs(sources...)
	seenArtifacts := make(map[string]struct{}, len(request.Candidates))
	for index, candidate := range request.Candidates {
		source, exists := byID[candidate.Asset.SourceAssetID]
		if !exists || !isGeneratedRole(candidate.Asset.Role) {
			return ReviewRequest{}, ErrInputInvalid
		}
		identity := artifactIdentity(candidate.Asset)
		if _, exists := seenArtifacts[identity]; exists {
			return ReviewRequest{}, ErrInputInvalid
		}
		cloned, err := validateCandidate(candidate, source, candidate.Asset.Role, "", allForbidden)
		if err != nil {
			return ReviewRequest{}, ErrInputInvalid
		}
		seenArtifacts[identity] = struct{}{}
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
	if asset.Role != RoleSource || asset.Bytes != nil || !isCanonicalRequired(asset.SourceAssetID) || !isCanonicalHTTPURL(asset.URL) || !isCanonicalHTTPURL(asset.SourceURL) {
		return Asset{}, ErrInputInvalid
	}
	if asset.MediaType != "" && !isCanonicalImageMediaType(asset.MediaType) {
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
	asset.Bytes = nil
	return asset, nil
}

func validateCandidate(candidate Candidate, source Asset, role Role, requiredOperation string, forbiddenURLs map[string]struct{}) (Candidate, error) {
	asset := candidate.Asset
	if asset.Role != role || role == RoleSource || !isCanonicalRequired(asset.SourceAssetID) || asset.SourceAssetID != source.SourceAssetID {
		return Candidate{}, ErrOutputValidation
	}
	if !isCanonicalHTTPURL(asset.SourceURL) || !validDimensions(asset.Width, asset.Height) || asset.Width == 0 {
		return Candidate{}, ErrOutputValidation
	}
	if err := validateGeneratedArtifact(asset); err != nil {
		return Candidate{}, ErrOutputValidation
	}
	if asset.URL != "" {
		if _, forbidden := forbiddenURLs[canonicalHTTPURL(asset.URL)]; forbidden {
			return Candidate{}, ErrOutputValidation
		}
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
	asset.Bytes = append([]byte(nil), asset.Bytes...)
	return Candidate{Asset: asset, Metadata: metadata}, nil
}

func validateGeneratedArtifact(asset Asset) error {
	hasURL := asset.URL != ""
	hasInline := asset.Bytes != nil
	if hasURL == hasInline {
		return ErrOutputValidation
	}
	if hasURL {
		if !isCanonicalHTTPURL(asset.URL) || asset.MediaType != "" && !isCanonicalImageMediaType(asset.MediaType) {
			return ErrOutputValidation
		}
		return nil
	}
	if len(asset.Bytes) == 0 || len(asset.Bytes) > MaxInlineArtifactBytes || !isCanonicalImageMediaType(asset.MediaType) {
		return ErrOutputValidation
	}
	return nil
}

func validateGeneratedArtifactAggregate(candidates []Candidate, maximum int) error {
	if maximum < 0 {
		return ErrOutputValidation
	}
	used := 0
	for _, candidate := range candidates {
		if err := validateGeneratedArtifact(candidate.Asset); err != nil {
			return ErrOutputValidation
		}
		size := len(candidate.Asset.Bytes)
		if size > maximum-used {
			return ErrOutputValidation
		}
		used += size
	}
	return nil
}

func forbiddenArtifactURLs(assets ...Asset) map[string]struct{} {
	forbidden := make(map[string]struct{}, len(assets)*2)
	for _, asset := range assets {
		for _, candidate := range []string{asset.URL, asset.SourceURL} {
			if canonical := canonicalHTTPURL(candidate); canonical != "" {
				forbidden[canonical] = struct{}{}
			}
		}
	}
	return forbidden
}

func artifactIdentity(asset Asset) string {
	if asset.URL != "" {
		return "url:" + canonicalHTTPURL(asset.URL)
	}
	digest := sha256.Sum256(asset.Bytes)
	return "sha256:" + hex.EncodeToString(digest[:])
}

func isCanonicalImageMediaType(value string) bool {
	if strings.TrimSpace(value) != value {
		return false
	}
	return value == "image/jpeg" || value == "image/png" || value == "image/webp"
}

func validateProductContext(product ProductContext) (ProductContext, error) {
	if err := preflightProductContextResources(product); err != nil {
		return ProductContext{}, ErrInputInvalid
	}
	if !isCanonicalRequired(product.ProductKey) || !isCanonicalOptional(product.Title) || !isCanonicalOptional(product.ProductType) {
		return ProductContext{}, ErrInputInvalid
	}
	for key, value := range product.Attributes {
		if !isCanonicalRequired(key) || !isCanonicalRequired(value) {
			return ProductContext{}, ErrInputInvalid
		}
	}
	attributes := make(map[string]string, len(product.Attributes))
	for key, value := range product.Attributes {
		attributes[key] = value
	}
	product.Attributes = attributes
	return product, nil
}

func preflightProductContextResources(product ProductContext) error {
	if len(product.Attributes) > maxProductAttributes {
		return ErrInputInvalid
	}
	used := 0
	if !addImageStringBytes(&used, product.ProductKey) || !addImageStringBytes(&used, product.Title) || !addImageStringBytes(&used, product.ProductType) {
		return ErrInputInvalid
	}
	for key, value := range product.Attributes {
		if !addImageStringBytes(&used, key) || !addImageStringBytes(&used, value) {
			return ErrInputInvalid
		}
	}
	return nil
}

func validateOperations(operations []string, generated bool) ([]string, error) {
	if len(operations) == 0 || len(operations) > maxOperations {
		return nil, ErrInputInvalid
	}
	used := 0
	for index, operation := range operations {
		if !addImageStringBytes(&used, operation) || !isCanonicalToken(operation) {
			return nil, ErrInputInvalid
		}
		if generated && (strings.Contains(operation, "pass_through") || strings.Contains(operation, "placeholder") || operation == "source") {
			return nil, ErrInputInvalid
		}
		for previous := 0; previous < index; previous++ {
			if operations[previous] == operation {
				return nil, ErrInputInvalid
			}
		}
	}
	return append([]string(nil), operations...), nil
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
	return len(value) <= maxImageStringBytes && strings.TrimSpace(value) == value && canonicalHTTPURL(value) != ""
}

func sourceURLEquivalent(left, right string) bool {
	leftCanonical, rightCanonical := canonicalHTTPURL(left), canonicalHTTPURL(right)
	return leftCanonical != "" && leftCanonical == rightCanonical
}

func canonicalHTTPURL(value string) string {
	if len(value) == 0 || len(value) > maxImageStringBytes {
		return ""
	}
	value = strings.TrimSpace(value)
	if !validRawHTTPAuthority(value) {
		return ""
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.Host == "" || parsed.User != nil || parsed.Fragment != "" {
		return ""
	}
	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" {
		return ""
	}
	host, ipv6 := canonicalHTTPHost(parsed.Hostname())
	if host == "" {
		return ""
	}
	port, valid := canonicalHTTPPort(parsed.Host, scheme)
	if !valid {
		return ""
	}
	if port != "" {
		host = net.JoinHostPort(host, port)
	} else if ipv6 {
		host = "[" + host + "]"
	}
	cleanPath := path.Clean(parsed.Path)
	if cleanPath == "." || cleanPath == "/" {
		cleanPath = ""
	} else if !strings.HasPrefix(cleanPath, "/") {
		cleanPath = "/" + cleanPath
	}
	query, err := url.ParseQuery(parsed.RawQuery)
	if err != nil {
		return ""
	}
	return (&url.URL{Scheme: scheme, Host: host, Path: cleanPath, RawQuery: query.Encode()}).String()
}

func validRawHTTPAuthority(value string) bool {
	schemeEnd := strings.Index(value, "://")
	if schemeEnd < 0 || !(strings.EqualFold(value[:schemeEnd], "http") || strings.EqualFold(value[:schemeEnd], "https")) {
		return false
	}
	authorityStart := schemeEnd + len("://")
	authorityEnd := len(value)
	if separator := strings.IndexAny(value[authorityStart:], "/?#"); separator >= 0 {
		authorityEnd = authorityStart + separator
	}
	authority := value[authorityStart:authorityEnd]
	if authority == "" || strings.Contains(authority, "@") {
		return false
	}
	if authority[0] == '[' {
		if strings.Count(authority, "[") != 1 || strings.Count(authority, "]") != 1 {
			return false
		}
		closing := strings.IndexByte(authority, ']')
		if closing <= 1 {
			return false
		}
		suffix := authority[closing+1:]
		return suffix == "" || len(suffix) > 1 && suffix[0] == ':' && isASCIIDigits(suffix[1:])
	}
	if strings.ContainsAny(authority, "[]") {
		return false
	}
	colon := strings.IndexByte(authority, ':')
	if colon < 0 {
		return true
	}
	return colon > 0 && colon == strings.LastIndexByte(authority, ':') && isASCIIDigits(authority[colon+1:])
}

func isASCIIDigits(value string) bool {
	if value == "" {
		return false
	}
	for index := 0; index < len(value); index++ {
		if value[index] < '0' || value[index] > '9' {
			return false
		}
	}
	return true
}

func canonicalHTTPPort(authority, scheme string) (string, bool) {
	rawPort, explicit := explicitHTTPPort(authority)
	if !explicit {
		return "", true
	}
	if rawPort == "" {
		return "", false
	}
	value := 0
	for index := 0; index < len(rawPort); index++ {
		digit := rawPort[index]
		if digit < '0' || digit > '9' {
			return "", false
		}
		digitValue := int(digit - '0')
		if value > (65535-digitValue)/10 {
			return "", false
		}
		value = value*10 + digitValue
	}
	if value == 0 {
		return "", false
	}
	if (scheme == "http" && value == 80) || (scheme == "https" && value == 443) {
		return "", true
	}
	normalized := strings.TrimLeft(rawPort, "0")
	return normalized, true
}

func explicitHTTPPort(authority string) (string, bool) {
	if strings.HasPrefix(authority, "[") {
		closing := strings.LastIndexByte(authority, ']')
		if closing < 0 || closing == len(authority)-1 {
			return "", false
		}
		suffix := authority[closing+1:]
		if !strings.HasPrefix(suffix, ":") {
			return "", false
		}
		return suffix[1:], true
	}
	colon := strings.LastIndexByte(authority, ':')
	if colon < 0 {
		return "", false
	}
	return authority[colon+1:], true
}

func canonicalHTTPHost(raw string) (string, bool) {
	if raw == "" {
		return "", false
	}
	address, zone, hasZone := strings.Cut(raw, "%")
	if parsedIP := net.ParseIP(address); parsedIP != nil {
		normalized := parsedIP.String()
		if hasZone {
			if zone == "" || !strings.Contains(normalized, ":") {
				return "", false
			}
			normalized += "%" + zone
		}
		return normalized, strings.Contains(normalized, ":")
	}
	if hasZone {
		return "", false
	}
	host := strings.ToLower(raw)
	if strings.HasSuffix(host, "..") {
		return "", false
	}
	host = strings.TrimSuffix(host, ".")
	return host, false
}

func validDimensions(width, height int) bool {
	return width >= 0 && height >= 0 && (width == 0) == (height == 0)
}

func normalizedStrings(values []string, limit int) ([]string, error) {
	if len(values) > limit {
		return nil, ErrInputInvalid
	}
	used := 0
	for _, value := range values {
		if !addImageStringBytes(&used, value) {
			return nil, ErrInputInvalid
		}
	}
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out, nil
}

func addImageStringBytes(used *int, value string) bool {
	if len(value) > maxImageStringBytes || len(value) > maxImageInputBytes-*used {
		return false
	}
	*used += len(value)
	return true
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
