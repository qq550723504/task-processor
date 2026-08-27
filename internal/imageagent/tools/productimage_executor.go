package tools

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"path"
	"strings"

	"task-processor/internal/imageagent"
	productimage "task-processor/internal/productimage"
)

// Dependencies are the ProductImage capabilities and explicitly authorized
// inputs available to a single image-agent slot execution.
type Dependencies struct {
	SubjectExtractor        productimage.SubjectExtractor
	WhiteBackgroundRenderer productimage.WhiteBackgroundRenderer
	SceneRenderer           productimage.SceneRenderer
	AssetPublisher          productimage.AssetPublisher
	ProductContext          *productimage.ProductContext
}

// ProductImageSlotExecutor adapts ProductImage capabilities to the
// imageagent one-slot execution contract.
type ProductImageSlotExecutor struct {
	dependencies Dependencies
}

func NewProductImageSlotExecutor(dependencies Dependencies) *ProductImageSlotExecutor {
	return &ProductImageSlotExecutor{dependencies: dependencies}
}

func (e *ProductImageSlotExecutor) ExecuteSlot(ctx context.Context, input imageagent.SlotExecutionInput) (imageagent.SlotExecutionResult, error) {
	generated, err := e.GenerateSlot(ctx, input)
	if err != nil {
		return imageagent.SlotExecutionResult{}, err
	}
	return e.PublishSlot(ctx, input, generated)
}

func (e *ProductImageSlotExecutor) GenerateSlot(ctx context.Context, input imageagent.SlotExecutionInput) (imageagent.SlotGeneratedOutput, error) {
	slot, sourceAssetID, source, err := e.validateAndResolve(input)
	if err != nil {
		return imageagent.SlotGeneratedOutput{}, err
	}

	styleReferences, err := authorizedStyleReferences(slot.StyleReferenceIDs, input.AssetCatalog)
	if err != nil {
		return imageagent.SlotGeneratedOutput{}, err
	}
	productContext := productContextForSlot(e.dependencies.ProductContext, slot, styleReferences)
	var assets []productimage.ImageAsset
	switch slot.Role {
	case imageagent.SlotRoleMain:
		assets, err = e.executeMain(ctx, source, productContext)
	case imageagent.SlotRoleScene, imageagent.SlotRoleDetail, imageagent.SlotRoleSellingPoint:
		assets, err = e.executeScene(ctx, source, productContext)
	case imageagent.SlotRoleSize:
		if source.Width <= 0 || source.Height <= 0 {
			return imageagent.SlotGeneratedOutput{}, fmt.Errorf("size slot %q requires reliable dimensions", slot.ID)
		}
		assets, err = e.executeScene(ctx, source, productContext)
	default:
		return imageagent.SlotGeneratedOutput{}, fmt.Errorf("slot %q has unsupported role %q", slot.ID, slot.Role)
	}
	if err != nil {
		return imageagent.SlotGeneratedOutput{}, fmt.Errorf("execute slot %q: %w", slot.ID, err)
	}
	generated := make([]imageagent.GeneratedAsset, len(assets))
	for index, asset := range assets {
		generated[index] = imageagent.GeneratedAsset{URL: asset.URL, Type: string(asset.Type), SourceURL: asset.SourceURL, Operations: append([]string(nil), asset.Operations...), Width: asset.Width, Height: asset.Height, Metadata: cloneMetadata(asset.Metadata)}
	}
	return imageagent.SlotGeneratedOutput{SlotID: slot.ID, Attempt: input.Attempt, SourceAssetID: sourceAssetID, Assets: generated}, nil
}

