# PAY-013 ListingKit Asset Isolation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make ListingKit uploads private, tenant-owned, decoded-image assets that can be read or deleted only through authenticated ListingKit paths.

**Architecture:** API paths expose a random upload ID; tenant-scoped metadata maps it to a private object key. The service validates bytes before storage, compensates failed metadata writes, and streams reads only after an ownership lookup. Studio uses owned bytes/data URLs instead of public bucket URLs.

**Tech Stack:** Go 1.24, Gin, GORM, AWS SDK v2 S3, `golang.org/x/image/webp`, Next.js App Router, Vitest.

## Global Constraints

- Scope is ListingKit uploads, metadata, local/S3 adapters, Studio consumers, and ListingKit UI proxy/thumbnail behavior only.
- Authenticated ListingKit context is the sole tenant authority.
- Object keys are `listingkit/tenants/<legacy-tenant-id>/uploads/<uuid>.<extension>`.
- Object stores return no public URL; errors never disclose bucket name, object key, or cross-tenant detail.
- Accept only decoded JPEG, PNG, GIF, and WebP; derive MIME and extension from bytes, not multipart metadata.
- Keep the existing 12 MiB limit and reject empty payloads before storage.
- Missing, malformed, or foreign upload IDs are `404 uploaded_image_not_found`; invalid images are `400 invalid_request`.
- Repeated DELETE is successful but refunds storage only on the first completed deletion.
- Do not change non-ListingKit publishers, unrelated S3 clients, browser direct-to-S3 upload, or pricing policy.

---

## File Structure

- `internal/listingkit/upload_model.go`, `upload_metadata_repository.go`: opaque IDs, tenant-scoped metadata, deletion states.
- `internal/listingkit/upload_validation.go`: decode-based validation shared by upload/read paths.
- `internal/listingkit/upload_{local,s3,fallback}_store.go`: private key write/open/delete adapters.
- `internal/listingkit/service_upload_logic.go`: atomic upload/read/delete orchestration.
- `internal/listingkit/task_studio_media_service*.go`: byte/data-URL Studio handoff.
- `internal/listingkit/api/upload_handler.go`: stable error/refund response contract.
- `web/listingkit-ui/src/lib/utils/{image-proxy-url,imgproxy-url}.ts`: protected-route thumbnail behavior.

### Task 1: Make uploaded-image metadata the tenant-scoped opaque-ID authority

**Files:**

- Modify: `internal/listingkit/upload_model.go`
- Modify: `internal/listingkit/upload_metadata_repository.go`
- Modify: `internal/listingkit/upload_metadata_repository_test.go`

**Interfaces:**

- Produces `UploadedImageRecord{UploadID, StorageKey, TenantID, ContentType, Size, DeleteState}`.
- Produces `GetUploadedImage(ctx context.Context, uploadID string) (*UploadedImageRecord, error)`.
- Produces `ClaimUploadedImageDeletion(ctx context.Context, uploadID string) (*UploadedImageDeletionClaim, error)`, `CompleteUploadedImageDeletion`, and `ReleaseUploadedImageDeletion`.

- [ ] **Step 1: Write the failing ownership and deletion-claim tests**

```go
func TestGormUploadedImageRepositoryHidesForeignAndMalformedUploadIDs(t *testing.T) {
	ctxA := tenantctx.WithTenantID(context.Background(), "101")
	ctxB := tenantctx.WithTenantID(context.Background(), "202")
	_ = repo.SaveUploadedImage(ctxA, &UploadedImageRecord{UploadID: "b4b7d3a5-5d06-4f2c-a13e-2735e9e963d5", StorageKey: "listingkit/tenants/101/uploads/b4b7d3a5-5d06-4f2c-a13e-2735e9e963d5.png"})
	if _, err := repo.GetUploadedImage(ctxB, "b4b7d3a5-5d06-4f2c-a13e-2735e9e963d5"); !errors.Is(err, ErrUploadedImageNotFound) { t.Fatalf("foreign lookup = %v", err) }
	if _, err := repo.GetUploadedImage(ctxA, "../secret"); !errors.Is(err, ErrUploadedImageNotFound) { t.Fatalf("malformed lookup = %v", err) }
}

func TestGormUploadedImageRepositoryClaimsOnlyOneDeletion(t *testing.T) {
	claim, err := repo.ClaimUploadedImageDeletion(ctx, uploadID)
	if err != nil || !claim.Claimed || claim.AlreadyDeleted { t.Fatalf("first claim = %#v, %v", claim, err) }
	if err := repo.CompleteUploadedImageDeletion(ctx, uploadID); err != nil { t.Fatal(err) }
	again, err := repo.ClaimUploadedImageDeletion(ctx, uploadID)
	if err != nil || again.Claimed || !again.AlreadyDeleted { t.Fatalf("second claim = %#v, %v", again, err) }
}
```

