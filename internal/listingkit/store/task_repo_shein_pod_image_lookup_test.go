package store_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	_ "modernc.org/sqlite"

	openaiclient "task-processor/internal/infra/clients/openai"
	"task-processor/internal/listingkit"
	"task-processor/internal/listingkit/sheinpodimage"
	"task-processor/internal/listingkit/store"
	commonpub "task-processor/internal/publishing/common"
	sheinpub "task-processor/internal/publishing/shein"
)

func TestTaskRepositoryLookupSheinPODImagesMatchesSellerSKUWithoutHyphen(t *testing.T) {
	t.Parallel()

	_, repo := newLookupRepository(t)
	lookupRepo, ok := repo.(sheinpodimage.SheinPODImageLookupRepository)
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

	items, total, err := lookupRepo.LookupSheinPODImages(ctx, &sheinpodimage.SheinPODImageLookupQuery{
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

	_, repo := newLookupRepository(t)
	lookupRepo, ok := repo.(sheinpodimage.SheinPODImageLookupRepository)
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

	items, total, err := lookupRepo.LookupSheinPODImages(ctx, &sheinpodimage.SheinPODImageLookupQuery{
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

func TestTaskRepositoryLookupSheinPODImagesReadsIndexWithoutTaskJSON(t *testing.T) {
	t.Parallel()

	db, repo := newLookupRepository(t)
	lookupRepo := repo.(sheinpodimage.SheinPODImageLookupRepository)
	ctx := tenantContext("tenant-a")
	task := makeSheinPODLookupTask("task-1", 869, "SUPPLIER", "SKU-1", time.Now())
	if err := repo.CreateTask(ctx, task); err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&listingkit.Task{}).
		Where("id = ?", task.ID).
		Updates(map[string]any{"request": nil, "result": nil}).Error; err != nil {
		t.Fatalf("clear task JSON: %v", err)
	}

	items, total, err := lookupRepo.LookupSheinPODImages(ctx, &sheinpodimage.SheinPODImageLookupQuery{
		StoreID: 869,
		Query:   "SKU1",
	})
	if err != nil || total != 1 || len(items) != 1 || items[0].TaskID != "task-1" {
		t.Fatalf("items=%+v total=%d err=%v", items, total, err)
	}
	if len(items[0].SDSGalleryImageURLs) != 2 {
		t.Fatalf("gallery=%v, want two indexed URLs", items[0].SDSGalleryImageURLs)
	}
}

func TestTaskRepositoryLookupSheinPODImagesUsesExactNormalizedMatch(t *testing.T) {
	t.Parallel()

	_, repo := newLookupRepository(t)
	lookupRepo := repo.(sheinpodimage.SheinPODImageLookupRepository)
	ctx := tenantContext("tenant-a")
	task := makeSheinPODLookupTask("task-exact", 869, "SUPPLIER", "SKU-123", time.Now())
	if err := repo.CreateTask(ctx, task); err != nil {
		t.Fatal(err)
	}

	items, total, err := lookupRepo.LookupSheinPODImages(ctx, &sheinpodimage.SheinPODImageLookupQuery{
		StoreID: 869,
		Query:   " sku_123 ",
	})
	if err != nil || total != 1 || len(items) != 1 {
		t.Fatalf("normalized exact lookup items=%+v total=%d err=%v", items, total, err)
	}
	items, total, err = lookupRepo.LookupSheinPODImages(ctx, &sheinpodimage.SheinPODImageLookupQuery{
		StoreID: 869,
		Query:   "SKU",
	})
	if err != nil || total != 0 || len(items) != 0 {
		t.Fatalf("partial lookup items=%+v total=%d err=%v, want no match", items, total, err)
	}
}

func TestBuildSheinPODImageLookupIndexUsesFixedLengthHashedLookupKeys(t *testing.T) {
	t.Parallel()

	task := makeSheinPODLookupTask(
		"task-hash",
		869,
		"SUPPLIER",
		"SKU-123",
		time.Now(),
	)
	task.Result.Shein.Images.MainImage = "https://cdn.example.com/" + strings.Repeat("very-long-path/", 80) + "main.jpg"

	index, ok := store.BuildSheinPODImageLookupIndex(task)
	if !ok {
		t.Fatal("expected task to produce lookup index")
	}
	keys := []struct {
		name string
		got  string
		raw  string
	}{
		{"task id", index.TaskIDLookupKey, task.ID},
		{"product name", index.ProductNameLookupKey, index.ProductName},
		{"supplier code", index.SupplierCodeLookupKey, index.SupplierCode},
		{"seller SKU", index.SellerSKULookupKey, index.SellerSKU},
		{"SHEIN SPU", index.SheinSPUNameLookupKey, index.SheinSPUName},
		{"SHEIN version", index.SheinVersionLookupKey, index.SheinVersion},
		{"AI original URL", index.AIOriginalImageURLLookupKey, index.AIOriginalImageURL},
		{"AI original key", index.AIOriginalImageKeyLookupKey, index.AIOriginalImageKey},
		{"SDS main URL", index.SDSMainImageURLLookupKey, index.SDSMainImageURL},
	}
	for _, key := range keys {
		normalized := sheinpodimage.NormalizeSheinPODImageLookupQueryToken(key.raw)
		sum := sha256.Sum256([]byte(normalized))
		want := hex.EncodeToString(sum[:])
		if key.got != want {
			t.Errorf("%s lookup key = %q, want SHA-256 %q", key.name, key.got, want)
		}
		if len(key.got) != 64 {
			t.Errorf("%s lookup key length = %d, want 64", key.name, len(key.got))
		}
	}
}

func TestTaskRepositoryLookupSheinPODImagesVerifiesNormalizedValueAfterHashMatch(t *testing.T) {
	t.Parallel()

	db, repo := newLookupRepository(t)
	lookupRepo := repo.(sheinpodimage.SheinPODImageLookupRepository)
	ctx := tenantContext("tenant-a")
	task := makeSheinPODLookupTask("task-hash-collision", 869, "SUPPLIER", "SKU-OTHER", time.Now())
	if err := repo.CreateTask(ctx, task); err != nil {
		t.Fatalf("create task: %v", err)
	}
	target := makeSheinPODLookupTask("target-key-source", 869, "SUPPLIER", "SKU-TARGET", time.Now())
	targetIndex, ok := store.BuildSheinPODImageLookupIndex(target)
	if !ok {
		t.Fatal("build target lookup index")
	}
	if err := db.Model(&listingkit.SheinPODImageLookupIndex{}).
		Where("task_id = ?", task.ID).
		Update("seller_sku_lookup_key", targetIndex.SellerSKULookupKey).Error; err != nil {
		t.Fatalf("force colliding lookup key: %v", err)
	}

	items, total, err := lookupRepo.LookupSheinPODImages(ctx, &sheinpodimage.SheinPODImageLookupQuery{
		StoreID: 869,
		Query:   "SKU-TARGET",
	})
	if err != nil {
		t.Fatalf("lookup: %v", err)
	}
	if total != 0 || len(items) != 0 {
		t.Fatalf("hash-only false match items=%+v total=%d", items, total)
	}
}

func TestTaskRepositoryMarkProcessingSynchronizesSheinPODImageLookupIndex(t *testing.T) {
	t.Parallel()

	db, repo := newLookupRepository(t)
	lookupRepo := repo.(sheinpodimage.SheinPODImageLookupRepository)
	ctx := tenantContext("tenant-a")
	base := time.Now().Add(-time.Hour).UTC()
	target := makeSheinPODLookupTask("task-mark-processing", 869, "PROCESSING-SCOPE", "SKU-TARGET", base)
	target.Status = listingkit.TaskStatusPending
	previouslyNewer := makeSheinPODLookupTask("task-previously-newer", 869, "PROCESSING-SCOPE", "SKU-OTHER", base.Add(time.Minute))
	for _, task := range []*listingkit.Task{target, previouslyNewer} {
		if err := repo.CreateTask(ctx, task); err != nil {
			t.Fatalf("CreateTask(%s): %v", task.ID, err)
		}
	}

	if err := repo.MarkProcessing(ctx, target.ID); err != nil {
		t.Fatalf("MarkProcessing: %v", err)
	}
	items, total, err := lookupRepo.LookupSheinPODImages(ctx, &sheinpodimage.SheinPODImageLookupQuery{
		StoreID: 869,
		Query:   "PROCESSING_SCOPE",
		Limit:   2,
	})
	if err != nil || total != 2 || len(items) != 2 {
		t.Fatalf("items=%+v total=%d err=%v", items, total, err)
	}
	if items[0].TaskID != target.ID || items[0].Status != string(listingkit.TaskStatusProcessing) {
		t.Fatalf("first item=%+v, want freshly processing target first", items[0])
	}
	var finalTask listingkit.Task
	if err := db.First(&finalTask, "id = ?", target.ID).Error; err != nil {
		t.Fatalf("load final task: %v", err)
	}
	if !items[0].UpdatedAt.Equal(finalTask.UpdatedAt) {
		t.Fatalf("index updated_at=%s task updated_at=%s", items[0].UpdatedAt, finalTask.UpdatedAt)
	}
}

func TestTaskRepositoryLookupSheinPODImagesHonorsTenantAndOwnerScope(t *testing.T) {
	t.Cleanup(listingkit.SetOwnerScopeRequiredForTesting(true))

	_, repo := newLookupRepository(t)
	lookupRepo := repo.(sheinpodimage.SheinPODImageLookupRepository)
	baseCtx := tenantContext("tenant-a")
	fixtures := []*listingkit.Task{
		makeSheinPODLookupTask("task-tenant-a-user-a", 869, "ACCESS-SCOPE", "SKU-A", time.Now()),
		makeSheinPODLookupTask("task-tenant-a-user-b", 869, "ACCESS-SCOPE", "SKU-B", time.Now()),
		makeSheinPODLookupTask("task-tenant-b-user-a", 869, "ACCESS-SCOPE", "SKU-C", time.Now()),
	}
	fixtures[1].UserID = "user-b"
	fixtures[2].TenantID = "tenant-b"
	for _, task := range fixtures {
		createCtx := baseCtx
		if task.TenantID == "tenant-b" {
			createCtx = tenantContext("tenant-b")
		}
		if err := repo.CreateTask(createCtx, task); err != nil {
			t.Fatalf("CreateTask(%s): %v", task.ID, err)
		}
	}

	userACtx := openaiclient.WithIdentity(
		tenantContext("tenant-a"),
		openaiclient.Identity{TenantID: "tenant-a", UserID: "user-a"},
	)
	items, total, err := lookupRepo.LookupSheinPODImages(userACtx, &sheinpodimage.SheinPODImageLookupQuery{
		StoreID: 869,
		Query:   "ACCESS-SCOPE",
	})
	if err != nil || total != 1 || len(items) != 1 || items[0].TaskID != "task-tenant-a-user-a" {
		t.Fatalf("tenant/owner scoped items=%+v total=%d err=%v", items, total, err)
	}
}

func TestTaskRepositoryLookupSheinPODImagesOrdersAndCapsResults(t *testing.T) {
	t.Parallel()

	_, repo := newLookupRepository(t)
	lookupRepo := repo.(sheinpodimage.SheinPODImageLookupRepository)
	ctx := tenantContext("tenant-a")
	base := time.Now().Add(-time.Hour).UTC()
	for i := 0; i < 55; i++ {
		task := makeSheinPODLookupTask(
			fmt.Sprintf("task-order-%02d", i),
			869,
			"ORDER-SCOPE",
			fmt.Sprintf("SKU-%02d", i),
			base.Add(time.Duration(i)*time.Minute),
		)
		if err := repo.CreateTask(ctx, task); err != nil {
			t.Fatalf("CreateTask(%s): %v", task.ID, err)
		}
	}

	items, total, err := lookupRepo.LookupSheinPODImages(ctx, &sheinpodimage.SheinPODImageLookupQuery{
		StoreID: 869,
		Query:   "ORDER_SCOPE",
		Limit:   100,
	})
	if err != nil || total != 55 || len(items) != 50 {
		t.Fatalf("items length=%d total=%d err=%v, want 50/55", len(items), total, err)
	}
	if items[0].TaskID != "task-order-54" || items[49].TaskID != "task-order-05" {
		t.Fatalf("ordered bounds=%s..%s, want task-order-54..task-order-05", items[0].TaskID, items[49].TaskID)
	}
}

func TestTaskRepositorySynchronizesSheinPODImageLookupIndexOnResultUpdates(t *testing.T) {
	t.Parallel()

	t.Run("updateTaskFields", func(t *testing.T) {
		db, repo := newLookupRepository(t)
		lookupRepo := repo.(sheinpodimage.SheinPODImageLookupRepository)
		ctx := tenantContext("tenant-a")
		task := makeSheinPODLookupTask("task-update-fields", 869, "SUPPLIER", "SKU-OLD", time.Now())
		result := task.Result
		task.Result = nil
		if err := repo.CreateTask(ctx, task); err != nil {
			t.Fatal(err)
		}
		assertLookupCount(t, lookupRepo, ctx, "SKUOLD", 0)
		if err := repo.SaveTaskResult(ctx, task.ID, result); err != nil {
			t.Fatalf("SaveTaskResult: %v", err)
		}
		assertLookupCount(t, lookupRepo, ctx, "SKUOLD", 1)

		var index listingkit.SheinPODImageLookupIndex
		if err := db.First(&index, "task_id = ?", task.ID).Error; err != nil {
			t.Fatalf("load index: %v", err)
		}
		if err := repo.SaveTaskResult(ctx, task.ID, nil); err != nil {
			t.Fatalf("clear task result: %v", err)
		}
		assertLookupCount(t, lookupRepo, ctx, "SKUOLD", 0)
		if err := db.First(&index, "task_id = ?", task.ID).Error; !errors.Is(err, gorm.ErrRecordNotFound) {
			t.Fatalf("load deleted index error=%v, want record not found", err)
		}
	})

	t.Run("MutateTaskResult", func(t *testing.T) {
		_, repo := newLookupRepository(t)
		lookupRepo := repo.(sheinpodimage.SheinPODImageLookupRepository)
		mutationRepo := repo.(interface {
			MutateTaskResult(context.Context, string, listingkit.TaskResultMutation) (*listingkit.Task, error)
		})
		ctx := tenantContext("tenant-a")
		task := makeSheinPODLookupTask("task-mutate-result", 869, "SUPPLIER", "SKU-OLD", time.Now())
		if err := repo.CreateTask(ctx, task); err != nil {
			t.Fatal(err)
		}
		_, err := mutationRepo.MutateTaskResult(ctx, task.ID, func(task *listingkit.Task) error {
			task.Result.Shein.RequestDraft.SKCList[0].SKUList[0].SupplierSKU = "SKU-NEW"
			task.Result.Shein.Images.Gallery = []string{"https://cdn.sdspod.com/refreshed-gallery.jpg"}
			return nil
		})
		if err != nil {
			t.Fatalf("MutateTaskResult: %v", err)
		}
		assertLookupCount(t, lookupRepo, ctx, "SKUOLD", 0)
		assertLookupCount(t, lookupRepo, ctx, "SKUNEW", 1)
		items, _, err := lookupRepo.LookupSheinPODImages(ctx, &sheinpodimage.SheinPODImageLookupQuery{
			StoreID: 869,
			Query:   "SKUNEW",
		})
		if err != nil || len(items) != 1 || len(items[0].SDSGalleryImageURLs) != 1 ||
			items[0].SDSGalleryImageURLs[0] != "https://cdn.sdspod.com/refreshed-gallery.jpg" {
			t.Fatalf("updated gallery items=%+v err=%v", items, err)
		}
	})

	t.Run("ReplaceTaskSDSOptionsForRetry", func(t *testing.T) {
		db, repo := newLookupRepository(t)
		repairRepo := repo.(interface {
			ReplaceTaskSDSOptionsForRetry(context.Context, string, *listingkit.SDSSyncOptions, listingkit.PodExecutionAuditEvent) (*listingkit.Task, error)
		})
		ctx := tenantContext("tenant-a")
		task := makeSheinPODLookupTask("task-replace-sds", 869, "SUPPLIER", "SKU-REPAIR", time.Now().Add(-time.Hour))
		task.Request.Options = &listingkit.GenerateOptions{SDS: &listingkit.SDSSyncOptions{ProductName: "old"}}
		task.Result.ChildTasks = []listingkit.ChildTaskState{{Kind: "sds_design_sync", Status: string(listingkit.TaskStatusFailed)}}
		if err := repo.CreateTask(ctx, task); err != nil {
			t.Fatal(err)
		}
		if _, err := repairRepo.ReplaceTaskSDSOptionsForRetry(
			ctx,
			task.ID,
			&listingkit.SDSSyncOptions{ProductName: "new"},
			listingkit.PodExecutionAuditEvent{Kind: "repair"},
		); err != nil {
			t.Fatalf("ReplaceTaskSDSOptionsForRetry: %v", err)
		}

		var finalTask listingkit.Task
		if err := db.First(&finalTask, "id = ?", task.ID).Error; err != nil {
			t.Fatalf("load final task: %v", err)
		}
		var index listingkit.SheinPODImageLookupIndex
		if err := db.First(&index, "task_id = ?", task.ID).Error; err != nil {
			t.Fatalf("load final index: %v", err)
		}
		if !index.UpdatedAt.Equal(finalTask.UpdatedAt) {
			t.Fatalf("index updated_at=%s task updated_at=%s", index.UpdatedAt, finalTask.UpdatedAt)
		}
	})
}

func TestSheinPODImageLookupQueryScopeDoesNotScanRawTaskJSON(t *testing.T) {
	t.Parallel()

	db, err := gorm.Open(sqlite.Dialector{DriverName: "sqlite", DSN: ":memory:"}, &gorm.Config{DryRun: true})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}

	var indexes []listingkit.SheinPODImageLookupIndex
	stmt := db.Model(&listingkit.SheinPODImageLookupIndex{})
	stmt = store.ApplySheinPODImageLookupStoreScopeForTest(stmt, 869)
	stmt = store.ApplySheinPODImageLookupQueryScopeForTest(stmt, "JJ0531027001-V34576-TFCBA9C")
	statement := stmt.Find(&indexes).Statement

	sql := strings.ToLower(statement.SQL.String())
	for _, forbidden := range []string{
		"listing_kit_tasks",
		"request",
		"result",
		"json_extract",
		"::jsonb",
		" ilike ",
	} {
		if strings.Contains(sql, forbidden) {
			t.Fatalf("lookup SQL contains slow raw JSON text predicate %q: %s", forbidden, sql)
		}
	}
	for _, required := range []string{"listingkit_shein_pod_image_indexes", "store_id", "seller_sku_lookup_key"} {
		if !strings.Contains(sql, required) {
			t.Fatalf("lookup SQL does not contain %q: %s", required, sql)
		}
	}
	if !strings.Contains(sql, "normalized_seller_sku") {
		t.Fatalf("lookup SQL does not verify normalized value after hash match: %s", sql)
	}
}

func newLookupRepository(t *testing.T) (*gorm.DB, listingkit.Repository) {
	t.Helper()

	db, err := gorm.Open(sqlite.Dialector{DriverName: "sqlite", DSN: ":memory:"}, &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&listingkit.Task{}, &listingkit.SheinPODImageLookupIndex{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	return db, store.NewTaskRepository(db)
}

func tenantContext(tenantID string) context.Context {
	return listingkit.WithTenantID(context.Background(), tenantID)
}

func assertLookupCount(
	t *testing.T,
	repo sheinpodimage.SheinPODImageLookupRepository,
	ctx context.Context,
	query string,
	want int64,
) {
	t.Helper()

	items, total, err := repo.LookupSheinPODImages(ctx, &sheinpodimage.SheinPODImageLookupQuery{
		StoreID: 869,
		Query:   query,
	})
	if err != nil || total != want || int64(len(items)) != want {
		t.Fatalf("query=%q items=%+v total=%d err=%v, want %d", query, items, total, err, want)
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
