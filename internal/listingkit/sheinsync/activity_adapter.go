package sheinsync

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"task-processor/internal/shein/api/marketing"
)

type SheinActivityEnrollmentCandidate struct {
	CandidateID          int64
	SyncedProductID      int64
	ActivityKey          string
	CandidateVersion     string
	SKCName              string
	EffectiveCostPrice   *float64
	SKUCostPriceInfoList []SheinSKUCostPrice
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
	GetPromotionStrategy(ctx context.Context, storeID int64, activityKey string) (*SheinPromotionStrategy, error)
}

type SheinPromotionBridge interface {
	RegisterPromotionProducts(ctx context.Context, strategy *SheinPromotionStrategy, activityKey string, products []marketing.SkcInfo) (*SheinPromotionRegistrationResult, error)
}

type SheinPromotionRegistrationSession interface {
	RegisterPromotionProducts(ctx context.Context, activityKey string, products []marketing.SkcInfo) (*SheinPromotionRegistrationResult, error)
}

type SheinPromotionBridgeSessionStarter interface {
	StartPromotionRegistrationSession(ctx context.Context, strategy *SheinPromotionStrategy, activityKey string) (SheinPromotionRegistrationSession, error)
}

type sheinActivityAdapter struct {
	strategyProvider       SheinPromotionStrategyProvider
	promotionBridge        SheinPromotionBridge
	promotionBridgeFactory SheinPromotionBridgeFactory
}

func newSheinActivityAdapter(strategyProvider SheinPromotionStrategyProvider, promotionBridge SheinPromotionBridge) SheinActivityAdapter {
	return &sheinActivityAdapter{
		strategyProvider: strategyProvider,
		promotionBridge:  promotionBridge,
	}
}

type SheinPromotionBridgeFactory interface {
	BuildPromotionBridge(ctx context.Context, storeID int64) (SheinPromotionBridge, error)
}

func newSheinActivityAdapterWithFactory(strategyProvider SheinPromotionStrategyProvider, promotionBridgeFactory SheinPromotionBridgeFactory) SheinActivityAdapter {
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
	normalizedActivityType := strings.ToUpper(strings.TrimSpace(activityType))
	switch normalizedActivityType {
	case "PROMOTION":
		return a.enrollPromotionCandidates(ctx, storeID, activityKey, "", candidates)
	case "TIME_LIMITED":
		return a.enrollTimeLimitedPromotionCandidates(ctx, storeID, activityKey, candidates)
	default:
		return nil, fmt.Errorf("unsupported SHEIN activity type %q", activityType)
	}
}

func buildTimeLimitedBridgeActivityKey(activityKey string, candidates []SheinActivityEnrollmentCandidate) string {
	parts := make([]string, 0, len(candidates)+1)
	parts = append(parts, strings.TrimSpace(activityKey))
	for _, candidate := range candidates {
		parts = append(parts, fmt.Sprintf("%d", candidate.CandidateID))
	}
	return strings.Join(parts, ":")
}

func (a *sheinActivityAdapter) enrollPromotionCandidates(
	ctx context.Context,
	storeID int64,
	activityKey string,
	bridgeActivityKey string,
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
	if err := strategy.ValidateForPromotionEnrollment(); err != nil {
		return nil, err
	}

	return registerPromotionCandidatesWithBridge(ctx, bridge, strategy, bridgeActivityKey, candidates)
}

func (a *sheinActivityAdapter) enrollTimeLimitedPromotionCandidates(
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
	if err := strategy.ValidateForPromotionEnrollment(); err != nil {
		return nil, err
	}

	var session SheinPromotionRegistrationSession
	if starter, ok := bridge.(SheinPromotionBridgeSessionStarter); ok {
		session, err = starter.StartPromotionRegistrationSession(ctx, strategy, activityKey)
		if err != nil {
			return nil, err
		}
	}

	register := func(chunkActivityKey string, chunk []SheinActivityEnrollmentCandidate) ([]SheinActivityEnrollmentResult, error) {
		if session != nil {
			return registerPromotionCandidatesWithSession(ctx, session, chunkActivityKey, chunk)
		}
		return registerPromotionCandidatesWithBridge(ctx, bridge, strategy, chunkActivityKey, chunk)
	}
	return executeTimeLimitedCandidateBatch(activityKey, candidates, register)
}

type sheinTimeLimitedCandidateRegister func(string, []SheinActivityEnrollmentCandidate) ([]SheinActivityEnrollmentResult, error)