- [ ] **Step 2: Run the focused tests and verify they fail**

Run: `$env:GOWORK='off'; go test ./internal/listingkit -run 'TestGormUploadedImageRepository(HidesForeignAndMalformedUploadIDs|ClaimsOnlyOneDeletion)' -count=1`

Expected: FAIL because `UploadID`, `StorageKey`, and deletion-claim methods do not exist.

- [ ] **Step 3: Implement the record and repository state machine**

```go
type UploadedImageRecord struct {
	ID          int64  `gorm:"primaryKey"`
	TenantID    string `gorm:"type:varchar(128);not null;uniqueIndex:idx_listingkit_uploaded_image_tenant_upload_id,priority:1"`
	UploadID    string `gorm:"type:char(36);not null;uniqueIndex:idx_listingkit_uploaded_image_tenant_upload_id,priority:2"`
	StorageKey  string `gorm:"type:varchar(512);not null;uniqueIndex"`
	ContentType string `gorm:"type:varchar(128);not null"`
	Size        int64  `gorm:"not null"`
	DeleteState string `gorm:"type:varchar(16);not null;default:active"`
}

type UploadedImageDeletionClaim struct {
	Record *UploadedImageRecord
	Claimed bool
	AlreadyDeleted bool
}
```

Use `uuid.Parse` before querying. In memory and GORM repositories, scope every lookup and state transition by `tenantctx.TenantIDFromContext(ctx)` plus `upload_id`. Atomically claim only `active` records by setting `deleting`; return `AlreadyDeleted: true` for the same tenant's `deleting` or `deleted` row, and `ErrUploadedImageNotFound` when no row belongs to the tenant. `Release...` restores only `deleting` to `active`.

- [ ] **Step 4: Run repository tests**

Run: `$env:GOWORK='off'; go test ./internal/listingkit -run 'TestGormUploadedImageRepository' -count=1`

Expected: PASS.

- [ ] **Step 5: Commit the persistence boundary**

```bash
git add internal/listingkit/upload_model.go internal/listingkit/upload_metadata_repository.go internal/listingkit/upload_metadata_repository_test.go
git commit -m "feat: scope listingkit uploaded image metadata"
```

### Task 2: Validate decoded images and write only private tenant keys

**Files:**

- Create: `internal/listingkit/upload_validation.go`
- Create: `internal/listingkit/upload_validation_test.go`
- Modify: `internal/listingkit/upload_store.go`
- Modify: `internal/listingkit/upload_local_store.go`
- Modify: `internal/listingkit/upload_local_store_test.go`
- Modify: `internal/listingkit/upload_s3_store.go`
- Modify: `internal/listingkit/upload_s3_store_test.go`
- Modify: `internal/listingkit/upload_fallback_store.go`
- Modify: `internal/listingkit/httpapi/builders_image_store.go`
- Modify: `go.mod`
- Modify: `go.sum`

**Interfaces:**

- Produces `validateUploadedImage(input ImageUploadInput) (validatedUploadedImage, error)` with byte-derived MIME and extension.
- Changes store writes to `Save(ctx context.Context, storageKey string, input *ImageUploadInput) (*StoredUploadedImage, error)`.

- [ ] **Step 1: Write failing validator and storage-adapter tests**

