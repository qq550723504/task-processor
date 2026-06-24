package listingkit

import (
	"context"
	"strings"
	"testing"
	"time"

	"task-processor/internal/catalog/canonical"
)

func TestSDSRetirementBuildItemsRejectsMissingSourceSKUs(t *testing.T) {
	service := NewSDSRetirementService(nil, nil, nil).(*sdsRetirementService)
	_, err := service.buildSDSRetirementItems(context.Background(), "1", 177, []Task{{
		Result: &ListingKitResult{CanonicalProduct: &canonical.Product{}},
	}})
	if err == nil || !strings.Contains(err.Error(), "no source SDS SKUs") {
		t.Fatalf("error = %v, want missing source SKU error", err)
	}
}

func TestSDSRetirementCreateRunRefreshesSheinProductsAndPersistsMatches(t *testing.T) {
	repo := &sdsRetirementServiceRepoStub{
		listTasks: []Task{{
			ID: "task-1",
			Request: &GenerateRequest{Options: &GenerateOptions{SDS: &SDSSyncOptions{
				ParentProductID:  238915,
				PrototypeGroupID: 28345,
				VariantID:        238916,
			}}},
			Result: &ListingKitResult{
				CanonicalProduct: &canonical.Product{
					Variants: []canonical.Variant{{
						Attributes: map[string]canonical.Attribute{
							"source_sds_sku": {Value: "MG8006905001"},
						},
					}},
				},
			},
		}},
	}
	baseline := &sdsRetirementBaselineStub{
		readiness: &SDSBaselineReadiness{
			Status:           SDSBaselineStatusReady,
			ValidationStatus: SDSBaselineValidationStatusReady,
			ReasonCode:       "ready",
			Reason:           "baseline cached",
		},
	}
	shein := &sdsRetirementSheinSyncStub{
		supportsImmediateRefresh: true,
		products: []SheinSyncedProductRecord{
			{ID: 11, SupplierCode: "MG8006905001", ShelfStatus: "ON_SHELF", BusinessModel: 7, SPUName: "spu", SKCName: "skc", SKCCode: "skc-1"},
			{ID: 12, SupplierCode: "OTHER", ShelfStatus: "ON_SHELF", BusinessModel: 9},
		},
	}

	service := NewSDSRetirementService(repo, baseline, shein)
	detail, err := service.CreateSDSRetirementRun(WithTenantID(context.Background(), "18"), &CreateSDSRetirementRunRequest{
		Platform:         "SHEIN",
		StoreID:          177,
		ParentProductID:  238915,
		PrototypeGroupID: 28345,
		VariantID:        238916,
		CreatedBy:        "tester",
	})
	if err != nil {
		t.Fatalf("CreateSDSRetirementRun() error = %v", err)
	}
	if shein.syncTenantID != 18 || shein.syncStoreID != 177 || shein.syncTrigger != SheinSyncTriggerModeManual {
		t.Fatalf("unexpected sync call = tenant %d store %d trigger %q", shein.syncTenantID, shein.syncStoreID, shein.syncTrigger)
	}
	if detail.Run.Platform != "shein" || detail.Run.Status != SDSRetirementRunStatusReady {
		t.Fatalf("run = %#v", detail.Run)
	}
	if detail.Run.TenantID != "18" || detail.Run.ValidationStatus != SDSBaselineValidationStatusReady {
		t.Fatalf("run tenant/validation = %#v", detail.Run)
	}
	if len(detail.Items) != 1 {
		t.Fatalf("items = %#v", detail.Items)
	}
	if detail.Items[0].Platform != "shein" || detail.Items[0].SyncedProductID != 11 || detail.Items[0].BusinessModel != 7 {
		t.Fatalf("item = %#v", detail.Items[0])
	}
	if repo.createdRun == nil || len(repo.createdItems) != 1 {
		t.Fatalf("created run/items = %#v %#v", repo.createdRun, repo.createdItems)
	}
}

