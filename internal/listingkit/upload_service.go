package listingkit

import (
	"strings"
)

func buildUploadedImagePath(key string) string {
	trimmedKey := strings.TrimLeft(key, "/")
	return "/api/v1/listing-kits/uploads/files/" + trimmedKey
}
