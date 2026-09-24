package listingsubscription

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"time"
)

const PurchasedPlanSourceCommercialOrder = "commercial_subscription_order"

var (
	ErrPurchasablePlanUnavailable         = errors.New("purchasable subscription plan is unavailable")
	ErrPurchasedPlanActivationNotFound    = errors.New("purchased plan activation is not found")
	ErrPurchasedPlanActivationConflict    = errors.New("purchased plan activation conflicts with the durable source")
	ErrPurchasedPlanActivationUnavailable = errors.New("purchased plan activation is unavailable")
)

type PurchasedPlanSnapshot struct {
	PlanCode    string
	DisplayName string
	Fingerprint string
}

// PurchasedSubscriptionState is the narrow subscription-owner projection used
// to derive offer availability for one Effective Organization. It exposes no
// entitlement mutation or catalog-management capability.
type PurchasedSubscriptionState struct {
	PlanCode       string
	BlocksPurchase bool
}

type PurchasedPlanActivationOutcome string

const (
	PurchasedPlanActivationActivated PurchasedPlanActivationOutcome = "ACTIVATED"
	PurchasedPlanActivationRejected  PurchasedPlanActivationOutcome = "REJECTED"
)

type PurchasedPlanActivationFailureCode string

const (
	PurchasedPlanActivationActiveSubscriptionExists PurchasedPlanActivationFailureCode = "ACTIVE_SUBSCRIPTION_EXISTS"
	PurchasedPlanActivationPlanChanged              PurchasedPlanActivationFailureCode = "PLAN_CHANGED"
)

type PurchasedPlanActivationInput struct {
	OperationID     string
	OrganizationID  string
	ActorID         string
	SourceType      string
	SourceID        string
	PlanCode        string
	PlanFingerprint string
	TermMonths      int
}

type PurchasedPlanActivationResult struct {
	OperationID                  string
	OrganizationID               string
	SourceType                   string
	SourceID                     string
	PlanCode                     string
	PlanFingerprint              string
	ActivationRequestFingerprint string
	Outcome                      PurchasedPlanActivationOutcome
	FailureCode                  PurchasedPlanActivationFailureCode
	SubscriptionID               int64
	StartsAt                     *time.Time
	ExpiresAt                    *time.Time
	EntitlementSetFingerprint    string
	DecidedAt                    time.Time
	Existing                     bool
}

type purchasedPlanActivationRepository interface {
	ResolvePurchasablePlan(context.Context, string) (PurchasedPlanSnapshot, error)
	ReadPurchasedSubscriptionState(context.Context, string, time.Time) (PurchasedSubscriptionState, error)
	ActivatePurchasedPlan(context.Context, PurchasedPlanActivationInput, time.Time) (PurchasedPlanActivationResult, error)
	ReadPurchasedPlanActivation(context.Context, string, string) (PurchasedPlanActivationResult, error)
}

// NewRuntimeService constructs the serving subscription owner without mutating
// the plan catalog. Catalog provisioning must complete before the runtime starts.
func NewRuntimeService(repo Repository) (*Service, error) {
	if repositoryIsNil(repo) {
		return nil, errors.New("subscription repository is required")
	}
	return &Service{repo: repo, now: time.Now, tenantDisplayNameResolver: fallbackTenantDisplayNameResolver{}}, nil
}

func (s *Service) ResolvePurchasablePlan(ctx context.Context, planCode string) (PurchasedPlanSnapshot, error) {
	repository, ok := s.repo.(purchasedPlanActivationRepository)
	if !ok {
		return PurchasedPlanSnapshot{}, ErrPurchasedPlanActivationUnavailable
	}
	return repository.ResolvePurchasablePlan(ctx, strings.TrimSpace(planCode))
}

func (s *Service) ReadPurchasedSubscriptionState(ctx context.Context, organizationID string) (PurchasedSubscriptionState, error) {
	repository, ok := s.repo.(purchasedPlanActivationRepository)
	if !ok {
		return PurchasedSubscriptionState{}, ErrPurchasedPlanActivationUnavailable
	}
	organizationID = strings.TrimSpace(organizationID)
	if !boundedPurchasedIdentifier(organizationID) {
		return PurchasedSubscriptionState{}, ErrPurchasedPlanActivationConflict
	}
	return repository.ReadPurchasedSubscriptionState(ctx, organizationID, s.now().UTC())
}

func (s *Service) ActivatePurchasedPlan(ctx context.Context, input PurchasedPlanActivationInput) (PurchasedPlanActivationResult, error) {
	repository, ok := s.repo.(purchasedPlanActivationRepository)
	if !ok {
		return PurchasedPlanActivationResult{}, ErrPurchasedPlanActivationUnavailable
	}
	if err := validatePurchasedPlanActivationInput(input); err != nil {
		return PurchasedPlanActivationResult{}, err
	}
	return repository.ActivatePurchasedPlan(ctx, normalizePurchasedPlanActivationInput(input), s.now().UTC())
}

func (s *Service) ReadPurchasedPlanActivation(ctx context.Context, organizationID, sourceID string) (PurchasedPlanActivationResult, error) {
	repository, ok := s.repo.(purchasedPlanActivationRepository)
	if !ok {
		return PurchasedPlanActivationResult{}, ErrPurchasedPlanActivationUnavailable
	}
	organizationID = strings.TrimSpace(organizationID)
	sourceID = strings.TrimSpace(sourceID)
	if !boundedPurchasedIdentifier(organizationID) || !boundedPurchasedIdentifier(sourceID) {
		return PurchasedPlanActivationResult{}, ErrPurchasedPlanActivationConflict
	}
	return repository.ReadPurchasedPlanActivation(ctx, organizationID, sourceID)
}

