package listingkit

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"strings"

	"task-processor/internal/asset"
)

func buildAssetRenderAssetRevision(item asset.Asset) string {
	version := ""
	if item.Metadata != nil {
		version = item.Metadata["asset_revision"]
	}
	if strings.TrimSpace(version) != "" {
		return version
	}
	return hashRenderRevision(
		string(item.Kind),
		item.ID,
		item.URL,
		item.SourceURL,
		item.Generator,
		item.RecipeID,
		fmt.Sprintf("%dx%d", item.Width, item.Height),
	)
}

func buildAssetRenderPreviewRevision(item asset.Asset) string {
	if item.Metadata != nil {
		if version := strings.TrimSpace(item.Metadata["preview_revision"]); version != "" {
			return version
		}
	}
	previewSVG := ""
	renderOutputVersion := ""
	drawOutputVersion := ""
	drawPreviewVersion := ""
	if item.Metadata != nil {
		previewSVG = item.Metadata["layout_draw_preview_svg"]
		renderOutputVersion = item.Metadata["render_output_version"]
		drawOutputVersion = item.Metadata["draw_output_version"]
		drawPreviewVersion = item.Metadata["draw_preview_version"]
	}
	return hashRenderRevision(item.ID, previewSVG, renderOutputVersion, drawOutputVersion, drawPreviewVersion)
}

func buildTaskRevision(result *ListingKitResult) string {
	if result == nil {
		return ""
	}
	return hashRenderRevision(result.TaskID, result.UpdatedAt.UTC().Format("2006-01-02T15:04:05.999999999Z07:00"))
}

func hashRenderRevision(parts ...string) string {
	normalized := strings.Join(parts, "|")
	sum := sha1.Sum([]byte(normalized))
	return hex.EncodeToString(sum[:8])
}
