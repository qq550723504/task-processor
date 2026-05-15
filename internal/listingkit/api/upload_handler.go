package api

import (
	"errors"
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"

	"task-processor/internal/listingkit"
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

		request.Files = append(request.Files, listingkit.ImageUploadInput{
			Filename:    fileHeader.Filename,
			ContentType: fileHeader.Header.Get("Content-Type"),
			Data:        data,
		})
	}

	response, err := h.service.UploadImages(requestContext(c), request)
	if err != nil {
		status := http.StatusInternalServerError
		if strings.Contains(err.Error(), "invalid request") {
			status = http.StatusBadRequest
		}
		c.JSON(status, gin.H{"error": "upload_failed", "message": err.Error()})
		return
	}
	response.ImageURLs = absolutizeUploadedImageURLs(c, response.ImageURLs)

	c.JSON(http.StatusOK, response)
}

func (h *handler) GetUploadedListingKitImage(c *gin.Context) {
	key := strings.TrimPrefix(c.Param("key"), "/")
	file, err := h.service.GetUploadedImage(requestContext(c), key)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, listingkit.ErrUploadedImageNotFound) {
			status = http.StatusNotFound
		}
		c.JSON(status, gin.H{"error": "image_not_found", "message": err.Error()})
		return
	}

	c.Header("Content-Type", file.ContentType)
	if file.Filename != "" {
		c.Header("Content-Disposition", `inline; filename="`+file.Filename+`"`)
	}
	c.Data(http.StatusOK, file.ContentType, file.Data)
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
