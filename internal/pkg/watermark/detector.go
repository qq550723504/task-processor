package watermark

import (
	"context"
	"fmt"
	"image"
	"time"

	"github.com/sirupsen/logrus"
)

// Detector 水印检测器接口
type Detector interface {
	// Detect 检测图片中的水印
	Detect(ctx context.Context, img image.Image) (*DetectionResult, error)
	// GetMethod 获取检测方法
	GetMethod() DetectionMethod
}

// DetectorManager 检测器管理器
type DetectorManager struct {
	config    *Config
	detectors map[DetectionMethod]Detector
	logger    *logrus.Logger
}

// NewDetectorManager 创建检测器管理器
func NewDetectorManager(config *Config, logger *logrus.Logger) *DetectorManager {
	if logger == nil {
		logger = logrus.New()
	}

	dm := &DetectorManager{
		config:    config,
		detectors: make(map[DetectionMethod]Detector),
		logger:    logger,
	}

	// 注册检测器
	dm.registerDetectors()

	return dm
}

// registerDetectors 注册所有可用的检测器
func (dm *DetectorManager) registerDetectors() {
	// 注册传统检测器
	dm.detectors[DetectionMethodTraditional] = NewTraditionalDetector(dm.config, dm.logger)

	// 注册AI检测器（如果启用）
	if dm.config.AI.VisionAPI.Enabled {
		dm.detectors[DetectionMethodAI] = NewAIDetector(dm.config, dm.logger)
	}

	// 注册混合检测器
	dm.detectors[DetectionMethodHybrid] = NewHybridDetector(dm.config, dm.logger, dm.detectors)
}

// Detect 检测水印
func (dm *DetectorManager) Detect(ctx context.Context, img image.Image) (*DetectionResult, error) {
	if !dm.config.Enabled {
		return &DetectionResult{
			HasWatermark: false,
			Method:       dm.config.Detection.Method,
		}, nil
	}

	// 获取对应的检测器
	detector, ok := dm.detectors[dm.config.Detection.Method]
	if !ok {
		return nil, fmt.Errorf("unsupported detection method: %s", dm.config.Detection.Method)
	}

	// 设置超时
	ctx, cancel := context.WithTimeout(ctx, time.Duration(dm.config.Performance.Timeout)*time.Second)
	defer cancel()

	// 执行检测
	startTime := time.Now()
	result, err := detector.Detect(ctx, img)
	if err != nil {
		dm.logger.Errorf("水印检测失败: %v", err)
		return nil, err
	}

	result.ProcessTime = time.Since(startTime).Seconds()
	dm.logger.Infof("水印检测完成: 方法=%s, 耗时=%.2fs, 发现水印=%v, 区域数=%d",
		result.Method, result.ProcessTime, result.HasWatermark, len(result.Regions))

	return result, nil
}

// GetDetector 获取指定方法的检测器
func (dm *DetectorManager) GetDetector(method DetectionMethod) (Detector, error) {
	detector, ok := dm.detectors[method]
	if !ok {
		return nil, fmt.Errorf("detector not found: %s", method)
	}
	return detector, nil
}
