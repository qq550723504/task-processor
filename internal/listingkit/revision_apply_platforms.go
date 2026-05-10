package listingkit

import "strings"

func applyAmazonRevision(pkg *AmazonPackage, req *AmazonRevisionInput) {
	if pkg == nil || pkg.Draft == nil || req == nil {
		return
	}
	if req.Title != nil {
		pkg.Draft.Title = strings.TrimSpace(*req.Title)
	}
	if req.Brand != nil {
		pkg.Draft.Brand = strings.TrimSpace(*req.Brand)
	}
	if req.BulletPoints != nil {
		pkg.Draft.BulletPoints = append([]string(nil), req.BulletPoints...)
	}
	if req.Description != nil {
		pkg.Draft.Description = strings.TrimSpace(*req.Description)
	}
}

func applyTemuRevision(pkg *TemuPackage, req *TemuRevisionInput) {
	if pkg == nil || req == nil {
		return
	}
	if req.GoodsName != nil {
		pkg.GoodsName = strings.TrimSpace(*req.GoodsName)
	}
	if req.ShortDescription != nil {
		pkg.ShortDescription = strings.TrimSpace(*req.ShortDescription)
	}
	if req.BulletPoints != nil {
		pkg.BulletPoints = append([]string(nil), req.BulletPoints...)
	}
	if req.Images != nil {
		pkg.Images = clonePlatformImageSet(req.Images)
	}
	if req.ReviewNotes != nil {
		pkg.ReviewNotes = uniqueStrings(append([]string(nil), req.ReviewNotes...))
	}
}

func applyWalmartRevision(pkg *WalmartPackage, req *WalmartRevisionInput) {
	if pkg == nil || req == nil {
		return
	}
	if req.ProductName != nil {
		pkg.ProductName = strings.TrimSpace(*req.ProductName)
	}
	if req.Brand != nil {
		pkg.Brand = strings.TrimSpace(*req.Brand)
	}
	if req.ShortDescription != nil {
		pkg.ShortDescription = strings.TrimSpace(*req.ShortDescription)
	}
	if req.LongDescription != nil {
		pkg.LongDescription = strings.TrimSpace(*req.LongDescription)
	}
	if req.KeyFeatures != nil {
		pkg.KeyFeatures = append([]string(nil), req.KeyFeatures...)
	}
	if req.Images != nil {
		pkg.Images = clonePlatformImageSet(req.Images)
	}
	if req.ReviewNotes != nil {
		pkg.ReviewNotes = uniqueStrings(append([]string(nil), req.ReviewNotes...))
	}
}

func clonePlatformImageSet(images *PlatformImageSet) *PlatformImageSet {
	if images == nil {
		return nil
	}
	return &PlatformImageSet{
		MainImage:    images.MainImage,
		WhiteBgImage: images.WhiteBgImage,
		Gallery:      append([]string(nil), images.Gallery...),
		SourceImages: append([]string(nil), images.SourceImages...),
	}
}
