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

var sha256Pattern = regexp.MustCompile(`^[a-f0-9]{64}$`)

func ValidateStagingManifest(manifest StagingManifest) error {
	if len(manifest.ProviderMetadata) != 0 {
		return ErrValidation
	}
	return validateArtifactRefs(manifest.Assets)
}

func ValidateFinalManifest(manifest FinalManifest) error {
	return validateArtifactRefs(manifest.Assets)
}

func StagingManifestFingerprint(manifest StagingManifest) (string, error) {
	if err := ValidateStagingManifest(manifest); err != nil {
		return "", err
	}
	encoded, err := json.Marshal(struct {
		Assets []StagedAssetRef `json:"assets"`
	}{Assets: manifest.Assets})
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:]), nil
}

func FinalManifestFingerprint(manifest FinalManifest) (string, error) {
	if err := ValidateFinalManifest(manifest); err != nil {
		return "", err
	}
	encoded, err := json.Marshal(manifest)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:]), nil
}

func validateArtifactRefs(assets []StagedAssetRef) error {
	if len(assets) == 0 {
		return ErrValidation
	}
	seen := make(map[string]struct{}, len(assets))
	for _, asset := range assets {
		if err := validateStagedAssetRef(asset); err != nil {
			return err
		}
		if _, exists := seen[asset.ObjectKey]; exists {
			return ErrValidation
		}
		seen[asset.ObjectKey] = struct{}{}
	}
	return nil
}

func validateStagedAssetRef(asset StagedAssetRef) error {
	if !isCanonicalObjectKey(asset.ObjectKey) || !sha256Pattern.MatchString(asset.SHA256) || asset.SizeBytes <= 0 || asset.Width <= 0 || asset.Height <= 0 || strings.TrimSpace(asset.SourceAssetID) == "" {
		return ErrValidation
	}
	switch asset.ContentType {
	case "image/jpeg", "image/png", "image/webp":
	default:
		return ErrValidation
	}
	for _, operation := range asset.Operations {
		if strings.TrimSpace(operation) == "" {
			return ErrValidation
		}
	}
	return nil
}

func isCanonicalObjectKey(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" || strings.Contains(value, "\\") || strings.Contains(value, ":") || strings.HasPrefix(value, "/") || path.IsAbs(value) {
		return false
	}
	for _, segment := range strings.Split(value, "/") {
		if segment == "" || segment == "." || segment == ".." {
			return false
		}
	}
	return true
}