```go
func TestValidateUploadedImageRejectsMismatchedMultipartContentType(t *testing.T) {
	_, err := validateUploadedImage(ImageUploadInput{Filename: "photo.jpg", ContentType: "image/jpeg", Data: []byte("not an image")})
	if err == nil || !strings.Contains(err.Error(), "invalid image") { t.Fatalf("error = %v", err) }
}

func TestS3ImageUploadStoreSavesPrivateTenantScopedKeyWithoutPublicURL(t *testing.T) {
	key := "listingkit/tenants/227/uploads/59fb51a1-b0bc-4ca3-9180-cb92947e8d6e.png"
	file, err := store.Save(context.Background(), key, &ImageUploadInput{Filename: "shirt.png", Data: validPNG})
	if err != nil { t.Fatal(err) }
	if file.Key != key || file.PublicURL != "" || uploader.lastKey != key { t.Fatalf("file = %#v, key = %q", file, uploader.lastKey) }
}
```

- [ ] **Step 2: Run the focused tests and verify they fail**

Run: `$env:GOWORK='off'; go test ./internal/listingkit -run 'Test(ValidateUploadedImage|S3ImageUploadStoreSavesPrivateTenantScopedKeyWithoutPublicURL)' -count=1`

Expected: FAIL because the validator and keyed store contract do not exist and S3 still emits `PublicURL`.

- [ ] **Step 3: Implement decode validation and private adapters**

```go
const MaxListingKitUploadBytes = 12 << 20

func validateUploadedImage(input ImageUploadInput) (validatedUploadedImage, error) {
	if len(input.Data) == 0 || len(input.Data) > MaxListingKitUploadBytes { return validatedUploadedImage{}, errInvalidUploadedImage }
	config, format, err := image.DecodeConfig(bytes.NewReader(input.Data))
	if err != nil || config.Width < 1 || config.Height < 1 { return validatedUploadedImage{}, errInvalidUploadedImage }
	if _, _, err = image.Decode(bytes.NewReader(input.Data)); err != nil { return validatedUploadedImage{}, errInvalidUploadedImage }
	contentType, extension, ok := uploadedImageFormat(format)
	if !ok { return validatedUploadedImage{}, errInvalidUploadedImage }
	return validatedUploadedImage{ContentType: contentType, Extension: extension}, nil
}
```

Register WebP with `_ "golang.org/x/image/webp"`. The service allocates the storage key; local and S3 stores only normalize and enforce that supplied tenant key. Remove `PublicBase` from `S3ImageUploadStoreConfig`, remove `storage.ResolveObjectURL`, leave `StoredUploadedImage.PublicURL` empty, and forward the supplied key through the fallback store. Keep S3 `PutObject` free of ACL fields and remove ListingKit's `PublicBase` construction from `buildS3ImageUploadStore`.

- [ ] **Step 4: Run adapter and validator tests**

Run: `$env:GOWORK='off'; go test ./internal/listingkit ./internal/listingkit/httpapi -run 'Test(ValidateUploadedImage|LocalImageUploadStore|S3ImageUploadStore|BuildImageUploadStore)' -count=1`

Expected: PASS.

- [ ] **Step 5: Commit validation and private storage**

```bash
git add go.mod go.sum internal/listingkit/upload_validation.go internal/listingkit/upload_validation_test.go internal/listingkit/upload_store.go internal/listingkit/upload_local_store.go internal/listingkit/upload_local_store_test.go internal/listingkit/upload_s3_store.go internal/listingkit/upload_s3_store_test.go internal/listingkit/upload_fallback_store.go internal/listingkit/httpapi/builders_image_store.go
git commit -m "feat: store listingkit uploads privately"
```

### Task 3: Make upload, read, and delete atomic at the service boundary

**Files:**

- Modify: `internal/listingkit/service_upload_logic.go`
- Modify: `internal/listingkit/upload_metadata_service_test.go`
- Modify: `internal/listingkit/workflow_assets_test.go`
- Modify: `internal/listingkit/workflow_sds_sync_uploaded_support.go`

**Interfaces:**

