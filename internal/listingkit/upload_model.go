package listingkit

import (
	"errors"
	"time"
)

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

type DeletedUploadedImage struct {
	Key  string `json:"key"`
	Size int64  `json:"size"`
}

type UploadedImageRecord struct {
	ID           int64      `json:"id" gorm:"primaryKey"`
	TenantID     string     `json:"tenant_id" gorm:"type:varchar(128);not null;uniqueIndex:idx_listingkit_uploaded_image_tenant_key,priority:1"`
	Key          string     `json:"key" gorm:"type:varchar(512);not null;uniqueIndex:idx_listingkit_uploaded_image_tenant_key,priority:2"`
	Filename     string     `json:"filename,omitempty" gorm:"type:varchar(255)"`
	PublicURL    string     `json:"public_url,omitempty" gorm:"type:text"`
	ContentType  string     `json:"content_type,omitempty" gorm:"type:varchar(128)"`
	Size         int64      `json:"size" gorm:"not null;default:0"`
	OriginalName string     `json:"original_name,omitempty" gorm:"type:varchar(255)"`
	DeletedAt    *time.Time `json:"deleted_at,omitempty" gorm:"index"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

func (UploadedImageRecord) TableName() string {
	return "listingkit_uploaded_images"
}
