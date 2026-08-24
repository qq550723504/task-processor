package listingkit

import "strings"

const StudioProductImageLegacyMirrorReleasePendingMetadataKey = "release_pending"

func StudioProductImageLegacyMirrorOperationKey(eventID, operation string) string {
	return "listingkit:legacy_product_image_mirror:" + strings.TrimSpace(eventID) + ":" + strings.TrimSpace(operation)
}