func executeTimeLimitedCandidateBatch(
	activityKey string,
	candidates []SheinActivityEnrollmentCandidate,
	register sheinTimeLimitedCandidateRegister,
) ([]SheinActivityEnrollmentResult, error) {
	if len(candidates) == 0 {
		return nil, nil
	}
	results, err := register(buildTimeLimitedBridgeActivityKey(activityKey, candidates), candidates)
	if err == nil {
		return results, nil
	}
	if len(candidates) <= 1 {
		return ensureSheinEnrollmentSingleCandidateResult(candidates, results, err), nil
	}

	mid := len(candidates) / 2
	leftResults, leftErr := executeTimeLimitedCandidateBatch(activityKey, candidates[:mid], register)
	rightResults, rightErr := executeTimeLimitedCandidateBatch(activityKey, candidates[mid:], register)
	combined := append(leftResults, rightResults...)
	if len(combined) > 0 {
		return combined, nil
	}
	return combined, joinSheinEnrollmentErrors(leftErr, rightErr)
}

func registerPromotionCandidatesWithBridge(
	ctx context.Context,
	bridge SheinPromotionBridge,
	strategy *SheinPromotionStrategy,
	bridgeActivityKey string,
	candidates []SheinActivityEnrollmentCandidate,
) ([]SheinActivityEnrollmentResult, error) {
	products, productBySKC, inputFilterReasons := buildPromotionCandidateProducts(candidates)
	if len(products) == 0 {
		return buildPromotionEnrollmentResults(candidates, nil, nil, productBySKC, inputFilterReasons), nil
	}
	bridgeResult, bridgeErr := bridge.RegisterPromotionProducts(ctx, strategy, bridgeActivityKey, products)
	return buildPromotionEnrollmentResults(candidates, bridgeResult, bridgeErr, productBySKC, inputFilterReasons), bridgeErr
}

func registerPromotionCandidatesWithSession(
	ctx context.Context,
	session SheinPromotionRegistrationSession,
	bridgeActivityKey string,
	candidates []SheinActivityEnrollmentCandidate,
) ([]SheinActivityEnrollmentResult, error) {
	products, productBySKC, inputFilterReasons := buildPromotionCandidateProducts(candidates)
	if len(products) == 0 {
		return buildPromotionEnrollmentResults(candidates, nil, nil, productBySKC, inputFilterReasons), nil
	}
	bridgeResult, bridgeErr := session.RegisterPromotionProducts(ctx, bridgeActivityKey, products)
	return buildPromotionEnrollmentResults(candidates, bridgeResult, bridgeErr, productBySKC, inputFilterReasons), bridgeErr
}

func buildPromotionCandidateProducts(candidates []SheinActivityEnrollmentCandidate) ([]marketing.SkcInfo, map[string]marketing.SkcInfo, map[string]string) {
	products := make([]marketing.SkcInfo, 0, len(candidates))
	productBySKC := make(map[string]marketing.SkcInfo, len(candidates))
	inputFilterReasons := make(map[string]string)
	for _, candidate := range candidates {
		product, reason, ok := buildPromotionCandidateProduct(candidate)
		if !ok {
			if reason != "" && candidate.SKCName != "" {
				inputFilterReasons[candidate.SKCName] = reason
			}
			continue
		}
		products = append(products, product)
		productBySKC[product.Skc] = product
	}
	sort.Slice(products, func(i, j int) bool {
		return products[i].Skc < products[j].Skc
	})
	return products, productBySKC, inputFilterReasons
}

