// Package readinessinspect exposes exact Product input diagnostics. It does
// not evaluate marketplace payload rules or authorize publication.
package readinessinspect

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"task-processor/internal/listing/readiness"
	"task-processor/internal/product/asset"
	"task-processor/internal/product/catalog"
)

const MaxSnapshotBytes = 2 << 20
const MaxOutputBytes = 64 << 10
const MaxDiagnostics = 128

// InputRuleVersion identifies this Tool's frozen composition of the existing
// ProductInputs rule. It is not a SHEIN marketplace rule version.
const InputRuleVersion = "listing.readiness.ProductInputs/tool-v1.0.0"

var ErrTooLarge = errors.New("input diagnostics exceed bounds")
var ErrCorrupt = errors.New("input diagnostics binding is invalid")

type Input struct {
	ProductKey     string `json:"product_key"`
	CatalogVersion string `json:"catalog_version"`
	TargetPlatform string `json:"target_platform"`
}

type AssetBinding struct {
	ProductVersion string `json:"product_version"`
	Status         string `json:"status"`
	Digest         string `json:"digest,omitempty"`
	Count          int    `json:"count"`
}

type MarketplaceDiagnostic struct {
	Status  string   `json:"status"`
	Reasons []string `json:"reasons"`
}

type Diagnostics struct {
	NeedsReview   bool              `json:"needs_review"`
	ReviewReasons []string          `json:"review_reasons"`
	Warnings      []catalog.Warning `json:"warnings"`
}

type Output struct {
	Scope            string                `json:"scope"`
	Status           string                `json:"status"`
	ProductKey       string                `json:"product_key"`
	ProductVersion   string                `json:"product_version"`
	PublicationID    string                `json:"publication_id"`
	TargetPlatform   string                `json:"target_platform"`
	InputRuleVersion string                `json:"input_rule_version,omitempty"`
	Reasons          []string              `json:"reasons"`
	MissingFacts     []string              `json:"missing_facts"`
	AssetBinding     AssetBinding          `json:"asset_binding"`
	Marketplace      MarketplaceDiagnostic `json:"marketplace"`
	Diagnostics      Diagnostics           `json:"diagnostics"`
}

// Project consumes only authoritative exact reads. A nil inventory means an
// exact miss; any supplied inventory must have the full matching binding.
func Project(product catalog.PublishedSnapshot, inventory *asset.ApprovedAssetInventory, target string) (json.RawMessage, error) {
	if catalog.ValidateSnapshotIdentity(product.Identity) != nil || product.Version == 0 || product.Version > 1<<63-1 ||
		!boundedText(product.PublicationID, 128) || !boundedText(target, 128) {
		return nil, ErrCorrupt
	}
	raw, err := json.Marshal(product.Snapshot)
	if err != nil {
		return nil, ErrCorrupt
	}
	if len(raw) > MaxSnapshotBytes {
		return nil, ErrTooLarge
	}
	if _, err := catalog.CloneProductSnapshot(product.Snapshot); err != nil {
		return nil, ErrCorrupt
	}
	out := Output{Scope: "product.inputs", Status: "not_evaluated", ProductKey: product.Identity.ProductKey,
		ProductVersion: strconv.FormatUint(product.Version, 10), PublicationID: product.PublicationID, TargetPlatform: target,
		Reasons: []string{}, MissingFacts: []string{},
		AssetBinding: AssetBinding{ProductVersion: strconv.FormatUint(product.Version, 10), Status: "missing"},
		Marketplace:  MarketplaceDiagnostic{Status: "not_evaluated", Reasons: []string{"marketplace_rules_out_of_scope"}},
		Diagnostics:  Diagnostics{ReviewReasons: []string{}, Warnings: append([]catalog.Warning{}, product.Snapshot.Warnings...)}}
	if product.Snapshot.Review != nil {
		out.Diagnostics.NeedsReview = product.Snapshot.Review.NeedsReview
		out.Diagnostics.ReviewReasons = append([]string{}, product.Snapshot.Review.Reasons...)
	}
	if len(out.Diagnostics.Warnings) > MaxDiagnostics || len(out.Diagnostics.ReviewReasons) > MaxDiagnostics {
		return nil, ErrTooLarge
	}
	if inventory != nil {
		scope := asset.InventoryScope{TenantID: product.Identity.TenantID, ProductKey: product.Identity.ProductKey,
			TargetPlatform: target, SourceSnapshotVersion: product.Version}
		if inventory.Scope != scope {
			return nil, ErrCorrupt
		}
		if len(inventory.Assets) > 1024 {
			return nil, ErrTooLarge
		}
		if len(inventory.Assets) != 0 {
			if err := asset.ValidateApprovalCommit(asset.ApprovalCommit{TenantID: scope.TenantID, ProductKey: scope.ProductKey,
				TargetPlatform: target, SourceSnapshotVersion: product.Version, ActionID: "readiness-exact-read", Assets: inventory.Assets}); err != nil {
				return nil, ErrCorrupt
			}
			copyInventory := *inventory
			copyInventory.Assets = append([]asset.ApprovedAsset(nil), inventory.Assets...)
			sort.Slice(copyInventory.Assets, func(i, j int) bool { return copyInventory.Assets[i].ID < copyInventory.Assets[j].ID })
			encoded, encodeErr := json.Marshal(copyInventory)
			if encodeErr != nil {
				return nil, ErrCorrupt
			}
			if len(encoded) > MaxSnapshotBytes {
				return nil, ErrTooLarge
			}
			sum := sha256.Sum256(encoded)
			out.AssetBinding.Status, out.AssetBinding.Count = "exact", len(copyInventory.Assets)
			out.AssetBinding.Digest = "sha256:" + hex.EncodeToString(sum[:])
		}
	}
	{
		gate := readiness.ProductInputs(product, inventory, target)
		out.InputRuleVersion = InputRuleVersion
		out.Status = "ready"
		if !gate.Ready {
			out.Status = "blocked"
		}
		out.Reasons = append(out.Reasons, gate.Blockers...)
		for _, code := range gate.Blockers {
			if code == "approved_assets_not_ready" {
				out.MissingFacts = append(out.MissingFacts, "approved_assets")
			}
		}
	}
	encoded, err := json.Marshal(out)
	if err != nil {
		return nil, fmt.Errorf("%w: output encoding", ErrCorrupt)
	}
	if len(encoded) > MaxOutputBytes {
		return nil, ErrTooLarge
	}
	return encoded, nil
}

func boundedText(value string, limit int) bool {
	return value != "" && strings.TrimSpace(value) == value && utf8.ValidString(value) &&
		utf8.RuneCountInString(value) <= limit && !strings.ContainsAny(value, "\x00\r\n\t")
}
