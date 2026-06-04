package sheinsync

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"strconv"

	"gorm.io/gorm"
)

const sheinCandidateRefreshPageSize = 100

type SheinCandidateService interface {
	RefreshCandidates(ctx context.Context, tenantID, storeID int64, activityType string) (*SheinCandidateRefreshResult, error)
	ListCandidates(ctx context.Context, query *SheinActivityCandidateQuery) ([]SheinActivityCandidateRecord, int64, error)
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
		ReviewStatus:       SheinCandidateReviewStatusPendingReview,
		AutoModeEligible:   false,
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
	return hex.EncodeToString(hash.Sum(nil))
}

func sheinCandidateStateKey(skcName, candidateVersion string) string {
	return skcName + "\x00" + candidateVersion
}
