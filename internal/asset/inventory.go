package asset

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

type Origin string

const (
	OriginSource    Origin = "source"
	OriginDerived   Origin = "derived"
	OriginGenerated Origin = "generated"
)

type InventoryRef struct {
	TaskID     string `json:"task_id,omitempty"`
	ProductKey string `json:"product_key,omitempty"`
}

type Inventory struct {
	Ref        InventoryRef       `json:"ref,omitempty"`
	Records    []AssetRecord      `json:"records,omitempty"`
	Summary    *InventorySummary  `json:"summary,omitempty"`
	Review     *ReviewSummary     `json:"review,omitempty"`
	Compliance *ComplianceSummary `json:"compliance,omitempty"`
	Quality    *QualitySummary    `json:"quality,omitempty"`
	IPRisk     *IPRiskSummary     `json:"ip_risk,omitempty"`
	CreatedAt  time.Time          `json:"created_at,omitempty"`
	UpdatedAt  time.Time          `json:"updated_at,omitempty"`
}

type AssetRecord struct {
	ID           string            `json:"id,omitempty"`
	TaskID       string            `json:"task_id,omitempty"`
	ProductKey   string            `json:"product_key,omitempty"`
	Kind         Kind              `json:"kind,omitempty"`
	Origin       Origin            `json:"origin,omitempty"`
	Role         string            `json:"role,omitempty"`
	URL          string            `json:"url,omitempty"`
	Generator    string            `json:"generator,omitempty"`
	RecipeID     string            `json:"recipe_id,omitempty"`
	Version      *AssetVersion     `json:"version,omitempty"`
	Lineage      *AssetLineage     `json:"lineage,omitempty"`
	Operations   []string          `json:"operations,omitempty"`
	Labels       []string          `json:"labels,omitempty"`
	PlatformTags []string          `json:"platform_tags,omitempty"`
	Width        int               `json:"width,omitempty"`
	Height       int               `json:"height,omitempty"`
	Metadata     map[string]string `json:"metadata,omitempty"`
}

type AssetVersion struct {
	Number int    `json:"number"`
	Label  string `json:"label,omitempty"`
}

type AssetLineage struct {
	ParentAssetIDs []string `json:"parent_asset_ids,omitempty"`
	SourceAssetIDs []string `json:"source_asset_ids,omitempty"`
	Step           string   `json:"step,omitempty"`
}

type InventorySummary struct {
	TotalRecords     int      `json:"total_records"`
	SourceRecords    int      `json:"source_records"`
	DerivedRecords   int      `json:"derived_records"`
	GeneratedRecords int      `json:"generated_records"`
	RecipeCount      int      `json:"recipe_count"`
	SelectedCount    int      `json:"selected_count"`
	MainAssetID      string   `json:"main_asset_id,omitempty"`
	SelectedAssetIDs []string `json:"selected_asset_ids,omitempty"`
	Platforms        []string `json:"platforms,omitempty"`
}

func (i Inventory) Value() (driver.Value, error) {
	return json.Marshal(i)
}

func (i *Inventory) Scan(value any) error {
	var b []byte
	switch v := value.(type) {
	case []byte:
		b = v
	case string:
		b = []byte(v)
	default:
		return errors.New("type assertion to []byte failed")
	}
	return json.Unmarshal(b, i)
}

