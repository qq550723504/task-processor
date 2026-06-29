package listingadmin

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

type PausedTaskGroup struct {
	TenantID   int64
	StoreID    int64
	ReasonCode string
	Stage      string
	Count      int64
}

type PausedTaskStoreState struct {
	TenantID           int64
	StoreID            int64
	StoreName          string
	StoreStatus        int16
	AutoListing        bool
	AutoLogin          bool
	DailyLimit         int
	DailyLimitType     string
	CompletedToday     int
	RuntimePauseReason string
}

type PausedTaskRecoveryOptions struct {
	AllowedReasonCodes []string
}

type PausedTaskRecoveryPlan struct {
	Groups           []PausedTaskRecoveryGroup
	TotalPaused      int64
	TotalRecoverable int64
}

type PausedTaskRecoveryResult struct {
	Recovered int64
}

type PausedTaskRecoveryGroup struct {
	PausedTaskGroup
	Store       PausedTaskStoreState
	Recoverable bool
	SkipReason  string
}

type PausedTaskRecoveryImportRepository interface {
	ListPausedTaskGroups(ctx context.Context, platform string) ([]PausedTaskGroup, error)
	CountDailyDispatchUsage(ctx context.Context, platform string, tenantID, storeID int64, day time.Time) (DailyDispatchUsage, error)
	RecoverPausedTaskGroup(ctx context.Context, platform string, group PausedTaskGroup) (int64, error)
}

type PausedTaskRecoveryStoreRepository interface {
	FindStoreByID(ctx context.Context, id int64) (*Store, error)
}

type RuntimePauseReader interface {
	Get(ctx context.Context, key string) (string, error)
}

type PausedTaskRecoveryService struct {
	Platform           string
	ImportTasks        PausedTaskRecoveryImportRepository
	Stores             PausedTaskRecoveryStoreRepository
	RuntimePauses      RuntimePauseReader
	AllowedReasonCodes []string
	StoreIDs           []int64
	Now                func() time.Time
}

func (s PausedTaskRecoveryService) Plan(ctx context.Context) (PausedTaskRecoveryPlan, error) {
	if s.ImportTasks == nil {
		return PausedTaskRecoveryPlan{}, errors.New("paused task recovery import repository is required")
	}
	if s.Stores == nil {
		return PausedTaskRecoveryPlan{}, errors.New("paused task recovery store repository is required")
	}
	platform := strings.TrimSpace(s.Platform)
	if platform == "" {
		return PausedTaskRecoveryPlan{}, errors.New("paused task recovery platform is required")
	}

	groups, err := s.ImportTasks.ListPausedTaskGroups(ctx, platform)
	if err != nil {
		return PausedTaskRecoveryPlan{}, err
	}
	groups = filterPausedTaskGroupsByStoreIDs(groups, s.StoreIDs)
	stores := make(map[int64]PausedTaskStoreState)
	day := time.Now()
	if s.Now != nil {
		day = s.Now()
	}
	for _, group := range groups {
		if _, ok := stores[group.StoreID]; ok {
			continue
		}
		state, err := s.loadStoreState(ctx, platform, group, day)
		if err != nil {
			return PausedTaskRecoveryPlan{}, err
		}
		stores[group.StoreID] = state
	}

	return BuildPausedTaskRecoveryPlan(groups, stores, PausedTaskRecoveryOptions{
		AllowedReasonCodes: s.AllowedReasonCodes,
	}), nil
}

func (s PausedTaskRecoveryService) Execute(ctx context.Context, plan PausedTaskRecoveryPlan) (PausedTaskRecoveryResult, error) {
	if s.ImportTasks == nil {
		return PausedTaskRecoveryResult{}, errors.New("paused task recovery import repository is required")
	}
	platform := strings.TrimSpace(s.Platform)
	if platform == "" {
		return PausedTaskRecoveryResult{}, errors.New("paused task recovery platform is required")
	}

	var result PausedTaskRecoveryResult
	for _, group := range plan.Groups {
		if !group.Recoverable {
			continue
		}
		affected, err := s.ImportTasks.RecoverPausedTaskGroup(ctx, platform, group.PausedTaskGroup)
		if err != nil {
			return result, err
		}
		result.Recovered += affected
	}
	return result, nil
}

