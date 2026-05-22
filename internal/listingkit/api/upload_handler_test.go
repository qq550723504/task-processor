package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"task-processor/internal/listingkit"
	"task-processor/internal/listingsubscription"
)

func TestUploadListingKitImagesReturnsImageURLs(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	svc := &stubGenerationTaskService{
		uploadResponse: &listingkit.UploadImagesResponse{
			ImageURLs: []string{
				"http://localhost:8080/api/v1/listing-kits/uploads/files/a.jpg",
				"http://localhost:8080/api/v1/listing-kits/uploads/files/b.jpg",
			},
		},
	}
	subscriptionService := activeOSSStorageSubscriptionService(t, nil)
	h, err := NewHandler(svc, WithSubscriptionService(subscriptionService))
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	first, err := writer.CreateFormFile("files", "a.jpg")
	if err != nil {
		t.Fatalf("CreateFormFile() error = %v", err)
	}
	if _, err := first.Write([]byte{0xFF, 0xD8, 0xFF}); err != nil {
		t.Fatalf("write first file: %v", err)
	}
	second, err := writer.CreateFormFile("files", "b.png")
	if err != nil {
		t.Fatalf("CreateFormFile() second error = %v", err)
	}
	if _, err := second.Write([]byte{0x89, 0x50, 0x4E, 0x47}); err != nil {
		t.Fatalf("write second file: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	router := gin.New()
	router.POST("/api/v1/listing-kits/uploads/images", h.UploadListingKitImages)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/listing-kits/uploads/images", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.Code)
	}
	if svc.uploadImagesReq == nil || len(svc.uploadImagesReq.Files) != 2 {
		t.Fatalf("upload req = %+v, want 2 files", svc.uploadImagesReq)
	}
	if svc.uploadImagesReq.Files[0].Filename != "a.jpg" {
		t.Fatalf("first filename = %q, want a.jpg", svc.uploadImagesReq.Files[0].Filename)
	}

	var payload listingkit.UploadImagesResponse
	if err := json.Unmarshal(resp.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	if len(payload.ImageURLs) != 2 {
		t.Fatalf("image_urls = %+v, want 2 URLs", payload.ImageURLs)
	}
	summary, err := subscriptionService.GetSummary(t.Context(), listingkit.DefaultTenantID)
	if err != nil {
		t.Fatalf("get subscription summary: %v", err)
	}
	for _, view := range summary.Entitlements {
		if view.Module.Code == listingsubscription.ModuleOSSStorage {
			if view.Used["storage_bytes"] != 7 {
				t.Fatalf("storage_bytes = %d, want 7", view.Used["storage_bytes"])
			}
			if view.Used["uploaded_bytes"] != 7 {
				t.Fatalf("uploaded_bytes = %d, want 7", view.Used["uploaded_bytes"])
			}
			return
		}
	}
	t.Fatal("oss storage entitlement view missing")
}

func TestUploadListingKitImagesReturnsBadRequestWithoutFiles(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	svc := &stubGenerationTaskService{}
	h, err := NewHandler(svc, WithSubscriptionService(activeOSSStorageSubscriptionService(t, nil)))
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	if err := writer.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	router := gin.New()
	router.POST("/api/v1/listing-kits/uploads/images", h.UploadListingKitImages)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/listing-kits/uploads/images", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.Code)
	}
}

func TestUploadListingKitImagesReturnsQuotaExceededWhenStorageLimitIsExceeded(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	svc := &stubGenerationTaskService{
		uploadResponse: &listingkit.UploadImagesResponse{
			ImageURLs: []string{"/api/v1/listing-kits/uploads/files/large.jpg"},
		},
	}
	subscriptionService := activeOSSStorageSubscriptionService(t, map[string]int{"storage_bytes": 3})
	h, err := NewHandler(svc, WithSubscriptionService(subscriptionService))
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	file, err := writer.CreateFormFile("files", "large.jpg")
	if err != nil {
		t.Fatalf("CreateFormFile() error = %v", err)
	}
	if _, err := file.Write([]byte{0xFF, 0xD8, 0xFF, 0x00}); err != nil {
		t.Fatalf("write file: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	router := gin.New()
	router.POST("/api/v1/listing-kits/uploads/images", h.UploadListingKitImages)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/listing-kits/uploads/images", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusPaymentRequired {
		t.Fatalf("status = %d, want 402: %s", resp.Code, resp.Body.String())
	}
	if svc.uploadImagesReq != nil {
		t.Fatalf("upload req = %+v, want service not to be called", svc.uploadImagesReq)
	}

	summary, err := subscriptionService.GetSummary(t.Context(), listingkit.DefaultTenantID)
	if err != nil {
		t.Fatalf("get subscription summary: %v", err)
	}
	for _, view := range summary.Entitlements {
		if view.Module.Code == listingsubscription.ModuleOSSStorage {
			if view.Used["storage_bytes"] != 0 {
				t.Fatalf("storage_bytes = %d, want 0", view.Used["storage_bytes"])
			}
			if view.Used["uploaded_bytes"] != 0 {
				t.Fatalf("uploaded_bytes = %d, want 0", view.Used["uploaded_bytes"])
			}
			return
		}
	}
	t.Fatal("oss storage entitlement view missing")
}

func TestUploadListingKitImagesAllowsStudioBackedStorageFallback(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	svc := &stubGenerationTaskService{
		uploadResponse: &listingkit.UploadImagesResponse{
			ImageURLs: []string{"/api/v1/listing-kits/uploads/files/style.jpg"},
		},
	}
	subscriptionService := activeStudioOnlySubscriptionService(t)
	h, err := NewHandler(svc, WithSubscriptionService(subscriptionService))
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	file, err := writer.CreateFormFile("files", "style.jpg")
	if err != nil {
		t.Fatalf("CreateFormFile() error = %v", err)
	}
	if _, err := file.Write([]byte{0xFF, 0xD8, 0xFF, 0x00}); err != nil {
		t.Fatalf("write file: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	router := gin.New()
	router.POST("/api/v1/listing-kits/uploads/images", h.UploadListingKitImages)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/listing-kits/uploads/images", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", resp.Code, resp.Body.String())
	}
}

func TestUploadListingKitImagesDoesNotRecordStorageUsageWhenUploadFails(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	svc := &stubGenerationTaskService{err: errors.New("store unavailable")}
	subscriptionService := activeOSSStorageSubscriptionService(t, map[string]int{"storage_bytes": 1024})
	h, err := NewHandler(svc, WithSubscriptionService(subscriptionService))
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	file, err := writer.CreateFormFile("files", "a.jpg")
	if err != nil {
		t.Fatalf("CreateFormFile() error = %v", err)
	}
	if _, err := file.Write([]byte{0xFF, 0xD8, 0xFF}); err != nil {
		t.Fatalf("write file: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	router := gin.New()
	router.POST("/api/v1/listing-kits/uploads/images", h.UploadListingKitImages)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/listing-kits/uploads/images", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", resp.Code)
	}
	summary, err := subscriptionService.GetSummary(t.Context(), listingkit.DefaultTenantID)
	if err != nil {
		t.Fatalf("get subscription summary: %v", err)
	}
	for _, view := range summary.Entitlements {
		if view.Module.Code == listingsubscription.ModuleOSSStorage {
			if view.Used["storage_bytes"] != 0 || view.Used["uploaded_bytes"] != 0 {
				t.Fatalf("usage = %#v, want no recorded bytes", view.Used)
			}
			return
		}
	}
	t.Fatal("oss storage entitlement view missing")
}

func TestDeleteUploadedListingKitImageDecrementsStorageUsage(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	svc := &stubGenerationTaskService{
		deletedUploadedImage: &listingkit.DeletedUploadedImage{
			Key:  "20260515/a.jpg",
			Size: 3,
		},
	}
	subscriptionService := activeOSSStorageSubscriptionService(t, nil)
	if _, err := subscriptionService.RecordUsage(t.Context(), listingkit.DefaultTenantID, listingsubscription.ModuleOSSStorage, "storage_bytes", 3); err != nil {
		t.Fatalf("seed storage usage: %v", err)
	}
	if _, err := subscriptionService.RecordUsage(t.Context(), listingkit.DefaultTenantID, listingsubscription.ModuleOSSStorage, "uploaded_bytes", 3); err != nil {
		t.Fatalf("seed uploaded usage: %v", err)
	}
	h, err := NewHandler(svc, WithSubscriptionService(subscriptionService))
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	router := gin.New()
	router.DELETE("/api/v1/listing-kits/uploads/files/*key", h.DeleteUploadedListingKitImage)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/listing-kits/uploads/files/20260515/a.jpg", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", resp.Code, resp.Body.String())
	}
	if svc.deletedUploadedImageKey != "20260515/a.jpg" {
		t.Fatalf("deleted key = %q, want 20260515/a.jpg", svc.deletedUploadedImageKey)
	}
	summary, err := subscriptionService.GetSummary(t.Context(), listingkit.DefaultTenantID)
	if err != nil {
		t.Fatalf("get subscription summary: %v", err)
	}
	for _, view := range summary.Entitlements {
		if view.Module.Code == listingsubscription.ModuleOSSStorage {
			if view.Used["storage_bytes"] != 0 {
				t.Fatalf("storage_bytes = %d, want 0", view.Used["storage_bytes"])
			}
			if view.Used["uploaded_bytes"] != 3 {
				t.Fatalf("uploaded_bytes = %d, want 3", view.Used["uploaded_bytes"])
			}
			return
		}
	}
	t.Fatal("oss storage entitlement view missing")
}

