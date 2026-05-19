package listingkit

import sheinpub "task-processor/internal/publishing/shein"

func taskListImageCount(task *Task) int {
	if task == nil {
		return 0
	}
	if count := listingKitResultImageCount(task.Result); count > 0 {
		return count
	}
	if task.Request != nil {
		return len(task.Request.ImageURLs)
	}
	return 0
}

func listingKitResultImageCount(result *ListingKitResult) int {
	if result == nil {
		return 0
	}
	count := 0
	if result.SDSSync != nil {
		count = max(count, sdsSyncImageCount(result.SDSSync))
	}
	if result.Shein != nil {
		count = max(count, sheinPackageImageCount(result.Shein))
	}
	if result.Summary != nil {
		count = max(count, result.Summary.ImageCount)
	}
	return count
}

func sdsSyncImageCount(summary *SDSSyncSummary) int {
	if summary == nil {
		return 0
	}
	urls := append([]string(nil), summary.MockupImageURLs...)
	for _, item := range summary.VariantResults {
		urls = append(urls, item.MockupImageURLs...)
	}
	return len(uniqueNonEmptyStrings(urls))
}

func sheinPackageImageCount(pkg *SheinPackage) int {
	if pkg == nil {
		return 0
	}
	urls := make([]string, 0)
	if pkg.RequestDraft != nil && pkg.RequestDraft.ImageInfo != nil {
		urls = appendImageDraftURLs(urls, pkg.RequestDraft.ImageInfo)
		for _, skc := range pkg.RequestDraft.SKCList {
			urls = appendImageDraftURLs(urls, skc.ImageInfo)
			for _, sku := range skc.SKUList {
				urls = append(urls, sku.MainImage)
			}
		}
	}
	if pkg.Images != nil {
		urls = append(urls, pkg.Images.MainImage, pkg.Images.WhiteBgImage)
		urls = append(urls, pkg.Images.Gallery...)
	}
	if pkg.ImageBundle != nil {
		if pkg.ImageBundle.Main != nil {
			urls = append(urls, pkg.ImageBundle.Main.URL)
		}
		for _, slot := range pkg.ImageBundle.Gallery {
			urls = append(urls, slot.URL)
		}
		for _, slot := range pkg.ImageBundle.Auxiliary {
			urls = append(urls, slot.URL)
		}
	}
	return len(uniqueNonEmptyStrings(urls))
}

func appendImageDraftURLs(urls []string, info *sheinpub.ImageDraft) []string {
	if info == nil {
		return urls
	}
	urls = append(urls, info.MainImage, info.WhiteBg)
	urls = append(urls, info.Gallery...)
	return urls
}
