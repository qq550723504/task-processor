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
	normalized, err := imageagent.NormalizeAssetCatalog(catalog)
	if err != nil {
		return err
	}
	wanted, err := catalogRows(scope, normalized)
	if err != nil {
		return err
	}
	wantedManifest, err := catalogManifestRow(scope, normalized)
	if err != nil {
		return err
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if _, err := r.findRunForUpdate(ctx, tx, scope); err != nil {
			return err
		}
		var existingManifest assetCatalogManifestRecord
		manifestErr := tx.Where("tenant_id = ? AND owner_user_id = ? AND run_id = ?", scope.TenantID, scope.OwnerUserID, scope.RunID).First(&existingManifest).Error
		var existing []assetCatalogRecord
		if err := tx.Where("tenant_id = ? AND owner_user_id = ? AND run_id = ?", scope.TenantID, scope.OwnerUserID, scope.RunID).Order("id ASC").Find(&existing).Error; err != nil {
			return fmt.Errorf("load image agent asset catalog: %w", err)
		}
		if manifestErr == nil {
			if existingManifest.Version == wantedManifest.Version && existingManifest.Hash == wantedManifest.Hash && string(existingManifest.ProductContextJSON) == string(wantedManifest.ProductContextJSON) && sameCatalogRows(existing, wanted) {
				return nil
			}
			return imageagent.ErrRevisionConflict
		}
		if manifestErr != nil && manifestErr != gorm.ErrRecordNotFound {
			return manifestErr
		}
		if err := tx.Create(&wantedManifest).Error; err != nil {
			return fmt.Errorf("save image agent asset catalog manifest: %w", err)
		}
		if err := tx.Create(&wanted).Error; err != nil {
			if len(wanted) == 0 {
				return nil
			}
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
	var manifest assetCatalogManifestRecord
	if err := r.db.WithContext(ctx).Where("tenant_id = ? AND owner_user_id = ? AND run_id = ?", scope.TenantID, scope.OwnerUserID, scope.RunID).First(&manifest).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return imageagent.AssetCatalog{}, imageagent.ErrCatalogSnapshotMissing
		}
		return imageagent.AssetCatalog{}, fmt.Errorf("load image agent asset catalog manifest: %w", err)
	}
	var rows []assetCatalogRecord
	if err := r.db.WithContext(ctx).Where("tenant_id = ? AND owner_user_id = ? AND run_id = ?", scope.TenantID, scope.OwnerUserID, scope.RunID).Order("id ASC").Find(&rows).Error; err != nil {
		return imageagent.AssetCatalog{}, fmt.Errorf("load image agent asset catalog: %w", err)
	}
	var productContext imageagent.ProductContextRef
	if len(manifest.ProductContextJSON) > 0 {
		if err := unmarshalJSON(manifest.ProductContextJSON, &productContext); err != nil {
			return imageagent.AssetCatalog{}, err
		}
	}
	assets := make([]imageagent.AuthorizedAsset, 0, len(rows))
	for _, row := range rows {
		var metadata map[string]string
		if err := unmarshalJSON(row.MetadataJSON, &metadata); err != nil {
			return imageagent.AssetCatalog{}, err
		}
		assets = append(assets, imageagent.AuthorizedAsset{ID: row.ID, Type: imageagent.AuthorizedAssetType(row.Type), URL: row.URL, SourceURL: row.SourceURL, DisplayURL: row.DisplayURL, Label: row.Label, Width: row.Width, Height: row.Height, Metadata: metadata})
	}
	return imageagent.NormalizeAssetCatalog(imageagent.AssetCatalog{Manifest: imageagent.CatalogManifest{Version: manifest.Version, Hash: manifest.Hash, CreatedAt: manifest.CreatedAt}, Assets: assets, ProductContext: productContext})
}

func catalogRows(scope imageagent.RunScope, catalog imageagent.AssetCatalog) ([]assetCatalogRecord, error) {
	rows := make([]assetCatalogRecord, 0, len(catalog.Assets))
	for _, asset := range catalog.Assets {
		metadata, err := marshalJSON(asset.Metadata)
		if err != nil {
			return nil, err
		}
		rows = append(rows, assetCatalogRecord{TenantID: scope.TenantID, OwnerUserID: scope.OwnerUserID, RunID: scope.RunID, ID: strings.TrimSpace(asset.ID), Type: string(asset.Type), URL: strings.TrimSpace(asset.URL), SourceURL: strings.TrimSpace(asset.SourceURL), DisplayURL: strings.TrimSpace(asset.DisplayURL), Label: strings.TrimSpace(asset.Label), Width: asset.Width, Height: asset.Height, MetadataJSON: metadata})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].ID < rows[j].ID })
	return rows, nil
}

func catalogManifestRow(scope imageagent.RunScope, catalog imageagent.AssetCatalog) (assetCatalogManifestRecord, error) {
	var productContextJSON []byte
	var err error
	if !imageagent.ProductContextRefIsZero(catalog.ProductContext) {
		productContextJSON, err = marshalJSON(catalog.ProductContext)
		if err != nil {
			return assetCatalogManifestRecord{}, err
		}
	}
	return assetCatalogManifestRecord{TenantID: scope.TenantID, OwnerUserID: scope.OwnerUserID, RunID: scope.RunID, Version: catalog.Manifest.Version, Hash: catalog.Manifest.Hash, ProductContextJSON: productContextJSON, CreatedAt: catalog.Manifest.CreatedAt}, nil
}

func sameCatalogRows(left, right []assetCatalogRecord) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i].TenantID != right[i].TenantID || left[i].OwnerUserID != right[i].OwnerUserID || left[i].RunID != right[i].RunID || left[i].ID != right[i].ID || left[i].Type != right[i].Type || left[i].URL != right[i].URL || left[i].SourceURL != right[i].SourceURL || left[i].DisplayURL != right[i].DisplayURL || left[i].Label != right[i].Label || left[i].Width != right[i].Width || left[i].Height != right[i].Height || string(left[i].MetadataJSON) != string(right[i].MetadataJSON) {
			return false
		}
	}
	return true
}
