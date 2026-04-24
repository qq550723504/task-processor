package workflow

import (
	"context"
	"fmt"

	"task-processor/internal/productimage"
)

// SelectDesignAsset 从 productimage 结果中挑出最适合用于 SDS 设计保存的图片。
// 当前优先级：
// 1. 白底图
// 2. 主图
// 3. 抠图
// 4. 第一张画廊图
func SelectDesignAsset(result *productimage.ImageProcessResult) (*productimage.ImageAsset, error) {
	if result == nil {
		return nil, fmt.Errorf("image process result is nil")
	}
	if result.WhiteBgImage != nil {
		return result.WhiteBgImage, nil
	}
	if result.MainImage != nil {
		return result.MainImage, nil
	}
	if result.SubjectCutout != nil {
		return result.SubjectCutout, nil
	}
	if len(result.GalleryImages) > 0 {
		return &result.GalleryImages[0], nil
	}
	return nil, fmt.Errorf("no usable image asset found in image process result")
}

// SyncDesignFromProcessResult 从 productimage 结果中挑选设计图并同步到 SDS。
func (s *Service) SyncDesignFromProcessResult(ctx context.Context, input SyncInput, result *productimage.ImageProcessResult) (*SyncResult, error) {
	asset, err := SelectDesignAsset(result)
	if err != nil {
		return nil, err
	}
	return s.SyncDesignFromAsset(ctx, input, AssetSource{Asset: asset})
}
