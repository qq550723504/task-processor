package listingkit

import (
	"testing"
	"time"

	common "task-processor/internal/publishing/common"
	sheinpub "task-processor/internal/publishing/shein"
	sheinproduct "task-processor/internal/shein/api/product"
)

func TestBuildTaskListItemIncludesSheinRemoteSubmissionSummary(t *testing.T) {
	t.Parallel()

	checkedAt := time.Date(2026, 5, 7, 12, 30, 0, 0, time.UTC)
	task := &Task{
		ID:     "task-remote-summary",
		Status: TaskStatusCompleted,
		Result: &ListingKitResult{
			Shein: &SheinPackage{
				Submission: &sheinpub.SubmissionReport{
					LastAction:      "publish",
					RemoteStatus:    sheinpub.SubmissionRemoteStatusConfirmed,
					RemoteCheckedAt: &checkedAt,
					Publish: &sheinpub.SubmissionRecord{
						Action:         "publish",
						RemoteRecordID: "record-123",
					},
				},
			},
		},
	}

	item := buildTaskListItem(task)

	if item.SheinSubmissionRemoteStatus != sheinpub.SubmissionRemoteStatusConfirmed {
		t.Fatalf("remote status = %q, want confirmed", item.SheinSubmissionRemoteStatus)
	}
	if item.SheinSubmissionRemoteCheckedAt == nil || !item.SheinSubmissionRemoteCheckedAt.Equal(checkedAt) {
		t.Fatalf("remote checked at = %v, want %v", item.SheinSubmissionRemoteCheckedAt, checkedAt)
	}
	if item.SheinSubmissionRemoteRecordID != "record-123" {
		t.Fatalf("remote record id = %q, want record-123", item.SheinSubmissionRemoteRecordID)
	}
}

func TestBuildTaskListItemPrefersRenderedImageCount(t *testing.T) {
	t.Parallel()

	task := &Task{
		ID: "task-rendered-count",
		Request: &GenerateRequest{
			ImageURLs: []string{"https://cdn.example.com/input.png"},
		},
		Status: TaskStatusCompleted,
		Result: &ListingKitResult{
			SDSSync: &SDSSyncSummary{
				Status: "completed",
				MockupImageURLs: []string{
					"https://cdn.example.com/mockup-1.png",
					"https://cdn.example.com/mockup-2.png",
					"https://cdn.example.com/mockup-3.png",
				},
				VariantResults: []SDSSyncSummary{{
					Status:          "completed",
					MockupImageURLs: []string{"https://cdn.example.com/mockup-4.png"},
				}},
			},
		},
	}

	item := buildTaskListItem(task)

	if item.ImageCount != 4 {
		t.Fatalf("image count = %d, want rendered image count 4", item.ImageCount)
	}
}

func TestBuildTaskListItemDoesNotCountSourceImagesAsRenderedImages(t *testing.T) {
	t.Parallel()

	task := &Task{
		ID: "task-rendered-count-with-source",
		Request: &GenerateRequest{
			ImageURLs: []string{"https://cdn.example.com/input.png"},
		},
		Status: TaskStatusCompleted,
		Result: &ListingKitResult{
			Shein: &SheinPackage{
				RequestDraft: &sheinpub.RequestDraft{
					ImageInfo: &sheinpub.ImageDraft{
						MainImage: "https://cdn.example.com/main.png",
						Gallery: []string{
							"https://cdn.example.com/gallery-1.png",
							"https://cdn.example.com/gallery-2.png",
						},
						Source: []string{"https://cdn.example.com/input.png"},
					},
				},
			},
		},
	}

	item := buildTaskListItem(task)

	if item.ImageCount != 3 {
		t.Fatalf("image count = %d, want rendered SHEIN image count 3", item.ImageCount)
	}
}

