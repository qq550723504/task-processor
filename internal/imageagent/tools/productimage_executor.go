package tools

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
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

type quotedSlotOperation struct {
	public     imageagent.SlotUsageOperation
	capability productimage.CapabilityUsageQuote
}

type quotedSlotExecution struct {
	quote      imageagent.SlotUsageQuote
	operations []quotedSlotOperation
}

func (e *ProductImageSlotExecutor) quoteSlot(ctx context.Context, input imageagent.SlotExecutionInput, policy imageagent.BudgetPolicy) (quotedSlotExecution, error) {
	slot, _, source, err := e.validateAndResolve(input)
	if err != nil {
		return quotedSlotExecution{}, err
	}
	if slot.Role == imageagent.SlotRoleSize && (source.Width <= 0 || source.Height <= 0) {
		return quotedSlotExecution{}, fmt.Errorf("size slot %q requires reliable dimensions", slot.ID)
	}
	inputFingerprint := imageagent.SlotExecutionFingerprint(input)
	type componentOperation struct {
		name      string
		component any
	}
	var components []componentOperation
	switch slot.Role {
	case imageagent.SlotRoleMain:
		components = []componentOperation{{"extract_subject", e.dependencies.SubjectExtractor}, {"render_white_background", e.dependencies.WhiteBackgroundRenderer}}
	case imageagent.SlotRoleScene, imageagent.SlotRoleDetail, imageagent.SlotRoleSellingPoint, imageagent.SlotRoleSize:
		components = []componentOperation{{"render_scene", e.dependencies.SceneRenderer}}
	default:
		return quotedSlotExecution{}, fmt.Errorf("slot %q has unsupported role %q", slot.ID, slot.Role)
	}
	quoted := quotedSlotExecution{operations: make([]quotedSlotOperation, 0, len(components))}
	pricingVersions := make([]string, 0, len(components))
	for _, component := range components {
		quoter, ok := component.component.(productimage.CapabilityUsageQuoter)
		if !ok || quoter == nil {
			return quotedSlotExecution{}, fmt.Errorf("%w: %s capability does not provide a conservative quote", imageagent.ErrBudgetQuoteUnavailable, component.name)
		}
		capability, quoteErr := quoter.QuoteUsage(ctx, productimage.CapabilityUsageQuoteRequest{Operation: component.name, InputFingerprint: inputFingerprint})
		if quoteErr != nil {
			return quotedSlotExecution{}, fmt.Errorf("%w: quote %s: %v", imageagent.ErrBudgetQuoteUnavailable, component.name, quoteErr)
		}
		if capability.Operation == "" {
			capability.Operation = component.name
		}
		if capability.Operation != component.name || capability.Fingerprint == "" || capability.MaximumOutputs <= 0 || capability.MaximumModelCalls < 0 || capability.MaximumCostMicros < 0 {
			return quotedSlotExecution{}, fmt.Errorf("%w: %s capability returned an invalid quote", imageagent.ErrBudgetQuoteUnavailable, component.name)
		}
		if policy.CostMicros.Enabled && !capability.CostUpperBoundKnown {
			return quotedSlotExecution{}, fmt.Errorf("%w: %s capability has no trustworthy cost upper bound", imageagent.ErrBudgetQuoteUnavailable, component.name)
		}
		maximum := imageagent.UsageVector{Images: capability.MaximumOutputs, AgentSteps: 1, ModelCalls: capability.MaximumModelCalls}
		if capability.CostUpperBoundKnown {
			maximum.CostMicros = capability.MaximumCostMicros
		}
		public := imageagent.SlotUsageOperation{
			Name: component.name, Provider: capability.Provider, Model: capability.Model,
			PricingVersion: capability.PricingVersion, Fingerprint: capability.Fingerprint,
			Maximum: maximum, MaximumOutputs: capability.MaximumOutputs,
		}
		quoted.quote.Maximum, err = imageagent.CheckedAddUsage(quoted.quote.Maximum, maximum)
		if err != nil {
			return quotedSlotExecution{}, err
		}
		quoted.operations = append(quoted.operations, quotedSlotOperation{public: public, capability: capability})
		quoted.quote.Operations = append(quoted.quote.Operations, public)
		pricingVersions = append(pricingVersions, capability.PricingVersion)
	}
	quoted.quote.PricingVersion = strings.Join(pricingVersions, "+")
	fingerprintPayload := struct {
		InputFingerprint string
		Operations       []imageagent.SlotUsageOperation
		PricingVersion   string
	}{inputFingerprint, quoted.quote.Operations, quoted.quote.PricingVersion}
	encoded, err := json.Marshal(fingerprintPayload)
	if err != nil {
		return quotedSlotExecution{}, err
	}
	sum := sha256.Sum256(encoded)
	quoted.quote.Fingerprint = hex.EncodeToString(sum[:])
	if err := imageagent.ValidateSlotUsageQuote(quoted.quote); err != nil {
		return quotedSlotExecution{}, err
	}
	return quoted, nil
}

