package api

import (
	"fmt"
	"io"
	"mime/multipart"
)

const maxListingKitUploadBytes = 12 << 20

func readUploadedFile(file multipart.File) ([]byte, error) {
	limitedReader := io.LimitReader(file, maxListingKitUploadBytes+1)
	data, err := io.ReadAll(limitedReader)
	if err != nil {
		return nil, fmt.Errorf("read uploaded file: %w", err)
	}
	if len(data) == 0 {
		return nil, fmt.Errorf("uploaded file is empty")
	}
	if len(data) > maxListingKitUploadBytes {
		return nil, fmt.Errorf("uploaded file exceeds %d bytes", maxListingKitUploadBytes)
	}
	return data, nil
}
