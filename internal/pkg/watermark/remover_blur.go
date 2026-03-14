package watermark

import (
	"context"
	"image"
	"image/draw"

	"github.com/disintegration/imaging"
	"github.com/sirupsen/logrus"
)

// BlurRemover 模糊去除器
type BlurRemover struct {
	config *Config
	logger *logrus.Logger
}

// NewBlurRemover 创建模糊去除器
func NewBlurRemover(config *Config, logger *logrus.Logger) *BlurRemover {
	return &BlurRemover{
		config: config,
		logger: logger,
	}
}

// Remove 使用模糊算法去除水印
func (r *BlurRemover) Remove(ctx context.Context, img image.Image, regions []*WatermarkRegion) (*RemovalResult, error) {
	result := &RemovalResult{
		Method:  RemovalMethodBlur,
		Success: true,
	}

	// 创建图像副本
	bounds := img.Bounds()
	processedImg := image.NewRGBA(bounds)
	draw.Draw(processedImg, bounds, img, bounds.Min, draw.Src)

	// 对每个水印区域进行模糊处理
	for _, region := range regions {
		r.logger.Debugf("模糊水印区域: x=%d, y=%d, w=%d, h=%d", region.X, region.Y, region.Width, region.Height)
		r.blurRegion(processedImg, region)
	}

	// 评估处理质量
	result.Quality = 0.7 // 模糊方法质量一般
	result.Image = processedImg

	return result, nil
}

// GetMethod 获取去除方法
func (r *BlurRemover) GetMethod() RemovalMethod {
	return RemovalMethodBlur
}

// blurRegion 模糊指定区域
func (r *BlurRemover) blurRegion(img *image.RGBA, region *WatermarkRegion) {
	bounds := img.Bounds()

	// 确保区域在图像范围内
	x1 := max(region.X, 0)
	y1 := max(region.Y, 0)
	x2 := min(region.X+region.Width, bounds.Max.X)
	y2 := min(region.Y+region.Height, bounds.Max.Y)

	if x2 <= x1 || y2 <= y1 {
		return
	}

	// 裁剪水印区域
	regionImg := imaging.Crop(img, image.Rect(x1, y1, x2, y2))

	// 应用高斯模糊
	blurRadius := float64(r.config.Removal.BlurRadius)
	if blurRadius <= 0 {
		blurRadius = 5.0
	}
	blurredRegion := imaging.Blur(regionImg, blurRadius)

	// 将模糊后的区域绘制回原图
	draw.Draw(img, image.Rect(x1, y1, x2, y2), blurredRegion, image.Point{}, draw.Src)
}
