package assetinspect

import (
	"encoding/json"
	"errors"
	"math"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"task-processor/internal/product/asset"
	"task-processor/internal/product/catalog"
	"task-processor/internal/product/catalog/tools/canonicalinspect"
)

const MaxOutputBytes = 1 << 20

var (
	errProjectionTooLarge = errors.New("asset facts exceed output limit")
	errInvalidFacts       = errors.New("asset facts binding is invalid")
	errMissingFacts       = errors.New("approved asset facts are missing")
)

type AssetBinding struct {
	TargetPlatform        string `json:"target_platform"`
	SourceSnapshotVersion string `json:"source_snapshot_version"`
}

type Output struct {
	canonicalinspect.Output
	AssetBinding   AssetBinding          `json:"asset_binding"`
	ApprovedAssets []asset.ApprovedAsset `json:"approved_assets"`
}

func project(published catalog.PublishedSnapshot, inventory asset.ApprovedAssetInventory, expected asset.InventoryScope) (json.RawMessage, error) {
	if expected.SourceSnapshotVersion == 0 || expected.SourceSnapshotVersion > math.MaxInt64 || expected.TargetPlatform == "" ||
		asset.ValidateInventoryScope(expected) != nil || inventory.Scope != expected ||
		published.Identity != (catalog.SnapshotIdentity{TenantID: expected.TenantID, ProductKey: expected.ProductKey}) ||
		published.Version != expected.SourceSnapshotVersion || !validIdentity(published.PublicationID) {
		return nil, errInvalidFacts
	}
	if len(inventory.Assets) == 0 {
		return nil, errMissingFacts
	}
	// Reuse the Asset owner's validation only. This marker is not an approval
	// action or receipt and is never persisted or exposed in the projection.
	if err := asset.ValidateApprovalCommit(asset.ApprovalCommit{
		TenantID: expected.TenantID, ProductKey: expected.ProductKey, TargetPlatform: expected.TargetPlatform,
		SourceSnapshotVersion: expected.SourceSnapshotVersion, ActionID: "tool-a1-exact-read", Assets: inventory.Assets,
	}); err != nil {
		return nil, errInvalidFacts
	}
	product, err := canonicalinspect.Project(published)
	if errors.Is(err, canonicalinspect.ErrProjectionTooLarge) {
		return nil, errProjectionTooLarge
	}
	if err != nil {
		return nil, errInvalidFacts
	}
	var output Output
	if err := json.Unmarshal(product, &output.Output); err != nil {
		return nil, errInvalidFacts
	}
	output.AssetBinding = AssetBinding{TargetPlatform: expected.TargetPlatform, SourceSnapshotVersion: strconv.FormatUint(expected.SourceSnapshotVersion, 10)}
	output.ApprovedAssets = asset.CloneApprovedAssetInventory(inventory).Assets
	sort.Slice(output.ApprovedAssets, func(i, j int) bool { return output.ApprovedAssets[i].ID < output.ApprovedAssets[j].ID })
	encoded, err := json.Marshal(output)
	if err != nil {
		return nil, errInvalidFacts
	}
	if len(encoded) > MaxOutputBytes {
		return nil, errProjectionTooLarge
	}
	return encoded, nil
}

func validIdentity(value string) bool {
	return value != "" && value == strings.TrimSpace(value) && utf8.ValidString(value) && utf8.RuneCountInString(value) <= asset.MaxIdentityLength
}
