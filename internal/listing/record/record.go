// Package record owns immutable, locally saved SHEIN draft records.
package record

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"time"

	"task-processor/internal/authidentity"
	"task-processor/internal/authz"
	listingtask "task-processor/internal/listing/task"
	contract "task-processor/internal/marketplace/validator"
	productasset "task-processor/internal/product/asset"
	"task-processor/internal/product/catalog"

	"github.com/google/uuid"
)

const Timeout = 10 * time.Second
const MaxPayloadBytes = 2 << 20

var (
	ErrInvalid     = errors.New("invalid listing record request")
	ErrForbidden   = errors.New("listing record permission denied")
	ErrNotFound    = errors.New("listing record input or resource unavailable")
	ErrNotReady    = errors.New("listing record exact inputs are not ready")
	ErrConflict    = errors.New("listing record operation conflict")
	ErrUnavailable = errors.New("listing record dependency unavailable")
	ErrTooLarge    = errors.New("listing record payload exceeds limit")
)

type Input struct {
	ProductKey      string          `json:"product_key"`
	SnapshotVersion uint64          `json:"snapshot_version"`
	StoreID         string          `json:"store_id"`
	Country         string          `json:"country"`
	Language        string          `json:"language"`
	Action          contract.Action `json:"action"`
}

func (i Input) Validate() error {
	storeID, storeErr := uuid.Parse(i.StoreID)
	if i.ProductKey == "" || i.ProductKey != strings.TrimSpace(i.ProductKey) || len(i.ProductKey) > 128 ||
		i.SnapshotVersion == 0 || i.SnapshotVersion > 1<<63-1 || storeErr != nil || storeID == uuid.Nil || storeID.String() != i.StoreID ||
		i.Country != "US" || i.Language != "en" || (i.Action != contract.SaveDraft && i.Action != contract.Publish) {
		return ErrInvalid
	}
	return nil
}

type Record struct {
	ID                    string
	OrganizationID        string
	OwnerUserID           string
	OperationID           string
	Input                 Input
	InputHash             string
	ProductHash           string
	AssetInventoryVersion uint64
	AssetInventoryHash    string
	RuleRevision          string
	PolicyRevision        string
	PackageHash           string
	DiagnosticHash        string
	DiagnosticStatus      contract.Status
	Payload               []byte
	Diagnostic            []byte
	CreatedAt             time.Time
	ReadAt                time.Time
}

func (r Record) Clone() Record {
	r.Payload = append([]byte(nil), r.Payload...)
	r.Diagnostic = append([]byte(nil), r.Diagnostic...)
	return r
}

type Receipt struct {
	RecordID         string          `json:"record_id"`
	InputHash        string          `json:"input_hash"`
	DiagnosticHash   string          `json:"diagnostic_hash"`
	DiagnosticStatus contract.Status `json:"diagnostic_status"`
	DiagnosticOnly   bool            `json:"diagnostic_only"`
}

// Prepared can only be populated by the admitted creation use case. There is
// no exported arbitrary-payload constructor or mutable persistence record.
type Prepared struct{ record Record }

func (p Prepared) Record() Record { return p.record.Clone() }

type Reader interface {
	ReadOfflinePackage(context.Context, listingtask.Actor, string) (Record, error)
}

type Store interface {
	FindOperation(context.Context, listingtask.Actor, string) (Record, error)
	Insert(context.Context, Prepared) (Record, error)
}

type Authorizer interface {
	Authorize(string, []string, string) bool
	IsTenantAdmin(string, []string) bool
}

type ApprovedAssetReader interface {
	GetApprovedInventory(context.Context, productasset.InventoryScope) (productasset.ApprovedAssetInventory, error)
}

type StoreReference struct {
	OrganizationID string
	StoreID        string
	Platform       string
}

type StoreReferenceReader interface {
	GetStoreReference(context.Context, string, string) (StoreReference, error)
}

type Builder interface {
	Build(context.Context, catalog.ProductSnapshot, productasset.ApprovedAssetInventory, Input) ([]byte, error)
}

type ServiceDependencies struct {
	Products       catalog.VersionedSnapshotReader
	Assets         ApprovedAssetReader
	Stores         StoreReferenceReader
	Records        Store
	Builder        Builder
	Evaluator      DiagnosticEvaluator
	Authorizer     Authorizer
	Now            func() time.Time
	RuleRevision   string
	PolicyRevision string
}

type Service struct{ dependencies ServiceDependencies }

func NewService(dependencies ServiceDependencies) (*Service, error) {
	if dependencies.Products == nil || dependencies.Assets == nil || dependencies.Stores == nil || dependencies.Records == nil ||
		dependencies.Builder == nil || dependencies.Evaluator == nil || dependencies.Authorizer == nil || dependencies.Now == nil ||
		dependencies.RuleRevision == "" || dependencies.PolicyRevision == "" {
		return nil, ErrUnavailable
	}
	return &Service{dependencies: dependencies}, nil
}

