package sheinsync

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	managementapi "task-processor/internal/infra/clients/management/api"
	"task-processor/internal/shein/activity"
	"task-processor/internal/shein/api/marketing"
)

type SheinActivityEnrollmentCandidate struct {
	CandidateID          int64
	SyncedProductID      int64
	ActivityKey          string
	CandidateVersion     string
	SKCName              string
	EffectiveCostPrice   *float64
	PriceSnapshot        string
	InventorySnapshot    string
	CalculatedProfitRate *float64
}

type SheinActivityEnrollmentResult struct {
	CandidateID     int64
	Success         bool
	RequestPayload  string
	ResponsePayload string
	ErrorMessage    string
}

type SheinActivityAdapter interface {
	EnrollCandidates(ctx context.Context, storeID int64, activityType, activityKey string, candidates []SheinActivityEnrollmentCandidate) ([]SheinActivityEnrollmentResult, error)
}

type SheinPromotionStrategyProvider interface {
	GetPromotionStrategy(ctx context.Context, storeID int64, activityKey string) (*managementapi.OperationStrategyDTO, error)
}

type sheinActivityAdapter struct {
	strategyProvider       SheinPromotionStrategyProvider
	promotionBridge        activity.PromotionRegistrationBridge
	promotionBridgeFactory SheinPromotionBridgeFactory
}

func NewSheinActivityAdapter(strategyProvider SheinPromotionStrategyProvider, promotionBridge activity.PromotionRegistrationBridge) SheinActivityAdapter {
	return &sheinActivityAdapter{
		strategyProvider: strategyProvider,
		promotionBridge:  promotionBridge,
	}
}

type SheinPromotionBridgeFactory interface {
	BuildPromotionBridge(ctx context.Context, storeID int64) (activity.PromotionRegistrationBridge, error)
}

func NewSheinActivityAdapterWithFactory(strategyProvider SheinPromotionStrategyProvider, promotionBridgeFactory SheinPromotionBridgeFactory) SheinActivityAdapter {
	return &sheinActivityAdapter{
		strategyProvider:       strategyProvider,
		promotionBridgeFactory: promotionBridgeFactory,
	}
}

func (a *sheinActivityAdapter) EnrollCandidates(
	ctx context.Context,
	storeID int64,
	activityType string,
	activityKey string,
	candidates []SheinActivityEnrollmentCandidate,
) ([]SheinActivityEnrollmentResult, error) {
	switch strings.ToUpper(activityType) {
	case "PROMOTION":
		return a.enrollPromotionCandidates(ctx, storeID, activityKey, candidates)
	default:
		return nil, fmt.Errorf("unsupported SHEIN activity type %q", activityType)
	}
}

func (a *sheinActivityAdapter) enrollPromotionCandidates(
	ctx context.Context,
	storeID int64,
	activityKey string,
	candidates []SheinActivityEnrollmentCandidate,
) ([]SheinActivityEnrollmentResult, error) {
	if a == nil || a.strategyProvider == nil {
		return nil, fmt.Errorf("SHEIN promotion strategy provider is required")
	}
	if activityKey == "" {
		return nil, fmt.Errorf("SHEIN promotion activity key is required")
	}
	bridge, err := a.resolvePromotionBridge(ctx, storeID)
	if err != nil {
		return nil, err
	}

	strategy, err := a.strategyProvider.GetPromotionStrategy(ctx, storeID, activityKey)
	if err != nil {
		return nil, err
	}
	if strategy == nil {
		return nil, fmt.Errorf("SHEIN promotion strategy is required")
	}

	products := make([]marketing.SkcInfo, 0, len(candidates))
	productBySKC := make(map[string]marketing.SkcInfo, len(candidates))
	for _, candidate := range candidates {
		product, ok := buildPromotionCandidateProduct(candidate)
		if !ok {
			continue
		}
		products = append(products, product)
		productBySKC[product.Skc] = product
	}
	sort.Slice(products, func(i, j int) bool {
		return products[i].Skc < products[j].Skc
	})

	bridgeResult, bridgeErr := bridge.RegisterPromotionProducts(ctx, strategy, activityKey, products)
	return buildPromotionEnrollmentResults(candidates, bridgeResult, bridgeErr, productBySKC), bridgeErr
}

