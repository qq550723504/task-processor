package sheinsync

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"gorm.io/gorm"
)

const sheinCandidateRefreshPageSize = 100
const sheinCandidateSDSCostGroupFetchBatchSize = 100

type SheinCandidateService interface {
	RefreshCandidates(ctx context.Context, tenantID, storeID int64, activityType string) (*SheinCandidateRefreshResult, error)
	ListCandidates(ctx context.Context, query *SheinActivityCandidateQuery) ([]SheinActivityCandidateRecord, int64, error)
	ResetCandidates(ctx context.Context, tenantID, storeID int64, req SheinCandidateResetRequest) (*SheinCandidateResetResult, error)
	ReviewCandidate(
		ctx context.Context,
		tenantID, storeID, candidateID int64,
		reviewStatus SheinCandidateReviewStatus,
		autoModeEligible *bool,
		selectedForRun *bool,
	) (*SheinActivityCandidateRecord, error)
}

type SheinCandidateRefreshResult struct {
	TotalCount      int
	EligibleCount   int
	IneligibleCount int
}

type SheinCandidateResetRequest struct {
	ActivityType      string
	ActivityKey       string
	SKCName           string
	EligibilityReason string
	CandidateIDs      []int64
}

type SheinCandidateResetResult struct {
	MatchedCount int `json:"matched_count"`
	ResetCount   int `json:"reset_count"`
	SkippedCount int `json:"skipped_count"`
}

type SheinCandidateRepository interface {
	ListSyncedProducts(ctx context.Context, query *SheinSyncedProductQuery) ([]SheinSyncedProductRecord, int64, error)
	ListCandidates(ctx context.Context, query *SheinActivityCandidateQuery) ([]SheinActivityCandidateRecord, int64, error)
	SaveCandidates(ctx context.Context, records []*SheinActivityCandidateRecord) error
}

type sheinCandidateService struct {
	repo     SheinCandidateRepository
	pageSize int
}

func NewSheinCandidateService(repo SheinCandidateRepository) SheinCandidateService {
	return &sheinCandidateService{
		repo:     repo,
		pageSize: sheinCandidateRefreshPageSize,
	}
}

func (s *sheinCandidateService) RefreshCandidates(ctx context.Context, tenantID, storeID int64, activityType string) (*SheinCandidateRefreshResult, error) {
	if err := s.validate(); err != nil {
		return nil, err
	}
	if activityType == "" {
		return nil, fmt.Errorf("SHEIN candidate activity type is required")
	}

	products, err := s.listActiveProducts(ctx, tenantID, storeID)
	if err != nil {
		return nil, err
	}
	products = applySheinCandidateSharedSDSCosts(products)
	products, err = s.applySDSCostGroupOverrides(ctx, tenantID, storeID, products)
	if err != nil {
		return nil, err
	}

	activityKey := buildSheinActivityKey(activityType, tenantID, storeID)
	existingCandidates, err := s.listExistingCandidates(ctx, tenantID, storeID, activityType, activityKey)
	if err != nil {
		return nil, err
	}
	existingBySKCVersion := make(map[string]SheinActivityCandidateRecord, len(existingCandidates))
	existingBySKC := make(map[string][]SheinActivityCandidateRecord)
	for _, candidate := range existingCandidates {
		existingBySKCVersion[sheinCandidateStateKey(candidate.SKCName, candidate.CandidateVersion)] = candidate
		existingBySKC[candidate.SKCName] = append(existingBySKC[candidate.SKCName], candidate)
	}

	records := make([]*SheinActivityCandidateRecord, 0, len(products))
	activeSKCs := make(map[string]struct{}, len(products))
	result := &SheinCandidateRefreshResult{}
	for _, product := range products {
		record := buildSheinCandidateRecord(product, activityType, activityKey)
		activeSKCs[record.SKCName] = struct{}{}
		if existing, ok := existingBySKCVersion[sheinCandidateStateKey(record.SKCName, record.CandidateVersion)]; ok {
			record.ReviewStatus = existing.ReviewStatus
			record.AutoModeEligible = existing.AutoModeEligible
			record.SelectedForRun = existing.SelectedForRun
		}
		records = append(records, record)
		for _, existing := range existingBySKC[record.SKCName] {
			if existing.CandidateVersion == record.CandidateVersion {
				continue
			}
			stale := existing
			stale.EligibilityStatus = SheinCandidateEligibilityStatusIneligible
			stale.EligibilityReason = "superseded by newer candidate version"
			stale.ReviewStatus = SheinCandidateReviewStatusRejected
			stale.AutoModeEligible = false
			stale.SelectedForRun = false
			records = append(records, &stale)
		}
		result.TotalCount++
		if record.EligibilityStatus == SheinCandidateEligibilityStatusEligible {
			result.EligibleCount++
			continue
		}
		result.IneligibleCount++
	}
	for skcName, candidates := range existingBySKC {
		if _, ok := activeSKCs[skcName]; ok {
			continue
		}
		for _, existing := range candidates {
			stale := existing
			stale.EligibilityStatus = SheinCandidateEligibilityStatusIneligible
			stale.EligibilityReason = "superseded by newer candidate version"
			stale.ReviewStatus = SheinCandidateReviewStatusRejected
			stale.AutoModeEligible = false
			stale.SelectedForRun = false
			records = append(records, &stale)
		}
	}

	if err := s.repo.SaveCandidates(ctx, records); err != nil {
		return nil, err
	}
	return result, nil
}

