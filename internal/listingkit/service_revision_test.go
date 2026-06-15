package listingkit

import (
	"context"
	"strings"
	"testing"
	"time"

	"task-processor/internal/asset"
	"task-processor/internal/catalog/canonical"
	openaiclient "task-processor/internal/infra/clients/openai"
	common "task-processor/internal/publishing/common"
	sheinpub "task-processor/internal/publishing/shein"
	sheinattribute "task-processor/internal/shein/api/attribute"
	sheinclient "task-processor/internal/shein/client"
)

type stubApplyRevisionRepo struct {
	task           *Task
	saveCalls      int
	failOnSaveCall int
}

type stubRevisionSheinAttributeResolver struct{}

func (stubRevisionSheinAttributeResolver) Resolve(req *sheinpub.BuildRequest, canonical *canonical.Product, pkg *sheinpub.Package) *sheinpub.AttributeResolution {
	return &sheinpub.AttributeResolution{
		Status:        "resolved",
		ResolvedCount: 1,
		ResolvedAttributes: []sheinpub.ResolvedAttribute{{
			Name:        "Capacity",
			Value:       "420ml",
			AttributeID: 7001,
		}},
	}
}

type stubRevisionSheinSaleResolver struct{}

func (stubRevisionSheinSaleResolver) Resolve(req *sheinpub.BuildRequest, canonical *canonical.Product, pkg *sheinpub.Package) *sheinpub.SaleAttributeResolution {
	valueID := 2493
	return &sheinpub.SaleAttributeResolution{
		Status:                  "resolved",
		PrimaryAttributeID:      27,
		PrimarySourceDimension:  "颜色",
		RecommendCategoryReview: false,
		CategoryReviewReason:    "",
		SelectionSummary:        []string{"主销售属性使用源维度 颜色 映射到 Color"},
		SKCAttributes: []sheinpub.ResolvedSaleAttribute{{
			Scope:            "skc",
			Name:             "Color",
			Value:            "Black",
			AttributeID:      27,
			AttributeValueID: &valueID,
			MatchedBy:        "test",
		}},
	}
}

type stubRevisionSheinSaleReviewResolver struct{}

func (stubRevisionSheinSaleReviewResolver) Resolve(req *sheinpub.BuildRequest, canonical *canonical.Product, pkg *sheinpub.Package) *sheinpub.SaleAttributeResolution {
	valueID := 2493
	return &sheinpub.SaleAttributeResolution{
		Status:                  "resolved",
		PrimaryAttributeID:      27,
		RecommendCategoryReview: true,
		CategoryReviewReason:    "resolver still recommends category review",
		SKCAttributes: []sheinpub.ResolvedSaleAttribute{{
			Scope:            "skc",
			Name:             "Color",
			Value:            "Black",
			AttributeID:      27,
			AttributeValueID: &valueID,
			MatchedBy:        "test",
		}},
	}
}

type stubRevisionSheinSaleMissingValueResolver struct{}

func (stubRevisionSheinSaleMissingValueResolver) Resolve(req *sheinpub.BuildRequest, canonical *canonical.Product, pkg *sheinpub.Package) *sheinpub.SaleAttributeResolution {
	return &sheinpub.SaleAttributeResolution{
		Status:                  "resolved",
		Source:                  "sale_attribute_templates",
		PrimaryAttributeID:      1001466,
		PrimarySourceDimension:  "Color",
		RecommendCategoryReview: false,
		SelectionSummary: []string{
			"主销售属性使用源维度 Color 映射到 Plug(Voltage)",
		},
		SKCAttributes: []sheinpub.ResolvedSaleAttribute{{
			Scope:       "skc",
			Name:        "Plug(Voltage)",
			Value:       "white",
			AttributeID: 1001466,
			MatchedBy:   "test",
		}},
	}
}

type stubRevisionSheinContextAwareSaleResolver struct {
	tenantID string
	userID   string
}

type stubRevisionCookieProvider struct {
	result *sheinclient.CookieLookupResult
	err    error
}

func (p stubRevisionCookieProvider) GetCookie(context.Context, int64) (*sheinclient.CookieLookupResult, error) {
	return p.result, p.err
}

func (r *stubRevisionSheinContextAwareSaleResolver) Resolve(req *sheinpub.BuildRequest, canonical *canonical.Product, pkg *sheinpub.Package) *sheinpub.SaleAttributeResolution {
	if req != nil {
		identity := openaiclient.IdentityFromContext(req.Context)
		r.tenantID = identity.TenantID
		r.userID = identity.UserID
	}
	valueID := 2493
	return &sheinpub.SaleAttributeResolution{
		Status:                  "resolved",
		PrimaryAttributeID:      27,
		PrimarySourceDimension:  "Color",
		RecommendCategoryReview: false,
		SKCAttributes: []sheinpub.ResolvedSaleAttribute{{
			Scope:            "skc",
			Name:             "Color",
			Value:            "Black",
			AttributeID:      27,
			AttributeValueID: &valueID,
			MatchedBy:        "test",
		}},
	}
}

func (r *stubApplyRevisionRepo) CreateTask(ctx context.Context, task *Task) error {
	r.task = task
	return nil
}
func (r *stubApplyRevisionRepo) GetTask(ctx context.Context, taskID string) (*Task, error) {
	return r.task, nil
}
func (r *stubApplyRevisionRepo) ListTasks(ctx context.Context, query *TaskListQuery) ([]Task, int64, error) {
	if r.task == nil {
		return []Task{}, 0, nil
	}
	return []Task{*r.task}, 1, nil
}
func (r *stubApplyRevisionRepo) MarkProcessing(ctx context.Context, taskID string) error { return nil }
func (r *stubApplyRevisionRepo) MarkCompleted(ctx context.Context, taskID string, result *ListingKitResult) error {
	return nil
}
func (r *stubApplyRevisionRepo) MarkNeedsReview(ctx context.Context, taskID string, result *ListingKitResult, reason string) error {
	return nil
}
func (r *stubApplyRevisionRepo) MarkFailed(ctx context.Context, taskID string, errorMsg string) error {
	return nil
}
func (r *stubApplyRevisionRepo) MarkBlockedRetryable(ctx context.Context, taskID string, block *RetryableBlock, errorMsg string) error {
	if r.task == nil || r.task.ID != taskID {
		return ErrTaskNotFound
	}
	r.task.Status = TaskStatusBlockedRetryable
	r.task.RetryableBlock = block
	r.task.Error = errorMsg
	r.task.UpdatedAt = time.Now()
	return nil
}
func (r *stubApplyRevisionRepo) ListRecoverableTasks(context.Context, *RecoverableTaskQuery) ([]Task, error) {
	return []Task{}, nil
}
func (r *stubApplyRevisionRepo) RecoverBlockedTaskNow(ctx context.Context, taskID string, recoveredAt time.Time) error {
	if r.task == nil || r.task.ID != taskID {
		return ErrTaskNotFound
	}
	r.task.Status = TaskStatusPending
	r.task.RetryableBlock = nil
	r.task.Error = ""
	r.task.UpdatedAt = recoveredAt
	return nil
}
func (r *stubApplyRevisionRepo) BulkRecoverBlockedTasks(context.Context, *RecoverBlockedTasksQuery) (int64, error) {
	return 0, nil
}
func (r *stubApplyRevisionRepo) PrepareRetry(ctx context.Context, taskID string) error { return nil }
func (r *stubApplyRevisionRepo) IncrementRetryCount(ctx context.Context, taskID string) error {
	return nil
}
func (r *stubApplyRevisionRepo) SaveTaskResult(ctx context.Context, taskID string, result *ListingKitResult) error {
	r.saveCalls++
	if r.failOnSaveCall > 0 && r.saveCalls == r.failOnSaveCall {
		return context.DeadlineExceeded
	}
	r.task.Result = result
	return nil
}

