package listingadmin

import (
	"context"
	"fmt"
	"testing"
	"time"

	"task-processor/internal/model"
)

func TestPausedTaskRecoveryPlanAllowsOnlySafeStoreDispatchDisabledGroups(t *testing.T) {
	groups := []PausedTaskGroup{
		{TenantID: 375, StoreID: 1033, ReasonCode: "STORE_DISPATCH_DISABLED", Stage: "store_dispatch_guard", Count: 358},
		{TenantID: 375, StoreID: 1033, ReasonCode: "DAILY_LIMIT_REACHED", Stage: "check_daily_limit", Count: 4},
		{TenantID: 322, StoreID: 976, ReasonCode: "AUTH_EXPIRED", Stage: "cookie_provider", Count: 2},
		{TenantID: 246, StoreID: 1042, ReasonCode: "STORE_DISPATCH_DISABLED", Stage: "store_dispatch_guard", Count: 3573},
	}
	stores := map[int64]PausedTaskStoreState{
		1033: {TenantID: 375, StoreID: 1033, StoreStatus: 0, AutoListing: true, DailyLimit: 150, CompletedToday: 0},
		976:  {TenantID: 322, StoreID: 976, StoreStatus: 0, AutoListing: true, DailyLimit: 10000, CompletedToday: 40},
		1042: {TenantID: 246, StoreID: 1042, StoreStatus: 0, AutoListing: true, DailyLimit: 700, CompletedToday: 40, RuntimePauseReason: "quota_limit"},
	}

	plan := BuildPausedTaskRecoveryPlan(groups, stores, PausedTaskRecoveryOptions{
		AllowedReasonCodes: []string{"STORE_DISPATCH_DISABLED"},
	})

	if plan.TotalRecoverable != 358 {
		t.Fatalf("TotalRecoverable = %d, want 358", plan.TotalRecoverable)
	}
	if len(plan.Groups) != 4 {
		t.Fatalf("groups len = %d, want 4", len(plan.Groups))
	}
	assertRecoveryDecision(t, plan.Groups[0], true, "")
	assertRecoveryDecision(t, plan.Groups[1], false, "reason_not_allowed")
	assertRecoveryDecision(t, plan.Groups[2], false, "reason_not_allowed")
	assertRecoveryDecision(t, plan.Groups[3], false, "runtime_paused")
}

func TestPausedTaskRecoveryPlanRejectsDisabledStoresAndFilledQuota(t *testing.T) {
	groups := []PausedTaskGroup{
		{TenantID: 1, StoreID: 10, ReasonCode: "STORE_DISPATCH_DISABLED", Count: 5},
		{TenantID: 1, StoreID: 11, ReasonCode: "STORE_DISPATCH_DISABLED", Count: 6},
		{TenantID: 1, StoreID: 12, ReasonCode: "STORE_DISPATCH_DISABLED", Count: 7},
	}
	stores := map[int64]PausedTaskStoreState{
		10: {TenantID: 1, StoreID: 10, StoreStatus: 1, AutoListing: true, DailyLimit: 100},
		11: {TenantID: 1, StoreID: 11, StoreStatus: 0, AutoListing: false, DailyLimit: 100},
		12: {TenantID: 1, StoreID: 12, StoreStatus: 0, AutoListing: true, DailyLimit: 100, CompletedToday: 100},
	}

	plan := BuildPausedTaskRecoveryPlan(groups, stores, PausedTaskRecoveryOptions{
		AllowedReasonCodes: []string{"STORE_DISPATCH_DISABLED"},
	})

	if plan.TotalRecoverable != 0 {
		t.Fatalf("TotalRecoverable = %d, want 0", plan.TotalRecoverable)
	}
	assertRecoveryDecision(t, plan.Groups[0], false, "store_disabled")
	assertRecoveryDecision(t, plan.Groups[1], false, "auto_listing_disabled")
	assertRecoveryDecision(t, plan.Groups[2], false, "daily_limit_reached")
}