- Consumes Task 1's repository and Task 2's keyed store.
- Produces image URLs only through `buildUploadedImagePath(uploadID string)`.
- Produces `DeletedUploadedImage{Key string, Size int64, AlreadyDeleted bool}`.

- [ ] **Step 1: Write failing service tests**

```go
func TestUploadImagesDeletesObjectWhenMetadataSaveFails(t *testing.T) {
	store := &stubMetadataImageUploadStore{saveResult: &StoredUploadedImage{Key: "listingkit/tenants/227/uploads/id.png", Size: 3}}
	svc := newUploadService(store, failingUploadedImageRepository{saveErr: errors.New("db down")})
	if _, err := svc.UploadImages(tenantctx.WithTenantID(context.Background(), "227"), &UploadImagesRequest{Files: []ImageUploadInput{{Filename: "a.png", Data: validPNG}}}); err == nil { t.Fatal("UploadImages() error = nil") }
	if store.deletedKey != "listingkit/tenants/227/uploads/id.png" { t.Fatalf("rollback key = %q", store.deletedKey) }
}

func TestGetUploadedImageDoesNotOpenForeignObject(t *testing.T) {
	_, err := svc.GetUploadedImage(tenantctx.WithTenantID(context.Background(), "202"), ownerUploadID)
	if !errors.Is(err, ErrUploadedImageNotFound) || store.openCalls != 0 { t.Fatalf("err=%v openCalls=%d", err, store.openCalls) }
}

func TestDeleteUploadedImageIsIdempotent(t *testing.T) {
	first, _ := svc.DeleteUploadedImage(ctx, uploadID)
	second, _ := svc.DeleteUploadedImage(ctx, uploadID)
	if first.AlreadyDeleted || !second.AlreadyDeleted || store.deleteCalls != 1 { t.Fatalf("first=%#v second=%#v calls=%d", first, second, store.deleteCalls) }
}
```

- [ ] **Step 2: Run the focused tests and verify they fail**

Run: `$env:GOWORK='off'; go test ./internal/listingkit -run 'Test(UploadImagesDeletesObjectWhenMetadataSaveFails|GetUploadedImageDoesNotOpenForeignObject|DeleteUploadedImageIsIdempotent)' -count=1`

Expected: FAIL because the current service opens by caller key, ignores metadata-save errors, and deletes twice.

- [ ] **Step 3: Implement the service transaction ordering**

```go
validated, err := validateUploadedImage(file)

uploadID := uuid.NewString()

legacyTenantID, err := tenantbridge.ResolveLegacyTenantID(ctx, tenantctx.TenantIDFromContext(ctx))

storageKey := fmt.Sprintf("listingkit/tenants/%d/uploads/%s%s", legacyTenantID, uploadID, validated.Extension)

stored, err := uploadStore.Save(ctx, storageKey, &file)

if err := uploadedImageRepo.SaveUploadedImage(ctx, recordFrom(uploadID, stored, validated)); err != nil {
	return nil, errors.Join(fmt.Errorf("save uploaded image metadata: %w", err), uploadStore.Delete(ctx, storageKey))
}

response.ImageURLs = append(response.ImageURLs, buildUploadedImagePath(uploadID))
```

On reads, parse the opaque ID, query metadata first, open only `record.StorageKey`, re-run decode validation, and return stored validated MIME. On deletes, claim before deleting `record.StorageKey`; treat object `ErrUploadedImageNotFound` as cleanup success, release the claim on another storage error, then complete it. A tombstone returns `AlreadyDeleted: true` without opening/deleting an object. Update all SDS upload-path tests to extract the opaque ID, not a filesystem/S3 key.

- [ ] **Step 4: Run service and workflow tests**

Run: `$env:GOWORK='off'; go test ./internal/listingkit -run 'Test(UploadImages|GetUploadedImage|DeleteUploadedImage|SyncSDSDesign)' -count=1`

Expected: PASS.

- [ ] **Step 5: Commit the media-service boundary**

