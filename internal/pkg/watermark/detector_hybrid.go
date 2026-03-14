package watermark

import (
	"context"
	"image"

	"github.com/sirupsen/logrus"
)

// HybridDetector 混合检测器（传统+AI）
type HybridDetector struct {
	config              *Config
	logger              *logrus.Logger
	traditionalDetector Detector
	aiDetector          Detector
}

// NewHybridDetector 创建混合检测器
func NewHybridDetector(config *Config, logger *logrus.Logger, detectors map[DetectionMethod]Detector) *HybridDetector {
	return &HybridDetector{
		config:              config,
		logger:              logger,
		traditionalDetector: detectors[DetectionMethodTraditional],
		aiDetector:          detectors[DetectionMethodAI],
	}
}

// Detect 混合检测
func (d *HybridDetector) Detect(ctx context.Context, img image.Image) (*DetectionResult, error) {
	// 第一步：使用传统算法快速检测
	d.logger.Debug("混合检测: 第一步 - 传统算法检测")
	traditionalResult, err := d.traditionalDetector.Detect(ctx, img)
	if err != nil {
		d.logger.Warnf("传统检测失败: %v", err)
		traditionalResult = &DetectionResult{
			Method:  DetectionMethodTraditional,
			Regions: make([]*WatermarkRegion, 0),
		}
	}

	result := &DetectionResult{
		Method:  DetectionMethodHybrid,
		Regions: make([]*WatermarkRegion, 0),
	}

	// 如果传统算法没有检测到水印，直接返回
	if !traditionalResult.HasWatermark {
		d.logger.Debug("混合检测: 传统算法未检测到水印，跳过AI检测")
		result.HasWatermark = false
		return result, nil
	}

	// 如果传统算法检测到高置信度水印，直接返回
	hasHighConfidence := false
	for _, region := range traditionalResult.Regions {
		if region.Confidence >= 0.8 {
			hasHighConfidence = true
			result.Regions = append(result.Regions, region)
		}
	}

	if hasHighConfidence && !d.shouldUseAI() {
		d.logger.Debug("混合检测: 传统算法检测到高置信度水印，跳过AI检测")
		result.HasWatermark = true
		return result, nil
	}

	// 第二步：对于低置信度或复杂情况，使用AI二次确认
	if d.aiDetector != nil && d.shouldUseAI() {
		d.logger.Debug("混合检测: 第二步 - AI检测确认")
		aiResult, err := d.aiDetector.Detect(ctx, img)
		if err != nil {
			d.logger.Warnf("AI检测失败: %v，使用传统检测结果", err)
			result.Regions = traditionalResult.Regions
			result.HasWatermark = len(result.Regions) > 0
			return result, nil
		}

		// 合并结果
		result = d.mergeResults(traditionalResult, aiResult)
	} else {
		// 没有AI检测器，使用传统结果
		result.Regions = traditionalResult.Regions
		result.HasWatermark = len(result.Regions) > 0
	}

	return result, nil
}

// GetMethod 获取检测方法
func (d *HybridDetector) GetMethod() DetectionMethod {
	return DetectionMethodHybrid
}

// shouldUseAI 判断是否应该使用AI检测
func (d *HybridDetector) shouldUseAI() bool {
	// 检查AI是否启用
	if !d.config.AI.VisionAPI.Enabled {
		return false
	}

	// 可以根据其他条件判断，比如：
	// - 图片重要性
	// - 成本预算
	// - 时间要求
	// 这里简化处理，直接返回配置状态
	return true
}

// mergeResults 合并传统和AI检测结果
func (d *HybridDetector) mergeResults(traditional, ai *DetectionResult) *DetectionResult {
	result := &DetectionResult{
		Method:  DetectionMethodHybrid,
		Regions: make([]*WatermarkRegion, 0),
	}

	// 使用map去重
	regionMap := make(map[string]*WatermarkRegion)

	// 添加AI检测结果（优先级更高）
	for _, region := range ai.Regions {
		key := d.getRegionKey(region)
		regionMap[key] = region
	}

	// 添加传统检测结果（如果不重复）
	for _, region := range traditional.Regions {
		key := d.getRegionKey(region)
		if _, exists := regionMap[key]; !exists {
			// 如果AI没有检测到这个区域，但传统算法检测到了
			// 降低置信度
			region.Confidence *= 0.8
			if region.Confidence >= d.config.Detection.Threshold {
				regionMap[key] = region
			}
		}
	}

	// 转换为切片
	for _, region := range regionMap {
		result.Regions = append(result.Regions, region)
	}

	result.HasWatermark = len(result.Regions) > 0

	d.logger.Infof("混合检测完成: 传统检测=%d个区域, AI检测=%d个区域, 合并后=%d个区域",
		len(traditional.Regions), len(ai.Regions), len(result.Regions))

	return result
}

// getRegionKey 获取区域的唯一标识
func (d *HybridDetector) getRegionKey(region *WatermarkRegion) string {
	// 使用位置作为key，允许一定的误差
	x := region.X / 50 * 50
	y := region.Y / 50 * 50
	return string(region.Position) + "_" + string(region.Type) + "_" +
		string(rune(x)) + "_" + string(rune(y))
}