func (s *sheinCandidateService) ListCandidates(ctx context.Context, query *SheinActivityCandidateQuery) ([]SheinActivityCandidateRecord, int64, error) {
	if err := s.validate(); err != nil {
		return nil, 0, err
	}
	return s.repo.ListCandidates(ctx, query)
}

func (s *sheinCandidateService) ResetCandidates(ctx context.Context, tenantID, storeID int64, req SheinCandidateResetRequest) (*SheinCandidateResetResult, error) {
	if err := s.validate(); err != nil {
		return nil, err
	}
	activityType := strings.TrimSpace(req.ActivityType)
	if activityType == "" {
		return nil, fmt.Errorf("SHEIN candidate activity type is required")
	}

	query := &SheinActivityCandidateQuery{
		TenantID:     tenantID,
		StoreID:      storeID,
		ActivityType: activityType,
		ActivityKey:  strings.TrimSpace(req.ActivityKey),
		SKCName:      strings.TrimSpace(req.SKCName),
		CandidateIDs: append([]int64(nil), req.CandidateIDs...),
	}
	reason := strings.TrimSpace(req.EligibilityReason)
	result := &SheinCandidateResetResult{}
	pageSize := s.pageSize
	if pageSize <= 0 {
		pageSize = sheinCandidateRefreshPageSize
	}
	if len(query.CandidateIDs) > 0 {
		pageSize = len(query.CandidateIDs)
	}

	for page := 1; ; page++ {
		query.Page = page
		query.PageSize = pageSize
		rows, total, err := s.repo.ListCandidates(ctx, query)
		if err != nil {
			return nil, err
		}
		if len(rows) == 0 {
			break
		}

		toSave := make([]*SheinActivityCandidateRecord, 0, len(rows))
		for i := range rows {
			row := rows[i]
			if reason != "" && strings.TrimSpace(row.EligibilityReason) != reason {
				continue
			}
			result.MatchedCount++
			row.ReviewStatus = SheinCandidateReviewStatusPendingReview
			row.AutoModeEligible = false
			row.SelectedForRun = false
			toSave = append(toSave, &row)
		}
		if len(toSave) > 0 {
			if err := s.repo.SaveCandidates(ctx, toSave); err != nil {
				return nil, err
			}
			result.ResetCount += len(toSave)
		}
		if int64(page*pageSize) >= total {
			break
		}
	}
	return result, nil
}