func TestSDSRetirementGetAndUpdateSelectionUseRepository(t *testing.T) {
	repo := &sdsRetirementServiceRepoStub{
		storedRun: &SDSRetirementRunRecord{
			ID:       "run-1",
			Platform: "shein",
			Status:   SDSRetirementRunStatusReady,
		},
		storedItems: []SDSRetirementItemRecord{{
			ID:       "item-1",
			RunID:    "run-1",
			Platform: "shein",
			Selected: true,
			Status:   SDSRetirementItemStatusSelected,
		}},
	}

	service := NewSDSRetirementService(repo, nil, nil)
	ctx := WithTenantID(context.Background(), "tenant-a")
	detail, err := service.GetSDSRetirementRun(ctx, "run-1")
	if err != nil {
		t.Fatalf("GetSDSRetirementRun() error = %v", err)
	}
	if detail.Run.ID != "run-1" || len(detail.Items) != 1 {
		t.Fatalf("detail = %#v", detail)
	}

	updated, err := service.UpdateSDSRetirementSelection(ctx, "run-1", &UpdateSDSRetirementSelectionRequest{
		Items: []SDSRetirementItemSelectionUpdate{{ItemID: "item-1", Selected: false, SiteSelection: `["us"]`}},
	})
	if err != nil {
		t.Fatalf("UpdateSDSRetirementSelection() error = %v", err)
	}
	if len(repo.updatedItems) != 1 || repo.updatedItems[0].ItemID != "item-1" || repo.updatedItems[0].Selected {
		t.Fatalf("updated items = %#v", repo.updatedItems)
	}
	if updated == nil || updated.Run.ID != "run-1" {
		t.Fatalf("updated detail = %#v", updated)
	}
}

func TestSDSRetirementCreateRunRejectsNonNumericTenantForSheinRefresh(t *testing.T) {
	repo := &sdsRetirementServiceRepoStub{
		listTasks: []Task{{
			Request: &GenerateRequest{Options: &GenerateOptions{SDS: &SDSSyncOptions{
				ParentProductID:  238915,
				PrototypeGroupID: 28345,
				VariantID:        238916,
			}}},
			Result: &ListingKitResult{
				CanonicalProduct: &canonical.Product{
					Variants: []canonical.Variant{{
						Attributes: map[string]canonical.Attribute{
							"source_sds_sku": {Value: "MG8006905001"},
						},
					}},
				},
			},
		}},
	}

	service := NewSDSRetirementService(repo, nil, &sdsRetirementSheinSyncStub{})
	_, err := service.CreateSDSRetirementRun(WithTenantID(context.Background(), "tenant-a"), &CreateSDSRetirementRunRequest{
		Platform:         "shein",
		StoreID:          177,
		ParentProductID:  238915,
		PrototypeGroupID: 28345,
		VariantID:        238916,
	})
	if err == nil || !strings.Contains(err.Error(), "tenant id must be numeric") {
		t.Fatalf("error = %v, want numeric tenant error", err)
	}
}

func TestSDSRetirementCreateRunScopesTaskDiscoveryToResolvedTenant(t *testing.T) {
	repo := &sdsRetirementServiceRepoStub{
		listTasksByTenant: map[string][]Task{
			"18": {{
				ID: "task-tenant-a",
				Request: &GenerateRequest{Options: &GenerateOptions{SDS: &SDSSyncOptions{
					ParentProductID:  238915,
					PrototypeGroupID: 28345,
					VariantID:        238916,
				}}},
				Result: &ListingKitResult{
					CanonicalProduct: &canonical.Product{
						Variants: []canonical.Variant{{
							Attributes: map[string]canonical.Attribute{
								"source_sds_sku": {Value: "MG8006905001"},
							},
						}},
					},
				},
			}},
		},
	}
	shein := &sdsRetirementSheinSyncStub{
		supportsImmediateRefresh: true,
		products: []SheinSyncedProductRecord{
			{ID: 11, SupplierCode: "MG8006905001", ShelfStatus: "ON_SHELF"},
		},
	}

	service := NewSDSRetirementService(repo, nil, shein)
	detail, err := service.CreateSDSRetirementRun(context.Background(), &CreateSDSRetirementRunRequest{
		TenantID:         "18",
		Platform:         "shein",
		StoreID:          177,
		ParentProductID:  238915,
		PrototypeGroupID: 28345,
		VariantID:        238916,
	})
	if err != nil {
		t.Fatalf("CreateSDSRetirementRun() error = %v", err)
	}
	if len(repo.listTasksTenantIDs) == 0 || repo.listTasksTenantIDs[0] != "18" {
		t.Fatalf("list task tenant ids = %#v, want 18", repo.listTasksTenantIDs)
	}
	if len(detail.Tasks) != 1 || detail.Tasks[0].ID != "task-tenant-a" {
		t.Fatalf("tasks = %#v", detail.Tasks)
	}
	if len(detail.Items) != 1 || detail.Items[0].SyncedProductID != 11 {
		t.Fatalf("items = %#v", detail.Items)
	}
}

