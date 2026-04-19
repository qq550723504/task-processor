package listingkit

import "errors"

var ErrUploadedImageNotFound = errors.New("uploaded image not found")

type ImageUploadInput struct {
	Filename    string
	ContentType string
	Data        []byte
}

type UploadImagesRequest struct {
	Files []ImageUploadInput `json:"-"`
}

type StoredUploadedImage struct {
	Key          string `json:"key,omitempty"`
	Filename     string `json:"filename,omitempty"`
	Path         string `json:"path,omitempty"`
	PublicURL    string `json:"public_url,omitempty"`
	ContentType  string `json:"content_type,omitempty"`
	Size         int64  `json:"size,omitempty"`
	OriginalName string `json:"original_name,omitempty"`
	Data         []byte `json:"-"`
}

type UploadedImageFile struct {
	Filename    string
	ContentType string
	Data        []byte
}

type UploadImagesResponse struct {
	ImageURLs []string `json:"image_urls,omitempty"`
}
