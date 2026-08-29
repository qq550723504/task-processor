package imageagent

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"path"
	"regexp"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// StagedAssetRef is the complete allowlist for durable generated-artifact data.
// It intentionally excludes local paths, transient URLs, credentials, and
// provider-defined metadata.
type StagedAssetRef struct {
	ObjectKey         string   `json:"object_key"`
	SHA256            string   `json:"sha256"`
	SizeBytes         int64    `json:"size_bytes"`
	ContentType       string   `json:"content_type"`
	Width             int      `json:"width"`
	Height            int      `json:"height"`
	SourceAssetID     string   `json:"source_asset_id"`
	Operations        []string `json:"operations"`
	ProviderReceiptID string   `json:"provider_receipt_id,omitempty"`
}

// PublishedAssetRef is the final/public counterpart to StagedAssetRef. It
// retains the established manifest JSON representation while making a staged
// reference ineligible for v3 candidate construction by type.
type PublishedAssetRef struct {
	ObjectKey         string   `json:"object_key"`
	SHA256            string   `json:"sha256"`
	SizeBytes         int64    `json:"size_bytes"`
	ContentType       string   `json:"content_type"`
	Width             int      `json:"width"`
	Height            int      `json:"height"`
	SourceAssetID     string   `json:"source_asset_id"`
	Operations        []string `json:"operations"`
	ProviderReceiptID string   `json:"provider_receipt_id,omitempty"`
}

// StagingManifest carries only durable staging identities. ProviderMetadata is
// deliberately excluded from JSON and rejected so callers cannot smuggle an
// open-ended metadata map into a durable effect record.
type StagingManifest struct {
	Assets           []StagedAssetRef  `json:"assets"`
	ProviderMetadata map[string]string `json:"-"`
}

type FinalManifest struct {
	Assets []PublishedAssetRef `json:"assets"`
}

type DurableAssetIdentity struct {
	ObjectKey string `json:"object_key"`
	SHA256    string `json:"sha256"`
}