func TestPausedTaskRecoveryServiceBuildsPlanFromRepositories(t *testing.T) {
	t.Parallel()

	autoListing := true
	stores := map[int64]*Store{
		1033: {ID: 1033, TenantID: 375, Name: "store-1033", Platform: "shein", Status: 0, EnableAutoListing: &autoListing, DailyLimit: recoveryIntPtr(150), DailyLimitType: "SKC"},
		1042: {ID: 1042, TenantID: 246, Name: "store-1042", Platform: "shein", Status: 0, EnableAutoListing: &autoListing, DailyLimit: recoveryIntPtr(700), DailyLimitType: "SKC"},
	}
	imports := &fakePausedTaskRecoveryImportRepo{
		groups: []PausedTaskGroup{
			{TenantID: 375, StoreID: 1033, ReasonCode: "STORE_DISPATCH_DISABLED", Stage: "store_dispatch_guard", Count: 358},
			{TenantID: 246, StoreID: 1042, ReasonCode: "STORE_DISPATCH_DISABLED", Stage: "store_dispatch_guard", Count: 3573},
		},
		usage: map[int64]DailyDispatchUsage{
			1033: {Completed: 0},
			1042: {Completed: 40},
		},
	}
	service := PausedTaskRecoveryService{
		Platform:           "shein",
		ImportTasks:        imports,
		Stores:             fakePausedTaskRecoveryStoreRepo{stores: stores},
		RuntimePauses:      fakeRuntimePauseReader{values: map[string]string{"listing:task:pause:shein:246:1042": "quota_limit"}},
		AllowedReasonCodes: []string{"STORE_DISPATCH_DISABLED"},
		Now:                func() time.Time { return time.Date(2026, 6, 29, 12, 0, 0, 0, time.Local) },
	}

	plan, err := service.Plan(context.Background())
	if err != nil {
		t.Fatalf("Plan() error = %v", err)
	}

	if plan.TotalRecoverable != 358 {
		t.Fatalf("TotalRecoverable = %d, want 358", plan.TotalRecoverable)
	}
	assertRecoveryDecision(t, plan.Groups[0], true, "")
	assertRecoveryDecision(t, plan.Groups[1], false, "runtime_paused")
	if plan.Groups[0].Store.StoreName != "store-1033" || plan.Groups[0].Store.CompletedToday != 0 {
		t.Fatalf("store state = %+v, want name and completed usage", plan.Groups[0].Store)
	}
}

func TestPausedTaskRecoveryServiceFiltersPlanByStoreIDs(t *testing.T) {
	t.Parallel()

	autoListing := true
	stores := map[int64]*Store{
		877:  {ID: 877, TenantID: 246, Name: "store-877", Platform: "shein", Status: 0, EnableAutoListing: &autoListing, DailyLimit: recoveryIntPtr(600)},
		1033: {ID: 1033, TenantID: 375, Name: "store-1033", Platform: "shein", Status: 0, EnableAutoListing: &autoListing, DailyLimit: recoveryIntPtr(150)},
		1052: {ID: 1052, TenantID: 246, Name: "store-1052", Platform: "shein", Status: 0, EnableAutoListing: &autoListing, DailyLimit: recoveryIntPtr(500)},
	}
	imports := &fakePausedTaskRecoveryImportRepo{
		groups: []PausedTaskGroup{
			{TenantID: 246, StoreID: 877, ReasonCode: "STORE_DISPATCH_DISABLED", Stage: "store_dispatch_guard", Count: 3178},
			{TenantID: 375, StoreID: 1033, ReasonCode: "STORE_DISPATCH_DISABLED", Stage: "store_dispatch_guard", Count: 358},
			{TenantID: 246, StoreID: 1052, ReasonCode: "STORE_DISPATCH_DISABLED", Stage: "store_dispatch_guard", Count: 4734},
		},
	}
	service := PausedTaskRecoveryService{
		Platform:           "shein",
		ImportTasks:        imports,
		Stores:             fakePausedTaskRecoveryStoreRepo{stores: stores},
		RuntimePauses:      fakeRuntimePauseReader{},
		AllowedReasonCodes: []string{"STORE_DISPATCH_DISABLED"},
		StoreIDs:           []int64{877, 1033},
	}

	plan, err := service.Plan(context.Background())
	if err != nil {
		t.Fatalf("Plan() error = %v", err)
	}

	if plan.TotalRecoverable != 3536 {
		t.Fatalf("TotalRecoverable = %d, want 3536", plan.TotalRecoverable)
	}
	if len(plan.Groups) != 2 {
		t.Fatalf("groups len = %d, want 2: %+v", len(plan.Groups), plan.Groups)
	}
	if plan.Groups[0].StoreID != 877 || plan.Groups[1].StoreID != 1033 {
		t.Fatalf("store IDs = %d/%d, want 877/1033", plan.Groups[0].StoreID, plan.Groups[1].StoreID)
	}
}

func TestPausedTaskRecoveryServiceExecuteRecoversOnlyRecoverableGroups(t *testing.T) {
	t.Parallel()

	imports := &fakePausedTaskRecoveryImportRepo{}
	service := PausedTaskRecoveryService{Platform: "shein", ImportTasks: imports}
	plan := PausedTaskRecoveryPlan{
		Groups: []PausedTaskRecoveryGroup{
			{PausedTaskGroup: PausedTaskGroup{TenantID: 1, StoreID: 10, ReasonCode: "STORE_DISPATCH_DISABLED", Stage: "store_dispatch_guard", Count: 5}, Recoverable: true},
			{PausedTaskGroup: PausedTaskGroup{TenantID: 1, StoreID: 11, ReasonCode: "AUTH_EXPIRED", Stage: "auth", Count: 6}, Recoverable: false, SkipReason: "reason_not_allowed"},
		},
	}

	result, err := service.Execute(context.Background(), plan)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if result.Recovered != 5 {
		t.Fatalf("Recovered = %d, want 5", result.Recovered)
	}
	if len(imports.recovered) != 1 || imports.recovered[0].StoreID != 10 {
		t.Fatalf("recovered groups = %+v, want only store 10", imports.recovered)
	}
}

