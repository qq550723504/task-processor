package imageagent

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"path"
	"regexp"
	"strings"
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

// StagingManifest carries only durable staging identities. ProviderMetadata is
// deliberately excluded from JSON and rejected so callers cannot smuggle an
// open-ended metadata map into a durable effect record.
type StagingManifest struct {
	Assets           []StagedAssetRef  `json:"assets"`
	ProviderMetadata map[string]string `json:"-"`
}

type FinalManifest struct {
	Assets []StagedAssetRef `json:"assets"`
}

type DurableAssetIdentity struct {
	ObjectKey string `json:"object_key"`
	SHA256    string `json:"sha256"`
}

var sha256Pattern = regexp.MustCompile(`^[a-fA-F0-9]{64}$`)

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
	assets, err := normalizeArtifactRefs(manifest.Assets)
	if err != nil {
		return FinalManifest{}, err
	}
	return FinalManifest{Assets: assets}, nil
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

func normalizeStagedAssetRef(asset StagedAssetRef) (StagedAssetRef, error) {
	identity, err := NormalizeDurableAssetIdentity(DurableAssetIdentity{ObjectKey: asset.ObjectKey, SHA256: asset.SHA256})
	if err != nil || asset.SizeBytes <= 0 || asset.Width <= 0 || asset.Height <= 0 || asset.SourceAssetID == "" || asset.SourceAssetID != strings.TrimSpace(asset.SourceAssetID) || asset.ProviderReceiptID != strings.TrimSpace(asset.ProviderReceiptID) {
		return StagedAssetRef{}, ErrValidation
	}
	switch asset.ContentType {
	case "image/jpeg", "image/png", "image/webp":
	default:
		return StagedAssetRef{}, ErrValidation
	}
	for _, operation := range asset.Operations {
		if operation == "" || operation != strings.TrimSpace(operation) {
			return StagedAssetRef{}, ErrValidation
		}
	}
	asset.ObjectKey = identity.ObjectKey
	asset.SHA256 = identity.SHA256
	return asset, nil
}

func NormalizeDurableAssetIdentity(asset DurableAssetIdentity) (DurableAssetIdentity, error) {
	if !isCanonicalObjectKey(asset.ObjectKey) || !sha256Pattern.MatchString(asset.SHA256) {
		return DurableAssetIdentity{}, ErrValidation
	}
	return DurableAssetIdentity{ObjectKey: asset.ObjectKey, SHA256: strings.ToLower(asset.SHA256)}, nil
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