func (e *ProductImageSlotExecutor) PublishSlot(ctx context.Context, input imageagent.SlotExecutionInput, generated imageagent.SlotGeneratedOutput) (imageagent.SlotExecutionResult, error) {
	slot, sourceAssetID, source, err := e.validateAndResolve(input)
	if err != nil {
		return imageagent.SlotExecutionResult{}, err
	}
	if generated.SlotID != slot.ID || generated.Attempt != input.Attempt || generated.SourceAssetID != sourceAssetID || len(generated.Assets) == 0 {
		return imageagent.SlotExecutionResult{}, imageagent.ErrRevisionConflict
	}
	assets := make([]productimage.ImageAsset, len(generated.Assets))
	for index, asset := range generated.Assets {
		assets[index] = productimage.ImageAsset{URL: asset.URL, Type: productimage.AssetType(asset.Type), SourceURL: asset.SourceURL, Operations: append([]string(nil), asset.Operations...), Width: asset.Width, Height: asset.Height, Metadata: cloneMetadata(asset.Metadata)}
	}
	if e.dependencies.AssetPublisher != nil {
		published := &productimage.ImageProcessResult{GalleryImages: append([]productimage.ImageAsset(nil), assets...)}
		request := &productimage.ImageProcessRequest{Text: fmt.Sprintf("image-agent:%s:%d:%s:%d", strings.TrimSpace(input.RunID), input.PlanRevision, slot.ID, input.Attempt)}
		if err := e.dependencies.AssetPublisher.Publish(ctx, request, published); err != nil {
			return imageagent.SlotExecutionResult{}, fmt.Errorf("publish slot %q generated assets: %w", slot.ID, err)
		}
		assets = published.GalleryImages
	}

	candidates, err := generatedCandidates(input, slot, sourceAssetID, source, assets)
	if err != nil {
		return imageagent.SlotExecutionResult{}, err
	}
	return imageagent.SlotExecutionResult{SlotID: slot.ID, Attempt: input.Attempt, Candidates: candidates}, nil
}

// BuildSlotResult projects only artifact-store final references into the v3
// candidate contract. ProductImage's transient URLs, local paths, bytes, and
// provider metadata are deliberately not accepted at this boundary.
func (e *ProductImageSlotExecutor) BuildSlotResult(_ context.Context, input imageagent.SlotExecutionInput, published imageagent.PublishedSlotOutput) (imageagent.SlotExecutionResult, error) {
	slot, sourceAssetID, _, err := e.validateAndResolve(input)
	if err != nil {
		return imageagent.SlotExecutionResult{}, err
	}
	if published.SlotID != slot.ID || published.Attempt != input.Attempt {
		return imageagent.SlotExecutionResult{}, imageagent.ErrRevisionConflict
	}
	manifest, err := imageagent.NormalizeFinalManifest(imageagent.FinalManifest{Assets: published.Assets})
	if err != nil {
		return imageagent.SlotExecutionResult{}, err
	}
	if slot.Role == imageagent.SlotRoleMain && len(manifest.Assets) != 1 {
		return imageagent.SlotExecutionResult{}, fmt.Errorf("main slot %q requires exactly one durable result: %w", slot.ID, imageagent.ErrValidation)
	}

	candidates := make([]imageagent.AssetCandidate, len(manifest.Assets))
	for index, asset := range manifest.Assets {
		if asset.SourceAssetID != sourceAssetID {
			return imageagent.SlotExecutionResult{}, imageagent.ErrRevisionConflict
		}
		candidates[index] = imageagent.AssetCandidate{
			AssetID:       durableCandidateAssetID(input, slot, asset),
			SourceAssetID: asset.SourceAssetID,
			DurableAsset: imageagent.DurableAssetIdentity{
				ObjectKey: asset.ObjectKey,
				SHA256:    asset.SHA256,
			},
		}
	}
	return imageagent.SlotExecutionResult{SlotID: slot.ID, Attempt: input.Attempt, Candidates: candidates}, nil
}