func BuildInventory(taskID string, bundle *Bundle) *Inventory {
	if bundle == nil {
		return nil
	}

	now := time.Now()
	inventory := &Inventory{
		Ref:        InventoryRef{TaskID: strings.TrimSpace(taskID)},
		Review:     bundle.Review,
		Compliance: bundle.Compliance,
		Quality:    bundle.Quality,
		IPRisk:     bundle.IPRisk,
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	sourceByURL := map[string]string{}
	for _, item := range bundle.Assets {
		record := AssetRecord{
			ID:           item.ID,
			TaskID:       strings.TrimSpace(taskID),
			Kind:         item.Kind,
			Origin:       originForAsset(item),
			Role:         item.Role,
			URL:          item.URL,
			Generator:    item.Generator,
			RecipeID:     item.RecipeID,
			Version:      &AssetVersion{Number: 1},
			Operations:   append([]string(nil), item.Operations...),
			Labels:       append([]string(nil), item.Labels...),
			PlatformTags: append([]string(nil), item.PlatformTags...),
			Width:        item.Width,
			Height:       item.Height,
			Metadata:     cloneMetadataMap(item.Metadata),
		}
		if record.Metadata == nil && item.SourceURL != "" {
			record.Metadata = map[string]string{}
		}
		if item.SourceURL != "" {
			record.Metadata["source_url"] = item.SourceURL
		}
		if record.Origin != OriginSource {
			record.Lineage = &AssetLineage{
				ParentAssetIDs: append([]string(nil), item.SourceAssetIDs...),
				SourceAssetIDs: append([]string(nil), item.SourceAssetIDs...),
				Step:           record.Generator,
			}
		}
		inventory.Records = append(inventory.Records, record)
		if item.Kind == KindSourceImage {
			sourceByURL[item.URL] = item.ID
		}
	}

	for i := range inventory.Records {
		record := &inventory.Records[i]
		if record.Origin == OriginSource {
			continue
		}
		if record.Lineage == nil {
			record.Lineage = &AssetLineage{Step: record.Generator}
		}
		if len(record.Lineage.SourceAssetIDs) == 0 {
			sourceIDs := sourceIDsForRecord(bundle.Assets, record.URL)
			if len(sourceIDs) == 0 {
				sourceIDs = sourceIDsForGenerator(bundle.Assets, sourceByURL)
			}
			record.Lineage.SourceAssetIDs = append([]string(nil), sourceIDs...)
			record.Lineage.ParentAssetIDs = append([]string(nil), sourceIDs...)
		}
	}

	inventory.Summary = buildInventorySummary(bundle.Selection, inventory.Records)
	return inventory
}

func InventorySummaryFromBundle(bundle *Bundle) *InventorySummary {
	if bundle == nil {
		return nil
	}
	inventory := BuildInventory("", bundle)
	if inventory == nil {
		return nil
	}
	return inventory.Summary
}

func RebuildInventorySummary(inventory *Inventory) *InventorySummary {
	if inventory == nil {
		return nil
	}
	return buildInventorySummary(nil, inventory.Records)
}

func RebuildBundleWithRecords(bundle *Bundle, records []AssetRecord) *Bundle {
	if bundle == nil {
		bundle = &Bundle{}
	}
	out := &Bundle{
		Assets:     append([]Asset(nil), bundle.Assets...),
		Selection:  bundle.Selection,
		Stats:      bundle.Stats,
		Review:     bundle.Review,
		Compliance: bundle.Compliance,
		Quality:    bundle.Quality,
		IPRisk:     bundle.IPRisk,
	}
	for _, record := range records {
		out.Assets = append(out.Assets, assetFromRecord(record, false))
	}
	out.Stats = rebuildBundleStats(out.Assets)
	return out
}

func RebuildBundleFromInventory(bundle *Bundle, inventory *Inventory) *Bundle {
	if inventory == nil {
		return bundle
	}
	out := &Bundle{}
	if bundle != nil {
		out.Selection = bundle.Selection
		out.Review = bundle.Review
		out.Compliance = bundle.Compliance
		out.Quality = bundle.Quality
		out.IPRisk = bundle.IPRisk
	}
	out.Assets = make([]Asset, 0, len(inventory.Records))
	for _, record := range inventory.Records {
		out.Assets = append(out.Assets, assetFromRecord(record, true))
	}
	out.Stats = rebuildBundleStats(out.Assets)
	return out
}

func buildInventorySummary(selection *Selection, records []AssetRecord) *InventorySummary {
	summary := &InventorySummary{
		TotalRecords: len(records),
	}
	for _, record := range records {
		switch record.Origin {
		case OriginSource:
			summary.SourceRecords++
		case OriginGenerated:
			summary.GeneratedRecords++
		default:
			summary.DerivedRecords++
		}
		if strings.TrimSpace(record.RecipeID) != "" {
			summary.RecipeCount++
		}
	}
	if selection != nil {
		summary.MainAssetID = selection.MainAssetID
		selected := make([]string, 0, 4+len(selection.GalleryAssetIDs)+len(selection.SourceAssetIDs))
		selected = append(selected, selection.MainAssetID, selection.WhiteBgAssetID, selection.SubjectCutoutAssetID)
		selected = append(selected, selection.GalleryAssetIDs...)
		selected = append(selected, selection.SourceAssetIDs...)
		summary.SelectedAssetIDs = uniqueStrings(selected)
		summary.SelectedCount = len(summary.SelectedAssetIDs)
	}
	return summary
}

func assetFromRecord(record AssetRecord, promoteSourceURL bool) Asset {
	item := Asset{
		ID:             record.ID,
		Kind:           record.Kind,
		URL:            record.URL,
		Role:           record.Role,
		Generator:      record.Generator,
		RecipeID:       record.RecipeID,
		SourceAssetIDs: sourceAssetIDsFromLineage(record.Lineage),
		Operations:     append([]string(nil), record.Operations...),
		Labels:         append([]string(nil), record.Labels...),
		PlatformTags:   append([]string(nil), record.PlatformTags...),
		Width:          record.Width,
		Height:         record.Height,
		Metadata:       cloneMetadataMap(record.Metadata),
	}
	if promoteSourceURL && item.Metadata != nil {
		item.SourceURL = item.Metadata["source_url"]
	}
	return item
}

func sourceAssetIDsFromLineage(lineage *AssetLineage) []string {
	if lineage == nil {
		return nil
	}
	return append([]string(nil), lineage.SourceAssetIDs...)
}

func rebuildBundleStats(items []Asset) *Stats {
	stats := &Stats{TotalAssets: len(items)}
	for _, item := range items {
		switch {
		case item.Kind == KindSourceImage:
			stats.SourceAssets++
		case item.Kind == KindCleanImage || item.Kind == KindDetailCrop || item.Kind == KindSceneImage || item.Kind == KindSellingPointImage || item.Kind == KindSizeSceneImage || item.Kind == KindModelImage:
			stats.GeneratedAssets++
		default:
			stats.DerivedAssets++
		}
	}
	return stats
}

func originForAsset(item Asset) Origin {
	switch item.Kind {
	case KindSourceImage:
		return OriginSource
	case KindCleanImage, KindDetailCrop, KindSceneImage, KindSellingPointImage, KindSizeSceneImage, KindModelImage:
		return OriginGenerated
	default:
		return OriginDerived
	}
}

func sourceIDsForRecord(items []Asset, url string) []string {
	for _, item := range items {
		if item.URL == url && item.Kind == KindSourceImage {
			return []string{item.ID}
		}
	}
	return nil
}

func sourceIDsForGenerator(items []Asset, sourceByURL map[string]string) []string {
	out := make([]string, 0, len(sourceByURL))
	for _, item := range items {
		if item.Kind != KindSourceImage {
			continue
		}
		out = append(out, item.ID)
	}
	return uniqueStrings(out)
}

func cloneMetadataMap(input map[string]string) map[string]string {
	if len(input) == 0 {
		return nil
	}
	out := make(map[string]string, len(input))
	for key, value := range input {
		out[key] = value
	}
	return out
}