var sha256Pattern = regexp.MustCompile(`^[a-fA-F0-9]{64}$`)
var artifactKeyIdentifierPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`)

const maxArtifactOwnerIDLength = 128

// MaxProvenanceAssetIDLength mirrors the upstream varchar(128) business-asset
// identity contract. Provenance IDs are JSON metadata, not object-key path
// segments, so they deliberately do not use ValidateArtifactKeyIdentifier.
const MaxProvenanceAssetIDLength = 128

// ValidateArtifactKeyIdentifier enforces the canonical identifier grammar
// shared by run/slot commands and deterministic durable object keys.
func ValidateArtifactKeyIdentifier(value string) error {
	if !artifactKeyIdentifierPattern.MatchString(value) {
		return ErrValidation
	}
	return nil
}

func ValidateProvenanceAssetID(value string) error {
	if value == "" || value != strings.TrimSpace(value) || !utf8.ValidString(value) || utf8.RuneCountInString(value) > MaxProvenanceAssetIDLength {
		return ErrValidation
	}
	for _, character := range value {
		if unicode.IsControl(character) {
			return ErrValidation
		}
	}
	return nil
}

// ArtifactOwnerKey turns the owner-scoped run identity into a fixed-width,
// opaque object-key component. Owner IDs may use a wider grammar than object
// keys, and must not be exposed verbatim in storage paths.
func ArtifactOwnerKey(ownerUserID string) (string, error) {
	canonical := strings.TrimSpace(ownerUserID)
	if canonical == "" || canonical != ownerUserID || len(canonical) > maxArtifactOwnerIDLength {
		return "", ErrValidation
	}
	sum := sha256.Sum256([]byte(canonical))
	return hex.EncodeToString(sum[:]), nil
}

// PublishedArtifactPrefix is the deterministic Task 2 public-object prefix.
const PublishedArtifactPrefix = "image-agent/public"

const (
	maxArtifactOperations      = 8
	maxArtifactOperationLength = 64
)

// trustedArtifactOperations is the provider-neutral persistence vocabulary for
// generated-image provenance. It deliberately stores only finite operation
// names, never provider metadata or free-form descriptions.
var trustedArtifactOperations = map[string]struct{}{
	"select_subject": {}, "extract_subject_placeholder": {}, "cleanup_placeholder": {},
	"remove_overlay_text_placeholder": {}, "remove_promo_badge_placeholder": {},
	"remove_logo_overlay_placeholder": {}, "render_white_bg_placeholder": {},
	"extract_subject": {}, "cleanup_image": {}, "render_white_bg": {},
	"extract_subject_bbox": {}, "extract_subject_segmenter": {}, "render_white_bg_model": {},
	"compose_on_white_canvas": {}, "cleanup_overlay_signal": {}, "cleanup_quality": {},
	"remove_overlay_regions": {}, "resize": {},
	"render_scene_model": {}, "render_image_model": {}, "extract_subject_model": {},
	"normalize_for_amazon": {}, "download_source": {}, "optimize_for_amazon": {},
	"render_scene_canvas": {},
}

func ValidateStagingManifest(manifest StagingManifest) error {
	_, err := NormalizeStagingManifest(manifest)
	return err
}

func NormalizeStagingManifest(manifest StagingManifest) (StagingManifest, error) {
	if len(manifest.ProviderMetadata) != 0 {
		return StagingManifest{}, ErrValidation
	}
	assets, err := normalizeArtifactRefs(manifest.Assets)
	if err != nil {
		return StagingManifest{}, err
	}
	return StagingManifest{Assets: assets}, nil
}

func ValidateFinalManifest(manifest FinalManifest) error {
	_, err := NormalizeFinalManifest(manifest)
	return err
}

func NormalizeFinalManifest(manifest FinalManifest) (FinalManifest, error) {
	assets, err := normalizePublishedAssetRefs(manifest.Assets)
	if err != nil {
		return FinalManifest{}, err
	}
	return FinalManifest{Assets: assets}, nil
}

// ValidatePublishedAssetRefForSlot verifies that a final reference belongs to
// this exact slot attempt and uses the deterministic public object-key grammar.
func ValidatePublishedAssetRefForSlot(input SlotExecutionInput, asset PublishedAssetRef, expectedIndex int) error {
	if strings.TrimSpace(input.UserID) == "" {
		return ErrValidation
	}
	normalized, err := normalizePublishedAssetRef(asset)
	if err != nil {
		return err
	}
	_, extension, err := validatePublishedAssetIdentityForSlot(input, DurableAssetIdentity{ObjectKey: normalized.ObjectKey, SHA256: normalized.SHA256}, expectedIndex)
	if err != nil {
		return err
	}
	expectedExtension, ok := publishedArtifactExtensions[normalized.ContentType]
	if !ok || extension != expectedExtension {
		return ErrValidation
	}
	return nil
}

// ValidatePublishedAssetIdentityForSlot verifies the compact durable identity
// used after publication against the same deterministic public-key grammar as
// the full PublishedAssetRef validator.
func ValidatePublishedAssetIdentityForSlot(input SlotExecutionInput, asset DurableAssetIdentity, expectedIndex int) error {
	_, _, err := validatePublishedAssetIdentityForSlot(input, asset, expectedIndex)
	return err
}

func validatePublishedAssetIdentityForSlot(input SlotExecutionInput, asset DurableAssetIdentity, expectedIndex int) (DurableAssetIdentity, string, error) {
	tenantID := strings.TrimSpace(input.TenantID)
	runID, slotID := strings.TrimSpace(input.RunID), strings.TrimSpace(input.Slot.ID)
	ownerKey, err := ArtifactOwnerKey(input.UserID)
	if tenantID == "" || runID == "" || slotID == "" || err != nil || input.PlanRevision <= 0 || input.Attempt <= 0 || expectedIndex < 0 {
		return DurableAssetIdentity{}, "", ErrValidation
	}
	normalized, err := NormalizeDurableAssetIdentity(asset)
	if err != nil {
		return DurableAssetIdentity{}, "", err
	}
	segments := strings.Split(normalized.ObjectKey, "/")
	if len(segments) != 9 || strings.Join(segments[:2], "/") != PublishedArtifactPrefix || ValidateArtifactKeyIdentifier(segments[2]) != nil || !sha256Pattern.MatchString(segments[3]) || ValidateArtifactKeyIdentifier(segments[4]) != nil || ValidateArtifactKeyIdentifier(segments[6]) != nil {
		return DurableAssetIdentity{}, "", ErrValidation
	}
	if segments[2] != tenantID || segments[3] != ownerKey || segments[4] != runID || segments[5] != strconv.FormatInt(input.PlanRevision, 10) || segments[6] != slotID || segments[7] != strconv.Itoa(input.Attempt) {
		return DurableAssetIdentity{}, "", ErrValidation
	}
	filenamePrefix := strconv.Itoa(expectedIndex) + "-" + normalized.SHA256 + "."
	if !strings.HasPrefix(segments[8], filenamePrefix) {
		return DurableAssetIdentity{}, "", ErrValidation
	}
	extension := strings.TrimPrefix(segments[8], filenamePrefix)
	if !isPublishedArtifactExtension(extension) {
		return DurableAssetIdentity{}, "", ErrValidation
	}
	return normalized, extension, nil
}

func StagingManifestFingerprint(manifest StagingManifest) (string, error) {
	normalized, err := NormalizeStagingManifest(manifest)
	if err != nil {
		return "", err
	}
	encoded, err := json.Marshal(struct {
		Assets []StagedAssetRef `json:"assets"`
	}{Assets: normalized.Assets})
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:]), nil
}

func FinalManifestFingerprint(manifest FinalManifest) (string, error) {
	normalized, err := NormalizeFinalManifest(manifest)
	if err != nil {
		return "", err
	}
	encoded, err := json.Marshal(normalized)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:]), nil
}

func normalizeArtifactRefs(assets []StagedAssetRef) ([]StagedAssetRef, error) {
	if len(assets) == 0 {
		return nil, ErrValidation
	}
	seen := make(map[string]struct{}, len(assets))
	normalized := make([]StagedAssetRef, len(assets))
	for index, asset := range assets {
		var err error
		asset, err = normalizeStagedAssetRef(asset)
		if err != nil {
			return nil, err
		}
		if _, exists := seen[asset.ObjectKey]; exists {
			return nil, ErrValidation
		}
		seen[asset.ObjectKey] = struct{}{}
		normalized[index] = asset
	}
	return normalized, nil
}

func normalizePublishedAssetRefs(assets []PublishedAssetRef) ([]PublishedAssetRef, error) {
	if len(assets) == 0 {
		return nil, ErrValidation
	}
	seen := make(map[string]struct{}, len(assets))
	normalized := make([]PublishedAssetRef, len(assets))
	for index, asset := range assets {
		var err error
		asset, err = normalizePublishedAssetRef(asset)
		if err != nil {
			return nil, err
		}
		if _, exists := seen[asset.ObjectKey]; exists {
			return nil, ErrValidation
		}
		seen[asset.ObjectKey] = struct{}{}
		normalized[index] = asset
	}
	return normalized, nil
}

func normalizeStagedAssetRef(asset StagedAssetRef) (StagedAssetRef, error) {
	identity, err := NormalizeDurableAssetIdentity(DurableAssetIdentity{ObjectKey: asset.ObjectKey, SHA256: asset.SHA256})
	if err != nil || asset.SizeBytes <= 0 || asset.Width <= 0 || asset.Height <= 0 || ValidateProvenanceAssetID(asset.SourceAssetID) != nil || asset.ProviderReceiptID != strings.TrimSpace(asset.ProviderReceiptID) {
		return StagedAssetRef{}, ErrValidation
	}
	switch asset.ContentType {
	case "image/jpeg", "image/png", "image/webp":
	default:
		return StagedAssetRef{}, ErrValidation
	}
	operations, err := NormalizeArtifactOperations(asset.Operations)
	if err != nil {
		return StagedAssetRef{}, err
	}
	asset.ObjectKey = identity.ObjectKey
	asset.SHA256 = identity.SHA256
	asset.Operations = operations
	return asset, nil
}

func normalizePublishedAssetRef(asset PublishedAssetRef) (PublishedAssetRef, error) {
	identity, err := NormalizeDurableAssetIdentity(DurableAssetIdentity{ObjectKey: asset.ObjectKey, SHA256: asset.SHA256})
	if err != nil || asset.SizeBytes <= 0 || asset.Width <= 0 || asset.Height <= 0 || ValidateProvenanceAssetID(asset.SourceAssetID) != nil || asset.ProviderReceiptID != strings.TrimSpace(asset.ProviderReceiptID) {
		return PublishedAssetRef{}, ErrValidation
	}
	switch asset.ContentType {
	case "image/jpeg", "image/png", "image/webp":
	default:
		return PublishedAssetRef{}, ErrValidation
	}
	operations, err := NormalizeArtifactOperations(asset.Operations)
	if err != nil {
		return PublishedAssetRef{}, err
	}
	asset.ObjectKey = identity.ObjectKey
	asset.SHA256 = identity.SHA256
	asset.Operations = operations
	return asset, nil
}

// NormalizeArtifactOperations validates and defensively copies the only
// operation values permitted in a persisted artifact manifest.
func NormalizeArtifactOperations(operations []string) ([]string, error) {
	if len(operations) > maxArtifactOperations {
		return nil, ErrValidation
	}
	if operations == nil {
		return nil, nil
	}
	normalized := make([]string, len(operations))
	for index, operation := range operations {
		if len(operation) == 0 || len(operation) > maxArtifactOperationLength {
			return nil, ErrValidation
		}
		if _, ok := trustedArtifactOperations[operation]; !ok {
			return nil, ErrValidation
		}
		normalized[index] = operation
	}
	return normalized, nil
}

func NormalizeDurableAssetIdentity(asset DurableAssetIdentity) (DurableAssetIdentity, error) {
	if !isCanonicalObjectKey(asset.ObjectKey) || !sha256Pattern.MatchString(asset.SHA256) {
		return DurableAssetIdentity{}, ErrValidation
	}
	return DurableAssetIdentity{ObjectKey: asset.ObjectKey, SHA256: strings.ToLower(asset.SHA256)}, nil
}

var publishedArtifactExtensions = map[string]string{
	"image/jpeg": "jpg",
	"image/png":  "png",
	"image/webp": "webp",
}

func isPublishedArtifactExtension(extension string) bool {
	for _, allowed := range publishedArtifactExtensions {
		if extension == allowed {
			return true
		}
	}
	return false
}

func isCanonicalObjectKey(value string) bool {
	if value == "" || value != strings.TrimSpace(value) || strings.Contains(value, "\\") || strings.Contains(value, ":") || strings.HasPrefix(value, "/") || path.IsAbs(value) {
		return false
	}
	for _, segment := range strings.Split(value, "/") {
		if segment == "" || segment == "." || segment == ".." {
			return false
		}
	}
	return true
}