func (s *sheinCandidateService) ReviewCandidate(
	ctx context.Context,
	tenantID, storeID, candidateID int64,
	reviewStatus SheinCandidateReviewStatus,
	autoModeEligible *bool,
	selectedForRun *bool,
) (*SheinActivityCandidateRecord, error) {
	if err := s.validate(); err != nil {
		return nil, err
	}
	if candidateID <= 0 {
		return nil, fmt.Errorf("SHEIN candidate id is required")
	}
	if reviewStatus == "" {
		return nil, fmt.Errorf("SHEIN candidate review status is required")
	}

	rows, _, err := s.repo.ListCandidates(ctx, &SheinActivityCandidateQuery{
		TenantID:     tenantID,
		StoreID:      storeID,
		CandidateIDs: []int64{candidateID},
		Page:         1,
		PageSize:     1,
	})
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, gorm.ErrRecordNotFound
	}

	row := rows[0]
	row.ReviewStatus = reviewStatus
	if autoModeEligible != nil {
		row.AutoModeEligible = *autoModeEligible
	}
	if selectedForRun != nil {
		row.SelectedForRun = *selectedForRun
	}
	if err := s.repo.SaveCandidates(ctx, []*SheinActivityCandidateRecord{&row}); err != nil {
		return nil, err
	}
	return &row, nil
}

func (s *sheinCandidateService) listActiveProducts(ctx context.Context, tenantID, storeID int64) ([]SheinSyncedProductRecord, error) {
	active := true
	items := make([]SheinSyncedProductRecord, 0)
	page := 1
	for {
		rows, total, err := s.repo.ListSyncedProducts(ctx, &SheinSyncedProductQuery{
			TenantID: tenantID,
			StoreID:  storeID,
			IsActive: &active,
			Page:     page,
			PageSize: s.pageSize,
		})
		if err != nil {
			return nil, err
		}
		items = append(items, rows...)
		if len(rows) == 0 || int64(page*s.pageSize) >= total {
			break
		}
		page++
	}
	return items, nil
}

func (s *sheinCandidateService) listExistingCandidates(ctx context.Context, tenantID, storeID int64, activityType, activityKey string) ([]SheinActivityCandidateRecord, error) {
	items := make([]SheinActivityCandidateRecord, 0)
	page := 1
	for {
		rows, total, err := s.repo.ListCandidates(ctx, &SheinActivityCandidateQuery{
			TenantID:     tenantID,
			StoreID:      storeID,
			ActivityType: activityType,
			ActivityKey:  activityKey,
			Page:         page,
			PageSize:     s.pageSize,
		})
		if err != nil {
			return nil, err
		}
		items = append(items, rows...)
		if len(rows) == 0 || int64(page*s.pageSize) >= total {
			break
		}
		page++
	}
	return items, nil
}

func (s *sheinCandidateService) validate() error {
	switch {
	case s == nil:
		return fmt.Errorf("SHEIN candidate service is required")
	case s.repo == nil:
		return fmt.Errorf("SHEIN candidate repository is required")
	default:
		return nil
	}
}

func buildSheinCandidateRecord(product SheinSyncedProductRecord, activityType, activityKey string) *SheinActivityCandidateRecord {
	record := &SheinActivityCandidateRecord{
		TenantID:           product.TenantID,
		StoreID:            product.StoreID,
		SyncedProductID:    product.ID,
		ActivityType:       activityType,
		ActivityKey:        activityKey,
		SKCName:            product.SKCName,
		CandidateVersion:   buildSheinCandidateVersion(product),
		EffectiveCostPrice: cloneSheinSyncFloat64(product.EffectiveCostPrice),
		PriceSnapshot:      product.PriceSnapshot,
		InventorySnapshot:  product.InventorySnapshot,
		CalculatedProfitRate: calculateSheinCandidateProfitRate(
			product.EffectiveCostPrice,
			product.PriceSnapshot,
		),
		ReviewStatus:         SheinCandidateReviewStatusPendingReview,
		AutoModeEligible:     false,
		SKUCostPriceInfoList: cloneSheinSKUCostPriceList(product.SKUCostPriceInfoList),
	}

	switch {
	case product.ShelfStatus != "ON_SHELF":
		record.EligibilityStatus = SheinCandidateEligibilityStatusIneligible
		record.EligibilityReason = "product is not on shelf"
	case product.EffectiveCostPrice == nil:
		record.EligibilityStatus = SheinCandidateEligibilityStatusIneligible
		record.EligibilityReason = "missing effective cost price"
	default:
		record.EligibilityStatus = SheinCandidateEligibilityStatusEligible
	}

	return record
}