func TestSDSRetirementCreateRunPagesThroughAllTaskPages(t *testing.T) {
	repo := &sdsRetirementServiceRepoStub{
		listTasksPages: [][]Task{
			make([]Task, 100),
			{{
				ID: "task-page-2",
				Request: &GenerateRequest{Options: &GenerateOptions{SDS: &SDSSyncOptions{
					ParentProductID:  238915,
					PrototypeGroupID: 28345,
					VariantID:        238916,
				}}},
				Result: &ListingKitResult{
					CanonicalProduct: &canonical.Product{
						Variants: []canonical.Variant{{
							Attributes: map[string]canonical.Attribute{
								"source_sds_sku": {Value: "MG8006905001"},
							},
						}},
					},
				},
			}},
		},
	}
	for i := range repo.listTasksPages[0] {
		repo.listTasksPages[0][i] = Task{ID: "task-ignore"}
	}
	shein := &sdsRetirementSheinSyncStub{
		supportsImmediateRefresh: true,
		products: []SheinSyncedProductRecord{
			{ID: 11, SupplierCode: "MG8006905001", ShelfStatus: "ON_SHELF"},
		},
	}

	service := NewSDSRetirementService(repo, nil, shein)
	detail, err := service.CreateSDSRetirementRun(WithTenantID(context.Background(), "18"), &CreateSDSRetirementRunRequest{
		Platform:         "shein",
		StoreID:          177,
		ParentProductID:  238915,
		PrototypeGroupID: 28345,
		VariantID:        238916,
	})
	if err != nil {
		t.Fatalf("CreateSDSRetirementRun() error = %v", err)
	}
	if len(repo.listTaskQueries) < 2 {
		t.Fatalf("list task queries = %#v, want multiple pages", repo.listTaskQueries)
	}
	if len(detail.Tasks) != 1 || detail.Tasks[0].ID != "task-page-2" {
		t.Fatalf("tasks = %#v", detail.Tasks)
	}
	if len(detail.Items) != 1 || detail.Items[0].SupplierCode != "MG8006905001" {
		t.Fatalf("items = %#v", detail.Items)
	}
}

func TestSDSRetirementCreateRunPagesThroughAllSyncedProducts(t *testing.T) {
	repo := &sdsRetirementServiceRepoStub{
		listTasks: []Task{{
			ID: "task-1",
			Request: &GenerateRequest{Options: &GenerateOptions{SDS: &SDSSyncOptions{
				ParentProductID:  238915,
				PrototypeGroupID: 28345,
				VariantID:        238916,
			}}},
			Result: &ListingKitResult{
				CanonicalProduct: &canonical.Product{
					Variants: []canonical.Variant{{
						Attributes: map[string]canonical.Attribute{
							"source_sds_sku": {Value: "MG8006905001"},
						},
					}},
				},
			},
		}},
	}
	shein := &sdsRetirementSheinSyncStub{
		supportsImmediateRefresh: true,
		productPages: [][]SheinSyncedProductRecord{
			make([]SheinSyncedProductRecord, 100),
			{{ID: 22, SupplierCode: "MG8006905001", ShelfStatus: "ON_SHELF"}},
		},
	}
	for i := range shein.productPages[0] {
		shein.productPages[0][i] = SheinSyncedProductRecord{ID: int64(i + 1), SupplierCode: "OTHER", ShelfStatus: "ON_SHELF"}
	}

	service := NewSDSRetirementService(repo, nil, shein)
	detail, err := service.CreateSDSRetirementRun(WithTenantID(context.Background(), "18"), &CreateSDSRetirementRunRequest{
		Platform:         "shein",
		StoreID:          177,
		ParentProductID:  238915,
		PrototypeGroupID: 28345,
		VariantID:        238916,
	})
	if err != nil {
		t.Fatalf("CreateSDSRetirementRun() error = %v", err)
	}
	if len(shein.listQueries) < 2 {
		t.Fatalf("list synced product queries = %#v, want multiple pages", shein.listQueries)
	}
	if len(detail.Items) != 1 || detail.Items[0].SyncedProductID != 22 {
		t.Fatalf("items = %#v", detail.Items)
	}
}