func assertRecoveryDecision(t *testing.T, group PausedTaskRecoveryGroup, recoverable bool, reason string) {
	t.Helper()
	if group.Recoverable != recoverable {
		t.Fatalf("group %+v Recoverable = %v, want %v", group.PausedTaskGroup, group.Recoverable, recoverable)
	}
	if group.SkipReason != reason {
		t.Fatalf("group %+v SkipReason = %q, want %q", group.PausedTaskGroup, group.SkipReason, reason)
	}
}

type fakePausedTaskRecoveryImportRepo struct {
	groups    []PausedTaskGroup
	usage     map[int64]DailyDispatchUsage
	recovered []PausedTaskGroup
}

func (f *fakePausedTaskRecoveryImportRepo) ListPausedTaskGroups(context.Context, string) ([]PausedTaskGroup, error) {
	return f.groups, nil
}

func (f *fakePausedTaskRecoveryImportRepo) CountDailyDispatchUsage(_ context.Context, _ string, _ int64, storeID int64, _ time.Time) (DailyDispatchUsage, error) {
	return f.usage[storeID], nil
}

func (f *fakePausedTaskRecoveryImportRepo) RecoverPausedTaskGroup(_ context.Context, _ string, group PausedTaskGroup) (int64, error) {
	f.recovered = append(f.recovered, group)
	return group.Count, nil
}

type fakePausedTaskRecoveryStoreRepo struct {
	stores map[int64]*Store
}

func (f fakePausedTaskRecoveryStoreRepo) FindStoreByID(_ context.Context, id int64) (*Store, error) {
	store := f.stores[id]
	if store == nil {
		return nil, ErrStoreNotFound
	}
	return store, nil
}

type fakeRuntimePauseReader struct {
	values map[string]string
}

func (f fakeRuntimePauseReader) Get(_ context.Context, key string) (string, error) {
	value := f.values[key]
	if value == "" {
		return "", fmt.Errorf("key not found: %s", key)
	}
	return value, nil
}

func recoveryIntPtr(value int) *int {
	return &value
}

func TestListPausedTaskGroupsGroupsOnlyPausedTasksForPlatform(t *testing.T) {
	t.Parallel()

	db := newImportTaskDispatchTestDB(t)
	now := time.Now()
	seedDispatchTasks(t, db, []listingProductImportTask{
		{ID: 1, TenantID: 375, StoreID: 1033, Platform: "shein", Region: "us", ProductID: "paused-1", Status: model.TaskStatusPaused.Int16(), ReasonCode: "STORE_DISPATCH_DISABLED", Stage: "store_dispatch_guard", CreateTime: &now, UpdateTime: &now, Deleted: 0},
		{ID: 2, TenantID: 375, StoreID: 1033, Platform: "shein", Region: "us", ProductID: "paused-2", Status: model.TaskStatusPaused.Int16(), ReasonCode: "STORE_DISPATCH_DISABLED", Stage: "store_dispatch_guard", CreateTime: &now, UpdateTime: &now, Deleted: 0},
		{ID: 3, TenantID: 375, StoreID: 1033, Platform: "shein", Region: "us", ProductID: "quota", Status: model.TaskStatusPaused.Int16(), ReasonCode: "DAILY_LIMIT_REACHED", Stage: "check_daily_limit", CreateTime: &now, UpdateTime: &now, Deleted: 0},
		{ID: 4, TenantID: 375, StoreID: 1033, Platform: "shein", Region: "us", ProductID: "pending", Status: model.TaskStatusPending.Int16(), ReasonCode: "STORE_DISPATCH_DISABLED", Stage: "store_dispatch_guard", CreateTime: &now, UpdateTime: &now, Deleted: 0},
		{ID: 5, TenantID: 375, StoreID: 1033, Platform: "temu", TargetPlatform: "shein", Region: "us", ProductID: "target-platform", Status: model.TaskStatusPaused.Int16(), ReasonCode: "STORE_DISPATCH_DISABLED", Stage: "store_dispatch_guard", CreateTime: &now, UpdateTime: &now, Deleted: 0},
		{ID: 6, TenantID: 375, StoreID: 1033, Platform: "temu", Region: "us", ProductID: "wrong-platform", Status: model.TaskStatusPaused.Int16(), ReasonCode: "STORE_DISPATCH_DISABLED", Stage: "store_dispatch_guard", CreateTime: &now, UpdateTime: &now, Deleted: 0},
	})

	groups, err := NewGormImportTaskRepository(db).ListPausedTaskGroups(context.Background(), "shein")
	if err != nil {
		t.Fatalf("ListPausedTaskGroups() error = %v", err)
	}

	if len(groups) != 2 {
		t.Fatalf("groups len = %d, want 2: %+v", len(groups), groups)
	}
	if groups[0].ReasonCode != "STORE_DISPATCH_DISABLED" || groups[0].Count != 3 {
		t.Fatalf("first group = %+v, want STORE_DISPATCH_DISABLED count 3", groups[0])
	}
	if groups[1].ReasonCode != "DAILY_LIMIT_REACHED" || groups[1].Count != 1 {
		t.Fatalf("second group = %+v, want DAILY_LIMIT_REACHED count 1", groups[1])
	}
}

