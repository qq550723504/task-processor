package store_test

import (
	"context"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	_ "modernc.org/sqlite"

	"task-processor/internal/listingkit"
	"task-processor/internal/listingkit/store"
	commonpub "task-processor/internal/publishing/common"
	sheinpub "task-processor/internal/publishing/shein"
)

func TestTaskRepositoryLookupSheinPODImagesMatchesSellerSKUWithoutHyphen(t *testing.T) {
	t.Parallel()

	db, err := gorm.Open(sqlite.Dialector{DriverName: "sqlite", DSN: ":memory:"}, &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&listingkit.Task{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}

	repo := store.NewTaskRepository(db)
	lookupRepo, ok := repo.(listingkit.SheinPODImageLookupRepository)
	if !ok {
		t.Fatal("task repository does not implement SheinPODImageLookupRepository")
	}

	ctx := listingkit.WithTenantID(context.Background(), "tenant-a")
	now := time.Date(2026, 5, 30, 23, 54, 19, 0, time.UTC)
	matching := makeSheinPODLookupTask(
		"000a11f9-b41e-4e7f-bd9d-b3cefd739012",
		869,
		"XB0606012001-EEC0A584",
		"XB0606012001-V49720-T000A11F9-R4012C1-14624330",
		now,
	)
	otherStore := makeSheinPODLookupTask(
		"same-sku-other-store",
		870,
		"XB0606012001-EEC0A584",
		"XB0606012001-V49720-T000A11F9-R4012C1-14624330",
		now.Add(time.Minute),
	)
	for _, task := range []*listingkit.Task{matching, otherStore} {
		if err := repo.CreateTask(ctx, task); err != nil {
			t.Fatalf("create task %s: %v", task.ID, err)
		}
	}

	items, total, err := lookupRepo.LookupSheinPODImages(ctx, &listingkit.SheinPODImageLookupQuery{
		StoreID: 869,
		Query:   "XB0606012001V49720-T000A11F9-R4012C1-14624330",
		Limit:   20,
	})
	if err != nil {
		t.Fatalf("LookupSheinPODImages() error = %v", err)
	}
	if total != 1 {
		t.Fatalf("total = %d, want 1", total)
	}
	if len(items) != 1 {
		t.Fatalf("items length = %d, want 1", len(items))
	}
	item := items[0]
	if item.TaskID != matching.ID {
		t.Fatalf("task id = %q, want %q", item.TaskID, matching.ID)
	}
	if item.SellerSKU != "XB0606012001-V49720-T000A11F9-R4012C1-14624330" {
		t.Fatalf("seller sku = %q", item.SellerSKU)
	}
	if item.SheinSPUName != "g2605302354951131" {
		t.Fatalf("spu name = %q", item.SheinSPUName)
	}
	if item.SheinVersion != "SPMP260530352497648" {
		t.Fatalf("version = %q", item.SheinVersion)
	}
	if item.AIOriginalImageURL != "https://oss.shuomiai.com/listingkit-assets/20260530/d669b6d0-833c-4567-a39f-480e03a58fc3.png" {
		t.Fatalf("ai original image = %q", item.AIOriginalImageURL)
	}
	if item.SDSMainImageURL != "https://cdn.sdspod.com/out/0/202605/f95d77f558fa121c28ba51b1f1926f5d.jpg" {
		t.Fatalf("sds main image = %q", item.SDSMainImageURL)
	}
	if len(item.SDSGalleryImageURLs) != 2 {
		t.Fatalf("gallery length = %d, want 2", len(item.SDSGalleryImageURLs))
	}
}

func TestTaskRepositoryLookupSheinPODImagesMatchesSheinReturnedSPUName(t *testing.T) {
	t.Parallel()

	db, err := gorm.Open(sqlite.Dialector{DriverName: "sqlite", DSN: ":memory:"}, &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&listingkit.Task{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}

	repo := store.NewTaskRepository(db)
	lookupRepo, ok := repo.(listingkit.SheinPODImageLookupRepository)
	if !ok {
		t.Fatal("task repository does not implement SheinPODImageLookupRepository")
	}

	ctx := listingkit.WithTenantID(context.Background(), "tenant-a")
	task := makeSheinPODLookupTask(
		"000a11f9-b41e-4e7f-bd9d-b3cefd739012",
		869,
		"XB0606012001-EEC0A584",
		"XB0606012001-V49720-T000A11F9-R4012C1-14624330",
		time.Date(2026, 5, 30, 23, 54, 19, 0, time.UTC),
	)
	if err := repo.CreateTask(ctx, task); err != nil {
		t.Fatalf("create task: %v", err)
	}

	items, total, err := lookupRepo.LookupSheinPODImages(ctx, &listingkit.SheinPODImageLookupQuery{
		StoreID: 869,
		Query:   "g2605302354951131",
		Limit:   20,
	})
	if err != nil {
		t.Fatalf("LookupSheinPODImages() error = %v", err)
	}
	if total != 1 || len(items) != 1 {
		t.Fatalf("total/items = %d/%d, want 1/1", total, len(items))
	}
	if items[0].TaskID != task.ID {
		t.Fatalf("task id = %q, want %q", items[0].TaskID, task.ID)
	}
}

func makeSheinPODLookupTask(taskID string, storeID int64, supplierCode, sellerSKU string, at time.Time) *listingkit.Task {
	return &listingkit.Task{
		ID:       taskID,
		TenantID: "tenant-a",
		UserID:   "user-a",
		Request: &listingkit.GenerateRequest{
			Text:         "朋克叛逆人人格标签",
			SheinStoreID: storeID,
			ImageURLs: []string{
				"https://oss.shuomiai.com/listingkit-assets/20260530/d669b6d0-833c-4567-a39f-480e03a58fc3.png",
			},
		},
		SheinStoreResolutionSnapshot: &listingkit.SheinStoreResolutionSnapshot{StoreID: storeID, Site: "US"},
		Status:                       listingkit.TaskStatusCompleted,
		Result: &listingkit.ListingKitResult{
			TaskID: taskID,
			Status: string(listingkit.TaskStatusCompleted),
			Shein: &sheinpub.Package{
				Images: &commonpub.ImageSet{
					MainImage: "https://cdn.sdspod.com/out/0/202605/f95d77f558fa121c28ba51b1f1926f5d.jpg",
					Gallery: []string{
						"https://cdn.sdspod.com/out/36811/202605/1e49f4fd53b0807f99fbf58f9dae0e20.jpg",
						"https://cdn.sdspod.com/out/36811/202605/e681e4615928cbf8b086128886dea87b.jpg",
					},
				},
				RequestDraft: &sheinpub.RequestDraft{
					SKCList: []sheinpub.SKCRequestDraft{{
						SkcName:      "Graphic Print Cosmetic Bag",
						SupplierCode: supplierCode,
						SKUList: []sheinpub.SKUDraft{{
							SupplierSKU: sellerSKU,
						}},
					}},
				},
				SubmissionState: &sheinpub.SubmissionReport{
					Publish: &sheinpub.SubmissionRecord{
						Action:       "publish",
						Status:       "success",
						SupplierCode: supplierCode,
						Result: &sheinpub.SubmissionResponse{
							Code:    "0",
							Message: "OK",
							Success: true,
							SPUName: "g2605302354951131",
							Version: "SPMP260530352497648",
						},
					},
				},
			},
			CreatedAt: at,
			UpdatedAt: at,
		},
		CreatedAt: at,
		UpdatedAt: at,
	}
}
