package api

import (
	"fmt"
	"io"
	"mime/multipart"

	"task-processor/internal/listingkit"
)

func readUploadedFile(file multipart.File) ([]byte, error) {
	limitedReader := io.LimitReader(file, listingkit.MaxListingKitUploadBytes+1)
	data, err := io.ReadAll(limitedReader)
	if err != nil {
		return nil, fmt.Errorf("read uploaded file: %w", err)
	}
	if len(data) == 0 {
		return nil, fmt.Errorf("uploaded file is empty")
	}
	if len(data) > listingkit.MaxListingKitUploadBytes {
		return nil, fmt.Errorf("uploaded file exceeds %d bytes", listingkit.MaxListingKitUploadBytes)
	}
	return data, nil
}