func TestDeleteUploadedListingKitImageReturnsNotFound(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	svc := &stubGenerationTaskService{err: listingkit.ErrUploadedImageNotFound}
	h, err := NewHandler(svc, WithSubscriptionService(activeOSSStorageSubscriptionService(t, nil)))
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	router := gin.New()
	router.DELETE("/api/v1/listing-kits/uploads/files/*key", h.DeleteUploadedListingKitImage)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/listing-kits/uploads/files/missing.jpg", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", resp.Code)
	}
}

func TestGetUploadedListingKitImageReturnsFile(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	svc := &stubGenerationTaskService{
		uploadedImageFile: &listingkit.UploadedImageFile{
			Filename:    "shirt.jpg",
			ContentType: "image/jpeg",
			Data:        []byte{0xFF, 0xD8, 0xFF},
		},
	}
	h, err := NewHandler(svc)
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	router := gin.New()
	router.GET("/api/v1/listing-kits/uploads/files/*key", h.GetUploadedListingKitImage)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/listing-kits/uploads/files/folder/shirt.jpg", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.Code)
	}
	if svc.uploadedImageKey != "folder/shirt.jpg" {
		t.Fatalf("key = %q, want folder/shirt.jpg", svc.uploadedImageKey)
	}
	if got := resp.Header().Get("Content-Type"); got != "image/jpeg" {
		t.Fatalf("content-type = %q, want image/jpeg", got)
	}
}

