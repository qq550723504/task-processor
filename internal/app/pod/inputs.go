package podapp

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"gorm.io/gorm"
	"image"
	"net/http"
	"reflect"
	supplyapp "task-processor/internal/app/supplychain"
	"task-processor/internal/integration/httpimage"
	catalogstore "task-processor/internal/integration/persistence/product/catalog"
	collectionstore "task-processor/internal/integration/persistence/product/collection"
	reviewstore "task-processor/internal/integration/persistence/product/review"
	"task-processor/internal/product/asset"
	"task-processor/internal/product/catalog"
	"task-processor/internal/product/collection"
	"task-processor/internal/product/pod"
	"task-processor/internal/product/review"
	"task-processor/internal/product/supplymarket"
	"time"
)

type DesignExecutionAuthorizer interface {
	AuthorizePODDesign(context.Context, collection.Scope) error
}
type OriginalInputs struct {
	RequestAuthorization collection.Authorizer
	Collections          *collection.Service
	Authorization        collection.ExecutionAuthorizer
	DesignAuthorization  DesignExecutionAuthorizer
	Approvals            asset.ApprovalCommitReader
	Snapshots            catalog.VersionedSnapshotReader
	Applied              review.AppliedPublicationLookup
	ImageHTTP            *http.Client
}

func (r OriginalInputs) effective(ctx context.Context, scope collection.Scope, input pod.InputReference, version uint64, apply string) (catalog.PublishedSnapshot, error) {
	original, e := r.Snapshots.GetSnapshot(ctx, catalog.SnapshotIdentity{TenantID: scope.OrganizationID, ProductKey: input.Source.ProductKey}, input.Source.Version)
	if e != nil || original.PublicationID != input.Source.PublicationID || original.Version != input.Source.Version {
		return catalog.PublishedSnapshot{}, pod.ErrConflict
	}
	if version == original.Version && apply == "" {
		return original, nil
	}
	if version <= original.Version || !collection.ValidID(apply) || r.Applied == nil {
		return catalog.PublishedSnapshot{}, pod.ErrConflict
	}
	current, e := r.Snapshots.GetSnapshot(ctx, original.Identity, version)
	if e != nil {
		return catalog.PublishedSnapshot{}, pod.ErrConflict
	}
	lineage, e := review.ResolveAppliedSnapshot(ctx, review.Scope{Org: scope.OrganizationID, Actor: scope.ActorID}, current, r.Snapshots, r.Applied)
	if e != nil || len(lineage.Applied) == 0 || lineage.Applied[0].ProposalID != apply || !reflect.DeepEqual(lineage.Original, original) {
		return catalog.PublishedSnapshot{}, pod.ErrConflict
	}
	return current, nil
}
func inputSelection(input pod.InputReference) collection.SelectionInput {
	return collection.SelectionInput{ItemID: input.ItemID, ExpectedRevision: input.Revision, OriginalPublicationID: input.Source.PublicationID, OriginalVersion: input.Source.Version}
}
func (r OriginalInputs) requestInput(ctx context.Context, scope collection.Scope, input pod.InputReference) (collection.ItemDetail, error) {
	selected, e := r.Collections.Select(ctx, inputSelection(input))
	if e != nil {
		return collection.ItemDetail{}, pod.ErrForbidden
	}
	actual, e := selected.Scope(ctx)
	if e != nil || actual != scope {
		return collection.ItemDetail{}, pod.ErrForbidden
	}
	d, e := selected.Read(ctx)
	if e != nil || d.Item.Source != input.Source {
		return d, pod.ErrConflict
	}
	return d, nil
}
func (r OriginalInputs) ReadSourceSelection(ctx context.Context, in asset.SourceSelectionRequest) (asset.SourceSelection, error) {
	if ctx == nil || r.RequestAuthorization == nil || in.TargetPlatform != "sds" {
		return asset.SourceSelection{}, asset.ErrSourceApprovalForbidden
	}
	expected, err := r.RequestAuthorization.Authorize(ctx, supplymarket.PermissionDesign)
	if err != nil {
		return asset.SourceSelection{}, asset.ErrSourceApprovalForbidden
	}
	managed, err := r.RequestAuthorization.Authorize(ctx, collection.PermissionManage)
	if err != nil || managed != expected {
		return asset.SourceSelection{}, asset.ErrSourceApprovalForbidden
	}
	owner, e := r.Collections.AuthorizeOwner(ctx)
	if e != nil {
		return asset.SourceSelection{}, e
	}
	scope, e := owner.Scope(ctx)
	if e != nil || scope != expected {
		return asset.SourceSelection{}, asset.ErrSourceApprovalForbidden
	}
	d, e := r.Collections.ReadItem(ctx, in.ItemID)
	if e != nil || d.Item.Source.Kind == "sds_template" || d.Item.Source.PublicationID != in.OriginalPublicationID || d.Item.Source.Version != in.OriginalSnapshotVersion {
		return asset.SourceSelection{}, asset.ErrSourceApprovalForbidden
	}
	input := pod.InputReference{ItemID: d.Item.ID, Revision: d.Item.Revision, Source: d.Item.Source}
	if _, e = r.requestInput(ctx, scope, input); e != nil {
		return asset.SourceSelection{}, e
	}
	original, e := r.effective(ctx, scope, input, in.OriginalSnapshotVersion, "")
	if e != nil {
		return asset.SourceSelection{}, e
	}
	if _, e = r.effective(ctx, scope, input, in.EffectiveCatalogVersion, in.ApplyReceiptID); e != nil {
		return asset.SourceSelection{}, e
	}
	return asset.SourceSelection{TenantID: scope.OrganizationID, ActorID: scope.ActorID, MemberID: scope.MemberID, ItemID: d.Item.ID, ProductKey: d.Item.Source.ProductKey, OriginalPublicationID: in.OriginalPublicationID, OriginalSnapshotVersion: in.OriginalSnapshotVersion, EffectiveCatalogVersion: in.EffectiveCatalogVersion, TargetPlatform: "sds", Images: supplyapp.SourceImages(original)}, nil
}
func (r OriginalInputs) approved(ctx context.Context, scope collection.Scope, ref pod.ArtworkReference) (asset.ApprovedAsset, error) {
	commit, e := r.Approvals.ReadApprovalCommit(ctx, scope.OrganizationID, ref.ActionID)
	if e != nil || asset.ValidateApprovalCommit(commit) != nil || commit.TenantID != scope.OrganizationID || commit.ProductKey != ref.Input.Source.ProductKey || commit.TargetPlatform != "sds" || commit.SourceSnapshotVersion != ref.Version {
		return asset.ApprovedAsset{}, pod.ErrForbidden
	}
	if _, e = r.effective(ctx, scope, ref.Input, ref.Version, ref.ApplyReceiptID); e != nil {
		return asset.ApprovedAsset{}, e
	}
	for _, a := range commit.Assets {
		if a.ID == ref.AssetID && a.Role == asset.RoleDesign {
			if a.SourceApproval != nil && (a.SourceApproval.ActorID != scope.ActorID || a.SourceApproval.MemberID != scope.MemberID || a.SourceApproval.OriginalPublicationID != ref.Input.Source.PublicationID || a.SourceApproval.OriginalSnapshotVersion != ref.Input.Source.Version) {
				return asset.ApprovedAsset{}, pod.ErrForbidden
			}
			return a, nil
		}
	}
	return asset.ApprovedAsset{}, pod.ErrForbidden
}
func (r OriginalInputs) bytes(ctx context.Context, scope collection.Scope, ref pod.ArtworkReference) ([]byte, pod.ArtworkReference, error) {
	approved, e := r.approved(ctx, scope, ref)
	if e != nil {
		return nil, ref, e
	}
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	raw, e := httpimage.Download(ctx, r.ImageHTTP, approved.URL, pod.MaxArtworkBytes)
	if e != nil {
		return nil, ref, pod.ErrUnavailable
	}
	mime, w, h, e := httpimage.InspectGeneratedArtifact(raw)
	if e != nil || mime != "image/jpeg" && mime != "image/png" || w < 1 || h < 1 || w > 10000 || h > 10000 || int64(w)*int64(h) > 40000000 {
		return nil, ref, pod.ErrInvalid
	}
	decoded, _, e := image.Decode(bytes.NewReader(raw))
	if e != nil || decoded.Bounds().Dx() != w || decoded.Bounds().Dy() != h {
		return nil, ref, pod.ErrInvalid
	}
	sum := sha256.Sum256(raw)
	hash := hex.EncodeToString(sum[:])
	if ref.Hash != "" && (ref.Hash != hash || ref.Bytes != int64(len(raw)) || ref.Width != w || ref.Height != h || ref.MediaType != mime) {
		return nil, ref, pod.ErrConflict
	}
	ref.Hash = hash
	ref.Bytes = int64(len(raw))
	ref.Width = w
	ref.Height = h
	ref.MediaType = mime
	return raw, ref, nil
}
func (r OriginalInputs) Guard(ctx context.Context, tx *gorm.DB, p pod.Plan) error {
	if r.Authorization == nil || r.DesignAuthorization == nil || r.DesignAuthorization.AuthorizePODDesign(ctx, p.Scope) != nil || r.Authorization.AuthorizeExecution(ctx, p.Scope, collection.PermissionManage) != nil {
		return pod.ErrForbidden
	}
	snapshots, e := catalogstore.NewBoundedSnapshotReader(tx, 2<<20)
	if e != nil {
		return pod.ErrUnavailable
	}
	applied, e := reviewstore.NewAppliedPublicationReader(tx)
	if e != nil {
		return pod.ErrUnavailable
	}
	r.Snapshots = snapshots
	r.Applied = applied
	inputs := []pod.InputReference{p.TemplateItem, p.Artwork.Input}
	if inputs[1].ItemID < inputs[0].ItemID {
		inputs[0], inputs[1] = inputs[1], inputs[0]
	}
	for _, input := range inputs {
		owner, e := (collection.ExecutionOwnerAuthority{Authorization: r.Authorization}).AuthorizeExecutionOwner(ctx, p.Scope)
		if e != nil {
			return pod.ErrForbidden
		}
		if _, e = collectionstore.LockPrivateSource(ctx, tx, owner, input.ItemID, input.Revision, input.Source); e != nil {
			return e
		}
	}
	template, e := r.effective(ctx, p.Scope, p.TemplateItem, p.TemplateItem.Source.Version, "")
	if e != nil {
		return e
	}
	if !templateVariant(template.Snapshot, p.Template.ParentID, p.Template.VariantID) {
		return pod.ErrConflict
	}
	if _, e = r.approved(ctx, p.Scope, p.Artwork); e != nil {
		return e
	}
	return nil
}
func templateVariant(s catalog.ProductSnapshot, parent, variant string) bool {
	found := false
	for _, a := range s.Attributes {
		if a.Name == "sds_parent_id" && a.Value == parent {
			found = true
		}
	}
	if !found {
		return false
	}
	for _, v := range s.Variants {
		if v.SourceID == variant {
			for _, a := range v.Attributes {
				if a.Name == "sds_variant_id" && a.Value == variant {
					return true
				}
			}
		}
	}
	return false
}