func (s *Service) Create(ctx context.Context, operation string, input Input) (Receipt, error) {
	ctx, cancel := context.WithTimeout(ctx, Timeout)
	defer cancel()
	if err := ctx.Err(); err != nil {
		return Receipt{}, err
	}
	identity, ok := authidentity.AuthenticatedIdentityFromContext(ctx)
	actor := listingtask.Actor{TenantID: identity.EffectiveOrganizationID, UserID: identity.UserID, Roles: identity.Roles}
	if !ok || identity.TenantID != actor.TenantID || listingtask.ValidateActor(actor) != nil || !s.dependencies.Now().Before(identity.TokenExpiresAt) {
		return Receipt{}, ErrForbidden
	}
	if !s.dependencies.Authorizer.Authorize(actor.UserID, actor.Roles, authz.PermissionListingKitAdminWrite) {
		return Receipt{}, ErrForbidden
	}
	if input.Validate() != nil || listingtask.ValidateTaskID(operation) != nil {
		return Receipt{}, ErrInvalid
	}
	existing, lookupErr := s.dependencies.Records.FindOperation(ctx, actor, operation)
	if lookupErr == nil && (existing.OwnerUserID != actor.UserID || existing.Input != input) {
		return Receipt{}, ErrConflict
	}
	if lookupErr != nil && !errors.Is(lookupErr, ErrNotFound) {
		return Receipt{}, lookupErr
	}
	if err := ctx.Err(); err != nil {
		return Receipt{}, err
	}

	store, err := s.dependencies.Stores.GetStoreReference(ctx, actor.TenantID, input.StoreID)
	if err != nil {
		return Receipt{}, err
	}
	if store.OrganizationID != actor.TenantID || store.StoreID != input.StoreID || store.Platform != "shein" {
		return Receipt{}, ErrNotFound
	}
	if err := ctx.Err(); err != nil {
		return Receipt{}, err
	}

	sourceID := catalog.SnapshotIdentity{TenantID: actor.TenantID, ProductKey: input.ProductKey}
	published, err := s.dependencies.Products.GetSnapshot(ctx, sourceID, input.SnapshotVersion)
	if errors.Is(err, catalog.ErrSnapshotNotReady) {
		return Receipt{}, ErrNotFound
	}
	if errors.Is(err, catalog.ErrSnapshotTooLarge) {
		return Receipt{}, ErrTooLarge
	}
	if err != nil {
		return Receipt{}, err
	}
	if published.Identity != sourceID || published.Version != input.SnapshotVersion {
		return Receipt{}, ErrNotFound
	}
	productBytes, err := json.Marshal(published.Snapshot)
	if err != nil {
		return Receipt{}, ErrUnavailable
	}
	if len(productBytes) == 0 || len(productBytes) > MaxPayloadBytes {
		return Receipt{}, ErrTooLarge
	}
	productHash := digest(productBytes)

	inventoryScope := productasset.InventoryScope{
		TenantID: actor.TenantID, ProductKey: input.ProductKey, TargetPlatform: "shein", SourceSnapshotVersion: input.SnapshotVersion,
	}
	inventory, err := s.dependencies.Assets.GetApprovedInventory(ctx, inventoryScope)
	if errors.Is(err, productasset.ErrApprovedAssetsNotReady) {
		return Receipt{}, ErrNotReady
	}
	if errors.Is(err, productasset.ErrInventoryTooLarge) {
		return Receipt{}, ErrTooLarge
	}
	if err != nil {
		return Receipt{}, err
	}
	if inventory.Scope != inventoryScope || len(inventory.Assets) == 0 {
		return Receipt{}, ErrNotReady
	}
	inventory = canonicalInventory(inventory)
	if err := productasset.ValidateApprovalCommit(productasset.ApprovalCommit{
		TenantID: inventory.Scope.TenantID, ProductKey: inventory.Scope.ProductKey, TargetPlatform: inventory.Scope.TargetPlatform,
		SourceSnapshotVersion: inventory.Scope.SourceSnapshotVersion, ActionID: "draft-s1-exact-read", Assets: inventory.Assets,
	}); err != nil {
		return Receipt{}, ErrUnavailable
	}
	inventoryBytes, err := json.Marshal(inventory)
	if err != nil {
		return Receipt{}, ErrUnavailable
	}
	if len(inventoryBytes) == 0 || len(inventoryBytes) > MaxPayloadBytes {
		return Receipt{}, ErrTooLarge
	}
	inventoryHash := digest(inventoryBytes)
	if err := ctx.Err(); err != nil {
		return Receipt{}, err
	}

	payload, err := s.dependencies.Builder.Build(ctx, published.Snapshot, inventory, input)
	if err != nil {
		return Receipt{}, err
	}
	if len(payload) == 0 || len(payload) > MaxPayloadBytes {
		return Receipt{}, ErrTooLarge
	}
	packageHash := digest(payload)
	evaluatedAt := s.dependencies.Now().UTC()
	if evaluatedAt.IsZero() {
		return Receipt{}, ErrUnavailable
	}
	diagnostic, err := s.dependencies.Evaluator.Validate(contract.BoundRequest[[]byte]{
		Input: append([]byte(nil), payload...), Target: contract.Target{Marketplace: "shein"}, Action: input.Action,
		RuleVersion: s.dependencies.RuleRevision, BindingVersion: s.dependencies.PolicyRevision,
		ReadAt: evaluatedAt, EvaluatedAt: evaluatedAt, Freshness: contract.ExternalFreshness{Status: contract.NotEvaluated},
	})
	if err != nil {
		return Receipt{}, err
	}
	if !diagnostic.DiagnosticOnly || diagnostic.Target.Marketplace != "shein" || diagnostic.Target.Site != "" || diagnostic.Action != input.Action ||
		diagnostic.RuleVersion != s.dependencies.RuleRevision || diagnostic.Input.BindingVersion != s.dependencies.PolicyRevision ||
		!contract.ValidContentDigest(diagnostic.Input.Digest) || diagnostic.Freshness.Status != contract.NotEvaluated {
		return Receipt{}, ErrUnavailable
	}
	diagnosticBytes, err := json.Marshal(diagnostic)
	if err != nil || len(diagnosticBytes) == 0 || len(diagnosticBytes) > MaxPayloadBytes {
		return Receipt{}, ErrTooLarge
	}
	diagnosticHash := digest(diagnosticBytes)
	inputHash, err := canonicalInputHash(actor, input, productHash, inventoryHash, s.dependencies.RuleRevision, s.dependencies.PolicyRevision)
	if err != nil {
		return Receipt{}, ErrUnavailable
	}
	if err := ctx.Err(); err != nil {
		return Receipt{}, err
	}

	created, err := s.dependencies.Records.Insert(ctx, Prepared{record: Record{
		ID: uuid.NewString(), OrganizationID: actor.TenantID, OwnerUserID: actor.UserID, OperationID: operation,
		Input: input, InputHash: inputHash, ProductHash: productHash,
		AssetInventoryVersion: input.SnapshotVersion, AssetInventoryHash: inventoryHash,
		RuleRevision: s.dependencies.RuleRevision, PolicyRevision: s.dependencies.PolicyRevision,
		PackageHash: packageHash, DiagnosticHash: diagnosticHash, DiagnosticStatus: diagnostic.OfflineChecks.Status,
		Payload: append([]byte(nil), payload...), Diagnostic: diagnosticBytes,
	}})
	if err != nil {
		return Receipt{}, err
	}
	if err := ctx.Err(); err != nil {
		return Receipt{}, err
	}
	return receiptFor(created, actor, operation, inputHash)
}