func TestBuildTaskListItemIncludesSheinStatusOverview(t *testing.T) {
	t.Parallel()

	task := &Task{
		ID:     "task-status-overview",
		Status: TaskStatusCompleted,
		Result: &ListingKitResult{
			Shein: &SheinPackage{
				ReviewNotes: []string{"需要人工确认吊牌文案"},
				FinalDraft: &sheinpub.FinalDraft{
					Confirmed: false,
				},
				RequestDraft: &sheinpub.RequestDraft{
					ImageInfo: &sheinpub.ImageDraft{
						MainImage: "https://cdn.example.com/main.png",
						Gallery:   []string{"https://cdn.example.com/gallery-1.png"},
					},
					SKCList: []sheinpub.SKCRequestDraft{{
						SupplierCode: "SKC-1",
						ImageInfo: &sheinpub.ImageDraft{
							MainImage: "https://cdn.example.com/skc-main.png",
						},
						SKUList: []sheinpub.SKUDraft{{
							SupplierSKU: "SKU-1",
							BasePrice:   "19.99",
							SitePriceList: []sheinpub.SitePrice{{
								SubSite:   "US",
								BasePrice: "19.99",
								Currency:  "USD",
							}},
						}},
					}},
				},
				PreviewProduct: &sheinproduct.Product{},
				SkcList: []SheinSKCPackage{{
					SupplierCode: "SKC-1",
					SKUs: []PlatformVariant{{
						SKU: "SKU-1",
					}},
				}},
				CategoryResolution: &SheinCategoryResolution{
					Status:     "resolved",
					CategoryID: 3001,
				},
				CategoryID:     3001,
				CategoryIDList: []int{1, 2, 3001},
				ProductTypeID:  intPtr(901),
				AttributeResolution: &SheinAttributeResolution{
					Status:        "resolved",
					ResolvedCount: 1,
				},
				ResolvedAttributes: []SheinResolvedAttribute{{
					Name:        "Material",
					AttributeID: 160,
				}},
				SaleAttributeResolution: &SheinSaleAttributeResolution{
					Status:             "resolved",
					PrimaryAttributeID: 27,
				},
			},
		},
	}

	item := buildTaskListItem(task)

	if item.SheinStatusOverview == nil {
		t.Fatal("expected shein status overview")
	}
	if item.SheinStatusOverview.Status != "blocked" {
		t.Fatalf("status overview = %+v, want blocked", item.SheinStatusOverview)
	}
	if item.SheinStatusOverview.BlockingCount == 0 {
		t.Fatalf("status overview = %+v, want blocking count", item.SheinStatusOverview)
	}
	if item.SheinStatusOverview.PrimaryAction != "最终确认" || item.SheinStatusOverview.PrimaryActionKey != "final_review" {
		t.Fatalf("status overview = %+v, want final review primary action", item.SheinStatusOverview)
	}
	if item.SheinStatusOverview.Subheadline == "" {
		t.Fatalf("status overview = %+v, want subheadline", item.SheinStatusOverview)
	}
	if len(item.SheinBlockingKeys) == 0 || item.SheinBlockingKeys[0] != "final_review" {
		t.Fatalf("blocking keys = %+v, want final_review", item.SheinBlockingKeys)
	}
	if len(item.SheinWarningKeys) == 0 || item.SheinWarningKeys[0] != "manual_notes" {
		t.Fatalf("warning keys = %+v, want manual_notes", item.SheinWarningKeys)
	}
	if item.SheinWorkQueue != SheinWorkQueueRepair {
		t.Fatalf("work queue = %q, want %s", item.SheinWorkQueue, SheinWorkQueueRepair)
	}
	if item.SheinActionQueue != SheinActionQueueFinalReview {
		t.Fatalf("action queue = %q, want %s", item.SheinActionQueue, SheinActionQueueFinalReview)
	}
}

func TestBuildTaskListItemIncludesResolvedSheinStoreContext(t *testing.T) {
	t.Parallel()

	task := &Task{
		ID:     "task-store-context",
		Status: TaskStatusCompleted,
		Request: &GenerateRequest{
			SheinStoreID: 869,
			Country:      "US",
		},
		SheinStoreResolutionSnapshot: &SheinStoreResolutionSnapshot{
			StoreID:          903,
			Site:             "GB",
			Strategy:         "country",
			Reason:           "根据任务国家信息命中了对应店铺。",
			MatchedRuleKinds: []string{"country"},
			MatchedProfileID: 17,
			ResolvedAt:       time.Date(2026, 5, 18, 8, 15, 0, 0, time.UTC),
		},
		Result: &ListingKitResult{
			Shein: &SheinPackage{
				SiteList: []common.Site{{MainSite: "GB"}},
			},
		},
	}

	item := buildTaskListItem(task)

	if item.SheinStoreID != 903 {
		t.Fatalf("shein store id = %d, want 903 snapshot store", item.SheinStoreID)
	}
	if item.SheinStoreSite != "GB" {
		t.Fatalf("shein store site = %q, want GB", item.SheinStoreSite)
	}
	if item.SheinStoreProfileID != 17 {
		t.Fatalf("shein store profile id = %d, want 17", item.SheinStoreProfileID)
	}
	if item.SheinStoreResolvedAt == nil || item.SheinStoreResolvedAt.IsZero() {
		t.Fatalf("shein store resolved at = %v, want populated time", item.SheinStoreResolvedAt)
	}
	if item.SheinStoreStrategy != "country" || item.SheinStoreReason == "" {
		t.Fatalf("shein store audit fields = %+v, want snapshot strategy and reason", item)
	}
	if len(item.SheinStoreMatchedRuleKinds) != 1 || item.SheinStoreMatchedRuleKinds[0] != "country" {
		t.Fatalf("shein store matched rules = %+v, want [country]", item.SheinStoreMatchedRuleKinds)
	}
}

func intPtr(value int) *int {
	return &value
}