func (e *ProductImageSlotExecutor) QuoteSlot(ctx context.Context, input imageagent.SlotExecutionInput, policy imageagent.BudgetPolicy) (imageagent.SlotUsageQuote, error) {
	quoted, err := e.quoteSlot(ctx, input, policy)
	if err != nil {
		return imageagent.SlotUsageQuote{}, err
	}
	return quoted.quote, nil
}

func (e *ProductImageSlotExecutor) GenerateQuotedSlot(ctx context.Context, input imageagent.SlotExecutionInput, expected imageagent.SlotUsageQuote) (imageagent.SlotGeneratedOutput, error) {
	if err := imageagent.ValidateSlotUsageQuote(expected); err != nil {
		return imageagent.SlotGeneratedOutput{}, &imageagent.ProviderDispatchError{State: imageagent.ProviderNotDispatched, Err: err}
	}
	quoted, err := e.quoteSlot(ctx, input, imageagent.BudgetPolicy{})
	if err != nil {
		return imageagent.SlotGeneratedOutput{}, &imageagent.ProviderDispatchError{State: imageagent.ProviderNotDispatched, Err: err}
	}
	if quoted.quote.Fingerprint != expected.Fingerprint {
		return imageagent.SlotGeneratedOutput{}, &imageagent.ProviderDispatchError{State: imageagent.ProviderNotDispatched, Err: imageagent.ErrRevisionConflict}
	}
	return e.generateSlot(ctx, input, &quoted)
}

func (e *ProductImageSlotExecutor) ExecuteSlot(ctx context.Context, input imageagent.SlotExecutionInput) (imageagent.SlotExecutionResult, error) {
	generated, err := e.GenerateSlot(ctx, input)
	if err != nil {
		return imageagent.SlotExecutionResult{}, err
	}
	return e.PublishSlot(ctx, input, generated)
}

func (e *ProductImageSlotExecutor) GenerateSlot(ctx context.Context, input imageagent.SlotExecutionInput) (imageagent.SlotGeneratedOutput, error) {
	return e.generateSlot(ctx, input, nil)
}