func TestSDSRetirementCreateRunRejectsAsyncOnlySheinRefresh(t *testing.T) {
	repo := &sdsRetirementServiceRepoStub{
		listTasks: []Task{{
			ID: "task-1",
			Request: &GenerateRequest{Options: &GenerateOptions{SDS: &SDSSyncOptions{
				ParentProductID:  238915,
				PrototypeGroupID: 28345,
				VariantID:        238916,
			}}},
			Result: &ListingKitResult{
				CanonicalProduct: &canonical.Product{
					Variants: []canonical.Variant{{
						Attributes: map[string]canonical.Attribute{
							"source_sds_sku": {Value: "MG8006905001"},
						},
					}},
				},
			},
		}},
	}
	shein := &sdsRetirementSheinSyncStub{
		supportsImmediateRefresh: false,
		products: []SheinSyncedProductRecord{
			{ID: 11, SupplierCode: "MG8006905001", ShelfStatus: "ON_SHELF"},
		},
	}

	service := NewSDSRetirementService(repo, nil, shein)
	_, err := service.CreateSDSRetirementRun(WithTenantID(context.Background(), "18"), &CreateSDSRetirementRunRequest{
		Platform:         "shein",
		StoreID:          177,
		ParentProductID:  238915,
		PrototypeGroupID: 28345,
		VariantID:        238916,
	})
	if err == nil || !strings.Contains(err.Error(), "cannot guarantee refreshed SHEIN preview data") {
		t.Fatalf("error = %v, want async refresh safety error", err)
	}
	if repo.createdRun != nil {
		t.Fatalf("created run = %#v, want none", repo.createdRun)
	}
}

type sdsRetirementServiceRepoStub struct {
	listTasks          []Task
	listTasksPages     [][]Task
	listTasksByTenant  map[string][]Task
	listErr            error
	listTaskQueries    []TaskListQuery
	listTasksTenantIDs []string
	createdRun         *SDSRetirementRunRecord
	createdItems       []SDSRetirementItemRecord
	storedRun          *SDSRetirementRunRecord
	storedItems        []SDSRetirementItemRecord
	updatedItems       []SDSRetirementItemSelectionUpdate
}

