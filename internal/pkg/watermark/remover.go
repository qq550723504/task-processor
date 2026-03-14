package watermark

import (
	"context"
	"fmt"
	"image"
	"time"

	"github.com/sirupsen/logrus"
)

// Remover 水印去除器接口
type Remover interface {
	// Remove 去除水印
	Remove(ctx context.Context, img image.Image, regions []*WatermarkRegion) (*RemovalResult, error)
	// GetMethod 获取去除方法
	GetMethod() RemovalMethod
}

// RemoverManager 去除器管理器
type RemoverManager struct {
	config   *Config
	removers map[RemovalMethod]Remover
	logger   *logrus.Logger
}

// NewRemoverManager 创建去除器管理器
func NewRemoverManager(config *Config, logger *logrus.Logger) *RemoverManager {
	if logger == nil {
		logger = logrus.New()
	}

	rm := &RemoverManager{
		config:   config,
		removers: make(map[RemovalMethod]Remover),
		logger:   logger,
	}

	// 注册去除器
	rm.registerRemovers()

	return rm
}

// registerRemovers 注册所有可用的去除器
func (rm *RemoverManager) registerRemovers() {
	// 注册传统去除器
	rm.removers[RemovalMethodInpaint] = NewInpaintRemover(rm.config, rm.logger)
	rm.removers[RemovalMethodBlur] = NewBlurRemover(rm.config, rm.logger)
	rm.removers[RemovalMethodCrop] = NewCropRemover(rm.config, rm.logger)

	// 注册AI去除器（如果启用）
	if rm.config.AI.LamaModel.Enabled {
		rm.removers[RemovalMethodAI] = NewAIRemover(rm.config, rm.logger)
	}
}

// Remove 去除水印
func (rm *RemoverManager) Remove(ctx context.Context, img image.Image, regions []*WatermarkRegion) (*RemovalResult, error) {
	if !rm.config.Enabled {
		return &RemovalResult{
			Success: false,
			Image:   img,
			Method:  rm.config.Removal.Method,
			Error:   "watermark removal is disabled",
		}, nil
	}

	if len(regions) == 0 {
		return &RemovalResult{
			Success: true,
			Image:   img,
			Method:  rm.config.Removal.Method,
			Quality: 1.0,
		}, nil
	}

	// 获取对应的去除器
	remover, ok := rm.removers[rm.config.Removal.Method]
	if !ok {
		return nil, fmt.Errorf("unsupported removal method: %s", rm.config.Removal.Method)
	}

	// 设置超时
	ctx, cancel := context.WithTimeout(ctx, time.Duration(rm.config.Performance.Timeout)*time.Second)
	defer cancel()

	// 执行去除
	startTime := time.Now()
	result, err := remover.Remove(ctx, img, regions)
	if err != nil {
		rm.logger.Errorf("水印去除失败: %v", err)
		return nil, err
	}

	result.ProcessTime = time.Since(startTime).Seconds()
	rm.logger.Infof("水印去除完成: 方法=%s, 耗时=%.2fs, 成功=%v, 质量=%.2f",
		result.Method, result.ProcessTime, result.Success, result.Quality)

	return result, nil
}

// GetRemover 获取指定方法的去除器
func (rm *RemoverManager) GetRemover(method RemovalMethod) (Remover, error) {
	remover, ok := rm.removers[method]
	if !ok {
		return nil, fmt.Errorf("remover not found: %s", method)
	}
	return remover, nil
}
