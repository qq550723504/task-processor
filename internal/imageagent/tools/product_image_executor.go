package tools

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"task-processor/internal/imageagent"
	imagepolicy "task-processor/internal/marketplace/imagepolicy"
	productimage "task-processor/internal/product/image"
)

type ProfileResolver interface {
	Resolve(imagepolicy.ProfileInput) (imagepolicy.ProductImageProfile, error)
}

// Dependencies contains only provider-neutral capabilities and the exact
// marketplace policy resolver required by one ImageAgent slot.
type Dependencies struct {
	SubjectExtractor        productimage.SubjectExtractor
	WhiteBackgroundRenderer productimage.WhiteBackgroundRenderer
	SceneRenderer           productimage.SceneRenderer
	Reviewer                productimage.Reviewer
	UsageQuoter             productimage.UsageQuoter
	ProfileResolver         ProfileResolver
	LegacyAssetMaterializer LegacyAssetMaterializer
}

// LegacyAssetMaterializer bridges byte-producing providers to the frozen v2
// URL contract. It is intentionally absent from the v3 path, which has its
// own persisted staging protocol.
type LegacyAssetMaterializer interface {
	Materialize(context.Context, imageagent.SlotExecutionInput, int, imageagent.GeneratedAsset) (string, error)
}

type ProductImageSlotExecutor struct {
	dependencies Dependencies
	legacyV2     bool
}

func NewProductImageSlotExecutor(dependencies Dependencies) *ProductImageSlotExecutor {
	return &ProductImageSlotExecutor{dependencies: dependencies}
}

// NewFrozenV2ProductImageSlotExecutor preserves the historical V2 activity
// input contract. It deliberately does not require V3 policy fields or the
// V3 reviewer gate; new runs must use NewProductImageSlotExecutor instead.
func NewFrozenV2ProductImageSlotExecutor(dependencies Dependencies) *ProductImageSlotExecutor {
	return &ProductImageSlotExecutor{dependencies: dependencies, legacyV2: true}
}

type resolvedSlotInput struct {
	slot            imageagent.Slot
	source          productimage.Asset
	styles          []productimage.Asset
	product         productimage.ProductContext
	profile         imagepolicy.ProductImageProfile
	sourceAssetID   string
	styleReferences []string
	legacyPolicy    bool
}

type quotedSlotOperation struct {
	public     imageagent.SlotUsageOperation
	capability productimage.UsageQuote
}

type quotedSlotExecution struct {
	quote      imageagent.SlotUsageQuote
	operations []quotedSlotOperation
}

func (e *ProductImageSlotExecutor) QuoteSlot(ctx context.Context, input imageagent.SlotExecutionInput, policy imageagent.BudgetPolicy) (imageagent.SlotUsageQuote, error) {
	quoted, err := e.quoteSlot(ctx, input, policy)
	if err != nil {
		return imageagent.SlotUsageQuote{}, err
	}
	return quoted.quote, nil
}