func applySheinCandidateSharedSDSCosts(products []SheinSyncedProductRecord) []SheinSyncedProductRecord {
	if len(products) == 0 {
		return products
	}

	groupCosts := make(map[string]float64)
	for _, product := range products {
		key := sheinCandidateSDSCostGroupKey(product)
		if key == "" || product.EffectiveCostPrice == nil {
			continue
		}
		cost := *product.EffectiveCostPrice
		if existing, ok := groupCosts[key]; !ok || cost > existing {
			groupCosts[key] = cost
		}
	}
	if len(groupCosts) == 0 {
		return products
	}

	out := make([]SheinSyncedProductRecord, len(products))
	copy(out, products)
	for i := range out {
		key := sheinCandidateSDSCostGroupKey(out[i])
		if key == "" {
			continue
		}
		if cost, ok := groupCosts[key]; ok {
			out[i].EffectiveCostPrice = sheinFloat64Ptr(cost)
		}
	}
	return out
}

type sheinCandidateSDSCostGroupReader interface {
	ListSDSCostGroups(ctx context.Context, query *SheinSDSCostGroupQuery) ([]SheinSDSCostGroupRecord, int64, error)
}

func (s *sheinCandidateService) applySDSCostGroupOverrides(ctx context.Context, tenantID, storeID int64, products []SheinSyncedProductRecord) ([]SheinSyncedProductRecord, error) {
	reader, ok := s.repo.(sheinCandidateSDSCostGroupReader)
	if !ok {
		return products, nil
	}
	return applySheinSDSCostGroupOverrides(ctx, reader, tenantID, storeID, products)
}

func applySheinSDSCostGroupOverrides(
	ctx context.Context,
	reader sheinCandidateSDSCostGroupReader,
	tenantID, storeID int64,
	products []SheinSyncedProductRecord,
) ([]SheinSyncedProductRecord, error) {
	if reader == nil || len(products) == 0 {
		return products, nil
	}
	groupKeys := sheinCandidateSDSCostGroupKeys(products)
	if len(groupKeys) == 0 {
		return products, nil
	}
	groups, err := listSheinCandidateSDSCostGroups(ctx, reader, tenantID, storeID, groupKeys)
	if err != nil {
		return nil, err
	}
	fetchedCosts := make(map[string]float64, len(groups))
	for _, group := range groups {
		if group.ManualCostPrice == nil {
			continue
		}
		fetchedCosts[group.GroupKey] = *group.ManualCostPrice
	}
	if len(fetchedCosts) == 0 {
		return products, nil
	}
	groupOverrides := make(map[string]float64)
	productOverrides := make(map[int]float64)
	productSKUCostOverrides := make(map[int][]SheinSKUCostPrice)
	for index, product := range products {
		if cost, skuCosts, ok := sheinCandidateVariantCostGroupOverride(product, fetchedCosts); ok {
			productOverrides[index] = cost
			productSKUCostOverrides[index] = skuCosts
			continue
		}
		identity := ResolveSheinSDSCostGroupIdentity(product)
		if identity.GroupKey == "" {
			continue
		}
		if cost, ok := fetchedCosts[identity.GroupKey]; ok {
			groupOverrides[identity.GroupKey] = cost
			continue
		}
		for _, legacyKey := range identity.LegacyGroupKeys {
			if cost, ok := fetchedCosts[legacyKey]; ok {
				groupOverrides[identity.GroupKey] = cost
				break
			}
		}
	}
	if len(productOverrides) == 0 && len(groupOverrides) == 0 {
		return products, nil
	}

	out := make([]SheinSyncedProductRecord, len(products))
	copy(out, products)
	for i := range out {
		if cost, ok := productOverrides[i]; ok {
			out[i].EffectiveCostPrice = sheinFloat64Ptr(cost)
			out[i].SKUCostPriceInfoList = cloneSheinSKUCostPriceList(productSKUCostOverrides[i])
			continue
		}
		key := sheinCandidateSDSCostGroupKey(out[i])
		if cost, ok := groupOverrides[key]; ok {
			out[i].EffectiveCostPrice = sheinFloat64Ptr(cost)
			out[i].SKUCostPriceInfoList = sheinCandidateSKUCostsFromGroupCost(out[i], cost)
		}
	}
	return out, nil
}

