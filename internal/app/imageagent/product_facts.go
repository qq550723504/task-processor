package imageagentapp

import (
	"encoding/json"
	"strings"
	"task-processor/internal/agentconfig"
	"task-processor/internal/imageagent"
	"task-processor/internal/product/catalog"
)

// Source IDs are supplied by each existing source owner. Acquisition keeps
// Catalog indexes; Supply keeps its publication-scoped source-image IDs.
func PrepareImageProduct(contextID string, original, effective catalog.PublishedSnapshot, applyID string, assets []imageagent.AuthorizedAsset) (imageagent.ImageSetPreparation, error) {
	if original.Identity != effective.Identity || original.Version == 0 || effective.Version < original.Version || original.PublicationID == "" || contextID == "" || len(assets) == 0 || (effective.Version > original.Version) != (applyID != "") {
		return imageagent.ImageSetPreparation{}, imageagent.ErrRevisionConflict
	}
	attributes := map[string]string{}
	for _, attribute := range effective.Snapshot.Attributes {
		if name := strings.TrimSpace(attribute.Name); name != "" {
			attributes[name] = strings.TrimSpace(attribute.Value)
		}
	}
	resolved, err := imageagent.NormalizeAssetCatalog(imageagent.AssetCatalog{ProductContext: imageagent.ProductContextRef{ProductID: original.Identity.ProductKey, Title: effective.Snapshot.Title, ProductType: strings.Join(effective.Snapshot.CategoryPath, " / "), Attributes: attributes, SourceSnapshotVersion: effective.Version}, Assets: assets})
	if err != nil {
		return imageagent.ImageSetPreparation{}, err
	}
	facts, err := ImageProductEvidence(effective.Snapshot)
	if err != nil {
		return imageagent.ImageSetPreparation{}, err
	}
	return imageagent.ImageSetPreparation{Source: imageagent.ImageSourceBinding{ProductID: original.Identity.ProductKey, OperationID: contextID, OriginalPublicationID: original.PublicationID, OriginalVersion: original.Version, EffectiveVersion: effective.Version, ApplyReceiptID: applyID, CatalogHash: resolved.Manifest.Hash}, Catalog: resolved, Evidence: facts}, nil
}

func ImageProductEvidence(product catalog.ProductSnapshot) (map[string]string, error) {
	evidence := map[string]string{}
	putJSON := func(key string, value any) error {
		raw, err := json.Marshal(value)
		if err != nil {
			return err
		}
		evidence[key] = string(raw)
		return nil
	}
	if product.Brand != "" {
		evidence["brand"] = product.Brand
	}
	if product.Description != "" {
		evidence["description"] = product.Description
	}
	if len(product.SellingPoints) > 0 {
		if err := putJSON("selling_points", product.SellingPoints); err != nil {
			return nil, err
		}
	}
	if len(product.Variants) > 0 {
		// URLs and prices are not additional generation references or claims.
		type variantFacts struct {
			ID, Title, SKU string
			Attributes     []catalog.Attribute
		}
		variants := make([]variantFacts, 0, len(product.Variants))
		for _, variant := range product.Variants {
			variants = append(variants, variantFacts{variant.SourceID, variant.Title, variant.SKU, variant.Attributes})
		}
		if err := putJSON("variants", variants); err != nil {
			return nil, err
		}
	}
	if spec := product.Specifications; spec != nil {
		meaningful := spec.Dimensions != nil && (spec.Dimensions.Length > 0 || spec.Dimensions.Width > 0 || spec.Dimensions.Height > 0) && strings.TrimSpace(spec.Dimensions.Unit) != "" || spec.Weight != nil && spec.Weight.Value > 0 && strings.TrimSpace(spec.Weight.Unit) != "" || spec.Package != nil && spec.Package.Quantity > 0
		technical := map[string]string{}
		for name, value := range spec.Technical {
			if strings.TrimSpace(value) == "" {
				continue
			}
			if name == "instructions" || name == "accessories" {
				evidence[name] = value
			} else {
				technical[name] = value
			}
		}
		if meaningful || len(technical) > 0 {
			copy := *spec
			copy.Technical = technical
			if err := putJSON("specifications", copy); err != nil {
				return nil, err
			}
		}
	}
	total := 0
	for _, value := range evidence {
		total += len(value)
		if !agentconfig.ValidSetText(value, 8<<10) {
			return nil, imageagent.ErrValidation
		}
	}
	if total > 8<<10 {
		return nil, imageagent.ErrValidation
	}
	return evidence, nil
}
