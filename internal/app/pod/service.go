package podapp

import (
	"context"
	supplyapp "task-processor/internal/app/supplychain"
	"task-processor/internal/authidentity"
	podstore "task-processor/internal/integration/persistence/product/pod"
	"task-processor/internal/integration/sds"
	"task-processor/internal/listing/submission"
	"task-processor/internal/product/asset"
	"task-processor/internal/product/collection"
	"task-processor/internal/product/pod"
	"task-processor/internal/product/sourcing"
	"task-processor/internal/product/supplymarket"
	"time"

	"gorm.io/gorm"
)

type Starter interface {
	Ensure(context.Context, Execution) error
}
type ProductReceiver func(context.Context, *gorm.DB, collection.Authorizer, collection.Scope, string, string, string, sourcing.SourceEnvelope) (collection.Receipt, error)
type Service struct {
	Authorization  collection.Authorizer
	Repository     *podstore.Repository
	Inputs         OriginalInputs
	Templates      Templates
	Credentials    sds.CredentialSource
	Approvals      *asset.SourceApprovalService
	Processor      *Processor
	Starter        Starter
	Intents        submission.ExecutionIntentReader
	ReceiveProduct ProductReceiver
}
type DesignRequest struct {
	TemplateItem     pod.InputReference `json:"templateItem"`
	VariantID        string             `json:"variantId"`
	ManifestHash     string             `json:"manifestHash"`
	ArtworkHash      string             `json:"artworkHash"`
	ArtworkItem      pod.InputReference `json:"artworkItem"`
	EffectiveVersion uint64             `json:"effectiveVersion,string"`
	ApplyReceiptID   string             `json:"applyReceiptId,omitempty"`
	ActionID         string             `json:"actionId"`
	AssetID          string             `json:"assetId"`
	Transforms       []pod.Transform    `json:"transforms"`
	Name             string             `json:"name"`
}
type Progress struct {
	ID        string        `json:"id"`
	Name      string        `json:"name"`
	State     string        `json:"state"`
	CreatedAt time.Time     `json:"createdAt"`
	Finished  *FinishedView `json:"finished,omitempty"`
}
type FinishedView struct {
	ID     string   `json:"id"`
	Number string   `json:"number"`
	Images []string `json:"images"`
}
type ManifestView struct {
	Manifest pod.TemplateManifest `json:"manifest"`
	Hash     string               `json:"hash"`
}

