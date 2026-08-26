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
	SubjectExtractor            productimage.SubjectExtractor
	WhiteBackgroundRenderer     productimage.WhiteBackgroundRenderer
	SceneRenderer               productimage.SceneRenderer
	ProductContext              *productimage.ProductContext
	SourceAssets                map[string]productimage.ImageAsset
	AuthorizedStyleReferenceIDs map[string]struct{}
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
	slot, sourceAssetID, source, err := e.validateAndResolve(input)
	if err != nil {
		return imageagent.SlotExecutionResult{}, err
	}

	productContext := productContextForSlot(e.dependencies.ProductContext, slot, e.authorizedStyleReferences(slot.StyleReferenceIDs))
	var assets []productimage.ImageAsset
	switch slot.Role {
	case imageagent.SlotRoleMain:
		assets, err = e.executeMain(ctx, source, productContext)
	case imageagent.SlotRoleScene, imageagent.SlotRoleDetail, imageagent.SlotRoleSellingPoint:
		assets, err = e.executeScene(ctx, source, productContext)
	case imageagent.SlotRoleSize:
		if source.Width <= 0 || source.Height <= 0 {
			return imageagent.SlotExecutionResult{}, fmt.Errorf("size slot %q requires reliable dimensions", slot.ID)
		}
		assets, err = e.executeScene(ctx, source, productContext)
	default:
		return imageagent.SlotExecutionResult{}, fmt.Errorf("slot %q has unsupported role %q", slot.ID, slot.Role)
	}
	if err != nil {
		return imageagent.SlotExecutionResult{}, fmt.Errorf("execute slot %q: %w", slot.ID, err)
	}

	candidates, err := generatedCandidates(input, slot, sourceAssetID, source, assets)
	if err != nil {
		return imageagent.SlotExecutionResult{}, err
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
	source, ok := e.dependencies.SourceAssets[sourceAssetID]
	if ok {
		source = cloneImageAsset(source)
	} else if isReadableCompatibilityURL(sourceAssetID) {
		source = productimage.ImageAsset{URL: sourceAssetID, SourceURL: sourceAssetID, Type: productimage.AssetTypeSourceImage}
	} else {
		return imageagent.Slot{}, "", productimage.ImageAsset{}, fmt.Errorf("source asset %q is not resolved to a readable asset", sourceAssetID)
	}
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

func isReadableCompatibilityURL(value string) bool {
	parsed, err := url.ParseRequestURI(strings.TrimSpace(value))
	if err != nil || parsed.Host == "" {
		return false
	}
	switch strings.ToLower(parsed.Scheme) {
	case "http", "https":
		return true
	default:
		return false
	}
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

func (e *ProductImageSlotExecutor) authorizedStyleReferences(ids []string) []string {
	if len(e.dependencies.AuthorizedStyleReferenceIDs) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(ids))
	authorized := make([]string, 0, len(ids))
	for _, rawID := range ids {
		id := strings.TrimSpace(rawID)
		if id == "" {
			continue
		}
		if _, allowed := e.dependencies.AuthorizedStyleReferenceIDs[id]; !allowed {
			continue
		}
		if _, duplicate := seen[id]; duplicate {
			continue
		}
		seen[id] = struct{}{}
		authorized = append(authorized, id)
	}
	return authorized
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
		if !isGeneratedAsset(url, source, asset) {
			continue
		}
		candidates = append(candidates, imageagent.AssetCandidate{
			AssetID:       candidateAssetID(input, slot, providerOutputIndex),
			URL:           url,
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
