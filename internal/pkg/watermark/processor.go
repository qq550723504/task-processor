package watermark

import (
	"context"
	"fmt"
	"image"

	"github.com/sirupsen/logrus"
)

// Processor 水印处理器（检测+去除）
type Processor struct {
	config          *Config
	detectorManager *DetectorManager
	removerManager  *RemoverManager
	logger          *logrus.Logger
}

// NewProcessor 创建水印处理器
func NewProcessor(config *Config, logger *logrus.Logger) *Processor {
	if config == nil {
		config = DefaultConfig()
	}

	if logger == nil {
		logger = logrus.New()
	}

	return &Processor{
		config:          config,
		detectorManager: NewDetectorManager(config, logger),
		removerManager:  NewRemoverManager(config, logger),
		logger:          logger,
	}
}

// Process 完整处理流程（检测+去除）
func (p *Processor) Process(ctx context.Context, img image.Image) (*ProcessResult, error) {
	if !p.config.Enabled {
		return &ProcessResult{
			Original: img,
			Detection: &DetectionResult{
				HasWatermark: false,
			},
		}, nil
	}

	result := &ProcessResult{
		Original: img,
	}

	// 第一步：检测水印
	p.logger.Info("开始检测水印...")
	detection, err := p.detectorManager.Detect(ctx, img)
	if err != nil {
		return nil, fmt.Errorf("水印检测失败: %w", err)
	}
	result.Detection = detection

	// 如果没有检测到水印，直接返回
	if !detection.HasWatermark || len(detection.Regions) == 0 {
		p.logger.Info("未检测到水印，跳过去除步骤")
		return result, nil
	}

	p.logger.Infof("检测到 %d 个水印区域", len(detection.Regions))

	// 第二步：去除水印
	p.logger.Info("开始去除水印...")
	removal, err := p.removerManager.Remove(ctx, img, detection.Regions)
	if err != nil {
		return nil, fmt.Errorf("水印去除失败: %w", err)
	}
	result.Removal = removal

	// 检查质量是否达标
	if removal.Quality < p.config.Performance.QualityScore {
		p.logger.Warnf("去除质量不达标: %.2f < %.2f，可能需要人工检查",
			removal.Quality, p.config.Performance.QualityScore)
	}

	return result, nil
}

// DetectOnly 仅检测水印
func (p *Processor) DetectOnly(ctx context.Context, img image.Image) (*DetectionResult, error) {
	return p.detectorManager.Detect(ctx, img)
}

// RemoveOnly 仅去除水印（需要提供水印区域）
func (p *Processor) RemoveOnly(ctx context.Context, img image.Image, regions []*WatermarkRegion) (*RemovalResult, error) {
	return p.removerManager.Remove(ctx, img, regions)
}

// UpdateConfig 更新配置
func (p *Processor) UpdateConfig(config *Config) {
	p.config = config
	p.detectorManager = NewDetectorManager(config, p.logger)
	p.removerManager = NewRemoverManager(config, p.logger)
	p.logger.Info("水印处理器配置已更新")
}

// GetConfig 获取当前配置
func (p *Processor) GetConfig() *Config {
	return p.config
}