func listSheinCandidateSDSCostGroups(
	ctx context.Context,
	reader sheinCandidateSDSCostGroupReader,
	tenantID, storeID int64,
	groupKeys []string,
) ([]SheinSDSCostGroupRecord, error) {
	groups := make([]SheinSDSCostGroupRecord, 0)
	for start := 0; start < len(groupKeys); start += sheinCandidateSDSCostGroupFetchBatchSize {
		end := start + sheinCandidateSDSCostGroupFetchBatchSize
		if end > len(groupKeys) {
			end = len(groupKeys)
		}
		rows, _, err := reader.ListSDSCostGroups(ctx, &SheinSDSCostGroupQuery{
			TenantID:  tenantID,
			StoreID:   storeID,
			GroupKeys: groupKeys[start:end],
			Page:      1,
			PageSize:  end - start,
		})
		if err != nil {
			return nil, err
		}
		groups = append(groups, rows...)
	}
	return groups, nil
}

func sheinCandidateSDSCostGroupKeys(products []SheinSyncedProductRecord) []string {
	out := make([]string, 0, len(products))
	seen := map[string]struct{}{}
	for _, product := range products {
		for _, key := range sheinCandidateSDSCostGroupKeysForProduct(product) {
			if key == "" {
				continue
			}
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			out = append(out, key)
		}
	}
	return out
}

func sheinCandidateSDSCostGroupKey(product SheinSyncedProductRecord) string {
	return ResolveSheinSDSCostGroupIdentity(product).GroupKey
}

func sheinCandidateSDSCostGroupKeysForProduct(product SheinSyncedProductRecord) []string {
	identity := ResolveSheinSDSCostGroupIdentity(product)
	if identity.GroupKey == "" {
		return nil
	}
	keys := make([]string, 0, 2+len(identity.LegacyGroupKeys)+len(SheinSyncedProductSKUCodes(product)))
	for _, variantIdentity := range ResolveSheinSDSVariantCostGroupIdentities(product) {
		keys = append(keys, variantIdentity.GroupKey)
		keys = append(keys, variantIdentity.LegacyGroupKeys...)
	}
	keys = append(keys, identity.GroupKey)
	keys = append(keys, identity.LegacyGroupKeys...)
	return keys
}

func sheinCandidateVariantCostGroupOverride(product SheinSyncedProductRecord, fetchedCosts map[string]float64) (float64, []SheinSKUCostPrice, bool) {
	var (
		maxCost float64
		found   bool
	)
	costBySKU := make(map[string]SheinSKUCostPrice)
	for _, identity := range ResolveSheinSDSVariantCostGroupIdentities(product) {
		if cost, ok := sheinCandidateCostForIdentity(identity, fetchedCosts); ok {
			maxCost, found = sheinCandidateMaxCost(maxCost, found, cost)
			for _, skuCode := range identity.SKUCodes {
				skuCode = strings.TrimSpace(skuCode)
				if skuCode == "" {
					continue
				}
				costBySKU[skuCode] = SheinSKUCostPrice{SKUCode: skuCode, CostPrice: cost}
			}
			continue
		}
	}
	if !found {
		return 0, nil, false
	}
	return maxCost, sheinCandidateSortedSKUCosts(costBySKU), true
}

func sheinCandidateCostForIdentity(identity SheinSDSCostGroupIdentity, fetchedCosts map[string]float64) (float64, bool) {
	if cost, ok := fetchedCosts[identity.GroupKey]; ok {
		return cost, true
	}
	for _, legacyKey := range identity.LegacyGroupKeys {
		if cost, ok := fetchedCosts[legacyKey]; ok {
			return cost, true
		}
	}
	return 0, false
}

