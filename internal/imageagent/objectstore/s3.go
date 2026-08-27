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
	"task-processor/internal/pkg/imagex"
)

const (
	defaultStagingPrefix           = "image-agent/staging"
	defaultPublicPrefix            = "image-agent/public"
	defaultMaxArtifactCount        = 16
	defaultMaxAggregateBytes int64 = 64 << 20
	defaultMaxImageDimension       = 8192
	defaultMaxImagePixels    int64 = 32 << 20
	maxOperationCount              = 8
	maxOperationLength             = 64
)

var canonicalID = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`)
var canonicalSHA256 = regexp.MustCompile(`^[a-f0-9]{64}$`)

var (
	ErrObjectConflict      = errors.New("object storage identity conflicts with existing object")
	ErrArtifactUnavailable = errors.New("prepared artifact bytes are unavailable")
)

type S3DurableArtifactStoreConfig struct {
	MaxArtifactBytes  int64
	MaxArtifactCount  int
	MaxAggregateBytes int64
	MaxImageDimension int
	MaxImagePixels    int64
}

// S3DurableArtifactStore is the S3/COS adapter for deterministic, durable
// image-agent artifacts. It depends only on the repository's configured AWS
// SDK v2 uploader infrastructure.
type S3DurableArtifactStore struct {
	uploader          *storage.S3Uploader
	maxArtifactBytes  int64
	maxArtifactCount  int
	maxAggregateBytes int64
	maxImageDimension int
	maxImagePixels    int64
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
	if cfg.MaxArtifactCount <= 0 {
		cfg.MaxArtifactCount = defaultMaxArtifactCount
	}
	if cfg.MaxAggregateBytes <= 0 {
		cfg.MaxAggregateBytes = defaultMaxAggregateBytes
	}
	if cfg.MaxImageDimension <= 0 {
		cfg.MaxImageDimension = defaultMaxImageDimension
	}
	if cfg.MaxImagePixels <= 0 {
		cfg.MaxImagePixels = defaultMaxImagePixels
	}
	return &S3DurableArtifactStore{uploader: uploader, maxArtifactBytes: cfg.MaxArtifactBytes, maxArtifactCount: cfg.MaxArtifactCount, maxAggregateBytes: cfg.MaxAggregateBytes, maxImageDimension: cfg.MaxImageDimension, maxImagePixels: cfg.MaxImagePixels}, nil
}

func (s *S3DurableArtifactStore) PrepareSlotArtifacts(input PrepareSlotArtifactsInput) (PreparedSlotArtifacts, error) {
	if err := validateIdentity(input.Identity); err != nil || len(input.Assets) == 0 || len(input.Assets) > s.maxArtifactCount {
		return PreparedSlotArtifacts{}, imageagent.ErrValidation
	}
	var aggregateBytes int64
	for _, asset := range input.Assets {
		size := int64(len(asset.Bytes))
		if size <= 0 || size > s.maxArtifactBytes || aggregateBytes > s.maxAggregateBytes-size {
			return PreparedSlotArtifacts{}, imageagent.ErrValidation
		}
		aggregateBytes += size
	}
	assets := make([]imageagent.StagedAssetRef, len(input.Assets))
	contents := make(map[string][]byte, len(input.Assets))
	for index, asset := range input.Assets {
		if !safePersistedID(asset.SourceAssetID) || (asset.ProviderReceiptID != "" && !safePersistedID(asset.ProviderReceiptID)) {
			return PreparedSlotArtifacts{}, imageagent.ErrValidation
		}
		info, err := imagex.Inspect(asset.Bytes)
		if err != nil {
			return PreparedSlotArtifacts{}, imageagent.ErrValidation
		}
		contentType, ok := imageContentTypes[info.Format]
		if !ok || asset.ContentType != contentType || asset.Width != info.Width || asset.Height != info.Height || info.Width > s.maxImageDimension || info.Height > s.maxImageDimension || int64(info.Width) > s.maxImagePixels/int64(info.Height) {
			return PreparedSlotArtifacts{}, imageagent.ErrValidation
		}
		operations, err := trustedOperations(asset.Operations)
		if err != nil {
			return PreparedSlotArtifacts{}, imageagent.ErrValidation
		}
		extension, ok := contentExtensions[contentType]
		if !ok {
			return PreparedSlotArtifacts{}, imageagent.ErrValidation
		}
		sum := sha256.Sum256(asset.Bytes)
		hash := hex.EncodeToString(sum[:])
		key := fmt.Sprintf("%s/%s/%s/%d/%s/%d/%d-%s.%s", defaultStagingPrefix, input.Identity.TenantID, input.Identity.RunID, input.Identity.PlanRevision, input.Identity.SlotID, input.Identity.Attempt, index, hash, extension)
		assets[index] = imageagent.StagedAssetRef{ObjectKey: key, SHA256: hash, SizeBytes: int64(len(asset.Bytes)), ContentType: contentType, Width: info.Width, Height: info.Height, SourceAssetID: asset.SourceAssetID, Operations: operations, ProviderReceiptID: asset.ProviderReceiptID}
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
	if err := s.validatePersistedManifest(manifest); err != nil {
		return err
	}
	for index, asset := range manifest.Assets {
		if _, err := parseStagingKey(asset, index); err != nil {
			return err
		}
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
	if err := s.validatePersistedManifest(imageagent.StagingManifest{Assets: manifest.Assets}); err != nil {
		return imageagent.FinalManifest{}, err
	}
	finalAssets := make([]imageagent.StagedAssetRef, len(manifest.Assets))
	for index, staged := range manifest.Assets {
		parsed, err := parseStagingKey(staged, index)
		if err != nil {
			return imageagent.FinalManifest{}, err
		}
		if err := s.verifyExistingObject(ctx, staged); err != nil {
			return imageagent.FinalManifest{}, err
		}
		finalKey := parsed.publicKey()
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

func (s *S3DurableArtifactStore) validatePersistedManifest(manifest imageagent.StagingManifest) error {
	if len(manifest.Assets) == 0 || len(manifest.Assets) > s.maxArtifactCount {
		return imageagent.ErrValidation
	}
	var aggregateBytes int64
	for _, asset := range manifest.Assets {
		if asset.SizeBytes <= 0 || asset.SizeBytes > s.maxArtifactBytes || aggregateBytes > s.maxAggregateBytes-asset.SizeBytes || asset.Width > s.maxImageDimension || asset.Height > s.maxImageDimension || int64(asset.Width) > s.maxImagePixels/int64(asset.Height) || !safePersistedID(asset.SourceAssetID) || (asset.ProviderReceiptID != "" && !safePersistedID(asset.ProviderReceiptID)) {
			return imageagent.ErrValidation
		}
		if _, err := trustedOperations(asset.Operations); err != nil {
			return err
		}
		aggregateBytes += asset.SizeBytes
	}
	return nil
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

var imageContentTypes = map[string]string{
	"jpeg": "image/jpeg",
	"png":  "image/png",
	"webp": "image/webp",
}

var trustedProductImageOperations = map[string]struct{}{
	"select_subject": {}, "extract_subject_placeholder": {}, "cleanup_placeholder": {},
	"remove_overlay_text_placeholder": {}, "remove_promo_badge_placeholder": {},
	"remove_logo_overlay_placeholder": {}, "render_white_bg_placeholder": {},
	"extract_subject": {}, "cleanup_image": {}, "render_white_bg": {},
	"extract_subject_bbox": {}, "extract_subject_segmenter": {}, "render_white_bg_model": {},
	"compose_on_white_canvas": {}, "cleanup_overlay_signal": {}, "cleanup_quality": {},
	"remove_overlay_regions": {},
}

func trustedOperations(operations []string) ([]string, error) {
	if len(operations) > maxOperationCount {
		return nil, imageagent.ErrValidation
	}
	validated := make([]string, len(operations))
	for index, operation := range operations {
		if len(operation) == 0 || len(operation) > maxOperationLength {
			return nil, imageagent.ErrValidation
		}
		if _, ok := trustedProductImageOperations[operation]; !ok {
			return nil, imageagent.ErrValidation
		}
		validated[index] = operation
	}
	return validated, nil
}

func safePersistedID(value string) bool {
	if !canonicalID.MatchString(value) {
		return false
	}
	lower := strings.ToLower(value)
	for _, marker := range []string{"authorization", "credential", "password", "secret", "token", "api_key", "apikey"} {
		if strings.Contains(lower, marker) {
			return false
		}
	}
	return true
}

type stagingKeyIdentity struct {
	tenantID     string
	runID        string
	planRevision int64
	slotID       string
	attempt      int64
	assetIndex   int
	sha256       string
	extension    string
}

func (identity stagingKeyIdentity) publicKey() string {
	return fmt.Sprintf("%s/%s/%s/%d/%s/%d/%d-%s.%s", defaultPublicPrefix, identity.tenantID, identity.runID, identity.planRevision, identity.slotID, identity.attempt, identity.assetIndex, identity.sha256, identity.extension)
}

func parseStagingKey(asset imageagent.StagedAssetRef, expectedIndex int) (stagingKeyIdentity, error) {
	segments := strings.Split(asset.ObjectKey, "/")
	if len(segments) != 8 || strings.Join(segments[:2], "/") != defaultStagingPrefix || !canonicalID.MatchString(segments[2]) || !canonicalID.MatchString(segments[3]) || !canonicalID.MatchString(segments[5]) {
		return stagingKeyIdentity{}, imageagent.ErrValidation
	}
	planRevision, ok := canonicalPositiveDecimal(segments[4])
	if !ok {
		return stagingKeyIdentity{}, imageagent.ErrValidation
	}
	attempt, ok := canonicalPositiveDecimal(segments[6])
	if !ok {
		return stagingKeyIdentity{}, imageagent.ErrValidation
	}
	filename := segments[7]
	if strings.Count(filename, "-") != 1 || strings.Count(filename, ".") != 1 {
		return stagingKeyIdentity{}, imageagent.ErrValidation
	}
	parts := strings.SplitN(filename, "-", 2)
	assetIndex64, ok := canonicalNonNegativeDecimal(parts[0])
	if !ok || assetIndex64 > int64(^uint(0)>>1) || int(assetIndex64) != expectedIndex {
		return stagingKeyIdentity{}, imageagent.ErrValidation
	}
	hashAndExtension := strings.SplitN(parts[1], ".", 2)
	if len(hashAndExtension) != 2 || !canonicalSHA256.MatchString(hashAndExtension[0]) || hashAndExtension[0] != asset.SHA256 || contentExtensions[asset.ContentType] != hashAndExtension[1] {
		return stagingKeyIdentity{}, imageagent.ErrValidation
	}
	return stagingKeyIdentity{tenantID: segments[2], runID: segments[3], planRevision: planRevision, slotID: segments[5], attempt: attempt, assetIndex: int(assetIndex64), sha256: hashAndExtension[0], extension: hashAndExtension[1]}, nil
}

func canonicalPositiveDecimal(value string) (int64, bool) {
	parsed, ok := canonicalNonNegativeDecimal(value)
	return parsed, ok && parsed > 0
}

func canonicalNonNegativeDecimal(value string) (int64, bool) {
	if value == "" || (len(value) > 1 && value[0] == '0') {
		return 0, false
	}
	for _, character := range value {
		if character < '0' || character > '9' {
			return 0, false
		}
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	return parsed, err == nil
}