func (a *sheinActivityAdapter) resolvePromotionBridge(ctx context.Context, storeID int64) (activity.PromotionRegistrationBridge, error) {
	if a == nil {
		return nil, fmt.Errorf("SHEIN activity adapter is required")
	}
	if a.promotionBridge != nil {
		return a.promotionBridge, nil
	}
	if a.promotionBridgeFactory == nil {
		return nil, fmt.Errorf("SHEIN promotion enrollment bridge is required")
	}
	bridge, err := a.promotionBridgeFactory.BuildPromotionBridge(ctx, storeID)
	if err != nil {
		return nil, err
	}
	if bridge == nil {
		return nil, fmt.Errorf("SHEIN promotion enrollment bridge is required")
	}
	return bridge, nil
}

func buildPromotionCandidateProduct(candidate SheinActivityEnrollmentCandidate) (marketing.SkcInfo, bool) {
	if candidate.SKCName == "" {
		return marketing.SkcInfo{}, false
	}
	price, currency := parsePromotionPriceSnapshot(candidate.PriceSnapshot)
	if price <= 0 {
		return marketing.SkcInfo{}, false
	}
	stock := parsePromotionInventorySnapshot(candidate.InventorySnapshot)
	if stock <= 0 {
		return marketing.SkcInfo{}, false
	}

	product := marketing.SkcInfo{
		Skc:               candidate.SKCName,
		Stock:             stock,
		SitePriceInfoList: []marketing.SitePriceInfo{{SalePrice: price, Currency: currency, IsAvailable: true}},
	}
	return product, true
}

func buildPromotionEnrollmentResults(
	candidates []SheinActivityEnrollmentCandidate,
	bridgeResult *activity.PromotionRegistrationResult,
	bridgeErr error,
	productBySKC map[string]marketing.SkcInfo,
) []SheinActivityEnrollmentResult {
	configured := make(map[string]struct{})
	requestPayload := ""
	responsePayload := ""
	if bridgeResult != nil {
		requestPayload = marshalPromotionPayload(bridgeResult.Request)
		responsePayload = marshalPromotionPayload(bridgeResult.Response)
		if bridgeResult.Request != nil {
			for _, config := range bridgeResult.Request.ConfigList {
				configured[config.Skc] = struct{}{}
			}
		}
	}

	results := make([]SheinActivityEnrollmentResult, 0, len(candidates))
	for _, candidate := range candidates {
		result := SheinActivityEnrollmentResult{
			CandidateID:     candidate.CandidateID,
			RequestPayload:  requestPayload,
			ResponsePayload: responsePayload,
		}
		if _, ok := productBySKC[candidate.SKCName]; !ok {
			result.ErrorMessage = "candidate filtered from promotion bridge input"
			results = append(results, result)
			continue
		}
		if _, ok := configured[candidate.SKCName]; !ok {
			if bridgeErr != nil {
				result.ErrorMessage = bridgeErr.Error()
			} else {
				result.ErrorMessage = "candidate filtered from promotion registration request"
			}
			results = append(results, result)
			continue
		}
		if bridgeErr != nil {
			result.ErrorMessage = bridgeErr.Error()
			results = append(results, result)
			continue
		}
		result.Success = true
		results = append(results, result)
	}
	return results
}

func parsePromotionPriceSnapshot(raw string) (float64, string) {
	if raw == "" {
		return 0, ""
	}
	var payload struct {
		SalePrice float64 `json:"sale_price"`
		Currency  string  `json:"currency"`
	}
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return 0, ""
	}
	return payload.SalePrice, payload.Currency
}

func parsePromotionInventorySnapshot(raw string) int {
	if raw == "" {
		return 0
	}
	var payload struct {
		Available int `json:"available"`
	}
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return 0
	}
	return payload.Available
}

func marshalPromotionPayload(v any) string {
	if v == nil {
		return ""
	}
	encoded, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	return string(encoded)
}
