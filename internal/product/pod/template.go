package pod

import (
	"encoding/json"
	"net/url"
	"strconv"
	"task-processor/internal/product/collection"
	"task-processor/internal/product/sourcing"
	"time"
)

type Template struct {
	ID       string            `json:"id"`
	Name     string            `json:"name"`
	SKU      string            `json:"sku"`
	Images   []string          `json:"images"`
	Variants []TemplateVariant `json:"variants"`
}
type TemplateVariant struct {
	ID     string   `json:"id"`
	Name   string   `json:"name"`
	SKU    string   `json:"sku"`
	Size   string   `json:"size"`
	Color  string   `json:"color"`
	Type   string   `json:"type"`
	Images []string `json:"images"`
}
type TemplatePage struct {
	Items []Template `json:"items"`
	Total int64      `json:"total"`
	Page  int        `json:"page"`
	Size  int        `json:"size"`
}

func (t Template) Validate() error {
	if !remoteID(t.ID) || len(t.Name) == 0 || len(t.Name) > 1024 || len(t.SKU) > 128 || len(t.Images) < 1 || len(t.Images) > 32 || len(t.Variants) > 256 {
		return ErrUnknown
	}
	seen := map[string]bool{}
	for _, v := range t.Variants {
		if !remoteID(v.ID) || seen[v.ID] || len(v.Name) > 1024 || len(v.SKU) > 128 || len(v.Size) > 128 || len(v.Color) > 128 || len(v.Images) > 32 {
			return ErrUnknown
		}
		seen[v.ID] = true
		for _, im := range v.Images {
			if !templateImage(im) {
				return ErrUnknown
			}
		}
	}
	for _, im := range t.Images {
		if !templateImage(im) {
			return ErrUnknown
		}
	}
	return nil
}
func templateImage(s string) bool {
	u, e := url.Parse(s)
	return e == nil && u.Scheme == "https" && u.Host == "cdn.sdspod.com" && u.User == nil && u.Fragment == "" && u.RawQuery == "" && len(s) <= 2048
}

// Templates are retained as uncustomized source evidence. They carry no
// finished-good identity, role approval, or implicit production order.
func TemplateEnvelope(scope collection.Scope, operation string, t Template) (sourcing.SourceEnvelope, error) {
	if scope.Validate() != nil || !collection.ValidID(operation) || t.Validate() != nil {
		return sourcing.SourceEnvelope{}, ErrInvalid
	}
	raw, _ := json.Marshal(t)
	if len(raw) > 2<<20 {
		return sourcing.SourceEnvelope{}, ErrInvalid
	}
	id := collection.StableID(scope.OrganizationID, scope.ActorID, operation, "sds_template", t.ID)
	e := sourcing.SourceEnvelope{Identity: sourcing.SourceIdentity{SourceType: sourcing.SourceTypeWarehouseCatalog, SourcePlatform: "sds", SourceID: id, SourceVersion: "1"}, RawReference: sourcing.RawSourceReference{ReferenceType: "sds_template", ReferenceID: t.ID, SnapshotID: id, Checksum: sourcing.RawSnapshotChecksum(string(raw)), CapturedAt: time.Now().UTC(), Metadata: map[string]string{"customization": "not_started"}}, Trace: sourcing.SourceTrace{SourceRunID: operation}, ProductCandidate: sourcing.ProductCandidate{Title: t.Name, Attributes: map[string]string{"sds_parent_id": t.ID, "customization": "not_started"}}}
	for n, im := range t.Images {
		e.AssetCandidates = append(e.AssetCandidates, sourcing.AssetCandidate{SourceID: collection.StableID(id, im), URL: im, MediaType: "image", Role: map[bool]string{true: "main", false: "detail"}[n == 0]})
	}
	for _, v := range t.Variants {
		candidate := sourcing.ProductVariantCandidate{SourceID: v.ID, Title: v.Name, SKU: v.SKU, Attributes: map[string]string{"sds_variant_id": v.ID, "size": v.Size, "color": v.Color, "sds_design_type": v.Type}}
		for _, im := range v.Images {
			candidate.Images = append(candidate.Images, sourcing.AssetCandidate{SourceID: collection.StableID(id, v.ID, im), URL: im, MediaType: "image", Role: "variant"})
		}
		e.ProductCandidate.Variants = append(e.ProductCandidate.Variants, candidate)
	}
	_, err := sourcing.ToSnapshot(e)
	return e, err
}
func FinishedEnvelope(scope collection.Scope, operation string, p Plan, f FinishedReference) (sourcing.SourceEnvelope, error) {
	if scope != p.Scope || !collection.ValidID(operation) || f.OperationID != p.OperationID || f.MerchantID != p.Binding.MerchantID || !remoteID(f.ID) || !hexDigest(f.EvidenceDigest) || len(f.RenderURLs) != len(p.Template.RenderFiles) {
		return sourcing.SourceEnvelope{}, ErrInvalid
	}
	id := collection.StableID(scope.OrganizationID, scope.ActorID, operation, "sds_finished", f.ID)
	raw, _ := json.Marshal(f)
	e := sourcing.SourceEnvelope{Identity: sourcing.SourceIdentity{SourceType: sourcing.SourceTypeWarehouseCatalog, SourcePlatform: "sds", SourceID: id, SourceVersion: "1"}, RawReference: sourcing.RawSourceReference{ReferenceType: "sds_finished", ReferenceID: f.ID, SnapshotID: id, Checksum: sourcing.RawSnapshotChecksum(string(raw)), CapturedAt: time.Now().UTC(), Metadata: map[string]string{"customization": "saved", "operation_id": p.OperationID, "template_id": p.Template.ParentID, "variant_id": p.Template.VariantID, "design_digest": f.EvidenceDigest}}, Trace: sourcing.SourceTrace{SourceRunID: operation}, ProductCandidate: sourcing.ProductCandidate{Title: p.Name, Attributes: map[string]string{"sds_finished_id": f.ID, "sds_finished_number": f.KeyID, "customization": "saved"}}}
	for n, im := range f.RenderURLs {
		if _, ok := renderURL(im, false); !ok {
			return sourcing.SourceEnvelope{}, ErrInvalid
		}
		e.AssetCandidates = append(e.AssetCandidates, sourcing.AssetCandidate{SourceID: collection.StableID(id, strconv.Itoa(n)), URL: im, MediaType: "image", Role: map[bool]string{true: "main", false: "detail"}[n == 0]})
	}
	e.ProductCandidate.Variants = []sourcing.ProductVariantCandidate{{SourceID: p.Template.VariantID, SKU: f.KeyID, Title: p.Name, Attributes: map[string]string{"sds_variant_id": p.Template.VariantID}}}
	_, err := sourcing.ToSnapshot(e)
	return e, err
}
