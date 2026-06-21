package listingkit

import (
	sheinpub "task-processor/internal/publishing/shein"
	sheinimage "task-processor/internal/shein/api/image"
	sheinproduct "task-processor/internal/shein/api/product"
)

func uploadSheinImageInfo(info *sheinproduct.ImageInfo, uploader sheinimage.ImageAPI, uploaded map[string]string) (int, error) {
	return sheinpub.UploadImageInfo(info, uploader, uploaded, buildSheinColorBlockImageFromURL)
}