func (s *Service) scope(ctx context.Context, permission string) (collection.Scope, error) {
	if ctx == nil || s == nil || s.Authorization == nil {
		return collection.Scope{}, pod.ErrUnavailable
	}
	scope, e := s.Authorization.Authorize(ctx, permission)
	if e != nil || scope.Validate() != nil {
		return collection.Scope{}, pod.ErrForbidden
	}
	if permission != supplymarket.PermissionRead {
		managed, e := s.Authorization.Authorize(ctx, collection.PermissionManage)
		if e != nil || managed != scope {
			return collection.Scope{}, pod.ErrForbidden
		}
	}
	return scope, nil
}
func (s *Service) guard(ctx context.Context, scope collection.Scope, permission string) error {
	current, e := s.scope(ctx, permission)
	if e != nil || current != scope {
		return pod.ErrForbidden
	}
	return nil
}
func (s *Service) binding(ctx context.Context) (pod.AccountBinding, error) {
	c, e := s.Credentials.Current(ctx)
	if e != nil || !authidentity.IsBoundedIdentifier(c.BindingID) || !authidentity.IsBoundedIdentifier(c.Revision) || c.MerchantID == "" {
		return pod.AccountBinding{}, pod.ErrUnavailable
	}
	return pod.AccountBinding{ID: c.BindingID, Revision: c.Revision, MerchantID: c.MerchantID, ProtocolRevision: pod.ProtocolRevision}, nil
}
func (s *Service) ListTemplates(ctx context.Context, page, size int, keyword string) (pod.TemplatePage, error) {
	scope, e := s.scope(ctx, supplymarket.PermissionRead)
	if e != nil {
		return pod.TemplatePage{}, e
	}
	result, e := s.Templates.List(ctx, page, size, keyword)
	if e != nil {
		return result, e
	}
	return result, s.guard(ctx, scope, supplymarket.PermissionRead)
}
func (s *Service) Template(ctx context.Context, id string) (pod.Template, error) {
	scope, e := s.scope(ctx, supplymarket.PermissionRead)
	if e != nil {
		return pod.Template{}, e
	}
	result, e := s.Templates.Detail(ctx, id)
	if e != nil {
		return result, e
	}
	return result, s.guard(ctx, scope, supplymarket.PermissionRead)
}
func (s *Service) Manifest(ctx context.Context, itemID, variant string) (ManifestView, error) {
	scope, e := s.scope(ctx, supplymarket.PermissionDesign)
	if e != nil {
		return ManifestView{}, e
	}
	d, e := s.Inputs.Collections.ReadItem(ctx, itemID)
	if e != nil || d.Item.Source.Kind != "sds_template" {
		return ManifestView{}, pod.ErrNotFound
	}
	input := pod.InputReference{ItemID: itemID, Revision: d.Item.Revision, Source: d.Item.Source}
	if _, e = s.Inputs.requestInput(ctx, scope, input); e != nil {
		return ManifestView{}, e
	}
	parent := ""
	for _, a := range d.Product.Attributes {
		if a.Name == "sds_parent_id" {
			parent = a.Value
		}
	}
	if !templateVariant(d.Product, parent, variant) {
		return ManifestView{}, pod.ErrConflict
	}
	b, e := s.binding(ctx)
	if e != nil {
		return ManifestView{}, e
	}
	m, e := s.Templates.Manifest(ctx, b, parent, variant)
	if e != nil {
		return ManifestView{}, e
	}
	if e = s.guard(ctx, scope, supplymarket.PermissionDesign); e != nil {
		return ManifestView{}, e
	}
	return ManifestView{m, collection.Digest(m)}, nil
}
func (s *Service) Create(ctx context.Context, key string, in DesignRequest) (Progress, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	scope, e := s.scope(ctx, supplymarket.PermissionDesign)
	if e != nil {
		return Progress{}, e
	}
	if !collection.ValidID(key) || len(in.ManifestHash) != 64 || len(in.ArtworkHash) != 64 || len(in.Transforms) < 1 || len(in.Transforms) > 16 {
		return Progress{}, pod.ErrInvalid
	}
	id := collection.StableID(scope.OrganizationID, scope.ActorID, "pod-design", key)
	hash := collection.Digest(in)
	if existing, e := s.Repository.ByKey(ctx, scope, key); e == nil {
		o, e := s.Repository.Begin(ctx, key, hash, existing.Plan, func(ctx context.Context, _ *gorm.DB, _ pod.Plan) error {
			return s.guard(ctx, scope, supplymarket.PermissionDesign)
		})
		if e != nil {
			return Progress{}, e
		}
		return s.project(ctx, o), nil
	} else if e != pod.ErrNotFound {
		return Progress{}, e
	}
	template, e := s.Inputs.requestInput(ctx, scope, in.TemplateItem)
	if e != nil {
		return Progress{}, e
	}
	if in.TemplateItem.Source.Kind != "sds_template" {
		return Progress{}, pod.ErrInvalid
	}
	parent := ""
	for _, a := range template.Product.Attributes {
		if a.Name == "sds_parent_id" {
			parent = a.Value
		}
	}
	if !templateVariant(template.Product, parent, in.VariantID) {
		return Progress{}, pod.ErrConflict
	}
	if _, e = s.Inputs.requestInput(ctx, scope, in.ArtworkItem); e != nil {
		return Progress{}, e
	}
	b, e := s.binding(ctx)
	if e != nil {
		return Progress{}, e
	}
	manifest, e := s.Templates.Manifest(ctx, b, parent, in.VariantID)
	if e != nil || collection.Digest(manifest) != in.ManifestHash {
		return Progress{}, pod.ErrConflict
	}
	artwork := pod.ArtworkReference{Input: in.ArtworkItem, Version: in.EffectiveVersion, ApplyReceiptID: in.ApplyReceiptID, ActionID: in.ActionID, AssetID: in.AssetID}
	_, artwork, e = s.Inputs.bytes(ctx, scope, artwork)
	if e != nil {
		return Progress{}, e
	}
	if artwork.Hash != in.ArtworkHash {
		return Progress{}, pod.ErrConflict
	}
	plan := pod.Plan{OperationID: id, Scope: scope, Binding: b, Template: manifest, TemplateItem: in.TemplateItem, Artwork: artwork, Transforms: in.Transforms, Name: in.Name}
	if e = plan.Validate(); e != nil {
		return Progress{}, e
	}
	o, e := s.Repository.Begin(ctx, key, hash, plan, s.Inputs.Guard)
	if e != nil {
		return Progress{}, e
	}
	if s.guard(ctx, scope, supplymarket.PermissionDesign) != nil {
		return Progress{}, pod.ErrForbidden
	}
	return s.project(ctx, o), nil
}
func progress(o pod.Operation) Progress {
	state := "QUEUED"
	if o.Object != nil {
		state = "MATERIAL_PENDING"
	}
	if o.Material != nil {
		state = "DESIGN_PENDING"
	}
	if o.Intent != nil {
		state = "VERIFYING"
	}
	result := Progress{ID: o.Plan.OperationID, Name: o.Plan.Name, State: state, CreatedAt: o.CreatedAt}
	if o.Finished != nil {
		result.State = "SAVED"
		result.Finished = &FinishedView{ID: o.Finished.ID, Number: o.Finished.KeyID, Images: append([]string{}, o.Finished.RenderURLs...)}
	}
	return result
}
func (s *Service) project(ctx context.Context, o pod.Operation) Progress {
	result := progress(o)
	if o.Finished != nil {
		return result
	}
	if s.Intents == nil {
		result.State = "UNKNOWN"
		return result
	}
	if s.ensureUnstarted(ctx, o) != nil {
		result.State = "UNKNOWN"
		return result
	}
	for _, step := range []string{pod.StepOSS, pod.StepMaterial, pod.StepSync} {
		attempt, e := s.Intents.ReadIntent(ctx, submission.ExecutionScope{OrganizationID: o.Plan.Scope.OrganizationID}, pod.StepIntent(o.Plan.OperationID, step))
		if e != nil {
			if e == submission.ErrExecutionNotFound {
				continue
			}
			result.State = "UNKNOWN"
			return result
		}
		if attempt.Status == submission.ExecutionOutcomeUnknown || attempt.Status == submission.ExecutionFailedDefinitive || attempt.Status == submission.ExecutionCancelled || attempt.Status == submission.ExecutionClaimed && !time.Now().Before(attempt.LeaseExpiresAt) {
			result.State = "UNKNOWN"
			return result
		}
	}
	return result
}