func TestGetUploadedListingKitImageReturnsNotFound(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	svc := &stubGenerationTaskService{
		err: listingkit.ErrUploadedImageNotFound,
	}
	h, err := NewHandler(svc)
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	router := gin.New()
	router.GET("/api/v1/listing-kits/uploads/files/*key", h.GetUploadedListingKitImage)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/listing-kits/uploads/files/missing.jpg", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", resp.Code)
	}
}

var _ routeHandlerService = (*stubGenerationTaskService)(nil)

func (s *stubGenerationTaskService) UploadImages(ctx context.Context, req *listingkit.UploadImagesRequest) (*listingkit.UploadImagesResponse, error) {
	s.uploadImagesReq = req
	if s.uploadResponse != nil || s.err != nil {
		return s.uploadResponse, s.err
	}
	return nil, errors.New("not implemented")
}

func (s *stubGenerationTaskService) GetUploadedImage(ctx context.Context, key string) (*listingkit.UploadedImageFile, error) {
	s.uploadedImageKey = key
	if s.uploadedImageFile != nil || s.err != nil {
		return s.uploadedImageFile, s.err
	}
	return nil, errors.New("not implemented")
}

func (s *stubGenerationTaskService) DeleteUploadedImage(ctx context.Context, key string) (*listingkit.DeletedUploadedImage, error) {
	s.deletedUploadedImageKey = key
	if s.deletedUploadedImage != nil || s.err != nil {
		return s.deletedUploadedImage, s.err
	}
	return nil, errors.New("not implemented")
}

func activeOSSStorageSubscriptionService(t *testing.T, limits map[string]int) *listingsubscription.Service {
	t.Helper()
	svc, err := listingsubscription.NewService(listingsubscription.NewMemRepository())
	if err != nil {
		t.Fatalf("create subscription service: %v", err)
	}
	if _, err := svc.UpsertEntitlement(t.Context(), listingkit.DefaultTenantID, listingsubscription.ModuleOSSStorage, listingsubscription.EntitlementInput{
		Status: listingsubscription.StatusActive,
		Limits: limits,
	}); err != nil {
		t.Fatalf("upsert oss storage entitlement: %v", err)
	}
	return svc
}

func activeStudioOnlySubscriptionService(t *testing.T) *listingsubscription.Service {
	t.Helper()
	svc, err := listingsubscription.NewService(listingsubscription.NewMemRepository())
	if err != nil {
		t.Fatalf("create subscription service: %v", err)
	}
	if _, err := svc.UpsertEntitlement(t.Context(), listingkit.DefaultTenantID, listingsubscription.ModuleStudio, listingsubscription.EntitlementInput{
		Status: listingsubscription.StatusActive,
	}); err != nil {
		t.Fatalf("upsert studio entitlement: %v", err)
	}
	return svc
}
