package watermark

import (
	"context"
	"image"
	"image/color"
	"math"

	"github.com/disintegration/imaging"
	"github.com/sirupsen/logrus"
)

// TraditionalDetector 传统图像处理检测器
type TraditionalDetector struct {
	config *Config
	logger *logrus.Logger
}

// NewTraditionalDetector 创建传统检测器
func NewTraditionalDetector(config *Config, logger *logrus.Logger) *TraditionalDetector {
	return &TraditionalDetector{
		config: config,
		logger: logger,
	}
}

// Detect 检测水印
func (d *TraditionalDetector) Detect(ctx context.Context, img image.Image) (*DetectionResult, error) {
	result := &DetectionResult{
		Method:  DetectionMethodTraditional,
		Regions: make([]*WatermarkRegion, 0),
	}

	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	// 根据配置的区域进行检测
	for _, region := range d.config.Detection.Regions {
		switch region {
		case "corner":
			d.detectCorners(img, width, height, result)
		case "edge":
			d.detectEdges(img, width, height, result)
		case "center":
			d.detectCenter(img, width, height, result)
		case "full":
			d.detectFull(img, width, height, result)
		}
	}

	result.HasWatermark = len(result.Regions) > 0
	return result, nil
}

// GetMethod 获取检测方法
func (d *TraditionalDetector) GetMethod() DetectionMethod {
	return DetectionMethodTraditional
}

// detectCorners 检测四个角落
func (d *TraditionalDetector) detectCorners(img image.Image, width, height int, result *DetectionResult) {
	cornerSize := d.getCornerSize(width, height)

	corners := []struct {
		x        int
		y        int
		position Position
	}{
		{0, 0, PositionTopLeft},
		{width - cornerSize, 0, PositionTopRight},
		{0, height - cornerSize, PositionBottomLeft},
		{width - cornerSize, height - cornerSize, PositionBottomRight},
	}

	for _, corner := range corners {
		if region := d.analyzeRegion(img, corner.x, corner.y, cornerSize, cornerSize, corner.position); region != nil {
			result.Regions = append(result.Regions, region)
		}
	}
}

// detectEdges 检测边缘
func (d *TraditionalDetector) detectEdges(img image.Image, width, height int, result *DetectionResult) {
	edgeThickness := d.getEdgeThickness(width, height)

	// 检测上下边缘
	if region := d.analyzeRegion(img, 0, 0, width, edgeThickness, PositionCustom); region != nil {
		result.Regions = append(result.Regions, region)
	}
	if region := d.analyzeRegion(img, 0, height-edgeThickness, width, edgeThickness, PositionCustom); region != nil {
		result.Regions = append(result.Regions, region)
	}

	// 检测左右边缘
	if region := d.analyzeRegion(img, 0, 0, edgeThickness, height, PositionCustom); region != nil {
		result.Regions = append(result.Regions, region)
	}
	if region := d.analyzeRegion(img, width-edgeThickness, 0, edgeThickness, height, PositionCustom); region != nil {
		result.Regions = append(result.Regions, region)
	}
}

// detectCenter 检测中心区域
func (d *TraditionalDetector) detectCenter(img image.Image, width, height int, result *DetectionResult) {
	centerSize := d.getCenterSize(width, height)
	x := (width - centerSize) / 2
	y := (height - centerSize) / 2

	if region := d.analyzeRegion(img, x, y, centerSize, centerSize, PositionCenter); region != nil {
		result.Regions = append(result.Regions, region)
	}
}

// detectFull 全图检测
func (d *TraditionalDetector) detectFull(img image.Image, width, height int, result *DetectionResult) {
	// 使用滑动窗口检测
	windowSize := d.config.Detection.MinSize * 2
	step := windowSize / 2

	for y := 0; y < height-windowSize; y += step {
		for x := 0; x < width-windowSize; x += step {
			if region := d.analyzeRegion(img, x, y, windowSize, windowSize, PositionCustom); region != nil {
				result.Regions = append(result.Regions, region)
			}
		}
	}
}