type ArtworkOption struct {
	ID     string `json:"id"`
	URL    string `json:"url"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}
type ArtworkChoice struct {
	Item      pod.InputReference           `json:"item"`
	Selection asset.SourceSelectionRequest `json:"selection"`
	Images    []ArtworkOption              `json:"images"`
}

func (s *Service) Artwork(ctx context.Context, id string) (ArtworkChoice, error) {
	scope, e := s.scope(ctx, supplymarket.PermissionDesign)
	if e != nil {
		return ArtworkChoice{}, e
	}
	d, e := s.Inputs.Collections.ReadItem(ctx, id)
	if e != nil || d.Item.Source.Kind == "sds_template" {
		return ArtworkChoice{}, pod.ErrNotFound
	}
	input := pod.InputReference{ItemID: d.Item.ID, Revision: d.Item.Revision, Source: d.Item.Source}
	if _, e = s.Inputs.requestInput(ctx, scope, input); e != nil {
		return ArtworkChoice{}, e
	}
	original, e := s.Inputs.effective(ctx, scope, input, input.Source.Version, "")
	if e != nil {
		return ArtworkChoice{}, e
	}
	choice := ArtworkChoice{Item: input, Selection: asset.SourceSelectionRequest{ItemID: id, OriginalPublicationID: input.Source.PublicationID, OriginalSnapshotVersion: input.Source.Version, EffectiveCatalogVersion: input.Source.Version, TargetPlatform: "sds"}, Images: []ArtworkOption{}}
	images := supplyapp.SourceImages(original)
	if len(images) > 256 {
		return choice, pod.ErrUnavailable
	}
	for _, i := range images {
		choice.Images = append(choice.Images, ArtworkOption{i.ID, i.URL, i.Width, i.Height})
	}
	return choice, s.guard(ctx, scope, supplymarket.PermissionDesign)
}
func (s *Service) Operation(ctx context.Context, id string, verify bool) (Progress, error) {
	scope, e := s.scope(ctx, supplymarket.PermissionRead)
	if e != nil {
		return Progress{}, e
	}
	o, e := s.Repository.Read(ctx, scope, id)
	if e != nil {
		return Progress{}, e
	}
	if verify {
		if s.guard(ctx, scope, supplymarket.PermissionDesign) != nil {
			return Progress{}, pod.ErrForbidden
		}
		_, _ = s.Processor.Observe(ctx, Execution{scope, id})
		o, e = s.Repository.Read(ctx, scope, id)
		if e != nil {
			return Progress{}, e
		}
	}
	if e = s.guard(ctx, scope, supplymarket.PermissionRead); e != nil {
		return Progress{}, e
	}
	return s.project(ctx, o), nil
}
func (s *Service) ByKey(ctx context.Context, key string) (Progress, error) {
	scope, e := s.scope(ctx, supplymarket.PermissionRead)
	if e != nil {
		return Progress{}, e
	}
	o, e := s.Repository.ByKey(ctx, scope, key)
	if e != nil {
		return Progress{}, e
	}
	if e = s.guard(ctx, scope, supplymarket.PermissionRead); e != nil {
		return Progress{}, e
	}
	return s.project(ctx, o), nil
}

// SQL commit may outlive a lost Temporal start response. Original queries may
// ensure only that fixed workflow while no send attempt exists. Once an attempt
// exists (including UNKNOWN), queries never initiate execution again.
func (s *Service) ensureUnstarted(ctx context.Context, o pod.Operation) error {
	if o.Finished != nil || s.Intents == nil || s.Starter == nil {
		return nil
	}
	_, e := s.Intents.ReadIntent(ctx, submission.ExecutionScope{OrganizationID: o.Plan.Scope.OrganizationID}, pod.StepIntent(o.Plan.OperationID, pod.StepOSS))
	if e != submission.ErrExecutionNotFound {
		return nil
	}
	if s.guard(ctx, o.Plan.Scope, supplymarket.PermissionDesign) != nil {
		return pod.ErrUnknown
	}
	return s.Starter.Ensure(ctx, Execution{o.Plan.Scope, o.Plan.OperationID})
}
func (s *Service) ImportTemplate(ctx context.Context, key, id, digest string) (collection.Receipt, error) {
	scope, e := s.scope(ctx, supplymarket.PermissionSelect)
	if e != nil {
		return collection.Receipt{}, e
	}
	hash := collection.Digest([]string{"template", id, digest})
	return s.Repository.Import(ctx, scope, key, hash, "template", "", func(ctx context.Context, tx *gorm.DB, scope collection.Scope, operation string) (collection.Receipt, error) {
		t, e := s.Templates.Detail(ctx, id)
		if e != nil {
			return collection.Receipt{}, e
		}
		if digest != collection.Digest(t) {
			return collection.Receipt{}, pod.ErrConflict
		}
		envelope, e := pod.TemplateEnvelope(scope, operation, t)
		if e != nil {
			return collection.Receipt{}, e
		}
		if s.ReceiveProduct == nil {
			return collection.Receipt{}, pod.ErrUnavailable
		}
		return s.ReceiveProduct(ctx, tx, s.Authorization, scope, operation, "sds_template", "SDS 模板 · 未定制", envelope)
	}, func(ctx context.Context) error { return s.guard(ctx, scope, supplymarket.PermissionSelect) })
}
func (s *Service) ImportFinished(ctx context.Context, key, id string) (collection.Receipt, error) {
	scope, e := s.scope(ctx, supplymarket.PermissionSelect)
	if e != nil {
		return collection.Receipt{}, e
	}
	hash := collection.Digest([]string{"finished", id})
	return s.Repository.Import(ctx, scope, key, hash, "finished", id, func(ctx context.Context, tx *gorm.DB, scope collection.Scope, operation string) (collection.Receipt, error) {
		o, e := s.Repository.Read(ctx, scope, id)
		if e != nil || o.Finished == nil {
			return collection.Receipt{}, pod.ErrConflict
		}
		envelope, e := pod.FinishedEnvelope(scope, operation, o.Plan, *o.Finished)
		if e != nil {
			return collection.Receipt{}, e
		}
		if s.ReceiveProduct == nil {
			return collection.Receipt{}, pod.ErrUnavailable
		}
		return s.ReceiveProduct(ctx, tx, s.Authorization, scope, operation, "sds_finished", "SDS 定制成品", envelope)
	}, func(ctx context.Context) error { return s.guard(ctx, scope, supplymarket.PermissionSelect) })
}
func (s *Service) ImportByKey(ctx context.Context, key string) (collection.Receipt, error) {
	scope, e := s.scope(ctx, supplymarket.PermissionRead)
	if e != nil {
		return collection.Receipt{}, e
	}
	receipt, e := s.Repository.ReceiptByKey(ctx, scope, key)
	if e != nil {
		return receipt, e
	}
	return receipt, s.guard(ctx, scope, supplymarket.PermissionRead)
}

type ApprovalView struct {
	ActionID string            `json:"actionId"`
	AssetIDs []string          `json:"assetIds"`
	Image    ApprovedImageView `json:"image"`
}
type ApprovedImageView struct {
	URL    string `json:"url"`
	Hash   string `json:"hash"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}