func (e *ProductImageSlotExecutor) quoteSlot(ctx context.Context, input imageagent.SlotExecutionInput, policy imageagent.BudgetPolicy) (quotedSlotExecution, error) {
	resolved, err := e.resolveInput(input)
	if err != nil {
		return quotedSlotExecution{}, err
	}
	if e == nil || e.dependencies.UsageQuoter == nil {
		return quotedSlotExecution{}, fmt.Errorf("%w: image usage quoter is required", imageagent.ErrBudgetQuoteUnavailable)
	}
	operations, err := slotOperations(resolved.slot.Role, e.legacyV2 || resolved.legacyPolicy)
	if err != nil {
		return quotedSlotExecution{}, err
	}
	inputFingerprint := imageagent.SlotExecutionFingerprint(input)
	quoted := quotedSlotExecution{operations: make([]quotedSlotOperation, 0, len(operations))}
	pricingVersions := make([]string, 0, len(operations))
	for _, operation := range operations {
		request := productimage.UsageQuoteRequest{Operation: operation, InputFingerprint: inputFingerprint, MaximumOutputs: 1}
		capability, quoteErr := e.dependencies.UsageQuoter.QuoteUsage(ctx, request)
		if quoteErr != nil {
			return quotedSlotExecution{}, fmt.Errorf("%w: quote %s: %v", imageagent.ErrBudgetQuoteUnavailable, operation, quoteErr)
		}
		capability, quoteErr = productimage.NormalizeUsageAuthorization(capability, operation)
		if quoteErr != nil || capability.MaximumOutputs != 1 {
			return quotedSlotExecution{}, fmt.Errorf("%w: %s capability returned an invalid quote", imageagent.ErrBudgetQuoteUnavailable, operation)
		}
		if policy.CostMicros.Enabled && !capability.CostUpperBoundKnown {
			return quotedSlotExecution{}, fmt.Errorf("%w: %s capability has no trustworthy cost upper bound", imageagent.ErrBudgetQuoteUnavailable, operation)
		}
		maximum := imageagent.UsageVector{Images: 1, AgentSteps: 1, ModelCalls: capability.MaximumModelCalls}
		if capability.CostUpperBoundKnown {
			maximum.CostMicros = capability.MaximumCostMicros
		}
		public := imageagent.SlotUsageOperation{
			Name: operation, Provider: capability.Provider, Model: capability.Model,
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
	encoded, err := json.Marshal(struct {
		InputFingerprint string
		Policy           imagepolicy.ProductImageProfile
		Operations       []imageagent.SlotUsageOperation
		PricingVersion   string
	}{inputFingerprint, resolved.profile, quoted.quote.Operations, quoted.quote.PricingVersion})
	if err != nil {
		return quotedSlotExecution{}, err
	}
	digest := sha256.Sum256(encoded)
	quoted.quote.Fingerprint = hex.EncodeToString(digest[:])
	if err := imageagent.ValidateSlotUsageQuote(quoted.quote); err != nil {
		return quotedSlotExecution{}, err
	}
	return quoted, nil
}

func (e *ProductImageSlotExecutor) GenerateQuotedSlot(ctx context.Context, input imageagent.SlotExecutionInput, expected imageagent.SlotUsageQuote) (imageagent.SlotGeneratedOutput, error) {
	if err := imageagent.ValidateSlotUsageQuote(expected); err != nil {
		return imageagent.SlotGeneratedOutput{}, providerError(imageagent.ProviderNotDispatched, err)
	}
	quoted, err := e.quoteSlot(ctx, input, imageagent.BudgetPolicy{})
	if err != nil {
		return imageagent.SlotGeneratedOutput{}, providerError(imageagent.ProviderNotDispatched, err)
	}
	if quoted.quote.Fingerprint != expected.Fingerprint {
		return imageagent.SlotGeneratedOutput{}, providerError(imageagent.ProviderNotDispatched, imageagent.ErrRevisionConflict)
	}
	return e.generateSlot(ctx, input, &quoted)
}

func (e *ProductImageSlotExecutor) GenerateSlot(ctx context.Context, input imageagent.SlotExecutionInput) (imageagent.SlotGeneratedOutput, error) {
	return e.generateSlot(ctx, input, nil)
}

func (e *ProductImageSlotExecutor) generateSlot(ctx context.Context, input imageagent.SlotExecutionInput, quoted *quotedSlotExecution) (imageagent.SlotGeneratedOutput, error) {
	resolved, err := e.resolveInput(input)
	if err != nil {
		return imageagent.SlotGeneratedOutput{}, err
	}
	var candidates []productimage.Candidate
	var receipt imageagent.SlotUsageReceipt
	switch resolved.slot.Role {
	case imageagent.SlotRoleMain:
		candidates, receipt, err = e.generateMain(ctx, resolved, quoted)
	case imageagent.SlotRoleScene, imageagent.SlotRoleDetail, imageagent.SlotRoleSellingPoint, imageagent.SlotRoleSize:
		candidates, receipt, err = e.generateScene(ctx, resolved, quoted)
	default:
		err = fmt.Errorf("%w: unsupported slot role %q", imageagent.ErrValidation, resolved.slot.Role)
	}
	if err != nil {
		return imageagent.SlotGeneratedOutput{}, fmt.Errorf("execute slot %q: %w", resolved.slot.ID, err)
	}
	assets := make([]imageagent.GeneratedAsset, len(candidates))
	for index, candidate := range candidates {
		asset, mapErr := generatedAsset(candidate, resolved.source, resolved.styles)
		if mapErr != nil {
			return imageagent.SlotGeneratedOutput{}, mapErr
		}
		assets[index] = asset
	}
	output := imageagent.SlotGeneratedOutput{
		SlotID: resolved.slot.ID, Attempt: input.Attempt, SourceAssetID: resolved.sourceAssetID,
		Assets: assets, UsageReceipt: receipt,
	}
	if err := e.reviewGeneratedCandidates(ctx, resolved, candidates, quoted); err != nil {
		if e != nil && !e.legacyV2 && !resolved.legacyPolicy && errors.Is(err, imageagent.ErrReviewDecision) {
			return output, &imageagent.SlotReviewRequiredError{Output: output, Reason: err.Error(), Cause: err}
		}
		// Generation has already dispatched and produced durable candidate material.
		// Preserve it across reviewer transport failures so the activity can stage
		// the output and retry the read-only review step instead of losing work.
		return output, &imageagent.SlotReviewRequiredError{Output: output, Reason: err.Error(), Cause: err}
	}
	return output, nil
}

func (e *ProductImageSlotExecutor) reviewGeneratedCandidates(ctx context.Context, input resolvedSlotInput, candidates []productimage.Candidate, quoted *quotedSlotExecution) error {
	if e != nil && (e.legacyV2 || input.legacyPolicy) {
		return nil
	}
	if e == nil || e.dependencies.Reviewer == nil {
		return fmt.Errorf("%w: image reviewer is required", imageagent.ErrValidation)
	}
	review, err := e.dependencies.Reviewer.Review(ctx, productimage.ReviewRequest{
		Product: input.product, Sources: []productimage.Asset{input.source}, Candidates: candidates,
		Authorization: capabilityAuthorization(quoted, "review"),
	})
	if err != nil {
		return dispatchedCapabilityError("review image", err, true)
	}
	threshold := input.profile.Thresholds.MainReview
	if input.slot.Role == imageagent.SlotRoleMain {
		threshold = input.profile.Thresholds.WhiteBackgroundReview
	}
	if review.NeedsHumanReview || review.Score < threshold {
		return fmt.Errorf("%w: %w: score %.4f is below threshold %.4f", imageagent.ErrReviewDecision, imageagent.ErrValidation, review.Score, threshold)
	}
	return nil
}

func (e *ProductImageSlotExecutor) generateMain(ctx context.Context, input resolvedSlotInput, quoted *quotedSlotExecution) ([]productimage.Candidate, imageagent.SlotUsageReceipt, error) {
	if e.dependencies.SubjectExtractor == nil || e.dependencies.WhiteBackgroundRenderer == nil {
		return nil, imageagent.SlotUsageReceipt{}, fmt.Errorf("%w: main image capabilities are incomplete", imageagent.ErrValidation)
	}
	subject, err := e.dependencies.SubjectExtractor.Extract(ctx, productimage.ExtractRequest{
		Source: input.source, Product: input.product, Authorization: capabilityAuthorization(quoted, "extract_subject"),
	})
	if err != nil {
		return nil, imageagent.SlotUsageReceipt{}, dispatchedCapabilityError("extract subject", err, false)
	}
	white, err := e.dependencies.WhiteBackgroundRenderer.RenderWhiteBackground(ctx, productimage.RenderRequest{
		Source: input.source, Subject: subject, Product: input.product,
		Authorization: capabilityAuthorization(quoted, "render_white_background"),
	})
	if err != nil {
		return nil, imageagent.SlotUsageReceipt{}, dispatchedCapabilityError("render white background", err, true)
	}
	return []productimage.Candidate{white}, receiptForQuote(quoted, 2, []productimage.Candidate{subject, white}), nil
}

func (e *ProductImageSlotExecutor) generateScene(ctx context.Context, input resolvedSlotInput, quoted *quotedSlotExecution) ([]productimage.Candidate, imageagent.SlotUsageReceipt, error) {
	if e.dependencies.SceneRenderer == nil {
		return nil, imageagent.SlotUsageReceipt{}, fmt.Errorf("%w: scene renderer is required", imageagent.ErrValidation)
	}
	override := productimage.SceneOptions{
		SlotRole: string(input.slot.Role), SlotBrief: input.slot.Brief,
		StyleReferenceIDs: append([]string(nil), input.styleReferences...),
	}
	options, err := productimage.MergeSceneOptions(&input.profile.SceneDefaults, &override)
	if err != nil {
		return nil, imageagent.SlotUsageReceipt{}, err
	}
	candidates, err := e.dependencies.SceneRenderer.RenderScene(ctx, productimage.SceneRequest{
		Source: input.source, Product: input.product, Options: *options,
		StyleReferences: input.styles, MaximumOutputs: 1,
		Authorization: capabilityAuthorization(quoted, "render_scene"),
	})
	if err != nil {
		return nil, imageagent.SlotUsageReceipt{}, dispatchedCapabilityError("render scene", err, false)
	}
	if len(candidates) != 1 {
		return nil, imageagent.SlotUsageReceipt{}, providerError(imageagent.ProviderDispatchedUnknown, imageagent.ErrValidation)
	}
	return candidates, receiptForQuote(quoted, int64(len(candidates)), candidates), nil
}

func capabilityAuthorization(quoted *quotedSlotExecution, operation string) *productimage.UsageQuote {
	if quoted == nil {
		return nil
	}
	for _, candidate := range quoted.operations {
		if candidate.capability.Operation == operation {
			authorization := candidate.capability
			return &authorization
		}
	}
	return nil
}

func (e *ProductImageSlotExecutor) ExecuteSlot(ctx context.Context, input imageagent.SlotExecutionInput) (imageagent.SlotExecutionResult, error) {
	generated, err := e.GenerateSlot(ctx, input)
	if err != nil {
		return imageagent.SlotExecutionResult{}, err
	}
	return e.PublishSlot(ctx, input, generated)
}

// PublishSlot is retained only for the frozen v2 activity contract. Inline
// artifacts use the injected compatibility materializer; v3 uses its separate
// durable staging protocol.
func (e *ProductImageSlotExecutor) PublishSlot(ctx context.Context, input imageagent.SlotExecutionInput, generated imageagent.SlotGeneratedOutput) (imageagent.SlotExecutionResult, error) {
	resolved, err := e.resolveInput(input)
	if err != nil {
		return imageagent.SlotExecutionResult{}, err
	}
	if generated.SlotID != resolved.slot.ID || generated.Attempt != input.Attempt || generated.SourceAssetID != resolved.sourceAssetID || len(generated.Assets) == 0 {
		return imageagent.SlotExecutionResult{}, imageagent.ErrRevisionConflict
	}
	candidates := make([]imageagent.AssetCandidate, len(generated.Assets))
	for index, asset := range generated.Assets {
		url := strings.TrimSpace(asset.URL)
		if len(asset.Bytes) != 0 {
			if url != "" || e.dependencies.LegacyAssetMaterializer == nil {
				return imageagent.SlotExecutionResult{}, imageagent.ErrValidation
			}
			var materializeErr error
			url, materializeErr = e.dependencies.LegacyAssetMaterializer.Materialize(ctx, input, index, asset)
			if materializeErr != nil {
				return imageagent.SlotExecutionResult{}, fmt.Errorf("materialize v2 slot %q asset %d: %w", resolved.slot.ID, index, materializeErr)
			}
		}
		if _, err := imageagent.ValidateSafeImageURL(url); err != nil {
			return imageagent.SlotExecutionResult{}, err
		}
		candidates[index] = imageagent.AssetCandidate{
			AssetID: candidateAssetID(input, resolved.slot, index), URL: url,
			SourceAssetID: resolved.sourceAssetID, Metadata: cloneStringMap(asset.Metadata),
		}
	}
	return imageagent.SlotExecutionResult{SlotID: resolved.slot.ID, Attempt: input.Attempt, Candidates: candidates}, nil
}

func (e *ProductImageSlotExecutor) BuildSlotResult(_ context.Context, input imageagent.SlotExecutionInput, published imageagent.PublishedSlotOutput) (imageagent.SlotExecutionResult, error) {
	resolved, err := e.resolveInput(input)
	if err != nil {
		return imageagent.SlotExecutionResult{}, err
	}
	if published.SlotID != resolved.slot.ID || published.Attempt != input.Attempt {
		return imageagent.SlotExecutionResult{}, imageagent.ErrRevisionConflict
	}
	manifest, err := imageagent.NormalizeFinalManifest(imageagent.FinalManifest{Assets: published.Assets})
	if err != nil || len(manifest.Assets) == 0 || resolved.slot.Role == imageagent.SlotRoleMain && len(manifest.Assets) != 1 {
		return imageagent.SlotExecutionResult{}, imageagent.ErrValidation
	}
	candidates := make([]imageagent.AssetCandidate, len(manifest.Assets))
	for index, asset := range manifest.Assets {
		if err := imageagent.ValidatePublishedAssetRefForSlot(input, asset, index); err != nil || asset.SourceAssetID != resolved.sourceAssetID {
			return imageagent.SlotExecutionResult{}, imageagent.ErrRevisionConflict
		}
		candidates[index] = imageagent.AssetCandidate{
			AssetID: durableCandidateAssetID(input, resolved.slot, asset, index), SourceAssetID: asset.SourceAssetID,
			Width: asset.Width, Height: asset.Height, Operations: append([]string(nil), asset.Operations...),
			DurableAsset: imageagent.DurableAssetIdentity{ObjectKey: asset.ObjectKey, SHA256: asset.SHA256},
		}
	}
	return imageagent.SlotExecutionResult{SlotID: resolved.slot.ID, Attempt: input.Attempt, Candidates: candidates}, nil
}

func (e *ProductImageSlotExecutor) resolveInput(input imageagent.SlotExecutionInput) (resolvedSlotInput, error) {
	if e == nil {
		return resolvedSlotInput{}, fmt.Errorf("%w: image executor is required", imageagent.ErrValidation)
	}
	legacyPolicy := !e.legacyV2 && strings.TrimSpace(input.TargetPlatform) == "" && input.ImagePolicyContext == nil
	if !e.legacyV2 && !legacyPolicy && e.dependencies.ProfileResolver == nil {
		return resolvedSlotInput{}, fmt.Errorf("%w: image policy resolver is required", imageagent.ErrValidation)
	}
	if strings.TrimSpace(input.RunID) == "" || strings.TrimSpace(input.TenantID) == "" || strings.TrimSpace(input.UserID) == "" ||
		input.PlanRevision <= 0 || input.Attempt <= 0 || strings.TrimSpace(input.IdempotencyKey) == "" {
		return resolvedSlotInput{}, imageagent.ErrValidation
	}
	slot := cloneSlot(input.Slot)
	if strings.TrimSpace(slot.ID) == "" || strings.TrimSpace(slot.IdempotencyKey) == "" || len(slot.SourceAssetIDs) != 1 {
		return resolvedSlotInput{}, imageagent.ErrValidation
	}
	if !e.legacyV2 && !legacyPolicy && input.ImagePolicyContext == nil {
		return resolvedSlotInput{}, imageagent.ErrValidation
	}
	var profile imagepolicy.ProductImageProfile
	if input.ImagePolicyContext != nil && strings.TrimSpace(input.TargetPlatform) != "" {
		profileInput := imagepolicy.ProfileInput{
			Marketplace: strings.TrimSpace(input.TargetPlatform), Country: strings.TrimSpace(input.ImagePolicyContext.Country),
			Family: strings.TrimSpace(input.ImagePolicyContext.Family), SceneCategory: strings.TrimSpace(input.ImagePolicyContext.SceneCategory),
		}
		if err := imagepolicy.ValidateProfileInput(profileInput); err != nil {
			return resolvedSlotInput{}, err
		}
		if e.dependencies.ProfileResolver == nil {
			return resolvedSlotInput{}, fmt.Errorf("%w: image policy resolver is required", imageagent.ErrValidation)
		}
		var err error
		profile, err = e.dependencies.ProfileResolver.Resolve(profileInput)
		if err != nil {
			return resolvedSlotInput{}, err
		}
		if profile.Key != imagepolicy.PolicyKey(profileInput) || strings.TrimSpace(profile.PolicyVersion) == "" {
			return resolvedSlotInput{}, imageagent.ErrValidation
		}
	}
	sourceID := strings.TrimSpace(slot.SourceAssetIDs[0])
	source, err := authorizedProductAsset(input.AssetCatalog, sourceID, imageagent.AuthorizedAssetSource)
	if err != nil {
		return resolvedSlotInput{}, err
	}
	styles := make([]productimage.Asset, len(slot.StyleReferenceIDs))
	styleIDs := make([]string, len(slot.StyleReferenceIDs))
	seen := map[string]struct{}{sourceID: {}}
	for index, rawID := range slot.StyleReferenceIDs {
		id := strings.TrimSpace(rawID)
		if _, duplicate := seen[id]; id == "" || duplicate {
			return resolvedSlotInput{}, imageagent.ErrValidation
		}
		seen[id] = struct{}{}
		styles[index], err = authorizedProductAsset(input.AssetCatalog, id, imageagent.AuthorizedAssetStyle)
		if err != nil {
			return resolvedSlotInput{}, err
		}
		styleIDs[index] = id
	}
	product := productimage.ProductContext{
		ProductKey: strings.TrimSpace(input.ProductContext.ProductID), Title: strings.TrimSpace(input.ProductContext.Title),
		ProductType: strings.TrimSpace(input.ProductContext.ProductType), Attributes: cloneStringMap(input.ProductContext.Attributes),
	}
	if product.ProductKey == "" {
		return resolvedSlotInput{}, imageagent.ErrValidation
	}
	return resolvedSlotInput{
		slot: slot, source: source, styles: styles, product: product, profile: profile,
		sourceAssetID: sourceID, styleReferences: styleIDs, legacyPolicy: legacyPolicy,
	}, nil
}

func authorizedProductAsset(catalog imageagent.AssetCatalog, id string, kind imageagent.AuthorizedAssetType) (productimage.Asset, error) {
	var matched *imageagent.AuthorizedAsset
	for index := range catalog.Assets {
		candidate := &catalog.Assets[index]
		if strings.TrimSpace(candidate.ID) == id && candidate.Type == kind {
			if matched != nil {
				return productimage.Asset{}, imageagent.ErrValidation
			}
			matched = candidate
		}
	}
	if matched == nil {
		return productimage.Asset{}, imageagent.ErrValidation
	}
	url := strings.TrimSpace(matched.URL)
	if url == "" {
		url = strings.TrimSpace(matched.DisplayURL)
	}
	sourceURL := strings.TrimSpace(matched.SourceURL)
	if sourceURL == "" {
		sourceURL = url
	}
	validatedURL, err := imageagent.ValidateSafeImageURL(url)
	if err != nil {
		return productimage.Asset{}, err
	}
	validatedSourceURL, err := imageagent.ValidateSafeImageURL(sourceURL)
	if err != nil {
		return productimage.Asset{}, err
	}
	return productimage.Asset{
		URL: validatedURL, SourceURL: validatedSourceURL, SourceAssetID: id, Role: productimage.RoleSource,
		Width: matched.Width, Height: matched.Height, Operations: []string{"ingest"},
	}, nil
}

func generatedAsset(candidate productimage.Candidate, source productimage.Asset, styles []productimage.Asset) (imageagent.GeneratedAsset, error) {
	asset := candidate.Asset
	if asset.SourceAssetID != source.SourceAssetID || asset.Role == productimage.RoleSource || len(asset.Operations) == 0 ||
		asset.Width <= 0 || asset.Height <= 0 || strings.TrimSpace(asset.SourceURL) == "" {
		return imageagent.GeneratedAsset{}, imageagent.ErrValidation
	}
	if (asset.URL == "") == (asset.Bytes == nil) {
		return imageagent.GeneratedAsset{}, imageagent.ErrValidation
	}
	for _, forbidden := range append([]productimage.Asset{source}, styles...) {
		if asset.URL != "" && (strings.TrimSpace(asset.URL) == strings.TrimSpace(forbidden.URL) || strings.TrimSpace(asset.URL) == strings.TrimSpace(forbidden.SourceURL)) {
			return imageagent.GeneratedAsset{}, imageagent.ErrValidation
		}
	}
	for _, operation := range asset.Operations {
		if strings.Contains(operation, "pass_through") || strings.Contains(operation, "placeholder") || operation == "source" {
			return imageagent.GeneratedAsset{}, imageagent.ErrValidation
		}
	}
	if asset.URL != "" {
		if _, err := imageagent.ValidateSafeImageURL(asset.URL); err != nil {
			return imageagent.GeneratedAsset{}, err
		}
	} else if len(asset.Bytes) == 0 || len(asset.Bytes) > productimage.MaxInlineArtifactBytes || strings.TrimSpace(asset.MediaType) == "" {
		return imageagent.GeneratedAsset{}, imageagent.ErrValidation
	}
	metadata := cloneStringMap(candidate.Metadata.Values)
	return imageagent.GeneratedAsset{
		URL: asset.URL, Bytes: append([]byte(nil), asset.Bytes...), ContentType: asset.MediaType,
		SourceURL: asset.SourceURL, Operations: append([]string(nil), asset.Operations...),
		Width: asset.Width, Height: asset.Height, Metadata: metadata,
		ProviderReceiptID: candidate.Metadata.InvocationID,
	}, nil
}

func slotOperations(role imageagent.SlotRole, legacyV2 bool) ([]string, error) {
	switch role {
	case imageagent.SlotRoleMain:
		operations := []string{"extract_subject", "render_white_background"}
		if !legacyV2 {
			operations = append(operations, "review")
		}
		return operations, nil
	case imageagent.SlotRoleScene, imageagent.SlotRoleDetail, imageagent.SlotRoleSellingPoint, imageagent.SlotRoleSize:
		operations := []string{"render_scene"}
		if !legacyV2 {
			operations = append(operations, "review")
		}
		return operations, nil
	default:
		return nil, fmt.Errorf("%w: unsupported slot role %q", imageagent.ErrValidation, role)
	}
}

func receiptForQuote(quoted *quotedSlotExecution, actualImages int64, candidates []productimage.Candidate) imageagent.SlotUsageReceipt {
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
	for _, candidate := range candidates {
		if id := strings.TrimSpace(candidate.Metadata.InvocationID); id != "" {
			receipt.ProviderRequestIDs = append(receipt.ProviderRequestIDs, id)
		}
	}
	return receipt
}

func dispatchedCapabilityError(operation string, err error, priorEffect bool) error {
	state := imageagent.ProviderDispatchedUnknown
	if !priorEffect && (errors.Is(err, productimage.ErrInputInvalid) || errors.Is(err, productimage.ErrCapabilityUnsupported)) {
		state = imageagent.ProviderRejectedBeforeEffect
	}
	return providerError(state, fmt.Errorf("%s: %w", operation, err))
}

func providerError(state imageagent.ProviderDispatchState, err error) error {
	return &imageagent.ProviderDispatchError{State: state, Err: err}
}

func cloneSlot(slot imageagent.Slot) imageagent.Slot {
	cloned := slot
	cloned.SourceAssetIDs = append([]string(nil), slot.SourceAssetIDs...)
	cloned.StyleReferenceIDs = append([]string(nil), slot.StyleReferenceIDs...)
	return cloned
}

func cloneStringMap(values map[string]string) map[string]string {
	if values == nil {
		return nil
	}
	cloned := make(map[string]string, len(values))
	for key, value := range values {
		cloned[key] = value
	}
	return cloned
}

func candidateAssetID(input imageagent.SlotExecutionInput, slot imageagent.Slot, index int) string {
	encoded, _ := json.Marshal(struct {
		TenantID, RunID, SlotID string
		PlanRevision            int64
		Attempt, Index          int
	}{input.TenantID, input.RunID, slot.ID, input.PlanRevision, input.Attempt, index})
	digest := sha256.Sum256(encoded)
	return "imgcand_" + hex.EncodeToString(digest[:16])
}

func durableCandidateAssetID(input imageagent.SlotExecutionInput, slot imageagent.Slot, asset imageagent.PublishedAssetRef, index int) string {
	encoded, _ := json.Marshal(struct {
		TenantID, RunID, SlotID, ObjectKey, SHA256 string
		PlanRevision                               int64
		Attempt, Index                             int
	}{input.TenantID, input.RunID, slot.ID, asset.ObjectKey, asset.SHA256, input.PlanRevision, input.Attempt, index})
	digest := sha256.Sum256(encoded)
	return "imgcand_" + hex.EncodeToString(digest[:16])
}

var _ imageagent.SlotExecutor = (*ProductImageSlotExecutor)(nil)
var _ imageagent.RecoverableSlotExecutor = (*ProductImageSlotExecutor)(nil)
var _ imageagent.StagedSlotExecutor = (*ProductImageSlotExecutor)(nil)
var _ imageagent.BudgetedStagedSlotExecutor = (*ProductImageSlotExecutor)(nil)
