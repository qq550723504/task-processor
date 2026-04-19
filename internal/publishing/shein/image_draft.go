package shein

import common "task-processor/internal/publishing/common"

func BuildImageDraft(images *common.ImageSet) *ImageDraft {
	if images == nil {
		return nil
	}
	return &ImageDraft{
		MainImage: images.MainImage,
		Gallery:   append([]string(nil), images.Gallery...),
		WhiteBg:   images.WhiteBgImage,
		Source:    append([]string(nil), images.SourceImages...),
	}
}
