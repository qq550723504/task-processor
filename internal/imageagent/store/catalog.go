package store

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"gorm.io/gorm"

	"task-processor/internal/imageagent"
)

func (r *gormRepository) SaveAssetCatalog(ctx context.Context, scope imageagent.RunScope, catalog imageagent.AssetCatalog) error {
	if err := validateScope(scope); err != nil {
		return err
	}
	wanted := catalogRows(scope, catalog)
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if _, err := r.findRunForUpdate(ctx, tx, scope); err != nil {
			return err
		}
		var existing []assetCatalogRecord
		if err := tx.Where("tenant_id = ? AND run_id = ?", scope.TenantID, scope.RunID).Order("id ASC").Find(&existing).Error; err != nil {
			return fmt.Errorf("load image agent asset catalog: %w", err)
		}
		if len(existing) > 0 {
			if sameCatalogRows(existing, wanted) {
				return nil
			}
			return imageagent.ErrRevisionConflict
		}
		if len(wanted) == 0 {
			return nil
		}
		if err := tx.Create(&wanted).Error; err != nil {
			return fmt.Errorf("save image agent asset catalog: %w", err)
		}
		return nil
	})
}

func (r *gormRepository) GetAssetCatalog(ctx context.Context, scope imageagent.RunScope) (imageagent.AssetCatalog, error) {
	if err := validateScope(scope); err != nil {
		return imageagent.AssetCatalog{}, err
	}
	if _, err := r.findRun(ctx, r.db, scope); err != nil {
		return imageagent.AssetCatalog{}, err
	}
	var rows []assetCatalogRecord
	if err := r.db.WithContext(ctx).Where("tenant_id = ? AND run_id = ?", scope.TenantID, scope.RunID).Order("id ASC").Find(&rows).Error; err != nil {
		return imageagent.AssetCatalog{}, fmt.Errorf("load image agent asset catalog: %w", err)
	}
	assets := make([]imageagent.AuthorizedAsset, 0, len(rows))
	for _, row := range rows {
		assets = append(assets, imageagent.AuthorizedAsset{ID: row.ID, Type: imageagent.AuthorizedAssetType(row.Type), DisplayURL: row.DisplayURL, Label: row.Label, Width: row.Width, Height: row.Height})
	}
	return imageagent.AssetCatalog{Assets: assets}, nil
}

func catalogRows(scope imageagent.RunScope, catalog imageagent.AssetCatalog) []assetCatalogRecord {
	rows := make([]assetCatalogRecord, 0, len(catalog.Assets))
	for _, asset := range catalog.Assets {
		rows = append(rows, assetCatalogRecord{TenantID: scope.TenantID, RunID: scope.RunID, ID: strings.TrimSpace(asset.ID), Type: string(asset.Type), DisplayURL: strings.TrimSpace(asset.DisplayURL), Label: strings.TrimSpace(asset.Label), Width: asset.Width, Height: asset.Height})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].ID < rows[j].ID })
	return rows
}

func sameCatalogRows(left, right []assetCatalogRecord) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i].TenantID != right[i].TenantID || left[i].RunID != right[i].RunID || left[i].ID != right[i].ID || left[i].Type != right[i].Type || left[i].DisplayURL != right[i].DisplayURL || left[i].Label != right[i].Label || left[i].Width != right[i].Width || left[i].Height != right[i].Height {
			return false
		}
	}
	return true
}