```bash
git add internal/listingkit/service_upload_logic.go internal/listingkit/upload_metadata_service_test.go internal/listingkit/workflow_assets_test.go internal/listingkit/workflow_sds_sync_uploaded_support.go
git commit -m "feat: enforce listingkit asset ownership"
```

### Task 4: Keep Studio model inputs private

**Files:**

- Modify: `internal/listingkit/task_studio_media_service.go`
- Modify: `internal/listingkit/task_studio_media_service_support.go`
- Modify: `internal/listingkit/service_studio_session_wiring_support.go`
- Modify: `internal/listingkit/ai_contracts.go`
- Modify: `internal/listingkit/httpapi/ai_image_generator_adapter.go`
- Modify: `internal/listingkit/studio_reference_analysis.go`
- Modify: `internal/listingkit/studio_reference_analysis_test.go`
- Modify: `internal/listingkit/studio_prompts_test.go`

**Interfaces:**

- Consumes Task 3's `GetUploadedImage(ctx, uploadID)`.
- Produces `AIImageEditRequest{ImageData []byte, ImageContentType string}` for owned uploads.

- [ ] **Step 1: Write failing private-handoff tests**

```go
func TestStudioReferenceAnalysisUsesDataURLForOwnedUpload(t *testing.T) {
	svc := newTaskStudioMediaService(taskStudioMediaServiceConfig{loadUploadedImage: func(context.Context, string) (*UploadedImageFile, error) {
		return &UploadedImageFile{ContentType: "image/png", Data: validPNG}, nil
	}, promptDiversifier: completer})
	_, err := svc.AnalyzeStudioReferenceStyle(ctx, &StudioReferenceAnalysisRequest{ReferenceImageURLs: []string{"/api/v1/listing-kits/uploads/files/" + uploadID}})
	if err != nil || !strings.HasPrefix(completer.calls[0], "data:image/png;base64,") { t.Fatalf("calls=%#v err=%v", completer.calls, err) }
}

func TestStudioEditPassesOwnedUploadBytesInsteadOfObjectURL(t *testing.T) {
	_, _ = svc.GenerateStudioDesigns(ctx, requestWithOwnedUpload(uploadID))
	if len(generator.lastEdit.ImageData) == 0 || generator.lastEdit.ImageURL != "" { t.Fatalf("edit request=%#v", generator.lastEdit) }
}
```

- [ ] **Step 2: Run the focused tests and verify they fail**

Run: `$env:GOWORK='off'; go test ./internal/listingkit ./internal/listingkit/httpapi -run 'TestStudio(ReferenceAnalysisUsesDataURLForOwnedUpload|EditPassesOwnedUploadBytesInsteadOfObjectURL)' -count=1`

Expected: FAIL because Studio resolves `PublicURL` and has no byte payload.

- [ ] **Step 3: Implement bytes/data-URL handoff**

```go
type AIImageEditRequest struct {
	Model string
	Prompt string
	ImageData []byte
	ImageContentType string
	ImageURL string
	ImageURLs []string
	Size string
	ResponseFormat string
	N int
}

func uploadedImageDataURL(file *UploadedImageFile) string {
	return "data:" + file.ContentType + ";base64," + base64.StdEncoding.EncodeToString(file.Data)
}
```

Replace `resolveUploadedImagePublicURL` wiring with a tenant-scoped loader that calls `service.GetUploadedImage`. For ListingKit upload paths, pass `ImageData` through the ListingKit/OpenAI adapter for synchronous edits and a data URL to vision analysis. Preserve user-supplied remote HTTPS behavior. Async providers that require remote URLs return a stable `invalid request` error for owned uploads rather than exposing an object URL.

- [ ] **Step 4: Run Studio and adapter tests**

Run: `$env:GOWORK='off'; go test ./internal/listingkit ./internal/listingkit/httpapi -run 'Test(Studio|ListingKitAIImageGenerator)' -count=1`

Expected: PASS.

- [ ] **Step 5: Commit private Studio handoff**