func (e *ProductImageSlotExecutor) validateAndResolve(input imageagent.SlotExecutionInput) (imageagent.Slot, string, productimage.ImageAsset, error) {
	slot := cloneSlot(input.Slot)
	slot.ID = strings.TrimSpace(slot.ID)
	slot.Brief = strings.TrimSpace(slot.Brief)
	if slot.ID == "" {
		return imageagent.Slot{}, "", productimage.ImageAsset{}, fmt.Errorf("slot ID is required")
	}
	if input.Attempt <= 0 {
		return imageagent.Slot{}, "", productimage.ImageAsset{}, fmt.Errorf("slot %q requires a positive attempt", slot.ID)
	}
	if strings.TrimSpace(input.RunID) == "" || input.PlanRevision <= 0 || strings.TrimSpace(input.IdempotencyKey) == "" || strings.TrimSpace(slot.IdempotencyKey) == "" {
		return imageagent.Slot{}, "", productimage.ImageAsset{}, fmt.Errorf("slot %q requires stable execution identity", slot.ID)
	}
	for i := range slot.SourceAssetIDs {
		slot.SourceAssetIDs[i] = strings.TrimSpace(slot.SourceAssetIDs[i])
	}
	var sourceAssetID string
	for _, id := range slot.SourceAssetIDs {
		if id != "" {
			sourceAssetID = id
			break
		}
	}
	if sourceAssetID == "" {
		return imageagent.Slot{}, "", productimage.ImageAsset{}, fmt.Errorf("slot %q requires a source asset", slot.ID)
	}
	if len(input.AssetCatalog.Assets) == 0 {
		return imageagent.Slot{}, "", productimage.ImageAsset{}, fmt.Errorf("slot %q requires an authorized catalog", slot.ID)
	}
	var authorized *imageagent.AuthorizedAsset
	for index := range input.AssetCatalog.Assets {
		candidate := &input.AssetCatalog.Assets[index]
		if strings.TrimSpace(candidate.ID) == sourceAssetID && candidate.Type == imageagent.AuthorizedAssetSource {
			authorized = candidate
			break
		}
	}
	if authorized == nil {
		return imageagent.Slot{}, "", productimage.ImageAsset{}, fmt.Errorf("source asset %q is not authorized", sourceAssetID)
	}
	sourceURL := strings.TrimSpace(authorized.URL)
	if sourceURL == "" {
		sourceURL = strings.TrimSpace(authorized.DisplayURL)
	}
	sourceProvenanceURL := strings.TrimSpace(authorized.SourceURL)
	if sourceProvenanceURL == "" {
		sourceProvenanceURL = sourceURL
	}
	validatedURL, err := imageagent.ValidateSafeImageURL(sourceURL)
	if err != nil {
		return imageagent.Slot{}, "", productimage.ImageAsset{}, fmt.Errorf("source asset %q has unsafe URL: %w", sourceAssetID, err)
	}
	validatedSourceURL, err := imageagent.ValidateSafeImageURL(sourceProvenanceURL)
	if err != nil {
		return imageagent.Slot{}, "", productimage.ImageAsset{}, fmt.Errorf("source asset %q has unsafe source URL: %w", sourceAssetID, err)
	}
	source := productimage.ImageAsset{URL: validatedURL, SourceURL: validatedSourceURL, Type: productimage.AssetTypeSourceImage, Width: authorized.Width, Height: authorized.Height, Metadata: cloneMetadata(authorized.Metadata)}
	source.URL = strings.TrimSpace(source.URL)
	source.SourceURL = strings.TrimSpace(source.SourceURL)
	if source.URL == "" {
		return imageagent.Slot{}, "", productimage.ImageAsset{}, fmt.Errorf("source asset %q has no readable URL", sourceAssetID)
	}
	if source.SourceURL == "" {
		source.SourceURL = source.URL
	}
	return slot, sourceAssetID, source, nil
}

func cloneSlot(slot imageagent.Slot) imageagent.Slot {
	cloned := slot
	cloned.SourceAssetIDs = append([]string(nil), slot.SourceAssetIDs...)
	cloned.StyleReferenceIDs = append([]string(nil), slot.StyleReferenceIDs...)
	return cloned
}

func cloneImageAsset(asset productimage.ImageAsset) productimage.ImageAsset {
	cloned := asset
	cloned.Operations = append([]string(nil), asset.Operations...)
	cloned.Metadata = cloneMetadata(asset.Metadata)
	return cloned
}

func (e *ProductImageSlotExecutor) executeMain(ctx context.Context, source productimage.ImageAsset, productContext *productimage.ProductContext) ([]productimage.ImageAsset, error) {
	if e.dependencies.SubjectExtractor == nil {
		return nil, fmt.Errorf("subject extractor is required for main slot")
	}
	if e.dependencies.WhiteBackgroundRenderer == nil {
		return nil, fmt.Errorf("white background renderer is required for main slot")
	}
	subject, err := e.dependencies.SubjectExtractor.Extract(ctx, source.URL, productContext)
	if err != nil {
		return nil, fmt.Errorf("extract subject: %w", err)
	}
	if subject == nil || strings.TrimSpace(subject.URL) == "" {
		return nil, fmt.Errorf("subject extractor returned no generated asset")
	}
	main, err := e.dependencies.WhiteBackgroundRenderer.Render(ctx, subject, productContext)
	if err != nil {
		return nil, fmt.Errorf("render white background: %w", err)
	}
	if main == nil {
		return nil, fmt.Errorf("white background renderer returned no generated asset")
	}
	return []productimage.ImageAsset{*main}, nil
}

func (e *ProductImageSlotExecutor) executeScene(ctx context.Context, source productimage.ImageAsset, productContext *productimage.ProductContext) ([]productimage.ImageAsset, error) {
	if e.dependencies.SceneRenderer == nil {
		return nil, fmt.Errorf("scene renderer is required")
	}
	return e.dependencies.SceneRenderer.Render(ctx, &source, productContext)
}

