package objectstore

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"task-processor/internal/imageagent"
	"task-processor/internal/infra/storage"
)

const (
	defaultStagingPrefix = "image-agent/staging"
	defaultPublicPrefix  = "image-agent/public"
)

var canonicalID = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`)

var (
	ErrObjectConflict      = errors.New("object storage identity conflicts with existing object")
	ErrArtifactUnavailable = errors.New("prepared artifact bytes are unavailable")
)

type S3DurableArtifactStoreConfig struct {
	MaxArtifactBytes int64
}

// S3DurableArtifactStore is the S3/COS adapter for deterministic, durable
// image-agent artifacts. It depends only on the repository's configured AWS
// SDK v2 uploader infrastructure.
type S3DurableArtifactStore struct {
	uploader         *storage.S3Uploader
	maxArtifactBytes int64
}

type PrepareSlotArtifactsInput struct {
	Identity imageagent.SlotExternalEffectIdentity
	Assets   []ArtifactInput
}

type ArtifactInput struct {
	Bytes             []byte
	ContentType       string
	Width             int
	Height            int
	SourceAssetID     string
	Operations        []string
	ProviderReceiptID string
}

// PreparedSlotArtifacts is safe to persist: only its manifest is JSON
// visible. In-memory bytes exist solely while the owning activity is alive.
type PreparedSlotArtifacts struct {
	Manifest imageagent.StagingManifest `json:"manifest"`
	contents map[string][]byte
}

func (prepared PreparedSlotArtifacts) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Manifest imageagent.StagingManifest `json:"manifest"`
	}{Manifest: prepared.Manifest})
}

func NewS3DurableArtifactStore(uploader *storage.S3Uploader, cfg S3DurableArtifactStoreConfig) (*S3DurableArtifactStore, error) {
	if uploader == nil || cfg.MaxArtifactBytes <= 0 {
		return nil, fmt.Errorf("durable S3 artifact store requires uploader and positive artifact limit")
	}
	return &S3DurableArtifactStore{uploader: uploader, maxArtifactBytes: cfg.MaxArtifactBytes}, nil
}

func (s *S3DurableArtifactStore) PrepareSlotArtifacts(input PrepareSlotArtifactsInput) (PreparedSlotArtifacts, error) {
	if err := validateIdentity(input.Identity); err != nil || len(input.Assets) == 0 {
		return PreparedSlotArtifacts{}, imageagent.ErrValidation
	}
	assets := make([]imageagent.StagedAssetRef, len(input.Assets))
	contents := make(map[string][]byte, len(input.Assets))
	for index, asset := range input.Assets {
		if int64(len(asset.Bytes)) <= 0 || int64(len(asset.Bytes)) > s.maxArtifactBytes || asset.Width <= 0 || asset.Height <= 0 {
			return PreparedSlotArtifacts{}, imageagent.ErrValidation
		}
		if !canonicalID.MatchString(asset.SourceAssetID) || (asset.ProviderReceiptID != "" && !canonicalID.MatchString(asset.ProviderReceiptID)) {
			return PreparedSlotArtifacts{}, imageagent.ErrValidation
		}
		extension, ok := contentExtensions[asset.ContentType]
		if !ok {
			return PreparedSlotArtifacts{}, imageagent.ErrValidation
		}
		sum := sha256.Sum256(asset.Bytes)
		hash := hex.EncodeToString(sum[:])
		key := fmt.Sprintf("%s/%s/%s/%d/%s/%d/%d-%s.%s", defaultStagingPrefix, input.Identity.TenantID, input.Identity.RunID, input.Identity.PlanRevision, input.Identity.SlotID, input.Identity.Attempt, index, hash, extension)
		assets[index] = imageagent.StagedAssetRef{ObjectKey: key, SHA256: hash, SizeBytes: int64(len(asset.Bytes)), ContentType: asset.ContentType, Width: asset.Width, Height: asset.Height, SourceAssetID: asset.SourceAssetID, Operations: append([]string(nil), asset.Operations...), ProviderReceiptID: asset.ProviderReceiptID}
		contents[key] = append([]byte(nil), asset.Bytes...)
	}
	manifest, err := imageagent.NormalizeStagingManifest(imageagent.StagingManifest{Assets: assets})
	if err != nil {
		return PreparedSlotArtifacts{}, err
	}
	return PreparedSlotArtifacts{Manifest: manifest, contents: contents}, nil
}

func (s *S3DurableArtifactStore) EnsureStaged(ctx context.Context, prepared PreparedSlotArtifacts) error {
	manifest, err := imageagent.NormalizeStagingManifest(prepared.Manifest)
	if err != nil {
		return err
	}
	for _, asset := range manifest.Assets {
		if err := s.ensureObject(ctx, asset, prepared.contents[asset.ObjectKey]); err != nil {
			return err
		}
	}
	return nil
}

func (s *S3DurableArtifactStore) Finalize(ctx context.Context, manifest imageagent.StagingManifest) (imageagent.FinalManifest, error) {
	manifest, err := imageagent.NormalizeStagingManifest(manifest)
	if err != nil {
		return imageagent.FinalManifest{}, err
	}
	finalAssets := make([]imageagent.StagedAssetRef, len(manifest.Assets))
	for index, staged := range manifest.Assets {
		if err := s.verifyExistingObject(ctx, staged); err != nil {
			return imageagent.FinalManifest{}, err
		}
		finalKey, err := publicKey(staged.ObjectKey)
		if err != nil {
			return imageagent.FinalManifest{}, err
		}
		finalAsset := staged
		finalAsset.ObjectKey = finalKey
		inspection, err := s.uploader.InspectObject(ctx, finalKey)
		if err != nil {
			return imageagent.FinalManifest{}, err
		}
		if inspection.Exists {
			if err := verifyInspection(inspection, finalAsset); err != nil {
				return imageagent.FinalManifest{}, err
			}
		} else {
			copyErr := s.uploader.CopyImmutable(ctx, storage.ImmutableObjectCopy{SourceKey: staged.ObjectKey, Destination: immutablePut(finalAsset, nil)})
			if copyErr != nil {
				inspection, err = s.uploader.InspectObject(ctx, finalKey)
				if err != nil {
					return imageagent.FinalManifest{}, err
				}
				if !inspection.Exists {
					return imageagent.FinalManifest{}, copyErr
				}
			}
			if err := s.verifyExistingObject(ctx, finalAsset); err != nil {
				return imageagent.FinalManifest{}, err
			}
		}
		finalAssets[index] = finalAsset
	}
	return imageagent.NormalizeFinalManifest(imageagent.FinalManifest{Assets: finalAssets})
}

func (s *S3DurableArtifactStore) ensureObject(ctx context.Context, asset imageagent.StagedAssetRef, data []byte) error {
	inspection, err := s.uploader.InspectObject(ctx, asset.ObjectKey)
	if err != nil {
		return err
	}
	if inspection.Exists {
		return verifyInspection(inspection, asset)
	}
	if int64(len(data)) != asset.SizeBytes {
		return ErrArtifactUnavailable
	}
	putErr := s.uploader.PutImmutable(ctx, immutablePut(asset, data))
	if putErr != nil {
		inspection, err = s.uploader.InspectObject(ctx, asset.ObjectKey)
		if err != nil {
			return err
		}
		if !inspection.Exists {
			return putErr
		}
	}
	return s.verifyExistingObject(ctx, asset)
}

func (s *S3DurableArtifactStore) verifyExistingObject(ctx context.Context, asset imageagent.StagedAssetRef) error {
	inspection, err := s.uploader.InspectObject(ctx, asset.ObjectKey)
	if err != nil {
		return err
	}
	if !inspection.Exists {
		return ErrArtifactUnavailable
	}
	return verifyInspection(inspection, asset)
}

func immutablePut(asset imageagent.StagedAssetRef, data []byte) storage.ImmutableObjectPut {
	return storage.ImmutableObjectPut{Key: asset.ObjectKey, Data: data, ContentType: asset.ContentType, SHA256: asset.SHA256, SizeBytes: asset.SizeBytes}
}

func verifyInspection(inspection storage.ObjectInspection, asset imageagent.StagedAssetRef) error {
	if inspection.ContentLength != asset.SizeBytes || inspection.ContentType != asset.ContentType {
		return ErrObjectConflict
	}
	if inspection.ServerChecksumSHA256 != "" {
		serverHash, err := serverChecksumHex(inspection.ServerChecksumSHA256)
		if err != nil || !strings.EqualFold(serverHash, asset.SHA256) {
			return ErrObjectConflict
		}
		return nil
	}
	if !strings.EqualFold(inspection.Metadata["sha256"], asset.SHA256) || inspection.Metadata["size-bytes"] != strconv.FormatInt(asset.SizeBytes, 10) {
		return ErrObjectConflict
	}
	return nil
}

func serverChecksumHex(value string) (string, error) {
	decoded, err := base64.StdEncoding.DecodeString(value)
	if err != nil || len(decoded) != sha256.Size {
		return "", fmt.Errorf("invalid server SHA-256")
	}
	return hex.EncodeToString(decoded), nil
}

func publicKey(stagedKey string) (string, error) {
	prefix := defaultStagingPrefix + "/"
	if !strings.HasPrefix(stagedKey, prefix) {
		return "", imageagent.ErrValidation
	}
	return defaultPublicPrefix + "/" + strings.TrimPrefix(stagedKey, prefix), nil
}

func validateIdentity(identity imageagent.SlotExternalEffectIdentity) error {
	for _, value := range []string{identity.TenantID, identity.OwnerUserID, identity.RunID, identity.SlotID} {
		if !canonicalID.MatchString(value) {
			return imageagent.ErrValidation
		}
	}
	if identity.PlanRevision <= 0 || identity.Attempt <= 0 {
		return imageagent.ErrValidation
	}
	return nil
}

var contentExtensions = map[string]string{
	"image/jpeg": "jpg",
	"image/png":  "png",
	"image/webp": "webp",
}
