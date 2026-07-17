package api

import (
	"errors"
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"

	"task-processor/internal/listingkit"
	"task-processor/internal/listingsubscription"
)

func (h *handler) UploadListingKitImages(c *gin.Context) {
	form, err := c.MultipartForm()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": err.Error()})
		return
	}

	files := form.File["files"]
	if len(files) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": "files are required"})
		return
	}

	request := &listingkit.UploadImagesRequest{
		Files: make([]listingkit.ImageUploadInput, 0, len(files)),
	}
	totalBytes := 0
	for _, fileHeader := range files {
		file, err := fileHeader.Open()
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": err.Error()})
			return
		}

		data, err := readUploadedFile(file)
		_ = file.Close()
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": err.Error()})
			return
		}
		totalBytes += len(data)

		request.Files = append(request.Files, listingkit.ImageUploadInput{
			Filename:    fileHeader.Filename,
			ContentType: fileHeader.Header.Get("Content-Type"),
			Data:        data,
		})
	}

	if !h.authorizeSubscriptionUsage(c, listingsubscription.ModuleOSSStorage, "storage_bytes", totalBytes) {
		return
	}

	response, err := h.studioMediaService.UploadImages(requestContext(c), request)
	if err != nil {
		status := http.StatusInternalServerError
		if strings.Contains(err.Error(), "invalid request") {
			status = http.StatusBadRequest
		}
		c.JSON(status, gin.H{"error": "upload_failed", "message": err.Error()})
		return
	}
	h.recordSubscriptionUsage(c, listingsubscription.ModuleOSSStorage, "storage_bytes", totalBytes)
	h.recordSubscriptionUsage(c, listingsubscription.ModuleOSSStorage, "uploaded_bytes", totalBytes)
	response.ImageURLs = absolutizeUploadedImageURLs(c, response.ImageURLs)

	c.JSON(http.StatusOK, response)
}

func (h *handler) GetUploadedListingKitImage(c *gin.Context) {
	key := strings.TrimPrefix(c.Param("key"), "/")
	file, err := h.studioMediaService.GetUploadedImage(requestContext(c), key)
	if err != nil {
		if errors.Is(err, listingkit.ErrUploadedImageNotFound) {
			writeUploadedImageNotFound(c)
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "image_not_found", "message": err.Error()})
		return
	}

	c.Header("Content-Type", file.ContentType)
	if file.Filename != "" {
		c.Header("Content-Disposition", `inline; filename="`+file.Filename+`"`)
	}
	c.Data(http.StatusOK, file.ContentType, file.Data)
}

func (h *handler) DeleteUploadedListingKitImage(c *gin.Context) {
	key := strings.TrimPrefix(c.Param("key"), "/")
	if h.uploadedImageDeleteService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "image_delete_unavailable", "message": "uploaded image delete is not configured"})
		return
	}
	deleted, err := h.uploadedImageDeleteService.DeleteUploadedImage(requestContext(c), key)
	if err != nil {
		if errors.Is(err, listingkit.ErrUploadedImageNotFound) {
			writeUploadedImageNotFound(c)
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "image_delete_failed", "message": err.Error()})
		return
	}
	if !deleted.AlreadyDeleted && deleted.Size > 0 {
		h.recordSubscriptionUsage(c, listingsubscription.ModuleOSSStorage, "storage_bytes", -int(deleted.Size))
	}
	c.JSON(http.StatusOK, deleted)
}

func writeUploadedImageNotFound(c *gin.Context) {
	c.JSON(http.StatusNotFound, gin.H{"error": "uploaded_image_not_found", "message": "uploaded image not found"})
}

func absolutizeUploadedImageURLs(c *gin.Context, urls []string) []string {
	if len(urls) == 0 {
		return nil
	}
	scheme := "http"
	if c.Request.TLS != nil || strings.EqualFold(c.GetHeader("X-Forwarded-Proto"), "https") {
		scheme = "https"
	}
	return absolutizeUploadedImageURLsWithBase(scheme+"://"+c.Request.Host, urls)
}

func absolutizeUploadedImageURLsWithBase(baseURL string, urls []string) []string {
	if len(urls) == 0 {
		return nil
	}
	absolute := make([]string, 0, len(urls))
	for _, rawURL := range urls {
		parsed, err := url.Parse(rawURL)
		if err == nil && parsed.IsAbs() {
			absolute = append(absolute, rawURL)
			continue
		}
		absolute = append(absolute, baseURL+rawURL)
	}
	return absolute
}
