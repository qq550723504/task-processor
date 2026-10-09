package supplymarket

import (
	"encoding/json"
	"strconv"
	"task-processor/internal/product/collection"
	"task-processor/internal/product/sourcing"
)

// ReleasedEnvelope is evidence from one explicit immutable release. Private
// merchant identities, qualifications, source traces and approvals are absent.
// Each receiving member gets a different SourceID and retained source record.
func ReleasedEnvelope(scope collection.Scope, operation string, release Release) (sourcing.SourceEnvelope, error) {
	if scope.Validate() != nil || !collection.ValidID(operation) || !collection.ValidID(release.ID) || !release.Active || release.Revision < 1 || release.Channel != "official" && release.Channel != "selected" {
		return sourcing.SourceEnvelope{}, ErrInvalid
	}
	source := collection.StableID(scope.OrganizationID, scope.ActorID, "market", release.ID, strconv.FormatInt(release.Revision, 10), operation)
	raw, err := json.Marshal(struct {
		ID       string
		Revision int64
		Product  PublicProduct
		Supply   SupplyDeclaration
	}{release.ID, release.Revision, release.Product, release.Supply})
	if err != nil || len(raw) > 2<<20 {
		return sourcing.SourceEnvelope{}, ErrInvalid
	}
	e := sourcing.SourceEnvelope{Identity: sourcing.SourceIdentity{SourceType: sourcing.SourceTypeWarehouseCatalog, SourcePlatform: "supply_market", SourceID: source, SourceVersion: strconv.FormatInt(release.Revision, 10)}, RawReference: sourcing.RawSourceReference{ReferenceType: "supply_market_release", ReferenceID: release.ID, SnapshotID: release.ID + ":" + strconv.FormatInt(release.Revision, 10), Checksum: sourcing.RawSnapshotChecksum(string(raw)), CapturedAt: release.PublishedAt, Metadata: map[string]string{"channel": release.Channel}}, Trace: sourcing.SourceTrace{SourceRunID: operation}, ProductCandidate: sourcing.ProductCandidate{Title: release.Product.Title, Description: release.Product.Description, Brand: release.Product.Brand, Attributes: release.Product.Attributes}}
	for _, image := range release.Product.Images {
		if !publicImage(image) {
			return sourcing.SourceEnvelope{}, ErrInvalid
		}
		e.AssetCandidates = append(e.AssetCandidates, sourcing.AssetCandidate{SourceID: collection.StableID(source, image), URL: image, MediaType: "image", Role: map[bool]string{true: "main", false: "detail"}[len(e.AssetCandidates) == 0]})
	}
	for _, v := range release.Product.Variants {
		candidate := sourcing.ProductVariantCandidate{SourceID: v.SourceID, Title: v.Title, SKU: v.SKU, Attributes: v.Attributes, Currency: v.Currency, Price: v.Price, Stock: v.Stock}
		for _, image := range v.Images {
			if !publicImage(image) {
				return sourcing.SourceEnvelope{}, ErrInvalid
			}
			candidate.Images = append(candidate.Images, sourcing.AssetCandidate{SourceID: collection.StableID(source, v.SourceID, image), URL: image, MediaType: "image", Role: "variant"})
		}
		e.ProductCandidate.Variants = append(e.ProductCandidate.Variants, candidate)
	}
	snapshot, err := sourcing.ToSnapshot(e)
	if err != nil {
		return sourcing.SourceEnvelope{}, ErrInvalid
	}
	if _, err := PublicProjection(snapshot); err != nil {
		return sourcing.SourceEnvelope{}, err
	}
	return e, nil
}
