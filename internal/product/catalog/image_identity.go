package catalog

import "strconv"

type IdentifiedImage struct {
	ID string
	Image
}

// IdentifyImages preserves current Catalog image references across authorized
// consumers. Validation/filtering must happen after assigning original indexes.
func IdentifyImages(images []Image) []IdentifiedImage {
	identified := make([]IdentifiedImage, len(images))
	for index, image := range images {
		identified[index] = IdentifiedImage{ID: "catalog-image-" + strconv.Itoa(index+1), Image: image}
	}
	return identified
}