func authorizedStyleReferences(ids []string, catalog imageagent.AssetCatalog) ([]string, error) {
	allowed := make(map[string]struct{}, len(catalog.Assets))
	for _, asset := range catalog.Assets {
		if asset.Type == imageagent.AuthorizedAssetStyle {
			allowed[strings.TrimSpace(asset.ID)] = struct{}{}
		}
	}
	seen := make(map[string]struct{}, len(ids))
	authorized := make([]string, 0, len(ids))
	for _, rawID := range ids {
		id := strings.TrimSpace(rawID)
		if id == "" {
			continue
		}
		if _, ok := allowed[id]; !ok {
			return nil, fmt.Errorf("style reference %q is not authorized", id)
		}
		if _, duplicate := seen[id]; duplicate {
			continue
		}
		seen[id] = struct{}{}
		authorized = append(authorized, id)
	}
	return authorized, nil
}

func productContextForSlot(base *productimage.ProductContext, slot imageagent.Slot, styleReferenceIDs []string) *productimage.ProductContext {
	context := cloneProductContext(base)
	return productimage.ApplySceneOptionsToProductContext(context, &productimage.ImageProcessRequest{Scene: &productimage.SceneGenerationOptions{
		SlotRole: slotRoleName(slot.Role), SlotBrief: slot.Brief, StyleReferenceIDs: styleReferenceIDs,
	}})
}

func cloneProductContext(base *productimage.ProductContext) *productimage.ProductContext {
	if base == nil {
		return &productimage.ProductContext{}
	}
	cloned := *base
	if base.Attributes != nil {
		cloned.Attributes = make(map[string]string, len(base.Attributes))
		for key, value := range base.Attributes {
			cloned.Attributes[key] = value
		}
	}
	return &cloned
}

func slotRoleName(role imageagent.SlotRole) string {
	return string(role)
}

func generatedCandidates(input imageagent.SlotExecutionInput, slot imageagent.Slot, sourceAssetID string, source productimage.ImageAsset, assets []productimage.ImageAsset) ([]imageagent.AssetCandidate, error) {
	candidates := make([]imageagent.AssetCandidate, 0, len(assets))
	for providerOutputIndex, asset := range assets {
		url := strings.TrimSpace(asset.URL)
		validatedURL, err := imageagent.ValidateSafeImageURL(url)
		if err != nil {
			continue
		}
		if !isGeneratedAsset(url, source, asset) {
			continue
		}
		candidates = append(candidates, imageagent.AssetCandidate{
			AssetID:       candidateAssetID(input, slot, providerOutputIndex),
			URL:           validatedURL,
			SourceAssetID: sourceAssetID,
			Metadata:      cloneMetadata(asset.Metadata),
		})
	}
	if len(candidates) == 0 {
		return nil, fmt.Errorf("slot %q provider returned no generated candidates", slot.ID)
	}
	return candidates, nil
}

type candidateIdentity struct {
	RunID               string `json:"run_id"`
	PlanRevision        int64  `json:"plan_revision"`
	SlotID              string `json:"slot_id"`
	SlotIdempotencyKey  string `json:"slot_idempotency_key"`
	InputIdempotencyKey string `json:"input_idempotency_key"`
	Attempt             int    `json:"attempt"`
	ProviderOutputIndex int    `json:"provider_output_index"`
}

func candidateAssetID(input imageagent.SlotExecutionInput, slot imageagent.Slot, providerOutputIndex int) string {
	payload, err := json.Marshal(candidateIdentity{
		RunID: strings.TrimSpace(input.RunID), PlanRevision: input.PlanRevision,
		SlotID: strings.TrimSpace(slot.ID), SlotIdempotencyKey: strings.TrimSpace(slot.IdempotencyKey),
		InputIdempotencyKey: strings.TrimSpace(input.IdempotencyKey),
		Attempt:             input.Attempt, ProviderOutputIndex: providerOutputIndex,
	})
	if err != nil {
		panic(fmt.Sprintf("marshal candidate identity: %v", err))
	}
	sum := sha256.Sum256(payload)
	return "imageagent-candidate-" + hex.EncodeToString(sum[:])
}

type durableCandidateIdentity struct {
	RunID               string `json:"run_id"`
	PlanRevision        int64  `json:"plan_revision"`
	SlotID              string `json:"slot_id"`
	SlotIdempotencyKey  string `json:"slot_idempotency_key"`
	InputIdempotencyKey string `json:"input_idempotency_key"`
	Attempt             int    `json:"attempt"`
	ObjectKey           string `json:"object_key"`
	SHA256              string `json:"sha256"`
}

