package objectstore

import (
	"archive/tar"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"io"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"

	"task-processor/internal/imageagent"
	"task-processor/internal/pkg/imagex"
)

const (
	defaultStagingPrefix             = "image-agent/staging"
	defaultPublicPrefix              = imageagent.PublishedArtifactPrefix
	recoveryBundleVersion            = 1
	recoveryBundleContentType        = "application/x-tar"
	recoveryBundleManifestName       = "manifest.json"
	defaultMaxArtifactCount          = 16
	defaultMaxAggregateBytes   int64 = 64 << 20
	defaultMaxImageDimension         = 8192
	defaultMaxImagePixels      int64 = 32 << 20
)

var canonicalSHA256 = regexp.MustCompile(`^[a-f0-9]{64}$`)

var (
	ErrObjectConflict      = errors.New("object storage identity conflicts with existing object")
	ErrArtifactUnavailable = errors.New("prepared artifact bytes are unavailable")
)

type DurableArtifactStoreConfig struct {
	MaxArtifactBytes  int64
	MaxArtifactCount  int
	MaxAggregateBytes int64
	MaxImageDimension int
	MaxImagePixels    int64
	OperationTimeout  time.Duration
}

type ObjectInspection struct {
	Exists               bool
	ContentLength        int64
	ContentType          string
	Metadata             map[string]string
	ServerChecksumSHA256 string
	ETag                 string
}

type ImmutableObjectPut struct {
	Key         string
	Data        []byte
	ContentType string
	SHA256      string
	SizeBytes   int64
}

type ImmutableObjectCopy struct {
	SourceKey   string
	Destination ImmutableObjectPut
}

// ImmutableObjectStore is the narrow object-storage port consumed by durable
// image-agent artifact publication.
type ImmutableObjectStore interface {
	PublicURL(key string) string
	InspectObject(context.Context, string) (ObjectInspection, error)
	ReadObject(context.Context, string, int64) ([]byte, ObjectInspection, error)
	PutImmutable(context.Context, ImmutableObjectPut) error
	CopyImmutable(context.Context, ImmutableObjectCopy) error
}

// DurableArtifactStore owns deterministic image-agent artifact behavior while
// depending only on the domain-local immutable object-storage port.
type DurableArtifactStore struct {
	objectStore       ImmutableObjectStore
	maxArtifactBytes  int64
	maxArtifactCount  int
	maxAggregateBytes int64
	maxImageDimension int
	maxImagePixels    int64
	operationTimeout  time.Duration
	inspectImage      func([]byte) (*imagex.ImageInfo, error)
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

type recoveryBundleManifest struct {
	Version  int                                   `json:"version"`
	Identity imageagent.SlotExternalEffectIdentity `json:"identity"`
	Manifest imageagent.StagingManifest            `json:"manifest"`
}

func (prepared PreparedSlotArtifacts) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Manifest imageagent.StagingManifest `json:"manifest"`
	}{Manifest: prepared.Manifest})
}