func (s *sdsRetirementServiceRepoStub) CreateTask(context.Context, *Task) error { return nil }
func (s *sdsRetirementServiceRepoStub) GetTask(context.Context, string) (*Task, error) {
	return nil, nil
}
func (s *sdsRetirementServiceRepoStub) ListTasks(ctx context.Context, query *TaskListQuery) ([]Task, int64, error) {
	if query != nil {
		s.listTaskQueries = append(s.listTaskQueries, *query)
	}
	s.listTasksTenantIDs = append(s.listTasksTenantIDs, TenantIDFromContext(ctx))
	if s.listErr != nil {
		return nil, 0, s.listErr
	}
	if len(s.listTasksPages) > 0 {
		page := 1
		if query != nil && query.Page > 0 {
			page = query.Page
		}
		if page > len(s.listTasksPages) {
			return []Task{}, int64(s.totalListTasks()), nil
		}
		return append([]Task(nil), s.listTasksPages[page-1]...), int64(s.totalListTasks()), nil
	}
	if len(s.listTasksByTenant) > 0 {
		tasks := s.listTasksByTenant[TenantIDFromContext(ctx)]
		return append([]Task(nil), tasks...), int64(len(tasks)), nil
	}
	return append([]Task(nil), s.listTasks...), int64(len(s.listTasks)), s.listErr
}
func (s *sdsRetirementServiceRepoStub) MarkProcessing(context.Context, string) error { return nil }
func (s *sdsRetirementServiceRepoStub) MarkCompleted(context.Context, string, *ListingKitResult) error {
	return nil
}
func (s *sdsRetirementServiceRepoStub) MarkNeedsReview(context.Context, string, *ListingKitResult, string) error {
	return nil
}
func (s *sdsRetirementServiceRepoStub) MarkFailed(context.Context, string, string) error { return nil }
func (s *sdsRetirementServiceRepoStub) MarkBlockedRetryable(context.Context, string, *RetryableBlock, string) error {
	return nil
}
func (s *sdsRetirementServiceRepoStub) ListRecoverableTasks(context.Context, *RecoverableTaskQuery) ([]Task, error) {
	return nil, nil
}
func (s *sdsRetirementServiceRepoStub) RecoverBlockedTaskNow(context.Context, string, time.Time) error {
	return nil
}
func (s *sdsRetirementServiceRepoStub) BulkRecoverBlockedTasks(context.Context, *RecoverBlockedTasksQuery) (int64, error) {
	return 0, nil
}
func (s *sdsRetirementServiceRepoStub) PrepareRetry(context.Context, string) error { return nil }
func (s *sdsRetirementServiceRepoStub) IncrementRetryCount(context.Context, string) error {
	return nil
}
func (s *sdsRetirementServiceRepoStub) SaveTaskResult(context.Context, string, *ListingKitResult) error {
	return nil
}
func (s *sdsRetirementServiceRepoStub) CreateSDSRetirementRun(_ context.Context, run *SDSRetirementRunRecord, items []SDSRetirementItemRecord) error {
	clonedRun := *run
	s.createdRun = &clonedRun
	s.createdItems = append([]SDSRetirementItemRecord(nil), items...)
	s.storedRun = &clonedRun
	s.storedItems = append([]SDSRetirementItemRecord(nil), items...)
	for i := range s.storedItems {
		s.storedItems[i].RunID = clonedRun.ID
	}
	return nil
}
func (s *sdsRetirementServiceRepoStub) GetSDSRetirementRun(context.Context, string) (*SDSRetirementRunRecord, []SDSRetirementItemRecord, error) {
	if s.storedRun == nil {
		return nil, nil, nil
	}
	clonedRun := *s.storedRun
	return &clonedRun, append([]SDSRetirementItemRecord(nil), s.storedItems...), nil
}
func (s *sdsRetirementServiceRepoStub) UpdateSDSRetirementItems(_ context.Context, _ string, updates []SDSRetirementItemSelectionUpdate) error {
	s.updatedItems = append([]SDSRetirementItemSelectionUpdate(nil), updates...)
	for i := range s.storedItems {
		for _, update := range updates {
			if s.storedItems[i].ID == update.ItemID {
				s.storedItems[i].Selected = update.Selected
				s.storedItems[i].SiteSelection = update.SiteSelection
			}
		}
	}
	return nil
}
func (s *sdsRetirementServiceRepoStub) SaveSDSRetirementExecution(context.Context, *SDSRetirementRunRecord, []SDSRetirementItemRecord) error {
	return nil
}
func (s *sdsRetirementServiceRepoStub) MarkSyncedProductOffShelf(context.Context, int64, time.Time) error {
	return nil
}

func (s *sdsRetirementServiceRepoStub) totalListTasks() int {
	total := 0
	for _, page := range s.listTasksPages {
		total += len(page)
	}
	return total
}

type sdsRetirementBaselineStub struct {
	readiness *SDSBaselineReadiness
}

