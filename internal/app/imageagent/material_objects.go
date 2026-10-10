package imageagentapp

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"strings"

	"task-processor/internal/imageagent"
	"task-processor/internal/imageagent/objectstore"
	"task-processor/internal/product/asset"
	productimage "task-processor/internal/product/image"
)

type GeneratedMaterialByteReader func(context.Context, asset.SourceSelection, asset.ApprovedAsset, int64) ([]byte, error)

type publishedMaterialObjects interface {
	PublicURL(string) string
	ReadObject(context.Context, string, int64) ([]byte, objectstore.ObjectInspection, error)
}

// Inputs are already verified by candidate facts or the immutable approval
// receipt in ImageSetService. This port cannot accept arbitrary keys or URLs.
type GeneratedMaterialReader struct {
	PublicBase string
	Objects    publishedMaterialObjects
}

func (r GeneratedMaterialReader) Read(ctx context.Context, source asset.SourceSelection, approved asset.ApprovedAsset, maximum int64) ([]byte, error) {
	if ctx == nil || r.Objects == nil || r.PublicBase == "" || maximum <= 0 || maximum > productimage.MaxInlineArtifactBytes || approved.GenerationEvidence == nil || approved.SourceApproval != nil || source.TenantID == "" || approved.RunID == "" || approved.SlotID == "" || approved.PlanRevision < 1 || approved.Attempt < 1 {
		return nil, asset.ErrInvalidApproval
	}
	key := strings.TrimPrefix(approved.URL, r.PublicBase+"/")
	identity, err := imageagent.NormalizeDurableAssetIdentity(imageagent.DurableAssetIdentity{ObjectKey: key, SHA256: approved.GenerationEvidence.ArtifactHash})
	if err != nil || identity.SHA256 != approved.GenerationEvidence.ArtifactHash || key == approved.URL || r.Objects.PublicURL(key) != approved.URL {
		return nil, asset.ErrInvalidApproval
	}
	parts := strings.Split(key, "/")
	if len(parts) != 9 || strings.Join(parts[:2], "/") != imageagent.PublishedArtifactPrefix || parts[2] != source.TenantID || parts[4] != approved.RunID || parts[5] != strconv.FormatInt(approved.PlanRevision, 10) || parts[6] != approved.SlotID || parts[7] != strconv.Itoa(approved.Attempt) {
		return nil, asset.ErrInvalidApproval
	}
	owner, err := hex.DecodeString(parts[3])
	if err != nil || len(owner) != 32 || imageagent.ValidateArtifactKeyIdentifier(source.TenantID) != nil || imageagent.ValidateArtifactKeyIdentifier(approved.RunID) != nil || imageagent.ValidateArtifactKeyIdentifier(approved.SlotID) != nil {
		return nil, asset.ErrInvalidApproval
	}
	indexText, name, found := strings.Cut(parts[8], "-")
	index, err := strconv.Atoi(indexText)
	stem, extension, hasExtension := strings.Cut(name, ".")
	if !found || !hasExtension || err != nil || index < 0 || strconv.Itoa(index) != indexText || stem != identity.SHA256 || (extension != "png" && extension != "jpg" && extension != "webp") {
		return nil, asset.ErrInvalidApproval
	}
	content, inspection, err := r.Objects.ReadObject(ctx, key, maximum)
	if err != nil || !inspection.Exists || inspection.ContentLength != int64(len(content)) || len(content) == 0 || int64(len(content)) > maximum {
		return nil, asset.ErrInvalidApproval
	}
	hash := sha256.Sum256(content)
	if hex.EncodeToString(hash[:]) != identity.SHA256 {
		return nil, asset.ErrApprovalConflict
	}
	return content, nil
}
