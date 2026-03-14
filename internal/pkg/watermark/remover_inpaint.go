package watermark

import (
	"context"
	"image"
	"image/color"
	"image/draw"
	"math"

	"github.com/sirupsen/logrus"
)

// InpaintRemover 图像修复去除器
type InpaintRemover struct {
	config *Config
	logger *logrus.Logger
}

// NewInpaintRemover 创建修复去除器
func NewInpaintRemover(config *Config, logger *logrus.Logger) *InpaintRemover {
	return &InpaintRemover{
		config: config,
		logger: logger,
	}
}

// Remove 使用图像修复算法去除水印
func (r *InpaintRemover) Remove(ctx context.Context, img image.Image, regions []*WatermarkRegion) (*RemovalResult, error) {
	result := &RemovalResult{
		Method:  RemovalMethodInpaint,
		Success: true,
	}

	// 创建可编辑的图像副本
	bounds := img.Bounds()
	processedImg := image.NewRGBA(bounds)
	draw.Draw(processedImg, bounds, img, bounds.Min, draw.Src)

	// 对每个水印区域进行修复
	for _, region := range regions {
		r.logger.Debugf("修复水印区域: x=%d, y=%d, w=%d, h=%d", region.X, region.Y, region.Width, region.Height)
		r.inpaintRegion(processedImg, region)
	}

	// 评估处理质量
	result.Quality = r.evaluateQuality(img, processedImg, regions)
	result.Image = processedImg

	return result, nil
}

// GetMethod 获取去除方法
func (r *InpaintRemover) GetMethod() RemovalMethod {
	return RemovalMethodInpaint
}

// inpaintRegion 修复指定区域
func (r *InpaintRemover) inpaintRegion(img *image.RGBA, region *WatermarkRegion) {
	bounds := img.Bounds()
	radius := r.config.Removal.InpaintRadius

	// 确保区域在图像范围内
	x1 := max(region.X, 0)
	y1 := max(region.Y, 0)
	x2 := min(region.X+region.Width, bounds.Max.X)
	y2 := min(region.Y+region.Height, bounds.Max.Y)

	// 使用周围像素进行修复
	for y := y1; y < y2; y++ {
		for x := x1; x < x2; x++ {
			// 从周围采样像素
			newColor := r.sampleSurroundingPixels(img, x, y, radius, x1, y1, x2, y2)
			img.Set(x, y, newColor)
		}
	}
}

// sampleSurroundingPixels 从周围像素采样
func (r *InpaintRemover) sampleSurroundingPixels(img *image.RGBA, x, y, radius, x1, y1, x2, y2 int) color.Color {
	bounds := img.Bounds()
	var rSum, gSum, bSum, aSum uint32
	var count uint32

	// 采样周围的像素（避开水印区域）
	for dy := -radius; dy <= radius; dy++ {
		for dx := -radius; dx <= radius; dx++ {
			sx := x + dx
			sy := y + dy

			// 跳过水印区域内的像素
			if sx >= x1 && sx < x2 && sy >= y1 && sy < y2 {
				continue
			}

			// 确保在图像范围内
			if sx < bounds.Min.X || sx >= bounds.Max.X || sy < bounds.Min.Y || sy >= bounds.Max.Y {
				continue
			}

			// 计算权重（距离越近权重越大）
			dist := math.Sqrt(float64(dx*dx + dy*dy))
			if dist > float64(radius) {
				continue
			}

			weight := uint32((1.0 - dist/float64(radius)) * 100)

			r, g, b, a := img.At(sx, sy).RGBA()
			rSum += r * weight
			gSum += g * weight
			bSum += b * weight
			aSum += a * weight
			count += weight
		}
	}

	if count == 0 {
		// 如果没有采样到像素，返回白色
		return color.RGBA{255, 255, 255, 255}
	}

	// 计算加权平均
	return color.RGBA{
		R: uint8((rSum / count) >> 8),
		G: uint8((gSum / count) >> 8),
		B: uint8((bSum / count) >> 8),
		A: uint8((aSum / count) >> 8),
	}
}

// evaluateQuality 评估处理质量
func (r *InpaintRemover) evaluateQuality(original, processed image.Image, regions []*WatermarkRegion) float64 {
	// 简单的质量评估：比较处理区域与周围区域的相似度
	var totalScore float64
	var count int

	for _, region := range regions {
		score := r.evaluateRegionQuality(original, processed, region)
		totalScore += score
		count++
	}

	if count == 0 {
		return 1.0
	}

	avgScore := totalScore / float64(count)
	return math.Min(avgScore, 1.0)
}

// evaluateRegionQuality 评估单个区域的质量
func (r *InpaintRemover) evaluateRegionQuality(original, processed image.Image, region *WatermarkRegion) float64 {
	// 计算处理区域的平滑度
	smoothness := r.calculateSmoothness(processed, region)

	// 计算与周围区域的一致性
	consistency := r.calculateConsistency(processed, region)

	// 综合评分
	quality := (smoothness*0.4 + consistency*0.6)

	return quality
}