// analyzeRegion 分析指定区域是否包含水印
func (d *TraditionalDetector) analyzeRegion(img image.Image, x, y, width, height int, pos Position) *WatermarkRegion {
	// 裁剪区域
	subImg := imaging.Crop(img, image.Rect(x, y, x+width, y+height))

	// 计算多个特征
	edgeScore := d.calculateEdgeScore(subImg)
	contrastScore := d.calculateContrastScore(subImg)
	colorScore := d.calculateColorScore(subImg)
	textureScore := d.calculateTextureScore(subImg)

	// 综合评分
	confidence := d.calculateConfidence(edgeScore, contrastScore, colorScore, textureScore)

	// 根据灵敏度调整阈值
	threshold := d.getThreshold()

	if confidence >= threshold {
		return &WatermarkRegion{
			X:          x,
			Y:          y,
			Width:      width,
			Height:     height,
			Type:       d.guessWatermarkType(edgeScore, textureScore),
			Position:   pos,
			Confidence: confidence,
			Metadata: map[string]interface{}{
				"edge_score":     edgeScore,
				"contrast_score": contrastScore,
				"color_score":    colorScore,
				"texture_score":  textureScore,
			},
		}
	}

	return nil
}

// calculateEdgeScore 计算边缘得分（检测文字和logo）
func (d *TraditionalDetector) calculateEdgeScore(img image.Image) float64 {
	// 转换为灰度图
	gray := imaging.Grayscale(img)
	bounds := gray.Bounds()

	// Sobel边缘检测
	var edgeCount int
	var totalPixels int

	for y := bounds.Min.Y + 1; y < bounds.Max.Y-1; y++ {
		for x := bounds.Min.X + 1; x < bounds.Max.X-1; x++ {
			gx := d.sobelX(gray, x, y)
			gy := d.sobelY(gray, x, y)
			magnitude := math.Sqrt(float64(gx*gx + gy*gy))

			if magnitude > 30 { // 边缘阈值
				edgeCount++
			}
			totalPixels++
		}
	}

	if totalPixels == 0 {
		return 0
	}

	return float64(edgeCount) / float64(totalPixels)
}

// sobelX Sobel X方向算子
func (d *TraditionalDetector) sobelX(img image.Image, x, y int) int {
	return d.getGrayValue(img, x+1, y-1) + 2*d.getGrayValue(img, x+1, y) + d.getGrayValue(img, x+1, y+1) -
		d.getGrayValue(img, x-1, y-1) - 2*d.getGrayValue(img, x-1, y) - d.getGrayValue(img, x-1, y+1)
}

// sobelY Sobel Y方向算子
func (d *TraditionalDetector) sobelY(img image.Image, x, y int) int {
	return d.getGrayValue(img, x-1, y+1) + 2*d.getGrayValue(img, x, y+1) + d.getGrayValue(img, x+1, y+1) -
		d.getGrayValue(img, x-1, y-1) - 2*d.getGrayValue(img, x, y-1) - d.getGrayValue(img, x+1, y-1)
}

// getGrayValue 获取灰度值
func (d *TraditionalDetector) getGrayValue(img image.Image, x, y int) int {
	c := color.GrayModel.Convert(img.At(x, y)).(color.Gray)
	return int(c.Y)
}

// calculateContrastScore 计算对比度得分
func (d *TraditionalDetector) calculateContrastScore(img image.Image) float64 {
	bounds := img.Bounds()
	var sum, sumSq int64
	var count int64

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			gray := d.getGrayValue(img, x, y)
			sum += int64(gray)
			sumSq += int64(gray * gray)
			count++
		}
	}

	if count == 0 {
		return 0
	}

	mean := float64(sum) / float64(count)
	variance := float64(sumSq)/float64(count) - mean*mean
	stdDev := math.Sqrt(variance)

	// 标准化到0-1
	return math.Min(stdDev/128.0, 1.0)
}