func (e *ProductImageSlotExecutor) generateSlot(ctx context.Context, input imageagent.SlotExecutionInput, quoted *quotedSlotExecution) (imageagent.SlotGeneratedOutput, error) {
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
	var receipt imageagent.SlotUsageReceipt
	switch slot.Role {
	case imageagent.SlotRoleMain:
		assets, receipt, err = e.executeMain(ctx, source, productContext, quoted)
	case imageagent.SlotRoleScene, imageagent.SlotRoleDetail, imageagent.SlotRoleSellingPoint:
		assets, receipt, err = e.executeScene(ctx, source, productContext, quoted)
	case imageagent.SlotRoleSize:
		if source.Width <= 0 || source.Height <= 0 {
			return imageagent.SlotGeneratedOutput{}, fmt.Errorf("size slot %q requires reliable dimensions", slot.ID)
		}
		assets, receipt, err = e.executeScene(ctx, source, productContext, quoted)
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
	return imageagent.SlotGeneratedOutput{SlotID: slot.ID, Attempt: input.Attempt, SourceAssetID: sourceAssetID, Assets: generated, UsageReceipt: receipt}, nil
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
	if strings.TrimSpace(input.TenantID) == "" || strings.TrimSpace(input.UserID) == "" {
		return imageagent.SlotExecutionResult{}, imageagent.ErrValidation
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
		if err := imageagent.ValidatePublishedAssetRefForSlot(input, asset, index); err != nil {
			return imageagent.SlotExecutionResult{}, err
		}
		if asset.SourceAssetID != sourceAssetID {
			return imageagent.SlotExecutionResult{}, imageagent.ErrRevisionConflict
		}
		candidates[index] = imageagent.AssetCandidate{
			AssetID:       durableCandidateAssetID(input, slot, asset, index),
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

func (e *ProductImageSlotExecutor) executeMain(ctx context.Context, source productimage.ImageAsset, productContext *productimage.ProductContext, quoted *quotedSlotExecution) ([]productimage.ImageAsset, imageagent.SlotUsageReceipt, error) {
	if e.dependencies.SubjectExtractor == nil {
		return nil, imageagent.SlotUsageReceipt{}, fmt.Errorf("subject extractor is required for main slot")
	}
	if e.dependencies.WhiteBackgroundRenderer == nil {
		return nil, imageagent.SlotUsageReceipt{}, fmt.Errorf("white background renderer is required for main slot")
	}
	subjectCtx := operationQuoteContext(ctx, quoted, 0)
	subject, err := e.dependencies.SubjectExtractor.Extract(subjectCtx, source.URL, productContext)
	if err != nil {
		return nil, imageagent.SlotUsageReceipt{}, dispatchedError("extract subject", err)
	}
	if subject == nil || strings.TrimSpace(subject.URL) == "" {
		return nil, imageagent.SlotUsageReceipt{}, dispatchedContractError("subject extractor returned no generated asset")
	}
	whiteCtx := operationQuoteContext(ctx, quoted, 1)
	main, err := e.dependencies.WhiteBackgroundRenderer.Render(whiteCtx, subject, productContext)
	if err != nil {
		return nil, imageagent.SlotUsageReceipt{}, dispatchedError("render white background", err)
	}
	if main == nil {
		return nil, imageagent.SlotUsageReceipt{}, dispatchedContractError("white background renderer returned no generated asset")
	}
	receipt := receiptForQuote(quoted, 2)
	return []productimage.ImageAsset{*main}, receipt, nil
}

func (e *ProductImageSlotExecutor) executeScene(ctx context.Context, source productimage.ImageAsset, productContext *productimage.ProductContext, quoted *quotedSlotExecution) ([]productimage.ImageAsset, imageagent.SlotUsageReceipt, error) {
	if e.dependencies.SceneRenderer == nil {
		return nil, imageagent.SlotUsageReceipt{}, fmt.Errorf("scene renderer is required")
	}
	assets, err := e.dependencies.SceneRenderer.Render(operationQuoteContext(ctx, quoted, 0), &source, productContext)
	if err != nil {
		return nil, imageagent.SlotUsageReceipt{}, dispatchedError("render scene", err)
	}
	if quoted != nil && int64(len(assets)) > quoted.operations[0].public.MaximumOutputs {
		return nil, imageagent.SlotUsageReceipt{}, dispatchedContractError(fmt.Sprintf("scene renderer returned %d outputs above quoted maximum %d", len(assets), quoted.operations[0].public.MaximumOutputs))
	}
	return assets, receiptForQuote(quoted, int64(len(assets))), nil
}

func operationQuoteContext(ctx context.Context, quoted *quotedSlotExecution, index int) context.Context {
	if quoted == nil || index < 0 || index >= len(quoted.operations) {
		return ctx
	}
	return productimage.WithCapabilityUsageQuote(ctx, quoted.operations[index].capability)
}

func receiptForQuote(quoted *quotedSlotExecution, actualImages int64) imageagent.SlotUsageReceipt {
	if quoted == nil {
		return imageagent.SlotUsageReceipt{}
	}
	receipt := imageagent.SlotUsageReceipt{Actual: quoted.quote.Maximum, CostBasis: imageagent.UsageCostReservedUpperBound}
	receipt.Actual.Images = actualImages
	for _, operation := range quoted.operations {
		if !operation.capability.CostUpperBoundKnown {
			receipt.Actual.CostMicros = 0
			receipt.CostBasis = imageagent.UsageCostUnavailable
			break
		}
	}
	return receipt
}

func dispatchedError(operation string, err error) error {
	var dispatch *imageagent.ProviderDispatchError
	if errors.As(err, &dispatch) {
		return fmt.Errorf("%s: %w", operation, err)
	}
	var capabilityDispatch *productimage.CapabilityDispatchError
	if errors.As(err, &capabilityDispatch) {
		state := imageagent.ProviderDispatchedUnknown
		switch capabilityDispatch.State {
		case productimage.CapabilityNotDispatched:
			state = imageagent.ProviderNotDispatched
		case productimage.CapabilityRejectedBeforeEffect:
			state = imageagent.ProviderRejectedBeforeEffect
		}
		return &imageagent.ProviderDispatchError{State: state, ProviderRequestIDs: append([]string(nil), capabilityDispatch.ProviderRequestIDs...), Err: fmt.Errorf("%s: %w", operation, err)}
	}
	return &imageagent.ProviderDispatchError{State: imageagent.ProviderDispatchedUnknown, Err: fmt.Errorf("%s: %w", operation, err)}
}

func dispatchedContractError(message string) error {
	return &imageagent.ProviderDispatchError{State: imageagent.ProviderDispatchedUnknown, Err: fmt.Errorf("%w: %s", imageagent.ErrProviderContractViolation, message)}
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
	TenantID            string `json:"tenant_id"`
	UserID              string `json:"user_id"`
	RunID               string `json:"run_id"`
	PlanRevision        int64  `json:"plan_revision"`
	SlotID              string `json:"slot_id"`
	SlotIdempotencyKey  string `json:"slot_idempotency_key"`
	InputIdempotencyKey string `json:"input_idempotency_key"`
	Attempt             int    `json:"attempt"`
	AssetIndex          int    `json:"asset_index"`
	ObjectKey           string `json:"object_key"`
	SHA256              string `json:"sha256"`
	SourceAssetID       string `json:"source_asset_id"`
}

func durableCandidateAssetID(input imageagent.SlotExecutionInput, slot imageagent.Slot, asset imageagent.PublishedAssetRef, assetIndex int) string {
	payload, err := json.Marshal(durableCandidateIdentity{
		TenantID: strings.TrimSpace(input.TenantID), UserID: strings.TrimSpace(input.UserID),
		RunID: strings.TrimSpace(input.RunID), PlanRevision: input.PlanRevision,
		SlotID: strings.TrimSpace(slot.ID), SlotIdempotencyKey: strings.TrimSpace(slot.IdempotencyKey),
		InputIdempotencyKey: strings.TrimSpace(input.IdempotencyKey), Attempt: input.Attempt,
		AssetIndex: assetIndex, ObjectKey: asset.ObjectKey, SHA256: asset.SHA256, SourceAssetID: asset.SourceAssetID,
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