func (s *Service) Approval(ctx context.Context, itemID, actionID string) (ApprovalView, error) {
	choice, e := s.Artwork(ctx, itemID)
	if e != nil {
		return ApprovalView{}, e
	}
	scope, e := s.scope(ctx, supplymarket.PermissionDesign)
	if e != nil {
		return ApprovalView{}, e
	}
	commit, e := s.Inputs.Approvals.ReadApprovalCommit(ctx, scope.OrganizationID, actionID)
	if e != nil {
		return ApprovalView{}, pod.ErrNotFound
	}
	if len(commit.Assets) != 1 {
		return ApprovalView{}, pod.ErrConflict
	}
	ref := pod.ArtworkReference{Input: choice.Item, Version: choice.Item.Source.Version, ActionID: actionID, AssetID: commit.Assets[0].ID}
	a, e := s.Inputs.approved(ctx, scope, ref)
	if e != nil {
		return ApprovalView{}, e
	}
	_, ref, e = s.Inputs.bytes(ctx, scope, ref)
	if e != nil {
		return ApprovalView{}, e
	}
	return ApprovalView{ActionID: actionID, AssetIDs: []string{ref.AssetID}, Image: ApprovedImageView{URL: a.URL, Hash: ref.Hash, Width: ref.Width, Height: ref.Height}}, s.guard(ctx, scope, supplymarket.PermissionDesign)
}
func (s *Service) Approve(ctx context.Context, in asset.SourceApprovalCommand) (ApprovalView, error) {
	_, e := s.scope(ctx, supplymarket.PermissionDesign)
	if e != nil {
		return ApprovalView{}, e
	}
	if len(in.Approved) != 0 || len(in.Images) != 1 || in.Images[0].Role != asset.RoleDesign || in.Selection.TargetPlatform != "sds" {
		return ApprovalView{}, pod.ErrInvalid
	}
	receipt, e := s.Approvals.Approve(ctx, in)
	if e != nil {
		return ApprovalView{}, e
	}
	return s.Approval(ctx, in.Selection.ItemID, receipt.ActionID)
}
