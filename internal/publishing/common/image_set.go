package common

func CloneImageSet(set *ImageSet) *ImageSet {
	if set == nil {
		return nil
	}
	return &ImageSet{
		MainImage:    set.MainImage,
		WhiteBgImage: set.WhiteBgImage,
		Gallery:      append([]string(nil), set.Gallery...),
		SourceImages: append([]string(nil), set.SourceImages...),
	}
}