func NewDurableArtifactStore(objectStore ImmutableObjectStore, cfg DurableArtifactStoreConfig) (*DurableArtifactStore, error) {
	if objectStore == nil || cfg.MaxArtifactBytes <= 0 || cfg.OperationTimeout <= 0 {
		return nil, fmt.Errorf("durable artifact store requires object storage, positive artifact limit, and positive operation timeout")
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
	return &DurableArtifactStore{objectStore: objectStore, maxArtifactBytes: cfg.MaxArtifactBytes, maxArtifactCount: cfg.MaxArtifactCount, maxAggregateBytes: cfg.MaxAggregateBytes, maxImageDimension: cfg.MaxImageDimension, maxImagePixels: cfg.MaxImagePixels, operationTimeout: cfg.OperationTimeout, inspectImage: imagex.Inspect}, nil
}

// PublicURL delegates to the configured uploader so v3 approval uses the same
// public-base and endpoint rules as durable artifact publication.
func (s *DurableArtifactStore) PublicURL(key string) string {
	if s == nil || s.objectStore == nil {
		return ""
	}
	return s.objectStore.PublicURL(key)
}

func (s *DurableArtifactStore) PrepareSlotArtifacts(input PrepareSlotArtifactsInput) (PreparedSlotArtifacts, error) {
	if err := validateIdentity(input.Identity); err != nil || len(input.Assets) == 0 || len(input.Assets) > s.maxArtifactCount {
		return PreparedSlotArtifacts{}, imageagent.ErrValidation
	}
	ownerKey, err := imageagent.ArtifactOwnerKey(input.Identity.OwnerUserID)
	if err != nil {
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
		if imageagent.ValidateProvenanceAssetID(asset.SourceAssetID) != nil || (asset.ProviderReceiptID != "" && !safePersistedID(asset.ProviderReceiptID)) {
			return PreparedSlotArtifacts{}, imageagent.ErrValidation
		}
		config, format, err := image.DecodeConfig(bytes.NewReader(asset.Bytes))
		if err != nil {
			return PreparedSlotArtifacts{}, imageagent.ErrValidation
		}
		contentType, ok := imageContentTypes[format]
		if !ok || asset.ContentType != contentType || asset.Width != config.Width || asset.Height != config.Height || !s.withinImageLimits(config.Width, config.Height) {
			return PreparedSlotArtifacts{}, imageagent.ErrValidation
		}
		info, err := s.inspectImage(asset.Bytes)
		if err != nil || info.Width != config.Width || info.Height != config.Height || info.Format != format {
			return PreparedSlotArtifacts{}, imageagent.ErrValidation
		}
		operations, err := imageagent.NormalizeArtifactOperations(asset.Operations)
		if err != nil {
			return PreparedSlotArtifacts{}, imageagent.ErrValidation
		}
		extension, ok := contentExtensions[contentType]
		if !ok {
			return PreparedSlotArtifacts{}, imageagent.ErrValidation
		}
		sum := sha256.Sum256(asset.Bytes)
		hash := hex.EncodeToString(sum[:])
		key := fmt.Sprintf("%s/%s/%s/%s/%d/%s/%d/%d-%s.%s", defaultStagingPrefix, input.Identity.TenantID, ownerKey, input.Identity.RunID, input.Identity.PlanRevision, input.Identity.SlotID, input.Identity.Attempt, index, hash, extension)
		assets[index] = imageagent.StagedAssetRef{ObjectKey: key, SHA256: hash, SizeBytes: int64(len(asset.Bytes)), ContentType: contentType, Width: info.Width, Height: info.Height, SourceAssetID: asset.SourceAssetID, Operations: operations, ProviderReceiptID: asset.ProviderReceiptID}
		contents[key] = append([]byte(nil), asset.Bytes...)
	}
	manifest, err := imageagent.NormalizeStagingManifest(imageagent.StagingManifest{Assets: assets})
	if err != nil {
		return PreparedSlotArtifacts{}, err
	}
	return PreparedSlotArtifacts{Manifest: manifest, contents: contents}, nil
}

// PreserveSlotArtifacts writes one immutable recovery bundle before the
// activity persists StagingPrepared. The bundle is a single atomic object, so
// a later activity can rehydrate every generated byte even when individual
// staging uploads were only partially completed.
func (s *DurableArtifactStore) PreserveSlotArtifacts(ctx context.Context, identity imageagent.SlotExternalEffectIdentity, prepared PreparedSlotArtifacts) error {
	bundle, err := s.encodeRecoveryBundle(identity, prepared)
	if err != nil {
		return err
	}
	sum := sha256.Sum256(bundle)
	object := ImmutableObjectPut{
		Key: recoveryBundleKey(identity), Data: bundle, ContentType: recoveryBundleContentType,
		SHA256: hex.EncodeToString(sum[:]), SizeBytes: int64(len(bundle)),
	}
	return s.ensureImmutableObject(ctx, object)
}

// RecoverSlotArtifacts loads and verifies the immutable recovery bundle for a
// slot attempt. expected may be empty while recovering a provider claim whose
// staging transition was not committed; otherwise it must exactly match the
// persisted staging manifest.
func (s *DurableArtifactStore) RecoverSlotArtifacts(ctx context.Context, identity imageagent.SlotExternalEffectIdentity, expected imageagent.StagingManifest) (PreparedSlotArtifacts, error) {
	if err := validateIdentity(identity); err != nil {
		return PreparedSlotArtifacts{}, err
	}
	operationCtx, cancel := context.WithTimeout(ctx, s.operationTimeout)
	defer cancel()
	data, inspection, err := s.objectStore.ReadObject(operationCtx, recoveryBundleKey(identity), s.maxRecoveryBundleBytes())
	if err != nil {
		return PreparedSlotArtifacts{}, err
	}
	if !inspection.Exists {
		return PreparedSlotArtifacts{}, ErrArtifactUnavailable
	}
	sum := sha256.Sum256(data)
	object := ImmutableObjectPut{
		Key: recoveryBundleKey(identity), Data: data, ContentType: recoveryBundleContentType,
		SHA256: hex.EncodeToString(sum[:]), SizeBytes: int64(len(data)),
	}
	if err := verifyImmutableInspection(inspection, object); err != nil {
		return PreparedSlotArtifacts{}, err
	}
	return s.decodeRecoveryBundle(identity, expected, data)
}

func (s *DurableArtifactStore) EnsureStaged(ctx context.Context, prepared PreparedSlotArtifacts) error {
	manifest, err := imageagent.NormalizeStagingManifest(prepared.Manifest)
	if err != nil {
		return err
	}
	validated, err := s.prevalidateManifest(manifest)
	if err != nil {
		return err
	}
	for _, asset := range validated {
		if err := s.ensureObject(ctx, asset.ref, prepared.contents[asset.ref.ObjectKey]); err != nil {
			return err
		}
	}
	return nil
}

func (s *DurableArtifactStore) encodeRecoveryBundle(identity imageagent.SlotExternalEffectIdentity, prepared PreparedSlotArtifacts) ([]byte, error) {
	if err := validateIdentity(identity); err != nil {
		return nil, err
	}
	manifest, err := imageagent.NormalizeStagingManifest(prepared.Manifest)
	if err != nil {
		return nil, err
	}
	validated, err := s.prevalidateManifestForIdentity(manifest, identity)
	if err != nil {
		return nil, err
	}
	header, err := json.Marshal(recoveryBundleManifest{Version: recoveryBundleVersion, Identity: identity, Manifest: manifest})
	if err != nil {
		return nil, err
	}
	var buffer bytes.Buffer
	writer := tar.NewWriter(&buffer)
	if err := writeRecoveryBundleEntry(writer, recoveryBundleManifestName, header); err != nil {
		return nil, err
	}
	for index, asset := range validated {
		data := prepared.contents[asset.ref.ObjectKey]
		if err := s.validateRecoveredAssetBytes(asset.ref, data); err != nil {
			return nil, err
		}
		if err := writeRecoveryBundleEntry(writer, fmt.Sprintf("assets/%d", index), data); err != nil {
			return nil, err
		}
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	if int64(buffer.Len()) > s.maxRecoveryBundleBytes() {
		return nil, imageagent.ErrValidation
	}
	return buffer.Bytes(), nil
}

func (s *DurableArtifactStore) decodeRecoveryBundle(identity imageagent.SlotExternalEffectIdentity, expected imageagent.StagingManifest, data []byte) (PreparedSlotArtifacts, error) {
	reader := tar.NewReader(bytes.NewReader(data))
	headerData, err := readRecoveryBundleEntry(reader, recoveryBundleManifestName, 1<<20)
	if err != nil {
		return PreparedSlotArtifacts{}, err
	}
	var envelope recoveryBundleManifest
	if err := json.Unmarshal(headerData, &envelope); err != nil || envelope.Version != recoveryBundleVersion || envelope.Identity != identity {
		return PreparedSlotArtifacts{}, ErrObjectConflict
	}
	manifest, err := imageagent.NormalizeStagingManifest(envelope.Manifest)
	if err != nil {
		return PreparedSlotArtifacts{}, err
	}
	if len(expected.Assets) > 0 {
		normalizedExpected, normalizeErr := imageagent.NormalizeStagingManifest(expected)
		if normalizeErr != nil {
			return PreparedSlotArtifacts{}, normalizeErr
		}
		if !reflect.DeepEqual(normalizedExpected, manifest) {
			return PreparedSlotArtifacts{}, ErrObjectConflict
		}
	}
	validated, err := s.prevalidateManifestForIdentity(manifest, identity)
	if err != nil {
		return PreparedSlotArtifacts{}, err
	}
	contents := make(map[string][]byte, len(validated))
	for index, asset := range validated {
		assetData, readErr := readRecoveryBundleEntry(reader, fmt.Sprintf("assets/%d", index), asset.ref.SizeBytes)
		if readErr != nil {
			return PreparedSlotArtifacts{}, readErr
		}
		if err := s.validateRecoveredAssetBytes(asset.ref, assetData); err != nil {
			return PreparedSlotArtifacts{}, err
		}
		contents[asset.ref.ObjectKey] = assetData
	}
	if extra, err := reader.Next(); err != io.EOF || extra != nil {
		return PreparedSlotArtifacts{}, ErrObjectConflict
	}
	return PreparedSlotArtifacts{Manifest: manifest, contents: contents}, nil
}

func writeRecoveryBundleEntry(writer *tar.Writer, name string, data []byte) error {
	header := &tar.Header{Name: name, Mode: 0o600, Size: int64(len(data)), ModTime: time.Unix(0, 0).UTC(), Format: tar.FormatUSTAR}
	if err := writer.WriteHeader(header); err != nil {
		return err
	}
	_, err := writer.Write(data)
	return err
}

func readRecoveryBundleEntry(reader *tar.Reader, name string, maxBytes int64) ([]byte, error) {
	header, err := reader.Next()
	if err != nil || header.Name != name || header.Size <= 0 || header.Size > maxBytes {
		return nil, ErrObjectConflict
	}
	data, err := io.ReadAll(io.LimitReader(reader, maxBytes+1))
	if err != nil || int64(len(data)) != header.Size || int64(len(data)) > maxBytes {
		return nil, ErrObjectConflict
	}
	return data, nil
}

func (s *DurableArtifactStore) Finalize(ctx context.Context, manifest imageagent.StagingManifest) (imageagent.FinalManifest, error) {
	return s.FinalizeWithProgress(ctx, manifest, nil)
}

// FinalizeWithProgress invokes progress immediately before each bounded
// per-asset reconciliation. Callers use it to renew a publication lease between
// objects without weakening whole-manifest preflight.
func (s *DurableArtifactStore) FinalizeWithProgress(ctx context.Context, manifest imageagent.StagingManifest, progress func(context.Context, int) error) (imageagent.FinalManifest, error) {
	manifest, err := imageagent.NormalizeStagingManifest(manifest)
	if err != nil {
		return imageagent.FinalManifest{}, err
	}
	validated, err := s.prevalidateManifest(imageagent.StagingManifest{Assets: manifest.Assets})
	if err != nil {
		return imageagent.FinalManifest{}, err
	}
	finalAssets := make([]imageagent.PublishedAssetRef, len(validated))
	for index, staged := range validated {
		if progress != nil {
			if err := progress(ctx, index); err != nil {
				return imageagent.FinalManifest{}, err
			}
		}
		if err := s.verifyExistingObject(ctx, staged.ref); err != nil {
			return imageagent.FinalManifest{}, err
		}
		finalKey := staged.identity.publicKey()
		finalAsset := staged.ref
		finalAsset.ObjectKey = finalKey
		inspection, err := s.inspectObject(ctx, finalKey)
		if err != nil {
			return imageagent.FinalManifest{}, err
		}
		if inspection.Exists {
			if err := verifyInspection(inspection, finalAsset); err != nil {
				return imageagent.FinalManifest{}, err
			}
		} else {
			copyErr := s.copyImmutable(ctx, ImmutableObjectCopy{SourceKey: staged.ref.ObjectKey, Destination: immutablePut(finalAsset, nil)})
			if copyErr != nil {
				inspection, err = s.inspectObject(ctx, finalKey)
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
		finalAssets[index] = imageagent.PublishedAssetRef{
			ObjectKey: finalAsset.ObjectKey, SHA256: finalAsset.SHA256, SizeBytes: finalAsset.SizeBytes,
			ContentType: finalAsset.ContentType, Width: finalAsset.Width, Height: finalAsset.Height,
			SourceAssetID: finalAsset.SourceAssetID, Operations: cloneOperationsPreservingNil(finalAsset.Operations), ProviderReceiptID: finalAsset.ProviderReceiptID,
		}
	}
	return imageagent.NormalizeFinalManifest(imageagent.FinalManifest{Assets: finalAssets})
}

func cloneOperationsPreservingNil(operations []string) []string {
	if operations == nil {
		return nil
	}
	return append([]string{}, operations...)
}

func (s *DurableArtifactStore) ensureObject(ctx context.Context, asset imageagent.StagedAssetRef, data []byte) error {
	if int64(len(data)) != asset.SizeBytes {
		return ErrArtifactUnavailable
	}
	return s.ensureImmutableObject(ctx, immutablePut(asset, data))
}

func (s *DurableArtifactStore) ensureImmutableObject(ctx context.Context, object ImmutableObjectPut) error {
	inspection, err := s.inspectObject(ctx, object.Key)
	if err != nil {
		return err
	}
	if inspection.Exists {
		return verifyImmutableInspection(inspection, object)
	}
	if int64(len(object.Data)) != object.SizeBytes {
		return ErrArtifactUnavailable
	}
	putErr := s.putImmutable(ctx, object)
	if putErr != nil {
		inspection, err = s.inspectObject(ctx, object.Key)
		if err != nil {
			return err
		}
		if !inspection.Exists {
			return putErr
		}
	}
	inspection, err = s.inspectObject(ctx, object.Key)
	if err != nil {
		return err
	}
	if !inspection.Exists {
		return ErrArtifactUnavailable
	}
	return verifyImmutableInspection(inspection, object)
}

type prevalidatedStagedAsset struct {
	ref      imageagent.StagedAssetRef
	identity stagingKeyIdentity
}

func (s *DurableArtifactStore) prevalidateManifest(manifest imageagent.StagingManifest) ([]prevalidatedStagedAsset, error) {
	if len(manifest.Assets) == 0 || len(manifest.Assets) > s.maxArtifactCount {
		return nil, imageagent.ErrValidation
	}
	var aggregateBytes int64
	validated := make([]prevalidatedStagedAsset, len(manifest.Assets))
	for index, asset := range manifest.Assets {
		if asset.SizeBytes <= 0 || asset.SizeBytes > s.maxArtifactBytes || aggregateBytes > s.maxAggregateBytes-asset.SizeBytes || !s.withinImageLimits(asset.Width, asset.Height) || imageagent.ValidateProvenanceAssetID(asset.SourceAssetID) != nil || (asset.ProviderReceiptID != "" && !safePersistedID(asset.ProviderReceiptID)) {
			return nil, imageagent.ErrValidation
		}
		if _, err := imageagent.NormalizeArtifactOperations(asset.Operations); err != nil {
			return nil, err
		}
		identity, err := parseStagingKey(asset, index)
		if err != nil {
			return nil, err
		}
		aggregateBytes += asset.SizeBytes
		validated[index] = prevalidatedStagedAsset{ref: asset, identity: identity}
	}
	return validated, nil
}

func (s *DurableArtifactStore) prevalidateManifestForIdentity(manifest imageagent.StagingManifest, expected imageagent.SlotExternalEffectIdentity) ([]prevalidatedStagedAsset, error) {
	validated, err := s.prevalidateManifest(manifest)
	if err != nil {
		return nil, err
	}
	ownerKey, err := imageagent.ArtifactOwnerKey(expected.OwnerUserID)
	if err != nil {
		return nil, imageagent.ErrValidation
	}
	for _, asset := range validated {
		identity := asset.identity
		if identity.tenantID != expected.TenantID || identity.ownerKey != ownerKey || identity.runID != expected.RunID || identity.planRevision != expected.PlanRevision || identity.slotID != expected.SlotID || identity.attempt != int64(expected.Attempt) {
			return nil, ErrObjectConflict
		}
	}
	return validated, nil
}

func (s *DurableArtifactStore) validateRecoveredAssetBytes(asset imageagent.StagedAssetRef, data []byte) error {
	if int64(len(data)) != asset.SizeBytes {
		return ErrObjectConflict
	}
	sum := sha256.Sum256(data)
	if hex.EncodeToString(sum[:]) != asset.SHA256 {
		return ErrObjectConflict
	}
	config, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || imageContentTypes[format] != asset.ContentType || config.Width != asset.Width || config.Height != asset.Height || !s.withinImageLimits(config.Width, config.Height) {
		return ErrObjectConflict
	}
	info, err := s.inspectImage(data)
	if err != nil || info.Width != asset.Width || info.Height != asset.Height || info.Format != format {
		return ErrObjectConflict
	}
	return nil
}

func (s *DurableArtifactStore) maxRecoveryBundleBytes() int64 {
	return s.maxAggregateBytes + int64(s.maxArtifactCount+2)*1024 + (1 << 20)
}

func (s *DurableArtifactStore) withinImageLimits(width, height int) bool {
	return width > 0 && height > 0 && width <= s.maxImageDimension && height <= s.maxImageDimension && int64(width) <= s.maxImagePixels/int64(height)
}

func (s *DurableArtifactStore) verifyExistingObject(ctx context.Context, asset imageagent.StagedAssetRef) error {
	inspection, err := s.inspectObject(ctx, asset.ObjectKey)
	if err != nil {
		return err
	}
	if !inspection.Exists {
		return ErrArtifactUnavailable
	}
	return verifyInspection(inspection, asset)
}

func (s *DurableArtifactStore) inspectObject(ctx context.Context, key string) (ObjectInspection, error) {
	operationCtx, cancel := context.WithTimeout(ctx, s.operationTimeout)
	defer cancel()
	return s.objectStore.InspectObject(operationCtx, key)
}

func (s *DurableArtifactStore) putImmutable(ctx context.Context, object ImmutableObjectPut) error {
	operationCtx, cancel := context.WithTimeout(ctx, s.operationTimeout)
	defer cancel()
	return s.objectStore.PutImmutable(operationCtx, object)
}

func (s *DurableArtifactStore) copyImmutable(ctx context.Context, object ImmutableObjectCopy) error {
	operationCtx, cancel := context.WithTimeout(ctx, s.operationTimeout)
	defer cancel()
	return s.objectStore.CopyImmutable(operationCtx, object)
}

func immutablePut(asset imageagent.StagedAssetRef, data []byte) ImmutableObjectPut {
	return ImmutableObjectPut{Key: asset.ObjectKey, Data: data, ContentType: asset.ContentType, SHA256: asset.SHA256, SizeBytes: asset.SizeBytes}
}

func verifyInspection(inspection ObjectInspection, asset imageagent.StagedAssetRef) error {
	return verifyImmutableInspection(inspection, immutablePut(asset, nil))
}

func verifyImmutableInspection(inspection ObjectInspection, object ImmutableObjectPut) error {
	if inspection.ContentLength != object.SizeBytes || inspection.ContentType != object.ContentType {
		return ErrObjectConflict
	}
	if inspection.ServerChecksumSHA256 != "" {
		serverHash, err := serverChecksumHex(inspection.ServerChecksumSHA256)
		if err != nil || !strings.EqualFold(serverHash, object.SHA256) {
			return ErrObjectConflict
		}
		return nil
	}
	if !strings.EqualFold(inspection.Metadata["sha256"], object.SHA256) || inspection.Metadata["size-bytes"] != strconv.FormatInt(object.SizeBytes, 10) {
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
	for _, value := range []string{identity.TenantID, identity.RunID, identity.SlotID} {
		if imageagent.ValidateArtifactKeyIdentifier(value) != nil {
			return imageagent.ErrValidation
		}
	}
	if _, err := imageagent.ArtifactOwnerKey(identity.OwnerUserID); err != nil {
		return imageagent.ErrValidation
	}
	if identity.PlanRevision <= 0 || identity.Attempt <= 0 {
		return imageagent.ErrValidation
	}
	return nil
}

func recoveryBundleKey(identity imageagent.SlotExternalEffectIdentity) string {
	ownerKey, _ := imageagent.ArtifactOwnerKey(identity.OwnerUserID)
	return fmt.Sprintf("%s/%s/%s/%s/%d/%s/%d/recovery.tar", defaultStagingPrefix, identity.TenantID, ownerKey, identity.RunID, identity.PlanRevision, identity.SlotID, identity.Attempt)
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

func safePersistedID(value string) bool {
	return imageagent.ValidateArtifactKeyIdentifier(value) == nil
}

type stagingKeyIdentity struct {
	tenantID     string
	ownerKey     string
	runID        string
	planRevision int64
	slotID       string
	attempt      int64
	assetIndex   int
	sha256       string
	extension    string
}

func (identity stagingKeyIdentity) publicKey() string {
	return fmt.Sprintf("%s/%s/%s/%s/%d/%s/%d/%d-%s.%s", defaultPublicPrefix, identity.tenantID, identity.ownerKey, identity.runID, identity.planRevision, identity.slotID, identity.attempt, identity.assetIndex, identity.sha256, identity.extension)
}

func parseStagingKey(asset imageagent.StagedAssetRef, expectedIndex int) (stagingKeyIdentity, error) {
	segments := strings.Split(asset.ObjectKey, "/")
	if len(segments) != 9 || strings.Join(segments[:2], "/") != defaultStagingPrefix || imageagent.ValidateArtifactKeyIdentifier(segments[2]) != nil || !canonicalSHA256.MatchString(segments[3]) || imageagent.ValidateArtifactKeyIdentifier(segments[4]) != nil || imageagent.ValidateArtifactKeyIdentifier(segments[6]) != nil {
		return stagingKeyIdentity{}, imageagent.ErrValidation
	}
	planRevision, ok := canonicalPositiveDecimal(segments[5])
	if !ok {
		return stagingKeyIdentity{}, imageagent.ErrValidation
	}
	attempt, ok := canonicalPositiveDecimal(segments[7])
	if !ok {
		return stagingKeyIdentity{}, imageagent.ErrValidation
	}
	filename := segments[8]
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
	return stagingKeyIdentity{tenantID: segments[2], ownerKey: segments[3], runID: segments[4], planRevision: planRevision, slotID: segments[6], attempt: attempt, assetIndex: int(assetIndex64), sha256: hashAndExtension[0], extension: hashAndExtension[1]}, nil
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
