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
	h, err := NewHandler(svc)
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
}

func TestUploadListingKitImagesReturnsBadRequestWithoutFiles(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	svc := &stubGenerationTaskService{}
	h, err := NewHandler(svc)
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

var _ listingkit.HandlerService = (*stubGenerationTaskService)(nil)

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