func (a *sheinActivityAdapter) resolvePromotionBridge(ctx context.Context, storeID int64) (SheinPromotionBridge, error) {
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

func buildPromotionCandidateProduct(candidate SheinActivityEnrollmentCandidate) (marketing.SkcInfo, string, bool) {
	if candidate.SKCName == "" {
		return marketing.SkcInfo{}, "", false
	}
	priceSnapshot := parsePromotionCandidatePriceSnapshot(candidate.PriceSnapshot)
	if priceSnapshot.SalePrice <= 0 {
		return marketing.SkcInfo{}, "", false
	}
	stock := parsePromotionInventorySnapshot(candidate.InventorySnapshot)
	if stock <= 0 {
		return marketing.SkcInfo{}, "", false
	}

	product := marketing.SkcInfo{
		Skc:                 candidate.SKCName,
		Stock:               stock,
		SupplyPrice:         priceSnapshot.SalePrice,
		SupplyPriceCurrency: priceSnapshot.Currency,
		SitePriceInfoList:   []marketing.SitePriceInfo{{SalePrice: priceSnapshot.SalePrice, Currency: priceSnapshot.Currency, SiteCode: priceSnapshot.SubSite, IsAvailable: true}},
		SkuPriceInfoList:    promotionSnapshotSKUPricesToMarketing(priceSnapshot.SKUPrices),
		SkuCostPriceInfoList: promotionCandidateSKUCostsToMarketing(
			candidate.SKUCostPriceInfoList,
			priceSnapshot.Currency,
		),
	}
	return product, "", true
}

func promotionCandidateSKUCostsToMarketing(costs []SheinSKUCostPrice, fallbackCurrency string) []marketing.SkuCostPriceInfo {
	if len(costs) == 0 {
		return nil
	}
	out := make([]marketing.SkuCostPriceInfo, 0, len(costs))
	for _, cost := range costs {
		if strings.TrimSpace(cost.SKUCode) == "" || cost.CostPrice <= 0 {
			continue
		}
		currency := strings.TrimSpace(cost.Currency)
		if currency == "" {
			currency = strings.TrimSpace(fallbackCurrency)
		}
		out = append(out, marketing.SkuCostPriceInfo{
			SkuCode:   strings.TrimSpace(cost.SKUCode),
			CostPrice: cost.CostPrice,
			Currency:  currency,
		})
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func sheinActivityCandidateCostValue(value *float64) float64 {
	if value == nil {
		return 0
	}
	return *value
}

type promotionCandidatePriceSnapshot struct {
	SalePrice float64                              `json:"sale_price"`
	Currency  string                               `json:"currency"`
	SubSite   string                               `json:"sub_site"`
	SKUPrices []promotionCandidateSKUPriceSnapshot `json:"sku_prices"`
}

type promotionCandidateSKUPriceSnapshot struct {
	SKUCode   string  `json:"sku_code"`
	SalePrice float64 `json:"sale_price"`
	Currency  string  `json:"currency"`
	SubSite   string  `json:"sub_site"`
}

func parsePromotionCandidatePriceSnapshot(raw string) promotionCandidatePriceSnapshot {
	if raw == "" {
		return promotionCandidatePriceSnapshot{}
	}
	var payload promotionCandidatePriceSnapshot
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return promotionCandidatePriceSnapshot{}
	}
	return payload
}

func promotionSnapshotSKUPricesToMarketing(items []promotionCandidateSKUPriceSnapshot) []marketing.SkuSitePriceInfo {
	out := make([]marketing.SkuSitePriceInfo, 0, len(items))
	for _, item := range items {
		skuCode := strings.TrimSpace(item.SKUCode)
		if skuCode == "" || item.SalePrice <= 0 {
			continue
		}
		out = append(out, marketing.SkuSitePriceInfo{
			SkuCode: skuCode,
			SitePriceInfoList: []marketing.SitePriceInfo{{
				SiteCode:    item.SubSite,
				SalePrice:   item.SalePrice,
				Currency:    item.Currency,
				IsAvailable: true,
			}},
		})
	}
	return out
}

func buildPromotionEnrollmentResults(
	candidates []SheinActivityEnrollmentCandidate,
	bridgeResult *SheinPromotionRegistrationResult,
	bridgeErr error,
	productBySKC map[string]marketing.SkcInfo,
	inputFilterReasons map[string]string,
) []SheinActivityEnrollmentResult {
	configured := make(map[string]struct{})
	filterReasons := make(map[string]string)
	for skc, reason := range inputFilterReasons {
		filterReasons[skc] = reason
	}
	requestPayload := ""
	responsePayload := ""
	if bridgeResult != nil {
		requestPayload = marshalPromotionRequestPayload(bridgeResult)
		responsePayload = marshalPromotionResponsePayload(bridgeResult)
		for skc, reason := range bridgeResult.FilterReasons {
			filterReasons[skc] = reason
		}
		for skc, reason := range promotionResponseFailureReasons(bridgeResult) {
			filterReasons[skc] = reason
		}
		requests := bridgeResult.Requests
		if len(requests) == 0 && bridgeResult.Request != nil {
			requests = []*marketing.SaveConfigRequest{bridgeResult.Request}
		}
		for _, request := range requests {
			if request == nil {
				continue
			}
			for _, config := range request.ConfigList {
				configured[config.Skc] = struct{}{}
			}
		}
		if bridgeResult.ActivityRequest != nil {
			for _, item := range bridgeResult.ActivityRequest.AddCostAndStockInfoList {
				configured[item.Skc] = struct{}{}
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
			if reason := strings.TrimSpace(filterReasons[candidate.SKCName]); reason != "" {
				result.ErrorMessage = reason
			} else {
				result.ErrorMessage = "candidate filtered from promotion bridge input"
			}
			results = append(results, result)
			continue
		}
		if reason := strings.TrimSpace(filterReasons[candidate.SKCName]); reason != "" {
			result.ErrorMessage = reason
			results = append(results, result)
			continue
		}
		if _, ok := configured[candidate.SKCName]; !ok {
			if reason := strings.TrimSpace(filterReasons[candidate.SKCName]); reason != "" {
				result.ErrorMessage = reason
			} else if bridgeErr != nil {
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

func promotionResponseFailureReasons(result *SheinPromotionRegistrationResult) map[string]string {
	reasons := make(map[string]string)
	if result == nil || result.ActivityResponse == nil || result.ActivityResponse.Info == nil {
		return reasons
	}
	requestedSKCs := promotionRequestSKCs(result.ActivityRequest)

	for skc, reason := range normalizePromotionResponseErrors(result.ActivityResponse.Info.SkcErrorInfo) {
		reasons[skc] = reason
	}

	skuErrors := normalizePromotionResponseErrors(result.ActivityResponse.Info.SkuErrorInfo)
	if len(skuErrors) > 0 && result.ActivityRequest != nil {
		for _, product := range result.ActivityRequest.AddCostAndStockInfoList {
			for _, sku := range product.AddSkuList {
				if reason := strings.TrimSpace(skuErrors[sku.Sku]); reason != "" {
					reasons[product.Skc] = reason
					break
				}
			}
		}
	}
	if len(skuErrors) > 0 {
		for _, skc := range requestedSKCs {
			if _, exists := reasons[skc]; !exists {
				reasons[skc] = "SHEIN reported SKU enrollment errors"
			}
		}
	}
	if len(reasons) > 0 {
		return reasons
	}
	if result.ActivityResponse.Info.ErrorInfo != nil {
		return promotionResponseFailureForSKCs(requestedSKCs, promotionResponseErrorMessage(result.ActivityResponse.Info.ErrorInfo))
	}
	if result.ActivityResponse.Info.ActivityID <= 0 {
		return promotionResponseFailureForSKCs(requestedSKCs, "SHEIN did not create an activity")
	}
	return reasons
}

func promotionRequestSKCs(request *marketing.CreateActivityRequest) []string {
	if request == nil {
		return nil
	}
	values := make([]string, 0, len(request.AddCostAndStockInfoList))
	for _, product := range request.AddCostAndStockInfoList {
		if skc := strings.TrimSpace(product.Skc); skc != "" {
			values = append(values, skc)
		}
	}
	return values
}

func promotionResponseFailureForSKCs(skcs []string, reason string) map[string]string {
	reasons := make(map[string]string, len(skcs))
	for _, skc := range skcs {
		reasons[skc] = reason
	}
	return reasons
}

func normalizePromotionResponseErrors(value any) map[string]string {
	if value == nil {
		return nil
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return nil
	}
	var raw map[string]any
	if err := json.Unmarshal(encoded, &raw); err != nil {
		return nil
	}
	errorsByKey := make(map[string]string, len(raw))
	for key, detail := range raw {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		message := promotionResponseErrorMessage(detail)
		if message != "" {
			errorsByKey[key] = message
		}
	}
	return errorsByKey
}

func promotionResponseErrorMessage(detail any) string {
	if message, ok := detail.(string); ok {
		return strings.TrimSpace(message)
	}
	encoded, err := json.Marshal(detail)
	if err != nil {
		return "SHEIN reported an enrollment error"
	}
	return strings.TrimSpace(string(encoded))
}

func parsePromotionPriceSnapshot(raw string) (float64, string) {
	payload := parsePromotionCandidatePriceSnapshot(raw)
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

func marshalPromotionRequestPayload(result *SheinPromotionRegistrationResult) string {
	if result == nil {
		return ""
	}
	if result.ActivityRequest != nil {
		return marshalPromotionPayload(result.ActivityRequest)
	}
	if len(result.Requests) > 1 {
		return marshalPromotionPayload(struct {
			Requests []*marketing.SaveConfigRequest `json:"requests"`
		}{Requests: result.Requests})
	}
	return marshalPromotionPayload(result.Request)
}

func marshalPromotionResponsePayload(result *SheinPromotionRegistrationResult) string {
	if result == nil {
		return ""
	}
	if result.ActivityResponse != nil {
		return marshalPromotionPayload(result.ActivityResponse)
	}
	if len(result.Responses) > 1 {
		return marshalPromotionPayload(struct {
			Responses []*marketing.SaveConfigResponse `json:"responses"`
		}{Responses: result.Responses})
	}
	return marshalPromotionPayload(result.Response)
}