// calculateColorScore 计算颜色得分（检测半透明水印）
func (d *TraditionalDetector) calculateColorScore(img image.Image) float64 {
	bounds := img.Bounds()
	colorMap := make(map[uint32]int)
	var totalPixels int

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r, g, b, a := img.At(x, y).RGBA()
			// 简化颜色到8位
			colorKey := (r>>8)<<16 | (g>>8)<<8 | (b >> 8) | (a >> 8)
			colorMap[colorKey]++
			totalPixels++
		}
	}

	// 计算颜色多样性
	uniqueColors := len(colorMap)
	diversity := float64(uniqueColors) / float64(totalPixels)

	// 水印通常颜色单一
	return 1.0 - math.Min(diversity*10, 1.0)
}

// calculateTextureScore 计算纹理得分
func (d *TraditionalDetector) calculateTextureScore(img image.Image) float64 {
	bounds := img.Bounds()
	gray := imaging.Grayscale(img)

	var horizontalDiff, verticalDiff float64
	var count int

	for y := bounds.Min.Y; y < bounds.Max.Y-1; y++ {
		for x := bounds.Min.X; x < bounds.Max.X-1; x++ {
			current := d.getGrayValue(gray, x, y)
			right := d.getGrayValue(gray, x+1, y)
			down := d.getGrayValue(gray, x, y+1)

			horizontalDiff += math.Abs(float64(current - right))
			verticalDiff += math.Abs(float64(current - down))
			count++
		}
	}

	if count == 0 {
		return 0
	}

	avgDiff := (horizontalDiff + verticalDiff) / float64(count*2)
	return math.Min(avgDiff/50.0, 1.0)
}

// calculateConfidence 计算综合置信度
func (d *TraditionalDetector) calculateConfidence(edge, contrast, color, texture float64) float64 {
	// 根据灵敏度调整权重
	var weights map[string]float64

	switch d.config.Detection.Sensitivity {
	case "low":
		weights = map[string]float64{"edge": 0.4, "contrast": 0.3, "color": 0.2, "texture": 0.1}
	case "high":
		weights = map[string]float64{"edge": 0.3, "contrast": 0.2, "color": 0.3, "texture": 0.2}
	default: // medium
		weights = map[string]float64{"edge": 0.35, "contrast": 0.25, "color": 0.25, "texture": 0.15}
	}

	confidence := edge*weights["edge"] +
		contrast*weights["contrast"] +
		color*weights["color"] +
		texture*weights["texture"]

	return math.Min(confidence, 1.0)
}

// guessWatermarkType 推测水印类型
func (d *TraditionalDetector) guessWatermarkType(edgeScore, textureScore float64) WatermarkType {
	if edgeScore > 0.3 && textureScore < 0.2 {
		return WatermarkTypeText
	} else if edgeScore > 0.2 && textureScore > 0.3 {
		return WatermarkTypeLogo
	} else if textureScore > 0.4 {
		return WatermarkTypePattern
	}
	return WatermarkTypeUnknown
}

// getThreshold 获取检测阈值
func (d *TraditionalDetector) getThreshold() float64 {
	switch d.config.Detection.Sensitivity {
	case "low":
		return 0.7
	case "high":
		return 0.4
	default: // medium
		return 0.6
	}
}

// getCornerSize 获取角落检测尺寸
func (d *TraditionalDetector) getCornerSize(width, height int) int {
	size := int(math.Min(float64(width), float64(height)) * 0.15)
	return int(math.Max(float64(size), float64(d.config.Detection.MinSize)))
}

// getEdgeThickness 获取边缘检测厚度
func (d *TraditionalDetector) getEdgeThickness(width, height int) int {
	thickness := int(math.Min(float64(width), float64(height)) * 0.05)
	return int(math.Max(float64(thickness), float64(d.config.Detection.MinSize)))
}

// getCenterSize 获取中心检测尺寸
func (d *TraditionalDetector) getCenterSize(width, height int) int {
	size := int(math.Min(float64(width), float64(height)) * 0.3)
	return int(math.Max(float64(size), float64(d.config.Detection.MinSize*2)))
}
