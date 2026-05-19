package product

import "strings"

func CollectSizeMapImageURLs(product *Product) map[string]struct{} {
	if product == nil {
		return nil
	}
	out := map[string]struct{}{}
	add := func(info *ImageInfo) {
		if info == nil {
			return
		}
		for _, image := range info.ImageInfoList {
			url := strings.TrimSpace(image.ImageURL)
			if url == "" || !image.SizeImgFlag {
				continue
			}
			out[url] = struct{}{}
		}
	}
	add(product.ImageInfo)
	for i := range product.SKCList {
		add(&product.SKCList[i].ImageInfo)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