func TestRecoverPausedTaskGroupMovesOnlyMatchingPausedRowsToPending(t *testing.T) {
	t.Parallel()

	db := newImportTaskDispatchTestDB(t)
	now := time.Now()
	seedDispatchTasks(t, db, []listingProductImportTask{
		{ID: 11, TenantID: 375, StoreID: 1033, Platform: "shein", Region: "us", ProductID: "recover-1", Status: model.TaskStatusPaused.Int16(), ReasonCode: "STORE_DISPATCH_DISABLED", Stage: "store_dispatch_guard", ErrorMessage: "paused", ProcessingNode: "node-a", Remark: "old", CreateTime: &now, UpdateTime: &now, Deleted: 0},
		{ID: 12, TenantID: 375, StoreID: 1033, Platform: "shein", Region: "us", ProductID: "recover-2", Status: model.TaskStatusPaused.Int16(), ReasonCode: "STORE_DISPATCH_DISABLED", Stage: "store_dispatch_guard", ErrorMessage: "paused", ProcessingNode: "node-a", Remark: "old", CreateTime: &now, UpdateTime: &now, Deleted: 0},
		{ID: 13, TenantID: 375, StoreID: 1033, Platform: "shein", Region: "us", ProductID: "quota", Status: model.TaskStatusPaused.Int16(), ReasonCode: "DAILY_LIMIT_REACHED", Stage: "check_daily_limit", CreateTime: &now, UpdateTime: &now, Deleted: 0},
		{ID: 14, TenantID: 375, StoreID: 1034, Platform: "shein", Region: "us", ProductID: "other-store", Status: model.TaskStatusPaused.Int16(), ReasonCode: "STORE_DISPATCH_DISABLED", Stage: "store_dispatch_guard", CreateTime: &now, UpdateTime: &now, Deleted: 0},
		{ID: 15, TenantID: 375, StoreID: 1033, Platform: "shein", Region: "us", ProductID: "already-pending", Status: model.TaskStatusPending.Int16(), ReasonCode: "STORE_DISPATCH_DISABLED", Stage: "store_dispatch_guard", CreateTime: &now, UpdateTime: &now, Deleted: 0},
	})

	affected, err := NewGormImportTaskRepository(db).RecoverPausedTaskGroup(context.Background(), "shein", PausedTaskGroup{
		TenantID:   375,
		StoreID:    1033,
		ReasonCode: "STORE_DISPATCH_DISABLED",
		Stage:      "store_dispatch_guard",
	})
	if err != nil {
		t.Fatalf("RecoverPausedTaskGroup() error = %v", err)
	}
	if affected != 2 {
		t.Fatalf("affected = %d, want 2", affected)
	}

	var rows []listingProductImportTask
	if err := db.Table("listing_product_import_task").Where("id IN ?", []int64{11, 12, 13, 14, 15}).Order("id").Find(&rows).Error; err != nil {
		t.Fatalf("load rows: %v", err)
	}
	if rows[0].Status != model.TaskStatusPending.Int16() || rows[0].ReasonCode != "" || rows[0].Stage != "" || rows[0].ErrorMessage != "" || rows[0].ProcessingNode != "" {
		t.Fatalf("row 11 = %+v, want restored pending with cleared delay fields", rows[0])
	}
	if rows[1].Status != model.TaskStatusPending.Int16() {
		t.Fatalf("row 12 status = %d, want pending", rows[1].Status)
	}
	if rows[2].Status != model.TaskStatusPaused.Int16() || rows[3].Status != model.TaskStatusPaused.Int16() || rows[4].Status != model.TaskStatusPending.Int16() {
		t.Fatalf("non-matching statuses = %d/%d/%d, want paused/paused/pending", rows[2].Status, rows[3].Status, rows[4].Status)
	}
}