func durableCandidateAssetID(input imageagent.SlotExecutionInput, slot imageagent.Slot, asset imageagent.StagedAssetRef) string {
	payload, err := json.Marshal(durableCandidateIdentity{
		RunID: strings.TrimSpace(input.RunID), PlanRevision: input.PlanRevision,
		SlotID: strings.TrimSpace(slot.ID), SlotIdempotencyKey: strings.TrimSpace(slot.IdempotencyKey),
		InputIdempotencyKey: strings.TrimSpace(input.IdempotencyKey), Attempt: input.Attempt,
		ObjectKey: asset.ObjectKey, SHA256: asset.SHA256,
	})
	if err != nil {
		panic(fmt.Sprintf("marshal durable candidate identity: %v", err))
	}
	sum := sha256.Sum256(payload)
	return "imageagent-candidate-" + hex.EncodeToString(sum[:])
}

func isGeneratedAsset(url string, source productimage.ImageAsset, asset productimage.ImageAsset) bool {
	if url == "" || sourceURLEquivalent(url, source.URL) || sourceURLEquivalent(url, source.SourceURL) {
		return false
	}
	if strings.EqualFold(string(asset.Type), string(productimage.AssetTypeSourceImage)) || hasFallbackProvenance(asset) {
		return false
	}
	if strings.EqualFold(strings.TrimSpace(asset.Metadata["scene_mode"]), "local_canvas") ||
		strings.EqualFold(strings.TrimSpace(asset.Metadata["background_mode"]), "white_canvas") {
		return false
	}
	for _, operation := range asset.Operations {
		if strings.EqualFold(strings.TrimSpace(operation), "render_white_bg_placeholder") {
			return false
		}
	}
	return true
}

func hasFallbackProvenance(asset productimage.ImageAsset) bool {
	for _, operation := range asset.Operations {
		normalized := normalizeProvenance(operation)
		if strings.Contains(normalized, "pass_through") || strings.Contains(normalized, "placeholder") {
			return true
		}
	}
	for rawKey, rawValue := range asset.Metadata {
		key, value := normalizeProvenance(rawKey), normalizeProvenance(rawValue)
		switch key {
		case "mode", "scene_mode", "background_mode", "generation_mode", "operation":
			if isFallbackProvenanceValue(value) {
				return true
			}
		case "origin":
			if value == "source" || isFallbackProvenanceValue(value) {
				return true
			}
		case "fallback_reason":
			if strings.TrimSpace(rawValue) != "" {
				return true
			}
		case "fallback", "is_fallback", "tenant_model_gate":
			if value == "true" || value == "1" || value == "yes" {
				return true
			}
		}
	}
	return false
}

func isFallbackProvenanceValue(value string) bool {
	return strings.Contains(value, "pass_through") || strings.Contains(value, "placeholder") || value == "local_canvas" || value == "white_canvas"
}

func normalizeProvenance(value string) string {
	normalized := strings.ToLower(strings.TrimSpace(value))
	normalized = strings.NewReplacer("-", "_", " ", "_", ".", "_").Replace(normalized)
	return normalized
}

func sourceURLEquivalent(left, right string) bool {
	left, right = strings.TrimSpace(left), strings.TrimSpace(right)
	if left != "" && left == right {
		return true
	}
	return canonicalHTTPURLEqual(left, right)
}

func canonicalHTTPURLEqual(left, right string) bool {
	leftCanonical, rightCanonical := canonicalHTTPURL(left), canonicalHTTPURL(right)
	return leftCanonical != "" && leftCanonical == rightCanonical
}

func canonicalHTTPURL(value string) string {
	parsed, err := url.Parse(strings.TrimSpace(value))
	if err != nil || parsed.Host == "" {
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
	}
	if cleanPath != "" && !strings.HasPrefix(cleanPath, "/") {
		cleanPath = "/" + cleanPath
	}
	return (&url.URL{Scheme: scheme, Host: host, Path: cleanPath, RawQuery: parsed.Query().Encode()}).String()
}

func cloneMetadata(metadata map[string]string) map[string]string {
	if metadata == nil {
		return nil
	}
	cloned := make(map[string]string, len(metadata))
	for key, value := range metadata {
		cloned[key] = value
	}
	return cloned
}

var _ imageagent.SlotExecutor = (*ProductImageSlotExecutor)(nil)
var _ imageagent.RecoverableSlotExecutor = (*ProductImageSlotExecutor)(nil)
var _ imageagent.StagedSlotExecutor = (*ProductImageSlotExecutor)(nil)
