package listingkit

import (
	"bytes"
	"errors"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
)

const MaxListingKitUploadBytes = 12 << 20

var errInvalidUploadedImage = errors.New("invalid image upload")

type validatedUploadedImage struct {
	ContentType string
	Extension   string
}

func validateUploadedImage(input ImageUploadInput) (validatedUploadedImage, error) {
	if len(input.Data) == 0 || len(input.Data) > MaxListingKitUploadBytes {
		return validatedUploadedImage{}, errInvalidUploadedImage
	}
	config, format, err := image.DecodeConfig(bytes.NewReader(input.Data))
	if err != nil || config.Width < 1 || config.Height < 1 {
		return validatedUploadedImage{}, errInvalidUploadedImage
	}
	if _, _, err = image.Decode(bytes.NewReader(input.Data)); err != nil {
		return validatedUploadedImage{}, errInvalidUploadedImage
	}
	contentType, extension, ok := uploadedImageFormat(format)
	if !ok {
		return validatedUploadedImage{}, errInvalidUploadedImage
	}
	return validatedUploadedImage{ContentType: contentType, Extension: extension}, nil
}

func uploadedImageFormat(format string) (string, string, bool) {
	switch format {
	case "jpeg":
		return "image/jpeg", ".jpg", true
	case "png":
		return "image/png", ".png", true
	case "gif":
		return "image/gif", ".gif", true
	default:
		return "", "", false
	}
}