func (s *sdsRetirementBaselineStub) CreateGenerateTask(context.Context, *GenerateRequest) (*Task, error) {
	return nil, nil
}
func (s *sdsRetirementBaselineStub) ListTasks(context.Context, *TaskListQuery) (*TaskListPage, error) {
	return nil, nil
}
func (s *sdsRetirementBaselineStub) GetSDSBaselineReadiness(context.Context, *SDSBaselineReadinessQuery) (*SDSBaselineReadiness, error) {
	return s.readiness, nil
}
func (s *sdsRetirementBaselineStub) GetTaskResult(context.Context, string) (*TaskResult, error) {
	return nil, nil
}
func (s *sdsRetirementBaselineStub) GetTaskPreview(context.Context, string, string) (*ListingKitPreview, error) {
	return nil, nil
}
func (s *sdsRetirementBaselineStub) GetTaskRevisionHistory(context.Context, string, *RevisionHistoryQuery) (*ListingKitRevisionHistoryPage, error) {
	return nil, nil
}
func (s *sdsRetirementBaselineStub) GetTaskRevisionHistoryDetail(context.Context, string, string, *RevisionHistoryDetailQuery) (*ListingKitRevisionHistoryDetail, error) {
	return nil, nil
}
func (s *sdsRetirementBaselineStub) GetTaskExport(context.Context, string, string) (*ListingKitExport, error) {
	return nil, nil
}
func (s *sdsRetirementBaselineStub) ApplyTaskRevision(context.Context, string, *ApplyRevisionRequest) (*ListingKitPreview, error) {
	return nil, nil
}
func (s *sdsRetirementBaselineStub) ValidateTaskRevision(context.Context, string, *ApplyRevisionRequest) (*RevisionValidationResult, error) {
	return nil, nil
}
func (s *sdsRetirementBaselineStub) SubmitTask(context.Context, string, *SubmitTaskRequest) (*ListingKitPreview, error) {
	return nil, nil
}
func (s *sdsRetirementBaselineStub) RefreshSubmissionStatus(context.Context, string) (*ListingKitPreview, error) {
	return nil, nil
}

type sdsRetirementSheinSyncStub struct {
	syncTenantID             int64
	syncStoreID              int64
	syncTrigger              SheinSyncTriggerMode
	supportsImmediateRefresh bool
	products                 []SheinSyncedProductRecord
	productPages             [][]SheinSyncedProductRecord
	listQueries              []SheinSyncedProductQuery
}

func (s *sdsRetirementSheinSyncStub) SyncSheinOnShelfProducts(_ context.Context, tenantID, storeID int64, triggerMode SheinSyncTriggerMode) (*SheinSyncJobRecord, error) {
	s.syncTenantID = tenantID
	s.syncStoreID = storeID
	s.syncTrigger = triggerMode
	return &SheinSyncJobRecord{}, nil
}
func (s *sdsRetirementSheinSyncStub) ListSyncedProducts(_ context.Context, query *SheinSyncedProductQuery) ([]SheinSyncedProductRecord, int64, error) {
	if query != nil {
		s.listQueries = append(s.listQueries, *query)
	}
	if len(s.productPages) > 0 {
		page := 1
		if query != nil && query.Page > 0 {
			page = query.Page
		}
		if page > len(s.productPages) {
			return []SheinSyncedProductRecord{}, int64(s.totalProducts()), nil
		}
		return append([]SheinSyncedProductRecord(nil), s.productPages[page-1]...), int64(s.totalProducts()), nil
	}
	return append([]SheinSyncedProductRecord(nil), s.products...), int64(len(s.products)), nil
}
func (s *sdsRetirementSheinSyncStub) UpdateManualCostPrice(context.Context, int64, *float64) error {
	return nil
}

func (s *sdsRetirementSheinSyncStub) SupportsImmediateRefresh() bool {
	return s.supportsImmediateRefresh
}

func (s *sdsRetirementSheinSyncStub) totalProducts() int {
	total := 0
	for _, page := range s.productPages {
		total += len(page)
	}
	return total
}