// calculateSmoothness 计算平滑度
func (r *InpaintRemover) calculateSmoothness(img image.Image, region *WatermarkRegion) float64 {
	bounds := img.Bounds()
	x1 := max(region.X, bounds.Min.X)
	y1 := max(region.Y, bounds.Min.Y)
	x2 := min(region.X+region.Width, bounds.Max.X)
	y2 := min(region.Y+region.Height, bounds.Max.Y)

	var totalDiff float64
	var count int

	for y := y1; y < y2-1; y++ {
		for x := x1; x < x2-1; x++ {
			r1, g1, b1, _ := img.At(x, y).RGBA()
			r2, g2, b2, _ := img.At(x+1, y).RGBA()
			r3, g3, b3, _ := img.At(x, y+1).RGBA()

			diff1 := colorDiff(r1, g1, b1, r2, g2, b2)
			diff2 := colorDiff(r1, g1, b1, r3, g3, b3)

			totalDiff += (diff1 + diff2) / 2
			count++
		}
	}

	if count == 0 {
		return 1.0
	}

	avgDiff := totalDiff / float64(count)
	// 差异越小，平滑度越高
	smoothness := 1.0 - math.Min(avgDiff/100.0, 1.0)

	return smoothness
}

// calculateConsistency 计算与周围区域的一致性
func (r *InpaintRemover) calculateConsistency(img image.Image, region *WatermarkRegion) float64 {
	bounds := img.Bounds()

	// 计算水印区域的平均颜色
	regionColor := r.getAverageColor(img, region.X, region.Y, region.Width, region.Height)

	// 计算周围区域的平均颜色
	margin := 10
	x1 := max(region.X-margin, bounds.Min.X)
	y1 := max(region.Y-margin, bounds.Min.Y)
	x2 := min(region.X+region.Width+margin, bounds.Max.X)
	y2 := min(region.Y+region.Height+margin, bounds.Max.Y)

	surroundColor := r.getAverageColorExcluding(img, x1, y1, x2-x1, y2-y1, region)

	// 计算颜色差异
	diff := colorDiff(
		uint32(regionColor.R)<<8, uint32(regionColor.G)<<8, uint32(regionColor.B)<<8,
		uint32(surroundColor.R)<<8, uint32(surroundColor.G)<<8, uint32(surroundColor.B)<<8,
	)

	// 差异越小，一致性越高
	consistency := 1.0 - math.Min(diff/100.0, 1.0)

	return consistency
}

// getAverageColor 获取区域的平均颜色
func (r *InpaintRemover) getAverageColor(img image.Image, x, y, width, height int) color.RGBA {
	bounds := img.Bounds()
	x1 := max(x, bounds.Min.X)
	y1 := max(y, bounds.Min.Y)
	x2 := min(x+width, bounds.Max.X)
	y2 := min(y+height, bounds.Max.Y)

	var rSum, gSum, bSum uint64
	var count uint64

	for py := y1; py < y2; py++ {
		for px := x1; px < x2; px++ {
			r, g, b, _ := img.At(px, py).RGBA()
			rSum += uint64(r >> 8)
			gSum += uint64(g >> 8)
			bSum += uint64(b >> 8)
			count++
		}
	}

	if count == 0 {
		return color.RGBA{128, 128, 128, 255}
	}

	return color.RGBA{
		R: uint8(rSum / count),
		G: uint8(gSum / count),
		B: uint8(bSum / count),
		A: 255,
	}
}

// getAverageColorExcluding 获取区域的平均颜色（排除指定区域）
func (r *InpaintRemover) getAverageColorExcluding(img image.Image, x, y, width, height int, exclude *WatermarkRegion) color.RGBA {
	bounds := img.Bounds()
	x1 := max(x, bounds.Min.X)
	y1 := max(y, bounds.Min.Y)
	x2 := min(x+width, bounds.Max.X)
	y2 := min(y+height, bounds.Max.Y)

	var rSum, gSum, bSum uint64
	var count uint64

	for py := y1; py < y2; py++ {
		for px := x1; px < x2; px++ {
			// 跳过排除区域
			if px >= exclude.X && px < exclude.X+exclude.Width &&
				py >= exclude.Y && py < exclude.Y+exclude.Height {
				continue
			}

			r, g, b, _ := img.At(px, py).RGBA()
			rSum += uint64(r >> 8)
			gSum += uint64(g >> 8)
			bSum += uint64(b >> 8)
			count++
		}
	}

	if count == 0 {
		return color.RGBA{128, 128, 128, 255}
	}

	return color.RGBA{
		R: uint8(rSum / count),
		G: uint8(gSum / count),
		B: uint8(bSum / count),
		A: 255,
	}
}

// colorDiff 计算颜色差异
func colorDiff(r1, g1, b1, r2, g2, b2 uint32) float64 {
	dr := float64(r1>>8) - float64(r2>>8)
	dg := float64(g1>>8) - float64(g2>>8)
	db := float64(b1>>8) - float64(b2>>8)
	return math.Sqrt(dr*dr + dg*dg + db*db)
}

// min 返回两个整数中的较小值
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// max 返回两个整数中的较大值
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
