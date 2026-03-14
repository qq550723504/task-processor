package watermark

import (
	"context"
	"image"

	"github.com/disintegration/imaging"
	"github.com/sirupsen/logrus"
)

// CropRemover 裁剪去除器
type CropRemover struct {
	config *Config
	logger *logrus.Logger
}

// NewCropRemover 创建裁剪去除器
func NewCropRemover(config *Config, logger *logrus.Logger) *CropRemover {
	return &CropRemover{
		config: config,
		logger: logger,
	}
}

// Remove 通过裁剪去除水印
func (r *CropRemover) Remove(ctx context.Context, img image.Image, regions []*WatermarkRegion) (*RemovalResult, error) {
	result := &RemovalResult{
		Method:  RemovalMethodCrop,
		Success: true,
		Quality: 0.9, // 裁剪方法质量较高（如果水印在边缘）
	}

	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	// 计算需要裁剪的边界
	cropBounds := r.calculateCropBounds(width, height, regions)

	// 如果不需要裁剪，返回原图
	if cropBounds.Min.X == 0 && cropBounds.Min.Y == 0 &&
		cropBounds.Max.X == width && cropBounds.Max.Y == height {
		r.logger.Debug("水印不在边缘，无法通过裁剪去除")
		result.Success = false
		result.Image = img
		result.Quality = 0
		return result, nil
	}

	// 执行裁剪
	croppedImg := imaging.Crop(img, cropBounds)

	// 如果需要保持宽高比，进行调整
	if r.config.Removal.PreserveAspect {
		originalAspect := float64(width) / float64(height)
		newWidth := cropBounds.Dx()
		newHeight := cropBounds.Dy()
		newAspect := float64(newWidth) / float64(newHeight)

		// 如果宽高比变化太大，降低质量评分
		if abs(originalAspect-newAspect) > 0.1 {
			result.Quality *= 0.8
		}
	}

	r.logger.Infof("裁剪完成: 原始尺寸=%dx%d, 裁剪后=%dx%d",
		width, height, cropBounds.Dx(), cropBounds.Dy())

	result.Image = croppedImg
	return result, nil
}

// GetMethod 获取去除方法
func (r *CropRemover) GetMethod() RemovalMethod {
	return RemovalMethodCrop
}

// calculateCropBounds 计算裁剪边界
func (r *CropRemover) calculateCropBounds(width, height int, regions []*WatermarkRegion) image.Rectangle {
	// 初始边界为整个图像
	cropLeft := 0
	cropTop := 0
	cropRight := width
	cropBottom := height

	// 检查每个水印区域
	for _, region := range regions {
		// 只处理边缘的水印
		if r.isEdgeWatermark(region, width, height) {
			// 根据水印位置调整裁剪边界
			switch region.Position {
			case PositionTopLeft, PositionTopRight:
				// 顶部水印，裁剪顶部
				if region.Y+region.Height > cropTop {
					cropTop = region.Y + region.Height
				}
			case PositionBottomLeft, PositionBottomRight:
				// 底部水印，裁剪底部
				if region.Y < cropBottom {
					cropBottom = region.Y
				}
			}

			// 检查左右边缘
			edgeThreshold := width / 10
			if region.X < edgeThreshold {
				// 左边缘水印
				if region.X+region.Width > cropLeft {
					cropLeft = region.X + region.Width
				}
			}
			if region.X+region.Width > width-edgeThreshold {
				// 右边缘水印
				if region.X < cropRight {
					cropRight = region.X
				}
			}
		}
	}

	// 确保裁剪后的图像不会太小
	minWidth := width / 2
	minHeight := height / 2

	if cropRight-cropLeft < minWidth || cropBottom-cropTop < minHeight {
		r.logger.Warn("裁剪后图像过小，保持原始尺寸")
		return image.Rect(0, 0, width, height)
	}

	return image.Rect(cropLeft, cropTop, cropRight, cropBottom)
}

// isEdgeWatermark 判断是否为边缘水印
func (r *CropRemover) isEdgeWatermark(region *WatermarkRegion, width, height int) bool {
	edgeThreshold := 0.1 // 10%的边缘区域

	// 检查是否在顶部边缘
	if float64(region.Y) < float64(height)*edgeThreshold {
		return true
	}

	// 检查是否在底部边缘
	if float64(region.Y+region.Height) > float64(height)*(1-edgeThreshold) {
		return true
	}

	// 检查是否在左边缘
	if float64(region.X) < float64(width)*edgeThreshold {
		return true
	}

	// 检查是否在右边缘
	if float64(region.X+region.Width) > float64(width)*(1-edgeThreshold) {
		return true
	}

	return false
}

// abs 返回浮点数的绝对值
func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