func TestApplyTaskRevisionReturnsAppliedChanges(t *testing.T) {
	t.Parallel()

	repo := &stubApplyRevisionRepo{}
	task := &Task{
		ID:     "task-apply-1",
		Status: TaskStatusCompleted,
		Result: &ListingKitResult{
			TaskID: "task-apply-1",
			AssetBundle: &asset.Bundle{
				Assets: []asset.Asset{{
					ID:   "asset-main",
					Kind: asset.KindMainImage,
					URL:  "https://cdn.example.com/old.jpg",
					Metadata: map[string]string{
						"prompt_key":            "productimage.scene.bags",
						"scene_defaults_source": "explicit",
						"scene_category":        "bags",
						"scene_style":           "studio",
					},
				}},
			},
			Shein: &SheinPackage{
				SpuName:       "Old Bottle",
				ProductNameEn: "Old Bottle",
				BrandName:     "Old Brand",
				Description:   "old desc",
				Images: &PlatformImageSet{
					MainImage: "https://cdn.example.com/old.jpg",
				},
				RequestDraft: &SheinRequestDraft{
					SKCList: []SheinSKCRequestDraft{{SupplierCode: "SKC-1"}},
				},
				ImageBundle: &common.PublishImageBundle{
					Platform: "shein",
					Main: &common.BundleSlot{
						AssetID: "asset-main",
					},
				},
			},
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	_ = repo.CreateTask(context.Background(), task)
	svc := &service{repo: repo}

	newName := "New Bottle"
	preview, err := svc.ApplyTaskRevision(context.Background(), task.ID, &ApplyRevisionRequest{
		Platform: "shein",
		Shein: &SheinRevisionInput{
			SpuName:       &newName,
			ProductNameEn: &newName,
		},
	})
	if err != nil {
		t.Fatalf("apply task revision: %v", err)
	}
	if preview == nil || preview.AppliedChanges == nil || preview.AppliedChanges.ChangeCount == 0 {
		t.Fatalf("applied changes = %+v", preview)
	}
	if preview.ApplyResult == nil || preview.ApplyResult.SuccessPayload == nil || preview.ApplyResult.SuccessPayload.Presentation == nil || preview.ApplyResult.SuccessPayload.Presentation.Scene != revisionPresentationSceneApplySuccess || preview.ApplyResult.SuccessPayload.Presentation.SummaryCard == nil || preview.ApplyResult.SuccessPayload.Presentation.SummaryCard.Title == "" {
		t.Fatalf("apply result = %+v", preview.ApplyResult)
	}
	if preview.Shein == nil || len(preview.Shein.ScenePresets) != 1 {
		t.Fatalf("shein scene presets = %+v", preview.Shein)
	}
	if preview.Shein.ScenePresets[0].ScenePreset == nil || preview.Shein.ScenePresets[0].ScenePreset.PromptKey != "productimage.scene.bags" {
		t.Fatalf("shein scene presets = %+v", preview.Shein.ScenePresets)
	}
	if len(preview.ApplyResult.SuccessPayload.Presentation.NextActions) == 0 {
		t.Fatalf("apply result = %+v", preview.ApplyResult)
	}
	if preview.ApplyResult.SuccessPayload.Core == nil || preview.ApplyResult.SuccessPayload.Core.StatusSummary == nil {
		t.Fatalf("apply result = %+v", preview.ApplyResult)
	}
	if preview.ApplyResult.SuccessPayload.Presentation.Messages == nil || preview.ApplyResult.SuccessPayload.Presentation.Messages.SuccessLabel == "" {
		t.Fatalf("apply result = %+v", preview.ApplyResult)
	}
	if preview.ApplyResult.SuccessPayload.Presentation.RecommendedView == nil || preview.ApplyResult.SuccessPayload.Presentation.RecommendedView.View == "" {
		t.Fatalf("apply result = %+v", preview.ApplyResult)
	}
	if preview.ApplyResult.SuccessPayload.Core.FollowUpChecklist == nil {
		t.Fatalf("apply result = %+v", preview.ApplyResult)
	}
	if preview.ApplyResult.SuccessPayload.Core.FollowUpOverview == nil || preview.ApplyResult.SuccessPayload.Core.FollowUpOverview.Headline == "" {
		t.Fatalf("apply result = %+v", preview.ApplyResult)
	}
	if preview.ApplyResult.SuccessPayload.Core.SuggestedFollowUpRevision == nil || preview.ApplyResult.SuccessPayload.Core.SuggestedFollowUpRevision.Platform != "shein" {
		t.Fatalf("apply result = %+v", preview.ApplyResult)
	}
	if preview.ApplyResult.SuccessPayload == nil || preview.ApplyResult.SuccessPayload.Mode != "apply" {
		t.Fatalf("apply result = %+v", preview.ApplyResult)
	}
	if len(preview.RevisionHistory) != 1 || preview.RevisionHistory[0].AppliedChanges == nil {
		t.Fatalf("revision history = %+v", preview.RevisionHistory)
	}
	if preview.RevisionHistory[0].RevisionID == "" {
		t.Fatalf("revision history record missing revision_id: %+v", preview.RevisionHistory[0])
	}
	if preview.RevisionHistory[0].ActionType != RevisionActionTypeEdit {
		t.Fatalf("revision history action type = %+v", preview.RevisionHistory[0])
	}
	if preview.RevisionHistory[0].Timeline == nil || preview.RevisionHistory[0].Timeline.Badge != "编辑" {
		t.Fatalf("revision history timeline = %+v", preview.RevisionHistory[0].Timeline)
	}
	if preview.RevisionHistory[0].EditorContext == nil || preview.RevisionHistory[0].EditorContext.Basics == nil {
		t.Fatalf("revision history snapshot = %+v", preview.RevisionHistory[0].EditorContext)
	}
	if preview.RevisionHistoryMeta == nil || preview.RevisionHistoryMeta.TotalRecords != 1 {
		t.Fatalf("revision history meta = %+v", preview.RevisionHistoryMeta)
	}
}

func TestApplyTaskRevisionTrimsRevisionHistory(t *testing.T) {
	t.Parallel()

	repo := &stubApplyRevisionRepo{}
	history := make([]ListingKitRevisionRecord, 0, maxRevisionHistoryRecords)
	for i := 0; i < maxRevisionHistoryRecords; i++ {
		history = append(history, ListingKitRevisionRecord{
			UpdatedAt: time.Now().Add(time.Duration(i) * time.Minute),
			UpdatedBy: "tester",
			Reason:    "seed",
			Platform:  "shein",
		})
	}
	task := &Task{
		ID:     "task-apply-2",
		Status: TaskStatusCompleted,
		Result: &ListingKitResult{
			TaskID:               "task-apply-2",
			RevisionHistoryTotal: maxRevisionHistoryRecords,
			RevisionHistory:      history,
			Shein: &SheinPackage{
				SpuName:       "Old Bottle",
				ProductNameEn: "Old Bottle",
				RequestDraft: &SheinRequestDraft{
					SKCList: []SheinSKCRequestDraft{{SupplierCode: "SKC-1"}},
				},
			},
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	_ = repo.CreateTask(context.Background(), task)
	svc := &service{repo: repo}

	newName := "Trimmed Bottle"
	preview, err := svc.ApplyTaskRevision(context.Background(), task.ID, &ApplyRevisionRequest{
		Platform: "shein",
		Shein: &SheinRevisionInput{
			SpuName: &newName,
		},
	})
	if err != nil {
		t.Fatalf("apply task revision: %v", err)
	}
	if len(preview.RevisionHistory) != maxRevisionHistoryRecords {
		t.Fatalf("revision history length = %d, want %d", len(preview.RevisionHistory), maxRevisionHistoryRecords)
	}
	if preview.RevisionHistoryMeta == nil {
		t.Fatal("expected revision history meta")
	}
	if preview.RevisionHistoryMeta.TotalRecords != maxRevisionHistoryRecords+1 {
		t.Fatalf("total records = %d, want %d", preview.RevisionHistoryMeta.TotalRecords, maxRevisionHistoryRecords+1)
	}
	if !preview.RevisionHistoryMeta.HasMore {
		t.Fatalf("history meta = %+v, want has_more", preview.RevisionHistoryMeta)
	}
	last := preview.RevisionHistory[len(preview.RevisionHistory)-1]
	if last.RevisionID == "" || last.AppliedChanges == nil || last.EditorContext == nil {
		t.Fatalf("latest history record = %+v", last)
	}
}

func TestApplyTaskRevisionRefreshesSheinDerivedStateAfterCategoryChange(t *testing.T) {
	t.Parallel()

	repo := &stubApplyRevisionRepo{}
	task := &Task{
		ID: "task-apply-shein-category-refresh",
		Request: &GenerateRequest{
			Platforms:    []string{"shein"},
			Country:      "US",
			Language:     "en_US",
			SheinStoreID: 869,
			Text:         "420ml stainless steel tumbler",
		},
		Status: TaskStatusNeedsReview,
		Result: &ListingKitResult{
			TaskID: "task-apply-shein-category-refresh",
			CanonicalProduct: &canonical.Product{
				Title: "420ml stainless steel tumbler",
				Attributes: map[string]canonical.Attribute{
					"颜色": {Value: "黑色"},
				},
				Variants: []canonical.Variant{{
					SKU: "SKU-1",
					Attributes: map[string]canonical.Attribute{
						"颜色": {Value: "黑色"},
					},
				}},
			},
			Shein: &SheinPackage{
				CategoryID:   12143,
				CategoryPath: []string{"家居&生活", "家庭用品", "鞋用品", "鞋配饰"},
				CategoryResolution: &SheinCategoryResolution{
					Status:     "resolved",
					CategoryID: 12143,
					MatchedPath: []string{
						"家居&生活", "家庭用品", "鞋用品", "鞋配饰",
					},
				},
				ProductAttributes: []PlatformAttribute{
					{Name: "旧属性", Value: "stale"},
				},
				SaleAttributeResolution: &SheinSaleAttributeResolution{
					Status:                  "partial",
					RecommendCategoryReview: true,
					CategoryReviewReason:    "当前类目路径与商品语义明显不一致，建议优先人工复核 SHEIN 类目是否正确",
				},
				RequestDraft: &SheinRequestDraft{
					SKCList: []SheinSKCRequestDraft{{SupplierCode: "SKC-1"}},
				},
			},
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	_ = repo.CreateTask(context.Background(), task)
	svc := &service{
		repo: repo,
		sheinRuntimeDeps: sheinRuntimeDependencies{
			attributeResolver:     stubRevisionSheinAttributeResolver{},
			saleAttributeResolver: stubRevisionSheinSaleResolver{},
		},
	}

	categoryID := 3221
	productTypeID := 2163
	topCategoryID := 31
	_, err := svc.ApplyTaskRevision(context.Background(), task.ID, &ApplyRevisionRequest{
		Platform: "shein",
		Shein: &SheinRevisionInput{
			CategoryResolution: &SheinCategoryResolutionPatch{
				Status:         stringPtr("resolved"),
				Source:         stringPtr("manual_revision"),
				MatchedPath:    []string{"家居&生活", "厨房&餐厅", "饮具", "真空瓶和保温杯"},
				CategoryID:     &categoryID,
				CategoryIDList: []int{31, 3188, 3219, 3221},
				ProductTypeID:  &productTypeID,
				TopCategoryID:  &topCategoryID,
			},
			SaleAttributeResolution: &SheinSaleAttributeResolutionPatch{
				RecommendCategoryReview: boolPtr(false),
				CategoryReviewReason:    stringPtr("stale"),
			},
		},
	})
	if err != nil {
		t.Fatalf("apply task revision: %v", err)
	}
	if repo.task.Result.Shein.SaleAttributeResolution == nil {
		t.Fatal("expected refreshed sale attribute resolution")
	}
	if repo.task.Result.Shein.SaleAttributeResolution.RecommendCategoryReview {
		t.Fatalf("sale attribute resolution = %+v, want recommend_category_review false", repo.task.Result.Shein.SaleAttributeResolution)
	}
	if repo.task.Result.Shein.SaleAttributeResolution.CategoryReviewReason != "" {
		t.Fatalf("sale attribute reason = %q, want empty", repo.task.Result.Shein.SaleAttributeResolution.CategoryReviewReason)
	}
	if repo.task.Result.Shein.SaleAttributeResolution.PrimaryAttributeID != 27 {
		t.Fatalf("primary attribute id = %d, want 27", repo.task.Result.Shein.SaleAttributeResolution.PrimaryAttributeID)
	}
	if repo.task.Result.Shein.AttributeResolution == nil || repo.task.Result.Shein.AttributeResolution.ResolvedCount != 1 {
		t.Fatalf("attribute resolution = %+v", repo.task.Result.Shein.AttributeResolution)
	}
	if len(repo.task.Result.Shein.ProductAttributes) != 1 || repo.task.Result.Shein.ProductAttributes[0].Name != "颜色" || repo.task.Result.Shein.ProductAttributes[0].Value != "黑色" {
		t.Fatalf("product attributes = %+v, want rebuilt canonical attributes", repo.task.Result.Shein.ProductAttributes)
	}
	if repo.task.Result.Shein.RequestDraft == nil || len(repo.task.Result.Shein.RequestDraft.SKCList) != 1 {
		t.Fatalf("request draft = %+v", repo.task.Result.Shein.RequestDraft)
	}
	if len(repo.task.Result.Shein.RequestDraft.ProductAttributeList) != 1 || repo.task.Result.Shein.RequestDraft.ProductAttributeList[0].Name != "颜色" || repo.task.Result.Shein.RequestDraft.ProductAttributeList[0].Value != "黑色" {
		t.Fatalf("request draft product attributes = %+v, want rebuilt canonical attributes", repo.task.Result.Shein.RequestDraft.ProductAttributeList)
	}
	if repo.task.Result.Shein.RequestDraft.SKCList[0].SaleAttribute == nil || repo.task.Result.Shein.RequestDraft.SKCList[0].SaleAttribute.AttributeID != 27 {
		t.Fatalf("request draft skc sale attribute = %+v", repo.task.Result.Shein.RequestDraft.SKCList[0].SaleAttribute)
	}
}

func TestApplyTaskRevisionKeepsManualCategoryReviewConfirmationAfterRefresh(t *testing.T) {
	t.Parallel()

	repo := &stubApplyRevisionRepo{}
	categoryID := 2645
	productTypeID := 539
	topCategoryID := 2374
	task := &Task{
		ID: "task-apply-shein-category-confirm",
		Request: &GenerateRequest{
			Platforms:    []string{"shein"},
			Country:      "US",
			Language:     "en_US",
			SheinStoreID: 869,
			Text:         "pet bandana",
		},
		Status: TaskStatusCompleted,
		Result: &ListingKitResult{
			TaskID: "task-apply-shein-category-confirm",
			CanonicalProduct: &canonical.Product{
				Title: "pet bandana",
				Variants: []canonical.Variant{{
					SKU: "SKU-1",
					Attributes: map[string]canonical.Attribute{
						"Color": {Value: "Black"},
					},
				}},
			},
			Shein: &SheinPackage{
				CategoryID:     categoryID,
				CategoryIDList: []int{2374, 2638, 2645},
				CategoryPath:   []string{"宠物用品", "宠物配饰", "宠物围巾"},
				ProductTypeID:  &productTypeID,
				TopCategoryID:  topCategoryID,
				CategoryResolution: &SheinCategoryResolution{
					Status:         "resolved",
					Source:         "ai_category_tree",
					CategoryID:     categoryID,
					CategoryIDList: []int{2374, 2638, 2645},
					MatchedPath:    []string{"宠物用品", "宠物配饰", "宠物围巾"},
					ProductTypeID:  productTypeID,
					TopCategoryID:  topCategoryID,
				},
				SaleAttributeResolution: &SheinSaleAttributeResolution{
					Status:                  "resolved",
					RecommendCategoryReview: true,
					CategoryReviewReason:    "当前类目路径与商品语义明显不一致，建议优先人工复核 SHEIN 类目是否正确",
				},
				RequestDraft: &SheinRequestDraft{
					SKCList: []SheinSKCRequestDraft{{SupplierCode: "SKC-1"}},
				},
			},
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	_ = repo.CreateTask(context.Background(), task)
	svc := &service{
		repo: repo,
		sheinRuntimeDeps: sheinRuntimeDependencies{
			attributeResolver:     stubRevisionSheinAttributeResolver{},
			saleAttributeResolver: stubRevisionSheinSaleReviewResolver{},
		},
	}

	preview, err := svc.ApplyTaskRevision(context.Background(), task.ID, &ApplyRevisionRequest{
		Platform: "shein",
		Actor:    "workspace",
		Reason:   "Confirm current SHEIN category",
		Shein: &SheinRevisionInput{
			CategoryResolution: &SheinCategoryResolutionPatch{
				Status:         stringPtr("resolved"),
				Source:         stringPtr("ai_category_tree"),
				MatchedPath:    []string{"宠物用品", "宠物配饰", "宠物围巾"},
				CategoryID:     &categoryID,
				CategoryIDList: []int{2374, 2638, 2645},
				ProductTypeID:  &productTypeID,
				TopCategoryID:  &topCategoryID,
			},
			SaleAttributeResolution: &SheinSaleAttributeResolutionPatch{
				RecommendCategoryReview: boolPtr(false),
				CategoryReviewReason:    stringPtr(""),
			},
		},
	})
	if err != nil {
		t.Fatalf("apply task revision: %v", err)
	}
	if preview == nil || preview.AppliedChanges == nil || preview.AppliedChanges.ChangeCount == 0 {
		t.Fatalf("applied changes = %+v", preview)
	}
	if repo.task.Result.Shein.SaleAttributeResolution == nil {
		t.Fatal("expected sale attribute resolution")
	}
	if repo.task.Result.Shein.SaleAttributeResolution.RecommendCategoryReview {
		t.Fatalf("sale attribute resolution = %+v, want confirmed category review", repo.task.Result.Shein.SaleAttributeResolution)
	}
	if repo.task.Result.Shein.SaleAttributeResolution.CategoryReviewReason != "" {
		t.Fatalf("sale attribute reason = %q, want empty", repo.task.Result.Shein.SaleAttributeResolution.CategoryReviewReason)
	}
	if repo.task.Result.Shein.CategoryResolution != nil && repo.task.Result.Shein.CategoryResolution.SuggestedCategory != nil {
		t.Fatalf("suggested category = %+v, want nil after manual confirmation", repo.task.Result.Shein.CategoryResolution.SuggestedCategory)
	}
}

func TestApplyTaskRevisionDowngradesSaleAttributesWhenCategoryRefreshLacksValueIDs(t *testing.T) {
	t.Parallel()

	repo := &stubApplyRevisionRepo{}
	task := &Task{
		ID: "task-apply-shein-category-rerun-sale-attributes",
		Request: &GenerateRequest{
			Platforms:    []string{"shein"},
			Country:      "US",
			Language:     "en_US",
			SheinStoreID: 869,
			Text:         "metal wall sign",
		},
		Status: TaskStatusNeedsReview,
		Result: &ListingKitResult{
			TaskID: "task-apply-shein-category-rerun-sale-attributes",
			CanonicalProduct: &canonical.Product{
				Title: "metal wall sign",
				Variants: []canonical.Variant{{
					SKU: "SKU-1",
					Attributes: map[string]canonical.Attribute{
						"Color": {Value: "white"},
					},
				}},
			},
			Shein: &SheinPackage{
				CategoryID:   3894,
				CategoryPath: []string{"家居&生活", "家居装饰", "家居装饰摆件&配件", "铁皮画标牌"},
				CategoryResolution: &SheinCategoryResolution{
					Status:     "resolved",
					CategoryID: 3894,
				},
				SaleAttributeResolution: &SheinSaleAttributeResolution{
					Status:             "resolved",
					PrimaryAttributeID: 27,
				},
				RequestDraft: &SheinRequestDraft{
					SKCList: []SheinSKCRequestDraft{{SupplierCode: "SKC-1"}},
				},
			},
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	_ = repo.CreateTask(context.Background(), task)
	svc := &service{
		repo: repo,
		sheinRuntimeDeps: sheinRuntimeDependencies{
			attributeResolver:     stubRevisionSheinAttributeResolver{},
			saleAttributeResolver: stubRevisionSheinSaleMissingValueResolver{},
		},
	}

	categoryID := 2486
	productTypeID := 999
	topCategoryID := 31
	_, err := svc.ApplyTaskRevision(context.Background(), task.ID, &ApplyRevisionRequest{
		Platform: "shein",
		Shein: &SheinRevisionInput{
			CategoryResolution: &SheinCategoryResolutionPatch{
				Status:         stringPtr("resolved"),
				Source:         stringPtr("manual_revision"),
				MatchedPath:    []string{"家居&生活", "家居装饰", "装饰挂饰和风铃", "装饰挂饰"},
				CategoryID:     &categoryID,
				CategoryIDList: []int{31, 2478, 2484, 2486},
				ProductTypeID:  &productTypeID,
				TopCategoryID:  &topCategoryID,
			},
			SaleAttributeResolution: &SheinSaleAttributeResolutionPatch{
				RecommendCategoryReview: boolPtr(false),
				CategoryReviewReason:    stringPtr(""),
			},
		},
	})
	if err != nil {
		t.Fatalf("apply task revision: %v", err)
	}
	if repo.task.Result.Shein.SaleAttributeResolution == nil {
		t.Fatal("expected refreshed sale attribute resolution")
	}
	if repo.task.Result.Shein.SaleAttributeResolution.Status != "partial" {
		t.Fatalf("sale attribute status = %q, want partial", repo.task.Result.Shein.SaleAttributeResolution.Status)
	}
	if repo.task.Result.Shein.SaleAttributeResolution.PrimaryAttributeID != 1001466 {
		t.Fatalf("primary attribute id = %d, want 1001466", repo.task.Result.Shein.SaleAttributeResolution.PrimaryAttributeID)
	}
	if len(repo.task.Result.Shein.RequestDraft.SKCList) == 0 || repo.task.Result.Shein.RequestDraft.SKCList[0].SaleAttribute != nil {
		t.Fatalf("request draft skc sale attribute = %+v, want nil because value id is missing", repo.task.Result.Shein.RequestDraft.SKCList[0].SaleAttribute)
	}
	if repo.task.Result.Shein.PreviewProduct == nil || repo.task.Result.Shein.PreviewProduct.SKCList[0].SaleAttribute.AttributeID != 0 {
		t.Fatalf("preview skc sale attribute = %+v, want unresolved placeholder", repo.task.Result.Shein.PreviewProduct)
	}
	if len(repo.task.Result.Shein.SaleAttributeResolution.ReviewNotes) == 0 {
		t.Fatalf("sale attribute review notes = %+v, want rerun warning", repo.task.Result.Shein.SaleAttributeResolution.ReviewNotes)
	}
}

func TestApplyTaskRevisionDowngradesManualResolvedSaleAttributesWithoutValueIDs(t *testing.T) {
	t.Parallel()

	repo := &stubApplyRevisionRepo{}
	task := &Task{
		ID: "task-apply-shein-sale-confirm-without-value-ids",
		Request: &GenerateRequest{
			Platforms:    []string{"shein"},
			Country:      "US",
			Language:     "en_US",
			SheinStoreID: 869,
			Text:         "metal wall sign",
		},
		Status: TaskStatusNeedsReview,
		Result: &ListingKitResult{
			TaskID: "task-apply-shein-sale-confirm-without-value-ids",
			CanonicalProduct: &canonical.Product{
				Title: "metal wall sign",
				Variants: []canonical.Variant{{
					SKU: "SKU-1",
					Attributes: map[string]canonical.Attribute{
						"Color": {Value: "white"},
					},
				}},
			},
			Shein: &SheinPackage{
				CategoryID: 2486,
				CategoryResolution: &SheinCategoryResolution{
					Status:     "resolved",
					CategoryID: 2486,
				},
				SaleAttributeResolution: &SheinSaleAttributeResolution{
					Status:             "partial",
					PrimaryAttributeID: 1001466,
				},
				RequestDraft: &SheinRequestDraft{
					SKCList: []SheinSKCRequestDraft{{
						SupplierCode: "SKC-1",
						SKUList: []SheinSKUDraft{{
							SupplierSKU: "SKU-1",
						}},
					}},
				},
			},
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	_ = repo.CreateTask(context.Background(), task)
	svc := &service{repo: repo}

	status := "resolved"
	source := "manual_review"
	_, err := svc.ApplyTaskRevision(context.Background(), task.ID, &ApplyRevisionRequest{
		Platform: "shein",
		Shein: &SheinRevisionInput{
			SaleAttributeResolution: &SheinSaleAttributeResolutionPatch{
				Status:             &status,
				Source:             &source,
				PrimaryAttributeID: intPtr(1001466),
				SKCAttributes: []SheinResolvedSaleAttribute{{
					Scope:       "skc",
					Name:        "Plug(Voltage)",
					Value:       "white",
					AttributeID: 1001466,
				}},
				ReviewNotes: []string{"SHEIN 销售属性已按当前主规格和其他规格人工确认。"},
			},
		},
	})
	if err != nil {
		t.Fatalf("apply task revision: %v", err)
	}
	if repo.task.Result.Shein.SaleAttributeResolution == nil {
		t.Fatal("expected sale attribute resolution")
	}
	if repo.task.Result.Shein.SaleAttributeResolution.Status != "partial" {
		t.Fatalf("sale attribute status = %q, want partial", repo.task.Result.Shein.SaleAttributeResolution.Status)
	}
	if len(repo.task.Result.Shein.SaleAttributeResolution.ReviewNotes) == 0 {
		t.Fatalf("review notes = %+v, want normalization note", repo.task.Result.Shein.SaleAttributeResolution.ReviewNotes)
	}
}

func TestApplyTaskRevisionRefreshUsesTaskIdentityForSheinRuntime(t *testing.T) {
	t.Parallel()

	repo := &stubApplyRevisionRepo{}
	capturedResolver := &stubRevisionSheinContextAwareSaleResolver{}
	task := &Task{
		ID:       "task-apply-shein-runtime-context",
		TenantID: "373211199677923496",
		UserID:   "user-ctx-1",
		Request: &GenerateRequest{
			Platforms:    []string{"shein"},
			Country:      "US",
			Language:     "en_US",
			SheinStoreID: 870,
			Text:         "insulated cooler bag",
		},
		Status: TaskStatusNeedsReview,
		Result: &ListingKitResult{
			TaskID: "task-apply-shein-runtime-context",
			CanonicalProduct: &canonical.Product{
				Title: "insulated cooler bag",
				Variants: []canonical.Variant{{
					SKU: "SKU-1",
					Attributes: map[string]canonical.Attribute{
						"Color": {Value: "white"},
					},
				}},
			},
			Shein: &SheinPackage{
				CategoryID:   10489,
				CategoryPath: []string{"运动&户外", "露营&远足", "野餐和营地厨房", "户外保温包"},
				CategoryResolution: &SheinCategoryResolution{
					Status:     "resolved",
					CategoryID: 10489,
				},
				RequestDraft: &SheinRequestDraft{
					SKCList: []SheinSKCRequestDraft{{SupplierCode: "SKC-1"}},
				},
			},
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	_ = repo.CreateTask(context.Background(), task)
	svc := &service{
		repo: repo,
		sheinRuntimeDeps: sheinRuntimeDependencies{
			attributeResolver:     stubRevisionSheinAttributeResolver{},
			saleAttributeResolver: capturedResolver,
		},
	}

	categoryID := 10489
	productTypeID := 7190
	topCategoryID := 2866
	_, err := svc.ApplyTaskRevision(context.Background(), task.ID, &ApplyRevisionRequest{
		Platform: "shein",
		Shein: &SheinRevisionInput{
			CategoryResolution: &SheinCategoryResolutionPatch{
				Status:         stringPtr("resolved"),
				Source:         stringPtr("manual_search"),
				MatchedPath:    []string{"运动&户外", "露营&远足", "野餐和营地厨房", "户外保温包"},
				CategoryID:     &categoryID,
				CategoryIDList: []int{2866, 4396, 4425, 10489},
				ProductTypeID:  &productTypeID,
				TopCategoryID:  &topCategoryID,
			},
			SaleAttributeResolution: &SheinSaleAttributeResolutionPatch{
				RecommendCategoryReview: boolPtr(false),
				CategoryReviewReason:    stringPtr(""),
			},
		},
	})
	if err != nil {
		t.Fatalf("apply task revision: %v", err)
	}
	if capturedResolver.tenantID != task.TenantID {
		t.Fatalf("tenant id = %q, want %q", capturedResolver.tenantID, task.TenantID)
	}
	if capturedResolver.userID != task.UserID {
		t.Fatalf("user id = %q, want %q", capturedResolver.userID, task.UserID)
	}
}

func TestApplyTaskRevisionClearsStaleSheinCookieBlockersAfterOnlineRefresh(t *testing.T) {
	t.Parallel()

	repo := &stubApplyRevisionRepo{}
	cookieNote := "SHEIN 店铺 cookie 不可用，已降级为离线解析"
	task := &Task{
		ID:       "task-apply-shein-cookie-stale",
		TenantID: "227",
		UserID:   "user-cookie-1",
		Request: &GenerateRequest{
			Platforms:    []string{"shein"},
			Country:      "US",
			Language:     "en_US",
			SheinStoreID: 870,
			Text:         "insulated cooler bag",
		},
		Status: TaskStatusNeedsReview,
		Result: &ListingKitResult{
			TaskID: "task-apply-shein-cookie-stale",
			CanonicalProduct: &canonical.Product{
				Title: "insulated cooler bag",
				Variants: []canonical.Variant{{
					SKU: "SKU-1",
					Attributes: map[string]canonical.Attribute{
						"Color": {Value: "white"},
						"Size":  {Value: "One size"},
					},
				}},
			},
			Shein: &SheinPackage{
				CategoryID:   10489,
				CategoryPath: []string{"运动&户外", "露营&远足", "野餐和营地厨房", "户外保温包"},
				ReviewNotes:  []string{cookieNote},
				CategoryResolution: &SheinCategoryResolution{
					Status:      "resolved",
					CategoryID:  10489,
					ReviewNotes: []string{cookieNote},
				},
				AttributeResolution: &SheinAttributeResolution{
					Status:        "resolved",
					ResolvedCount: 1,
					ReviewNotes:   []string{cookieNote},
				},
				SaleAttributeResolution: &SheinSaleAttributeResolution{
					Status:             "partial",
					PrimaryAttributeID: 1001184,
					ReviewNotes:        []string{cookieNote},
				},
				RequestDraft: &SheinRequestDraft{
					SKCList: []SheinSKCRequestDraft{{SupplierCode: "SKC-1"}},
				},
			},
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	_ = repo.CreateTask(context.Background(), task)

	apiClient := sheinclient.NewAPIClientWithStoreConfig(870, &sheinclient.StoreConfig{
		ID:       870,
		TenantID: 227,
		LoginURL: "sso.geiwohuo.com",
	}, stubRevisionCookieProvider{
		result: &sheinclient.CookieLookupResult{
			TenantID:   227,
			CookieJSON: `[{"name":"sid","value":"abc","domain":".shein.com","path":"/"}]`,
		},
	})

	svc := &service{
		repo: repo,
		sheinRuntimeDeps: sheinRuntimeDependencies{
			storeCatalog:          &stubSheinStoreCatalog{storeInfo: &SheinStoreInfo{ID: 870, TenantID: 227, StoreID: "870", Platform: "shein", LoginURL: "sso.geiwohuo.com"}},
			apiClientFactory:      stubSheinAPIClientFactory{client: apiClient},
			attributeResolver:     stubRevisionSheinAttributeResolver{},
			saleAttributeResolver: stubRevisionSheinSaleResolver{},
		},
	}

	categoryID := 10489
	productTypeID := 7190
	topCategoryID := 2866
	preview, err := svc.ApplyTaskRevision(context.Background(), task.ID, &ApplyRevisionRequest{
		Platform: "shein",
		Shein: &SheinRevisionInput{
			CategoryResolution: &SheinCategoryResolutionPatch{
				Status:         stringPtr("resolved"),
				Source:         stringPtr("manual_search"),
				MatchedPath:    []string{"运动&户外", "露营&远足", "野餐和营地厨房", "户外保温包"},
				CategoryID:     &categoryID,
				CategoryIDList: []int{2866, 4396, 4425, 10489},
				ProductTypeID:  &productTypeID,
				TopCategoryID:  &topCategoryID,
			},
			SaleAttributeResolution: &SheinSaleAttributeResolutionPatch{
				RecommendCategoryReview: boolPtr(false),
				CategoryReviewReason:    stringPtr(""),
			},
		},
	})
	if err != nil {
		t.Fatalf("apply task revision: %v", err)
	}
	if preview == nil || preview.Shein == nil || preview.Shein.SubmitReadiness == nil {
		t.Fatalf("preview = %+v", preview)
	}
	for _, item := range preview.Shein.SubmitReadiness.BlockingItems {
		if item.Key == sheinCookieUnavailableIssueCode {
			t.Fatalf("blocking items = %+v, want stale shein cookie blocker removed", preview.Shein.SubmitReadiness.BlockingItems)
		}
	}
	if len(repo.task.Result.Shein.ReviewNotes) != 0 {
		t.Fatalf("shein review notes = %#v, want stale cookie notes cleared", repo.task.Result.Shein.ReviewNotes)
	}
	if len(repo.task.Result.Shein.CategoryResolution.ReviewNotes) != 0 {
		t.Fatalf("category review notes = %#v, want stale cookie notes cleared", repo.task.Result.Shein.CategoryResolution.ReviewNotes)
	}
	if len(repo.task.Result.Shein.AttributeResolution.ReviewNotes) != 0 {
		t.Fatalf("attribute review notes = %#v, want stale cookie notes cleared", repo.task.Result.Shein.AttributeResolution.ReviewNotes)
	}
	if len(repo.task.Result.Shein.SaleAttributeResolution.ReviewNotes) != 1 || strings.Contains(repo.task.Result.Shein.SaleAttributeResolution.ReviewNotes[0], "cookie 不可用") {
		t.Fatalf("sale attribute review notes = %#v, want only non-cookie follow-up note", repo.task.Result.Shein.SaleAttributeResolution.ReviewNotes)
	}
}

func TestApplyTaskRevisionDecoratesPreviewWithLiveSheinCookieBlocker(t *testing.T) {
	t.Parallel()

	repo := &stubApplyRevisionRepo{}
	task := &Task{
		ID:       "task-apply-shein-cookie-live-blocker",
		TenantID: "227",
		UserID:   "user-cookie-2",
		Request: &GenerateRequest{
			Platforms:    []string{"shein"},
			Country:      "US",
			Language:     "en_US",
			SheinStoreID: 870,
			Text:         "insulated cooler bag",
		},
		Status: TaskStatusNeedsReview,
		Result: &ListingKitResult{
			TaskID: "task-apply-shein-cookie-live-blocker",
			CanonicalProduct: &canonical.Product{
				Title: "insulated cooler bag",
				Variants: []canonical.Variant{{
					SKU: "SKU-1",
					Attributes: map[string]canonical.Attribute{
						"Color": {Value: "white"},
						"Size":  {Value: "One size"},
					},
				}},
			},
			Shein: &SheinPackage{
				CategoryID:   10489,
				CategoryPath: []string{"运动&户外", "露营&远足", "野餐和营地厨房", "户外保温包"},
				CategoryResolution: &SheinCategoryResolution{
					Status:     "resolved",
					CategoryID: 10489,
				},
				AttributeResolution: &SheinAttributeResolution{
					Status:        "resolved",
					ResolvedCount: 1,
				},
				SaleAttributeResolution: &SheinSaleAttributeResolution{
					Status:                  "resolved",
					PrimaryAttributeID:      27,
					RecommendCategoryReview: false,
				},
				RequestDraft: &SheinRequestDraft{
					SKCList: []SheinSKCRequestDraft{{SupplierCode: "SKC-1"}},
				},
			},
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	_ = repo.CreateTask(context.Background(), task)

	apiClient := sheinclient.NewAPIClientWithStoreConfig(870, &sheinclient.StoreConfig{
		ID:       870,
		TenantID: 227,
		LoginURL: "sso.geiwohuo.com",
	}, stubRevisionCookieProvider{})

	svc := &service{
		repo: repo,
		sheinRuntimeDeps: sheinRuntimeDependencies{
			storeCatalog:          &stubSheinStoreCatalog{storeInfo: &SheinStoreInfo{ID: 870, TenantID: 227, StoreID: "870", Platform: "shein", LoginURL: "sso.geiwohuo.com"}},
			apiClientFactory:      stubSheinAPIClientFactory{client: apiClient},
			attributeResolver:     stubRevisionSheinAttributeResolver{},
			saleAttributeResolver: stubRevisionSheinSaleResolver{},
		},
	}

	categoryID := 10489
	productTypeID := 7190
	topCategoryID := 2866
	preview, err := svc.ApplyTaskRevision(context.Background(), task.ID, &ApplyRevisionRequest{
		Platform: "shein",
		Shein: &SheinRevisionInput{
			CategoryResolution: &SheinCategoryResolutionPatch{
				Status:         stringPtr("resolved"),
				Source:         stringPtr("manual_search"),
				MatchedPath:    []string{"运动&户外", "露营&远足", "野餐和营地厨房", "户外保温包"},
				CategoryID:     &categoryID,
				CategoryIDList: []int{2866, 4396, 4425, 10489},
				ProductTypeID:  &productTypeID,
				TopCategoryID:  &topCategoryID,
			},
			SaleAttributeResolution: &SheinSaleAttributeResolutionPatch{
				RecommendCategoryReview: boolPtr(false),
				CategoryReviewReason:    stringPtr(""),
			},
		},
	})
	if err != nil {
		t.Fatalf("apply task revision: %v", err)
	}
	if preview == nil || preview.Shein == nil || preview.Shein.SubmitReadiness == nil {
		t.Fatalf("preview = %+v", preview)
	}
	found := false
	for _, item := range preview.Shein.SubmitReadiness.BlockingItems {
		if item.Key == sheinCookieUnavailableIssueCode {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("blocking items = %+v, want live shein cookie blocker in preview", preview.Shein.SubmitReadiness.BlockingItems)
	}
	if !strings.Contains(strings.Join(preview.Shein.ReviewNotes, "\n"), "cookie 不可用") {
		t.Fatalf("preview review notes = %#v, want live cookie note", preview.Shein.ReviewNotes)
	}
}

func TestApplyTaskRevisionRegeneratesSheinSaleAttributesWithoutCategoryConfirmation(t *testing.T) {
	t.Parallel()

	repo := &stubApplyRevisionRepo{}
	task := &Task{
		ID: "task-apply-shein-regenerate-sale-attributes",
		Request: &GenerateRequest{
			Platforms:    []string{"shein"},
			Country:      "US",
			Language:     "en_US",
			SheinStoreID: 869,
			Text:         "metal wall sign",
		},
		Status: TaskStatusNeedsReview,
		Result: &ListingKitResult{
			TaskID: "task-apply-shein-regenerate-sale-attributes",
			CanonicalProduct: &canonical.Product{
				Title: "metal wall sign",
				Variants: []canonical.Variant{{
					SKU: "SKU-1",
					Attributes: map[string]canonical.Attribute{
						"Color": {Value: "white"},
					},
				}},
			},
			Shein: &SheinPackage{
				CategoryID:   3894,
				CategoryPath: []string{"家居&生活", "家居装饰", "家居装饰摆件&配件", "铁皮画标牌"},
				CategoryResolution: &SheinCategoryResolution{
					Status:     "resolved",
					CategoryID: 3894,
				},
				SaleAttributeResolution: &SheinSaleAttributeResolution{
					Status:             "resolved",
					PrimaryAttributeID: 27,
				},
				RequestDraft: &SheinRequestDraft{
					SKCList: []SheinSKCRequestDraft{{SupplierCode: "SKC-1"}},
				},
			},
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	_ = repo.CreateTask(context.Background(), task)
	svc := &service{
		repo: repo,
		sheinRuntimeDeps: sheinRuntimeDependencies{
			attributeResolver:     stubRevisionSheinAttributeResolver{},
			saleAttributeResolver: stubRevisionSheinSaleMissingValueResolver{},
		},
	}

	preview, err := svc.ApplyTaskRevision(context.Background(), task.ID, &ApplyRevisionRequest{
		Platform: "shein",
		Actor:    "workspace",
		Reason:   "Regenerate SHEIN sale attributes",
		Shein: &SheinRevisionInput{
			RegenerateSaleAttributes: true,
		},
	})
	if err != nil {
		t.Fatalf("apply task revision: %v", err)
	}
	if preview == nil || preview.Shein == nil || preview.Shein.EditorContext == nil || preview.Shein.EditorContext.SaleAttributes == nil {
		t.Fatalf("preview = %+v", preview)
	}
	if repo.task.Result.Shein.SaleAttributeResolution == nil {
		t.Fatal("expected sale attribute resolution after regenerate")
	}
	if repo.task.Result.Shein.SaleAttributeResolution.PrimaryAttributeID != 1001466 {
		t.Fatalf("primary attribute id = %d, want regenerated 1001466", repo.task.Result.Shein.SaleAttributeResolution.PrimaryAttributeID)
	}
	if repo.task.Result.Shein.SaleAttributeResolution.Status != "partial" {
		t.Fatalf("sale attribute status = %q, want partial after regenerate without value ids", repo.task.Result.Shein.SaleAttributeResolution.Status)
	}
	if !containsReviewNote(repo.task.Result.Shein.SaleAttributeResolution.ReviewNotes, "当前销售属性仍缺少真实 sale attribute value 映射，请重新确认规格。") {
		t.Fatalf("sale attribute review notes = %#v, want follow-up note after regenerate", repo.task.Result.Shein.SaleAttributeResolution.ReviewNotes)
	}
}

func TestApplyTaskRevisionRegeneratesSheinAttributesWithoutTouchingSaleAttributes(t *testing.T) {
	t.Parallel()

	repo := &stubApplyRevisionRepo{}
	secondaryValueID := 417
	task := &Task{
		ID: "task-apply-shein-regenerate-attributes",
		Request: &GenerateRequest{
			Platforms:    []string{"shein"},
			Country:      "US",
			Language:     "en_US",
			SheinStoreID: 869,
			Text:         "metal wall sign",
		},
		Status: TaskStatusNeedsReview,
		Result: &ListingKitResult{
			TaskID: "task-apply-shein-regenerate-attributes",
			CanonicalProduct: &canonical.Product{
				Title: "metal wall sign",
				Attributes: map[string]canonical.Attribute{
					"Material": {Value: "Metal"},
				},
				Variants: []canonical.Variant{{
					SKU: "SKU-1",
					Attributes: map[string]canonical.Attribute{
						"Color": {Value: "white"},
						"Size":  {Value: "90x180cm"},
					},
				}},
			},
			Shein: &SheinPackage{
				CategoryID:   3894,
				CategoryPath: []string{"家居&生活", "家居装饰", "家居装饰摆件&配件", "铁皮画标牌"},
				CategoryResolution: &SheinCategoryResolution{
					Status:     "resolved",
					CategoryID: 3894,
				},
				AttributeResolution: &SheinAttributeResolution{
					Status:        "partial",
					ResolvedCount: 0,
				},
				SaleAttributeResolution: &SheinSaleAttributeResolution{
					Status:               "resolved",
					PrimaryAttributeID:   27,
					SecondaryAttributeID: 87,
					SKCAttributes: []SheinResolvedSaleAttribute{{
						Scope:            "skc",
						Name:             "Color",
						Value:            "white",
						AttributeID:      27,
						AttributeValueID: &secondaryValueID,
					}},
				},
				RequestDraft: &SheinRequestDraft{
					SKCList: []SheinSKCRequestDraft{{
						SupplierCode: "SKC-1",
						SKUList: []SheinSKUDraft{{
							SupplierSKU: "SKU-1",
							SaleAttributes: []SheinResolvedSaleAttribute{{
								Scope:            "sku",
								Name:             "Size",
								Value:            "90x180cm",
								AttributeID:      87,
								AttributeValueID: &secondaryValueID,
							}},
						}},
					}},
				},
			},
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	_ = repo.CreateTask(context.Background(), task)
	svc := &service{
		repo: repo,
		sheinRuntimeDeps: sheinRuntimeDependencies{
			attributeResolver:     stubRevisionSheinAttributeResolver{},
			saleAttributeResolver: stubRevisionSheinSaleMissingValueResolver{},
		},
	}

	preview, err := svc.ApplyTaskRevision(context.Background(), task.ID, &ApplyRevisionRequest{
		Platform: "shein",
		Actor:    "workspace",
		Reason:   "Regenerate SHEIN attributes",
		Shein: &SheinRevisionInput{
			RegenerateAttributes: true,
		},
	})
	if err != nil {
		t.Fatalf("apply task revision: %v", err)
	}
	if preview == nil || preview.Shein == nil || preview.Shein.EditorContext == nil || preview.Shein.EditorContext.Attributes == nil {
		t.Fatalf("preview = %+v", preview)
	}
	if repo.task.Result.Shein.AttributeResolution == nil || repo.task.Result.Shein.AttributeResolution.ResolvedCount != 1 {
		t.Fatalf("attribute resolution = %+v, want regenerated attributes", repo.task.Result.Shein.AttributeResolution)
	}
	if len(repo.task.Result.Shein.ResolvedAttributes) != 1 || repo.task.Result.Shein.ResolvedAttributes[0].AttributeID != 7001 {
		t.Fatalf("resolved attributes = %+v, want regenerated attribute id 7001", repo.task.Result.Shein.ResolvedAttributes)
	}
	if repo.task.Result.Shein.SaleAttributeResolution == nil || repo.task.Result.Shein.SaleAttributeResolution.PrimaryAttributeID != 27 {
		t.Fatalf("sale attribute resolution = %+v, want untouched current sale attributes", repo.task.Result.Shein.SaleAttributeResolution)
	}
	if repo.task.Result.Shein.SaleAttributeResolution.SecondaryAttributeID != 87 {
		t.Fatalf("secondary attribute id = %d, want 87", repo.task.Result.Shein.SaleAttributeResolution.SecondaryAttributeID)
	}
}

func TestApplyTaskRevisionSupportsRestoreFromRevisionID(t *testing.T) {
	t.Parallel()

	repo := &stubApplyRevisionRepo{}
	restoreName := "Restored Bottle"
	task := &Task{
		ID:     "task-apply-restore",
		Status: TaskStatusCompleted,
		Result: &ListingKitResult{
			TaskID: "task-apply-restore",
			AssetBundle: &asset.Bundle{
				Assets: []asset.Asset{{
					ID:   "asset-main",
					Kind: asset.KindMainImage,
					URL:  "https://cdn.example.com/current.jpg",
					Metadata: map[string]string{
						"prompt_key":            "productimage.scene.shoes",
						"scene_defaults_source": "platform_category",
						"scene_category":        "shoes",
						"scene_style":           "lifestyle",
					},
				}},
			},
			Shein: &SheinPackage{
				SpuName:       "Current Bottle",
				ProductNameEn: "Current Bottle",
				RequestDraft: &SheinRequestDraft{
					SKCList: []SheinSKCRequestDraft{{SupplierCode: "SKC-1"}},
				},
				ImageBundle: &common.PublishImageBundle{
					Platform: "shein",
					Main: &common.BundleSlot{
						AssetID: "asset-main",
					},
				},
			},
			RevisionHistory: []ListingKitRevisionRecord{{
				RevisionID: "rev-restore-1",
				Platform:   "shein",
				Reason:     "manual adjustment",
				EditorContext: &SheinEditorContext{
					RevisionSkeleton: &SheinEditorRevisionSkeleton{
						Platform: "shein",
						Reason:   "manual adjustment",
						Shein: &SheinRevisionInput{
							SpuName:       &restoreName,
							ProductNameEn: &restoreName,
						},
					},
				},
			}},
			RevisionHistoryTotal: 1,
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	_ = repo.CreateTask(context.Background(), task)
	svc := &service{repo: repo}

	preview, err := svc.ApplyTaskRevision(context.Background(), task.ID, &ApplyRevisionRequest{
		Platform:              "shein",
		RestoreFromRevisionID: "rev-restore-1",
	})
	if err != nil {
		t.Fatalf("apply task revision: %v", err)
	}
	if preview.Shein == nil || preview.Shein.Headline != restoreName {
		t.Fatalf("preview shein = %+v", preview.Shein)
	}
	if len(preview.Shein.ScenePresets) != 1 || preview.Shein.ScenePresets[0].ScenePreset == nil || preview.Shein.ScenePresets[0].ScenePreset.PromptKey != "productimage.scene.shoes" {
		t.Fatalf("shein scene presets = %+v", preview.Shein.ScenePresets)
	}
	if repo.task.Result.Shein == nil || repo.task.Result.Shein.SpuName != restoreName {
		t.Fatalf("result shein = %+v", repo.task.Result.Shein)
	}
	if repo.task.Result.Revision == nil || repo.task.Result.Revision.Reason != "restore: manual adjustment" {
		t.Fatalf("revision summary = %+v", repo.task.Result.Revision)
	}
	if repo.task.Result.Revision.ActionType != RevisionActionTypeRestore {
		t.Fatalf("revision summary = %+v", repo.task.Result.Revision)
	}
	if repo.task.Result.Revision.Timeline == nil || repo.task.Result.Revision.Timeline.Badge != "回滚" {
		t.Fatalf("revision summary = %+v", repo.task.Result.Revision)
	}
	if repo.task.Result.Revision.RestoredFromRevisionID != "rev-restore-1" {
		t.Fatalf("revision summary = %+v", repo.task.Result.Revision)
	}
	if preview.RestoreResult == nil || preview.RestoreResult.SuccessPayload == nil || preview.RestoreResult.SuccessPayload.Core == nil || preview.RestoreResult.SuccessPayload.Core.SourceRevisionID != "rev-restore-1" {
		t.Fatalf("restore result = %+v", preview.RestoreResult)
	}
	if preview.ApplyResult == nil || preview.ApplyResult.SuccessPayload == nil || preview.ApplyResult.SuccessPayload.Core == nil || preview.ApplyResult.SuccessPayload.Core.ActionType != RevisionActionTypeRestore {
		t.Fatalf("apply result = %+v", preview.ApplyResult)
	}
	if len(preview.ApplyResult.SuccessPayload.Presentation.NextActions) == 0 {
		t.Fatalf("apply result = %+v", preview.ApplyResult)
	}
	if preview.ApplyResult.SuccessPayload.Core.StatusSummary == nil {
		t.Fatalf("apply result = %+v", preview.ApplyResult)
	}
	if preview.ApplyResult.SuccessPayload.Presentation.Messages == nil || preview.ApplyResult.SuccessPayload.Presentation.Messages.SuccessLabel == "" {
		t.Fatalf("apply result = %+v", preview.ApplyResult)
	}
	if preview.ApplyResult.SuccessPayload.Presentation.RecommendedView == nil || preview.ApplyResult.SuccessPayload.Presentation.RecommendedView.View == "" {
		t.Fatalf("apply result = %+v", preview.ApplyResult)
	}
	if preview.ApplyResult.SuccessPayload.Core.FollowUpChecklist == nil {
		t.Fatalf("apply result = %+v", preview.ApplyResult)
	}
	if preview.ApplyResult.SuccessPayload.Core.FollowUpOverview == nil || preview.ApplyResult.SuccessPayload.Core.FollowUpOverview.Headline == "" {
		t.Fatalf("apply result = %+v", preview.ApplyResult)
	}
	if preview.ApplyResult.SuccessPayload.Core.SuggestedFollowUpRevision == nil || preview.ApplyResult.SuccessPayload.Core.SuggestedFollowUpRevision.Platform != "shein" {
		t.Fatalf("apply result = %+v", preview.ApplyResult)
	}
	if preview.ApplyResult.SuccessPayload == nil || preview.ApplyResult.SuccessPayload.Mode != "apply" {
		t.Fatalf("apply result = %+v", preview.ApplyResult)
	}
	if preview.RestoreResult.SuccessPayload.Core.ActionType != RevisionActionTypeRestore {
		t.Fatalf("restore result = %+v", preview.RestoreResult)
	}
	if len(preview.RestoreResult.SuccessPayload.Presentation.NextActions) == 0 {
		t.Fatalf("restore result = %+v", preview.RestoreResult)
	}
	if preview.RestoreResult.SuccessPayload.Core.StatusSummary == nil {
		t.Fatalf("restore result = %+v", preview.RestoreResult)
	}
	if preview.RestoreResult.SuccessPayload.Presentation.Messages == nil || preview.RestoreResult.SuccessPayload.Presentation.Messages.SuccessLabel == "" {
		t.Fatalf("restore result = %+v", preview.RestoreResult)
	}
	if preview.RestoreResult.SuccessPayload.Presentation.RecommendedView == nil || preview.RestoreResult.SuccessPayload.Presentation.RecommendedView.View == "" {
		t.Fatalf("restore result = %+v", preview.RestoreResult)
	}
	if preview.RestoreResult.SuccessPayload.Core.FollowUpChecklist == nil {
		t.Fatalf("restore result = %+v", preview.RestoreResult)
	}
	if preview.RestoreResult.SuccessPayload.Core.FollowUpOverview == nil || preview.RestoreResult.SuccessPayload.Core.FollowUpOverview.Headline == "" {
		t.Fatalf("restore result = %+v", preview.RestoreResult)
	}
	if preview.RestoreResult.SuccessPayload.Core.SuggestedFollowUpRevision == nil || preview.RestoreResult.SuccessPayload.Core.SuggestedFollowUpRevision.Platform != "shein" {
		t.Fatalf("restore result = %+v", preview.RestoreResult)
	}
	if preview.RestoreResult.SuccessPayload.Presentation.Scene != revisionPresentationSceneRestoreSuccess || preview.RestoreResult.SuccessPayload.Presentation.SummaryCard == nil || preview.RestoreResult.SuccessPayload.Presentation.SummaryCard.Title == "" {
		t.Fatalf("restore result = %+v", preview.RestoreResult)
	}
	if preview.RestoreResult.SuccessPayload == nil || preview.RestoreResult.SuccessPayload.Mode != "restore" {
		t.Fatalf("restore result = %+v", preview.RestoreResult)
	}
	if len(preview.RevisionHistory) != 2 {
		t.Fatalf("revision history = %+v", preview.RevisionHistory)
	}
	last := preview.RevisionHistory[len(preview.RevisionHistory)-1]
	if last.ActionType != RevisionActionTypeRestore {
		t.Fatalf("latest revision history = %+v", last)
	}
	if last.Timeline == nil || last.Timeline.RelationText != "恢复自 rev-restore-1" {
		t.Fatalf("latest revision history = %+v", last)
	}
	if last.RestoredFromRevisionID != "rev-restore-1" {
		t.Fatalf("latest revision history = %+v", last)
	}
}

func TestApplyTaskRevisionReturnsNotFoundForMissingRestoreRevision(t *testing.T) {
	t.Parallel()

	repo := &stubApplyRevisionRepo{}
	task := &Task{
		ID:     "task-apply-restore-missing",
		Status: TaskStatusCompleted,
		Result: &ListingKitResult{
			TaskID: "task-apply-restore-missing",
			Shein: &SheinPackage{
				SpuName: "Current Bottle",
				RequestDraft: &SheinRequestDraft{
					SKCList: []SheinSKCRequestDraft{{SupplierCode: "SKC-1"}},
				},
			},
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	_ = repo.CreateTask(context.Background(), task)
	svc := &service{repo: repo}

	_, err := svc.ApplyTaskRevision(context.Background(), task.ID, &ApplyRevisionRequest{
		Platform:              "shein",
		RestoreFromRevisionID: "missing",
	})
	if err == nil || err != ErrRevisionHistoryRecordNotFound {
		t.Fatalf("error = %v, want %v", err, ErrRevisionHistoryRecordNotFound)
	}
}

func TestApplyTaskRevisionPersistsAtomically(t *testing.T) {
	t.Parallel()

	repo := &stubApplyRevisionRepo{failOnSaveCall: 2}
	task := &Task{
		ID:     "task-apply-atomic",
		Status: TaskStatusCompleted,
		Result: &ListingKitResult{
			TaskID: "task-apply-atomic",
			Shein: &SheinPackage{
				SpuName: "Before",
				RequestDraft: &SheinRequestDraft{
					SKCList: []SheinSKCRequestDraft{{SupplierCode: "SKC-1"}},
				},
			},
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	_ = repo.CreateTask(context.Background(), task)
	svc := &service{repo: repo}

	newName := "After"
	preview, err := svc.ApplyTaskRevision(context.Background(), task.ID, &ApplyRevisionRequest{
		Platform: "shein",
		Shein: &SheinRevisionInput{
			SpuName: &newName,
		},
	})
	if err != nil {
		t.Fatalf("apply task revision: %v", err)
	}
	if preview == nil || preview.Shein == nil || preview.Shein.Headline != newName {
		t.Fatalf("preview = %+v, want updated shein headline", preview)
	}
	if repo.saveCalls != 1 {
		t.Fatalf("save calls = %d, want 1 atomic save", repo.saveCalls)
	}
	if repo.task.Result == nil || repo.task.Result.Shein == nil || repo.task.Result.Shein.SpuName != newName {
		t.Fatalf("stored shein result = %+v, want updated spu name", repo.task.Result)
	}
	if len(repo.task.Result.RevisionHistory) != 1 {
		t.Fatalf("revision history = %+v, want 1 record", repo.task.Result.RevisionHistory)
	}
}

func boolPtr(v bool) *bool {
	return &v
}

func stringPtr(v string) *string {
	return &v
}

type stubManualSaleAttributeAPI struct {
	templates *sheinattribute.AttributeTemplateInfo
}

func (s stubManualSaleAttributeAPI) GetAttributeTemplates(categoryID int) (*sheinattribute.AttributeTemplateInfo, error) {
	return s.templates, nil
}

func (s stubManualSaleAttributeAPI) ValidateCustomAttributeValue(attributeID int, attributeValue string, categoryID int, spuName string) (*sheinattribute.ValidateAttributeResponse, error) {
	resp := &sheinattribute.ValidateAttributeResponse{}
	resp.Data.AttributeID = attributeID
	resp.Data.PreAttributeValueID = 3001
	resp.Data.AttributeValueNameMultis = []struct {
		Language                string `json:"language"`
		AttributeValueNameMulti string `json:"attribute_value_name_multi"`
		WarningType             int    `json:"warning_type"`
	}{
		{Language: "en", AttributeValueNameMulti: attributeValue},
	}
	return resp, nil
}

func (s stubManualSaleAttributeAPI) AddCustomAttributeValue(req *sheinattribute.AddCustomAttributeValueRequest) (*sheinattribute.AddCustomAttributeValueResponse, error) {
	resp := &sheinattribute.AddCustomAttributeValueResponse{}
	resp.Info.Data.CustomAttributeRelation = []sheinattribute.CustomAttributeRelation{{
		PreAttributeValueID: req.PreAttributeValueList[0].PreAttributeValueID,
		AttributeValueID:    9001,
	}}
	return resp, nil
}

func TestResolveManualSheinSaleAttributeValueIDsCreatesCustomValueIDs(t *testing.T) {
	t.Parallel()

	pkg := &SheinPackage{
		SpuName:    "Bench Cushion",
		CategoryID: 12143,
		SkcList: []sheinpub.SKCPackage{{
			SupplierCode: "SKC-1",
			Attributes:   map[string]string{"Color": "米驼"},
			SKUs: []common.Variant{{
				SKU:        "SKU-1",
				Attributes: map[string]string{"Size": `30"×40"`},
			}},
		}},
		SaleAttributeResolution: &SheinSaleAttributeResolution{
			PrimarySourceDimension:   "Color",
			SecondarySourceDimension: "Size",
		},
	}
	req := &SheinRevisionInput{
		SaleAttributeResolution: &SheinSaleAttributeResolutionPatch{
			PrimaryAttributeID:   intPtr(501),
			SecondaryAttributeID: intPtr(502),
		},
		SKCPatches: []SheinSKCRevisionPatch{{
			SupplierCode: "SKC-1",
			SaleAttribute: &SheinResolvedSaleAttribute{
				Scope:       "skc",
				Name:        "Color",
				AttributeID: 501,
			},
			SKUPatches: []SheinSKURevisionPatch{{
				SupplierSKU: "SKU-1",
				SaleAttributes: []SheinResolvedSaleAttribute{{
					Scope:       "sku",
					Name:        "Size",
					AttributeID: 502,
				}},
			}},
		}},
	}
	templates := &sheinattribute.AttributeTemplateInfo{
		Data: []sheinattribute.AttributeTemplate{{
			AttributeInfos: []sheinattribute.AttributeInfo{
				{AttributeID: 501, AttributeName: "颜色", AttributeNameEn: "Color", AttributeInputNum: 1},
				{AttributeID: 502, AttributeName: "尺码", AttributeNameEn: "Size", AttributeInputNum: 1},
			},
		}},
	}

	relations, notes, err := resolveManualSheinSaleAttributeValueIDs(
		pkg,
		req,
		stubManualSaleAttributeAPI{templates: templates},
		12143,
		flattenSheinAttributeTemplatesByID(templates),
	)
	if err != nil {
		t.Fatalf("resolve manual shein sale attributes: %v", err)
	}
	if got := req.SKCPatches[0].SaleAttribute; got == nil || got.AttributeValueID == nil || *got.AttributeValueID != 9001 {
		t.Fatalf("skc sale attribute = %+v, want custom value id 9001", got)
	}
	if got := req.SKCPatches[0].SKUPatches[0].SaleAttributes[0].AttributeValueID; got == nil || *got != 9001 {
		t.Fatalf("sku sale attribute value id = %v, want 9001", got)
	}
	if len(relations) == 0 || relations[0].AttributeValueID != 9001 {
		t.Fatalf("relations = %+v, want custom relation", relations)
	}
	if len(notes) == 0 {
		t.Fatalf("notes = %+v, want remote resolution notes", notes)
	}
	if len(req.SaleAttributeResolution.SKCAttributes) != 1 || req.SaleAttributeResolution.SKCAttributes[0].AttributeValueID == nil {
		t.Fatalf("sale attribute resolution skc attrs = %+v", req.SaleAttributeResolution.SKCAttributes)
	}
	if len(req.SaleAttributeResolution.SKUAttributes) != 1 || req.SaleAttributeResolution.SKUAttributes[0].AttributeValueID == nil {
		t.Fatalf("sale attribute resolution sku attrs = %+v", req.SaleAttributeResolution.SKUAttributes)
	}
}

func TestResolveManualSheinSaleAttributeValueIDsBackfillsMissingSKUSaleAttributesFromAssignments(t *testing.T) {
	t.Parallel()

	sizeMValueID := 9101
	sizeLValueID := 9102
	pkg := &SheinPackage{
		SpuName:    "Curtain Set",
		CategoryID: 12143,
		SkcList: []sheinpub.SKCPackage{{
			SupplierCode: "SKC-1",
			Attributes:   map[string]string{"Color": "white"},
			SKUs: []common.Variant{
				{
					SKU:        "SKU-1",
					Attributes: map[string]string{"Size": `60×70.8Inch (152×180cm)`},
				},
				{
					SKU:        "MG8014086001",
					Attributes: map[string]string{"Size": `70.8×70.8Inch (180×180cm)`},
				},
			},
		}},
		SaleAttributeResolution: &SheinSaleAttributeResolution{
			PrimarySourceDimension:   "Color",
			SecondarySourceDimension: "Size",
		},
	}
	req := &SheinRevisionInput{
		SaleAttributeResolution: &SheinSaleAttributeResolutionPatch{
			Status:                   stringPtr("resolved"),
			PrimaryAttributeID:       intPtr(27),
			SecondaryAttributeID:     intPtr(87),
			PrimarySourceDimension:   stringPtr("Color"),
			SecondarySourceDimension: stringPtr("Size"),
			SKUValueAssignments: map[string]SheinResolvedSaleAttribute{
				`60×70.8inch (152×180cm)`: {
					Scope:            "sku",
					Name:             "Size",
					Value:            `60×70.8Inch (152×180cm)`,
					AttributeID:      87,
					AttributeValueID: &sizeMValueID,
					MatchedBy:        "manual_review",
				},
				`70.8×70.8inch (180×180cm)`: {
					Scope:            "sku",
					Name:             "Size",
					Value:            `70.8×70.8Inch (180×180cm)`,
					AttributeID:      87,
					AttributeValueID: &sizeLValueID,
					MatchedBy:        "manual_review",
				},
			},
		},
		SKCPatches: []SheinSKCRevisionPatch{{
			SupplierCode: "SKC-1",
			SKUPatches: []SheinSKURevisionPatch{
				{
					SupplierSKU: "SKU-1",
					Attributes:  map[string]string{"Size": `60×70.8Inch (152×180cm)`},
					SaleAttributes: []SheinResolvedSaleAttribute{{
						Scope:            "sku",
						Name:             "Size",
						Value:            `60×70.8Inch (152×180cm)`,
						AttributeID:      87,
						AttributeValueID: &sizeMValueID,
						MatchedBy:        "manual_review",
					}},
				},
				{
					SupplierSKU:    "MG8014086001",
					Attributes:     map[string]string{"Size": `70.8×70.8Inch (180×180cm)`},
					SaleAttributes: nil,
				},
			},
		}},
	}

	_, _, err := resolveManualSheinSaleAttributeValueIDs(
		pkg,
		req,
		stubManualSaleAttributeAPI{},
		12143,
		nil,
	)
	if err != nil {
		t.Fatalf("resolve manual shein sale attributes: %v", err)
	}

	missingSKU := req.SKCPatches[0].SKUPatches[1]
	if len(missingSKU.SaleAttributes) != 1 {
		t.Fatalf("missing sku sale attributes = %+v, want one backfilled attribute", missingSKU.SaleAttributes)
	}
	if got := missingSKU.SaleAttributes[0].AttributeValueID; got == nil || *got != sizeLValueID {
		t.Fatalf("missing sku sale attributes = %+v, want value id %d", missingSKU.SaleAttributes, sizeLValueID)
	}
	if len(req.SaleAttributeResolution.SKUAttributes) != 2 {
		t.Fatalf("sale attribute resolution sku attrs = %+v, want two synced sku attributes", req.SaleAttributeResolution.SKUAttributes)
	}
}

func TestResolveManualSheinSaleAttributeValueIDsResolvesMissingSKUSaleAttributesWhenAssignmentsAreMissing(t *testing.T) {
	t.Parallel()

	sizeMValueID := 9101
	pkg := &SheinPackage{
		SpuName:    "Curtain Set",
		CategoryID: 12143,
		SkcList: []sheinpub.SKCPackage{{
			SupplierCode: "SKC-1",
			Attributes:   map[string]string{"Color": "white"},
			SKUs: []common.Variant{
				{
					SKU:        "SKU-1",
					Attributes: map[string]string{"Size": `60×70.8Inch (152×180cm)`},
				},
				{
					SKU:        "MG8014086001",
					Attributes: map[string]string{"Size": `70.8×70.8Inch (180×180cm)`},
				},
			},
		}},
		SaleAttributeResolution: &SheinSaleAttributeResolution{
			PrimarySourceDimension:   "Color",
			SecondarySourceDimension: "Size",
		},
	}
	req := &SheinRevisionInput{
		SaleAttributeResolution: &SheinSaleAttributeResolutionPatch{
			Status:                   stringPtr("resolved"),
			PrimaryAttributeID:       intPtr(27),
			SecondaryAttributeID:     intPtr(87),
			PrimarySourceDimension:   stringPtr("Color"),
			SecondarySourceDimension: stringPtr("Size"),
			SKUValueAssignments: map[string]SheinResolvedSaleAttribute{
				`60×70.8inch (152×180cm)`: {
					Scope:            "sku",
					Name:             "Size",
					Value:            `60×70.8Inch (152×180cm)`,
					AttributeID:      87,
					AttributeValueID: &sizeMValueID,
					MatchedBy:        "manual_review",
				},
			},
		},
		SKCPatches: []SheinSKCRevisionPatch{{
			SupplierCode: "SKC-1",
			SKUPatches: []SheinSKURevisionPatch{
				{
					SupplierSKU: "SKU-1",
					Attributes:  map[string]string{"Size": `60×70.8Inch (152×180cm)`},
					SaleAttributes: []SheinResolvedSaleAttribute{{
						Scope:            "sku",
						Name:             "Size",
						Value:            `60×70.8Inch (152×180cm)`,
						AttributeID:      87,
						AttributeValueID: &sizeMValueID,
						MatchedBy:        "manual_review",
					}},
				},
				{
					SupplierSKU:    "MG8014086001",
					Attributes:     map[string]string{"Size": `70.8×70.8Inch (180×180cm)`},
					SaleAttributes: nil,
				},
			},
		}},
	}
	templates := &sheinattribute.AttributeTemplateInfo{
		Data: []sheinattribute.AttributeTemplate{{
			AttributeInfos: []sheinattribute.AttributeInfo{
				{AttributeID: 87, AttributeName: "尺码", AttributeNameEn: "Size", AttributeInputNum: 1},
			},
		}},
	}

	_, notes, err := resolveManualSheinSaleAttributeValueIDs(
		pkg,
		req,
		stubManualSaleAttributeAPI{templates: templates},
		12143,
		flattenSheinAttributeTemplatesByID(templates),
	)
	if err != nil {
		t.Fatalf("resolve manual shein sale attributes: %v", err)
	}

	missingSKU := req.SKCPatches[0].SKUPatches[1]
	if len(missingSKU.SaleAttributes) != 1 {
		t.Fatalf("missing sku sale attributes = %+v, want one remotely resolved attribute", missingSKU.SaleAttributes)
	}
	if got := missingSKU.SaleAttributes[0].AttributeValueID; got == nil || *got != 9001 {
		t.Fatalf("missing sku sale attributes = %+v, want remotely resolved value id 9001", missingSKU.SaleAttributes)
	}
	if len(req.SaleAttributeResolution.SKUAttributes) != 2 {
		t.Fatalf("sale attribute resolution sku attrs = %+v, want two synced sku attributes", req.SaleAttributeResolution.SKUAttributes)
	}
	if len(notes) == 0 {
		t.Fatalf("notes = %+v, want remote resolution notes", notes)
	}
}