func sheinCandidateMaxCost(current float64, found bool, cost float64) (float64, bool) {
	if !found || cost > current {
		return cost, true
	}
	return current, found
}

func sheinCandidateSKUCostsFromGroupCost(product SheinSyncedProductRecord, cost float64) []SheinSKUCostPrice {
	if cost <= 0 {
		return nil
	}
	costBySKU := make(map[string]SheinSKUCostPrice)
	for _, skuCode := range SheinSyncedProductSKUCodes(product) {
		skuCode = strings.TrimSpace(skuCode)
		if skuCode == "" {
			continue
		}
		costBySKU[skuCode] = SheinSKUCostPrice{SKUCode: skuCode, CostPrice: cost}
	}
	return sheinCandidateSortedSKUCosts(costBySKU)
}

func sheinCandidateSortedSKUCosts(costBySKU map[string]SheinSKUCostPrice) []SheinSKUCostPrice {
	if len(costBySKU) == 0 {
		return nil
	}
	out := make([]SheinSKUCostPrice, 0, len(costBySKU))
	for _, cost := range costBySKU {
		out = append(out, cost)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].SKUCode < out[j].SKUCode
	})
	return out
}

func calculateSheinCandidateProfitRate(costPrice *float64, priceSnapshot string) *float64 {
	if costPrice == nil || strings.TrimSpace(priceSnapshot) == "" {
		return nil
	}
	var payload struct {
		SalePrice float64 `json:"sale_price"`
	}
	if err := json.Unmarshal([]byte(priceSnapshot), &payload); err != nil {
		return nil
	}
	if payload.SalePrice <= 0 {
		return nil
	}
	profitRate := (payload.SalePrice - *costPrice) / payload.SalePrice
	return sheinFloat64Ptr(profitRate)
}

func buildSheinActivityKey(activityType string, tenantID, storeID int64) string {
	return activityType + ":" + strconv.FormatInt(tenantID, 10) + ":" + strconv.FormatInt(storeID, 10)
}

func buildSheinCandidateVersion(product SheinSyncedProductRecord) string {
	hash := sha1.New()
	hash.Write([]byte(strconv.FormatInt(product.ID, 10)))
	hash.Write([]byte{0})
	hash.Write([]byte(product.SKCName))
	hash.Write([]byte{0})
	hash.Write([]byte(product.ShelfStatus))
	hash.Write([]byte{0})
	hash.Write([]byte(product.SyncVersion))
	hash.Write([]byte{0})
	if product.EffectiveCostPrice != nil {
		hash.Write([]byte(strconv.FormatFloat(*product.EffectiveCostPrice, 'f', -1, 64)))
	}
	hash.Write([]byte{0})
	hash.Write([]byte(product.PriceSnapshot))
	hash.Write([]byte{0})
	hash.Write([]byte(product.InventorySnapshot))
	hash.Write([]byte{0})
	writeSheinCandidateSKUCostVersion(hash, product.SKUCostPriceInfoList)
	return hex.EncodeToString(hash.Sum(nil))
}

func writeSheinCandidateSKUCostVersion(hash hashWriter, source []SheinSKUCostPrice) {
	if len(source) == 0 {
		return
	}
	items := cloneSheinSKUCostPriceList(source)
	sort.Slice(items, func(i, j int) bool {
		if items[i].SKUCode != items[j].SKUCode {
			return items[i].SKUCode < items[j].SKUCode
		}
		if items[i].Currency != items[j].Currency {
			return items[i].Currency < items[j].Currency
		}
		return items[i].CostPrice < items[j].CostPrice
	})
	for _, item := range items {
		hash.Write([]byte(strings.TrimSpace(item.SKUCode)))
		hash.Write([]byte{0})
		hash.Write([]byte(strconv.FormatFloat(item.CostPrice, 'f', -1, 64)))
		hash.Write([]byte{0})
		hash.Write([]byte(strings.TrimSpace(item.Currency)))
		hash.Write([]byte{0})
	}
}

type hashWriter interface {
	Write([]byte) (int, error)
}

func sheinCandidateStateKey(skcName, candidateVersion string) string {
	return skcName + "\x00" + candidateVersion
}