```bash
git add internal/listingkit/task_studio_media_service.go internal/listingkit/task_studio_media_service_support.go internal/listingkit/service_studio_session_wiring_support.go internal/listingkit/ai_contracts.go internal/listingkit/httpapi/ai_image_generator_adapter.go internal/listingkit/studio_reference_analysis.go internal/listingkit/studio_reference_analysis_test.go internal/listingkit/studio_prompts_test.go
git commit -m "feat: keep listingkit studio upload inputs private"
```

### Task 5: Enforce the HTTP contract and protected UI delivery

**Files:**

- Modify: `internal/listingkit/api/upload_file_reader.go`
- Modify: `internal/listingkit/api/upload_handler.go`
- Modify: `internal/listingkit/api/upload_handler_test.go`
- Modify: `internal/app/httpapi/server_test.go`
- Modify: `web/listingkit-ui/src/lib/utils/image-proxy-url.test.ts`
- Modify: `web/listingkit-ui/src/lib/utils/imgproxy-url.test.ts`
- Modify: `docs/product/listingkit-paid-pilot-execution-plan.md`

**Interfaces:**

- Consumes `DeletedUploadedImage.AlreadyDeleted` from Task 3.
- Produces `404 {"error":"uploaded_image_not_found"}` for missing, malformed, and foreign GET/DELETE IDs.
- Produces protected browser URLs beginning `/api/listing-kits/uploads/files/`; no ListingKit upload thumbnail becomes an `s3://` imgproxy source.

- [ ] **Step 1: Write failing API and UI regression tests**

```go
func TestDeleteUploadedListingKitImageRefundsOnlyFirstCompletedDelete(t *testing.T) {
	svc.deletedUploadedImage = &listingkit.DeletedUploadedImage{Key: uploadID, Size: 3}
	deleteOnce(router, uploadID)
	svc.deletedUploadedImage = &listingkit.DeletedUploadedImage{Key: uploadID, Size: 3, AlreadyDeleted: true}
	deleteOnce(router, uploadID)
	if usedStorageBytes(t, subscriptionService) != 0 { t.Fatalf("storage bytes refunded twice") }
}

func TestGetUploadedListingKitImageReturnsStableNotFound(t *testing.T) {
	resp := getUploadedImage(router, "not-a-uuid")
	if resp.Code != http.StatusNotFound || !strings.Contains(resp.Body.String(), `"uploaded_image_not_found"`) { t.Fatalf("response=%d %s", resp.Code, resp.Body.String()) }
}
```

```ts
it("keeps ListingKit upload URLs on the authenticated proxy when imgproxy is configured", () => {
  process.env.NEXT_PUBLIC_LISTINGKIT_IMGPROXY_BASE_URL = "https://img.example.test";
  expect(toThumbnailPreviewUrl("/api/v1/listing-kits/uploads/files/0b15bb5e-9f9e-4952-9a06-fd31aab99901", { width: 320, height: 320 }))
    .toBe("/api/listing-kits/uploads/files/0b15bb5e-9f9e-4952-9a06-fd31aab99901");
});
```

- [ ] **Step 2: Run the focused tests and verify they fail**

Run: `$env:GOWORK='off'; go test ./internal/listingkit/api ./internal/app/httpapi -run 'Test(DeleteUploadedListingKitImageRefundsOnlyFirstCompletedDelete|GetUploadedListingKitImageReturnsStableNotFound)' -count=1; npm --prefix web/listingkit-ui test -- --run src/lib/utils/image-proxy-url.test.ts src/lib/utils/imgproxy-url.test.ts`

Expected: FAIL because delete always refunds, the error uses the old code, and thumbnail behavior has no protected-route assertion.

- [ ] **Step 3: Implement stable delivery semantics**

```go
if errors.Is(err, listingkit.ErrUploadedImageNotFound) {
	c.JSON(http.StatusNotFound, gin.H{"error": "uploaded_image_not_found", "message": "uploaded image not found"})
	return
}

if !deleted.AlreadyDeleted && deleted.Size > 0 {
	h.recordSubscriptionUsage(c, listingsubscription.ModuleOSSStorage, "storage_bytes", -int(deleted.Size))
}
```