func filterPausedTaskGroupsByStoreIDs(groups []PausedTaskGroup, storeIDs []int64) []PausedTaskGroup {
	if len(storeIDs) == 0 {
		return groups
	}
	allowed := make(map[int64]struct{}, len(storeIDs))
	for _, storeID := range storeIDs {
		if storeID > 0 {
			allowed[storeID] = struct{}{}
		}
	}
	filtered := make([]PausedTaskGroup, 0, len(groups))
	for _, group := range groups {
		if _, ok := allowed[group.StoreID]; ok {
			filtered = append(filtered, group)
		}
	}
	return filtered
}

func (s PausedTaskRecoveryService) loadStoreState(ctx context.Context, platform string, group PausedTaskGroup, day time.Time) (PausedTaskStoreState, error) {
	store, err := s.Stores.FindStoreByID(ctx, group.StoreID)
	if err != nil {
		if errors.Is(err, ErrStoreNotFound) {
			return PausedTaskStoreState{}, nil
		}
		return PausedTaskStoreState{}, err
	}
	state := PausedTaskStoreState{
		TenantID:       store.TenantID,
		StoreID:        store.ID,
		StoreName:      store.Name,
		StoreStatus:    store.Status,
		AutoListing:    boolPtrValue(store.EnableAutoListing),
		AutoLogin:      boolPtrValue(store.EnableAutoLogin),
		DailyLimitType: strings.TrimSpace(store.DailyLimitType),
	}
	if store.DailyLimit != nil {
		state.DailyLimit = *store.DailyLimit
	}
	usage, err := s.ImportTasks.CountDailyDispatchUsage(ctx, platform, group.TenantID, group.StoreID, day)
	if err != nil {
		return PausedTaskStoreState{}, err
	}
	state.CompletedToday = usage.Completed + usage.Processing + usage.Queued
	if s.RuntimePauses != nil {
		key := fmt.Sprintf("listing:task:pause:%s:%d:%d", platform, group.TenantID, group.StoreID)
		value, err := s.RuntimePauses.Get(ctx, key)
		if err != nil && !isRuntimePauseKeyNotFound(err) {
			return PausedTaskStoreState{}, err
		}
		state.RuntimePauseReason = strings.TrimSpace(value)
	}
	return state, nil
}

func BuildPausedTaskRecoveryPlan(
	groups []PausedTaskGroup,
	stores map[int64]PausedTaskStoreState,
	opts PausedTaskRecoveryOptions,
) PausedTaskRecoveryPlan {
	allowedReasons := make(map[string]struct{}, len(opts.AllowedReasonCodes))
	for _, reason := range opts.AllowedReasonCodes {
		reason = strings.TrimSpace(reason)
		if reason != "" {
			allowedReasons[reason] = struct{}{}
		}
	}

	plan := PausedTaskRecoveryPlan{
		Groups: make([]PausedTaskRecoveryGroup, 0, len(groups)),
	}
	for _, group := range groups {
		item := PausedTaskRecoveryGroup{
			PausedTaskGroup: group,
			Store:           stores[group.StoreID],
		}
		item.Recoverable, item.SkipReason = evaluatePausedTaskRecoveryGroup(group, item.Store, allowedReasons)
		plan.TotalPaused += group.Count
		if item.Recoverable {
			plan.TotalRecoverable += group.Count
		}
		plan.Groups = append(plan.Groups, item)
	}
	return plan
}

func evaluatePausedTaskRecoveryGroup(
	group PausedTaskGroup,
	store PausedTaskStoreState,
	allowedReasons map[string]struct{},
) (bool, string) {
	if group.StoreID == 0 || store.StoreID == 0 {
		return false, "store_missing"
	}
	if len(allowedReasons) > 0 {
		if _, ok := allowedReasons[strings.TrimSpace(group.ReasonCode)]; !ok {
			return false, "reason_not_allowed"
		}
	}
	if store.StoreStatus != 0 {
		return false, "store_disabled"
	}
	if !store.AutoListing {
		return false, "auto_listing_disabled"
	}
	if strings.TrimSpace(store.RuntimePauseReason) != "" {
		return false, "runtime_paused"
	}
	if store.DailyLimit > 0 && store.CompletedToday >= store.DailyLimit {
		return false, "daily_limit_reached"
	}
	return true, ""
}

func boolPtrValue(value *bool) bool {
	return value != nil && *value
}

func isRuntimePauseKeyNotFound(err error) bool {
	if err == nil {
		return false
	}
	message := err.Error()
	return strings.Contains(message, "key not found") || strings.Contains(message, "runtime key not found")
}