func normalizePurchasedPlanActivationInput(input PurchasedPlanActivationInput) PurchasedPlanActivationInput {
	input.OperationID = strings.TrimSpace(input.OperationID)
	input.OrganizationID = strings.TrimSpace(input.OrganizationID)
	input.ActorID = strings.TrimSpace(input.ActorID)
	input.SourceType = strings.TrimSpace(input.SourceType)
	input.SourceID = strings.TrimSpace(input.SourceID)
	input.PlanCode = strings.TrimSpace(input.PlanCode)
	input.PlanFingerprint = strings.TrimSpace(input.PlanFingerprint)
	return input
}

func validatePurchasedPlanActivationInput(input PurchasedPlanActivationInput) error {
	input = normalizePurchasedPlanActivationInput(input)
	if !boundedPurchasedIdentifier(input.OperationID) ||
		!boundedPurchasedIdentifier(input.OrganizationID) ||
		!boundedPurchasedIdentifier(input.ActorID) ||
		input.SourceType != PurchasedPlanSourceCommercialOrder ||
		!boundedPurchasedIdentifier(input.SourceID) ||
		input.PlanCode == "" || len(input.PlanCode) > 64 ||
		input.PlanFingerprint == "" || len(input.PlanFingerprint) > 128 ||
		input.TermMonths < 1 || input.TermMonths > 120 {
		return ErrPurchasedPlanActivationConflict
	}
	return nil
}

func boundedPurchasedIdentifier(value string) bool {
	return value != "" && len(value) <= 128 && strings.TrimSpace(value) == value
}

type semanticPlanModule struct {
	ModuleCode string              `json:"module_code"`
	Limits     []semanticLimitPair `json:"limits"`
}

type semanticLimitPair struct {
	Key   string `json:"key"`
	Value int    `json:"value"`
}

func planSemanticFingerprint(plan Plan, modules []PlanModule) string {
	ordered := append([]PlanModule(nil), modules...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].ModuleCode < ordered[j].ModuleCode })
	semanticModules := make([]semanticPlanModule, 0, len(ordered))
	for _, module := range ordered {
		keys := make([]string, 0, len(module.Limits))
		for key := range module.Limits {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		limits := make([]semanticLimitPair, 0, len(keys))
		for _, key := range keys {
			limits = append(limits, semanticLimitPair{Key: key, Value: module.Limits[key]})
		}
		semanticModules = append(semanticModules, semanticPlanModule{ModuleCode: module.ModuleCode, Limits: limits})
	}
	return purchasedFingerprint(struct {
		Version string               `json:"version"`
		Code    string               `json:"plan_code"`
		Active  bool                 `json:"active"`
		Modules []semanticPlanModule `json:"modules"`
	}{Version: "subscription-plan-v1", Code: plan.Code, Active: plan.Active, Modules: semanticModules})
}

func purchasedActivationRequestFingerprint(input PurchasedPlanActivationInput) string {
	return purchasedFingerprint(struct {
		Version         string `json:"version"`
		OperationID     string `json:"operation_id"`
		OrganizationID  string `json:"organization_id"`
		ActorID         string `json:"actor_id"`
		SourceType      string `json:"source_type"`
		SourceID        string `json:"source_id"`
		PlanCode        string `json:"plan_code"`
		PlanFingerprint string `json:"plan_fingerprint"`
		TermMonths      int    `json:"term_months"`
	}{"subscription-activation-v1", input.OperationID, input.OrganizationID, input.ActorID, input.SourceType, input.SourceID, input.PlanCode, input.PlanFingerprint, input.TermMonths})
}

func entitlementSetFingerprint(organizationID, planCode string, startsAt, expiresAt time.Time, modules []PlanModule) string {
	ordered := append([]PlanModule(nil), modules...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].ModuleCode < ordered[j].ModuleCode })
	semanticModules := make([]semanticPlanModule, 0, len(ordered))
	for _, module := range ordered {
		keys := make([]string, 0, len(module.Limits))
		for key := range module.Limits {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		limits := make([]semanticLimitPair, 0, len(keys))
		for _, key := range keys {
			limits = append(limits, semanticLimitPair{Key: key, Value: module.Limits[key]})
		}
		semanticModules = append(semanticModules, semanticPlanModule{ModuleCode: module.ModuleCode, Limits: limits})
	}
	return purchasedFingerprint(struct {
		Version        string               `json:"version"`
		OrganizationID string               `json:"organization_id"`
		PlanCode       string               `json:"plan_code"`
		StartsAt       string               `json:"starts_at"`
		ExpiresAt      string               `json:"expires_at"`
		Status         string               `json:"status"`
		Entitlements   []semanticPlanModule `json:"entitlements"`
	}{"subscription-entitlement-set-v1", organizationID, planCode, startsAt.UTC().Format(time.RFC3339Nano), expiresAt.UTC().Format(time.RFC3339Nano), StatusActive, semanticModules})
}

func purchasedFingerprint(value any) string {
	encoded, _ := json.Marshal(value)
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:])
}