Change `readUploadedFile` to use `listingkit.MaxListingKitUploadBytes`. Keep the authenticated Next proxy route unchanged; add test coverage proving its existing rewrite wins before generic image proxy/imgproxy processing. Update the PAY-013 checkbox only after Task 6 passes.

- [ ] **Step 4: Run API, route, and UI tests**

Run: `$env:GOWORK='off'; go test ./internal/listingkit/api ./internal/app/httpapi -count=1; npm --prefix web/listingkit-ui test -- --run src/lib/utils/image-proxy-url.test.ts src/lib/utils/imgproxy-url.test.ts src/app/api/listing-kits/proxy-upstream-response.test.ts`

Expected: PASS.

- [ ] **Step 5: Commit the public contract and UI guard**

```bash
git add internal/listingkit/api/upload_file_reader.go internal/listingkit/api/upload_handler.go internal/listingkit/api/upload_handler_test.go internal/app/httpapi/server_test.go web/listingkit-ui/src/lib/utils/image-proxy-url.test.ts web/listingkit-ui/src/lib/utils/imgproxy-url.test.ts docs/product/listingkit-paid-pilot-execution-plan.md
git commit -m "feat: protect listingkit uploaded asset delivery"
```

### Task 6: Verify the complete isolated upload flow

**Files:**

- Modify: `docs/product/listingkit-paid-pilot-execution-plan.md`

**Interfaces:**

- Consumes all preceding tasks.
- Produces repository-validation evidence for PAY-013.

- [ ] **Step 1: Format changed Go sources**

Run: `gofmt -w internal/listingkit/upload_model.go internal/listingkit/upload_metadata_repository.go internal/listingkit/upload_validation.go internal/listingkit/upload_store.go internal/listingkit/upload_local_store.go internal/listingkit/upload_s3_store.go internal/listingkit/upload_fallback_store.go internal/listingkit/service_upload_logic.go internal/listingkit/task_studio_media_service.go internal/listingkit/task_studio_media_service_support.go internal/listingkit/service_studio_session_wiring_support.go internal/listingkit/ai_contracts.go internal/listingkit/studio_reference_analysis.go internal/listingkit/api/upload_file_reader.go internal/listingkit/api/upload_handler.go internal/listingkit/httpapi/ai_image_generator_adapter.go internal/listingkit/httpapi/builders_image_store.go`

Expected: command exits 0 and `git diff --check` has no output.

- [ ] **Step 2: Run complete targeted verification**

Run: `$env:GOWORK='off'; go test ./internal/listingkit ./internal/listingkit/api ./internal/listingkit/httpapi ./internal/app/httpapi -count=1; npm --prefix web/listingkit-ui test -- --run src/lib/utils/image-proxy-url.test.ts src/lib/utils/imgproxy-url.test.ts src/app/api/listing-kits/proxy-upstream-response.test.ts`

Expected: every Go package reports `ok` and Vitest reports all selected tests passed.

- [ ] **Step 3: Record the verified PAY-013 state**

```powershell
git diff --check
git status --short
```

Expected: no whitespace errors; only PAY-013 files and its readiness checkbox are staged.

- [ ] **Step 4: Commit the validation record**

```bash
git add docs/product/listingkit-paid-pilot-execution-plan.md
git commit -m "docs: record listingkit asset isolation validation"
```

## Self-Review

1. **Spec coverage:** Task 1 establishes trusted tenant plus opaque-ID ownership; Task 2 covers tenant keys, private local/S3 storage, MIME/magic/decode/format validation, and the byte limit; Task 3 covers metadata-first reads, compensation, and idempotent delete; Task 4 closes the Studio public-URL escape hatch; Task 5 covers stable HTTP errors, one-time refunds, and the authenticated UI route; Task 6 records final evidence.
2. **Placeholder scan:** Searched for deferred-work markers and vague validation/error-handling wording; none remain.
3. **Type consistency:** API paths carry `UploadID`; repositories resolve `UploadID` to `StorageKey`; stores receive `StorageKey`; `DeletedUploadedImage.AlreadyDeleted` gates handler refunds.