func canonicalInventory(inventory productasset.ApprovedAssetInventory) productasset.ApprovedAssetInventory {
	inventory = productasset.CloneApprovedAssetInventory(inventory)
	sort.Slice(inventory.Assets, func(i, j int) bool {
		left, right := inventory.Assets[i], inventory.Assets[j]
		if left.SlotID != right.SlotID {
			return left.SlotID < right.SlotID
		}
		if left.Attempt != right.Attempt {
			return left.Attempt < right.Attempt
		}
		return left.ID < right.ID
	})
	return inventory
}

func canonicalInputHash(actor listingtask.Actor, input Input, productHash, inventoryHash, ruleRevision, policyRevision string) (string, error) {
	wire, err := json.Marshal(struct {
		OrganizationID     string `json:"organization_id"`
		OwnerUserID        string `json:"owner_user_id"`
		ProductKey         string `json:"product_key"`
		SnapshotVersion    uint64 `json:"snapshot_version"`
		StoreID            string `json:"store_id"`
		Country            string `json:"country"`
		Language           string `json:"language"`
		Action             string `json:"action"`
		ProductHash        string `json:"product_hash"`
		AssetInventoryHash string `json:"asset_inventory_hash"`
		RuleRevision       string `json:"rule_revision"`
		PolicyRevision     string `json:"policy_revision"`
	}{actor.TenantID, actor.UserID, input.ProductKey, input.SnapshotVersion, input.StoreID, input.Country, input.Language, string(input.Action), productHash, inventoryHash, ruleRevision, policyRevision})
	if err != nil {
		return "", err
	}
	return digest(wire), nil
}

func digest(value []byte) string {
	sum := sha256.Sum256(value)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func receiptFor(r Record, actor listingtask.Actor, operation, inputHash string) (Receipt, error) {
	if r.OrganizationID != actor.TenantID || r.OwnerUserID != actor.UserID || r.OperationID != operation || r.InputHash != inputHash ||
		r.ID == "" || !contract.ValidContentDigest(r.DiagnosticHash) || r.DiagnosticStatus == "" {
		return Receipt{}, ErrConflict
	}
	return Receipt{RecordID: r.ID, InputHash: r.InputHash, DiagnosticHash: r.DiagnosticHash, DiagnosticStatus: r.DiagnosticStatus, DiagnosticOnly: true}, nil
}
