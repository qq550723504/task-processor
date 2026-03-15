// Package handlers 提供属性处理上下文，统一管理属性处理过程中的数据和状态
package property

import (
	"context"
	"sync"
	"time"

	models "task-processor/internal/platforms/temu/api/product"
	"task-processor/internal/platforms/temu/handlers/common"
	"task-processor/internal/platforms/temu/types"

	"github.com/sirupsen/logrus"
)

// PropertyContext 属性处理上下文，包含处理过程中需要的所有数据和状态
type PropertyContext struct {
	// 基础上下文
	Context context.Context
	Logger  *logrus.Entry

	// 模板数据
	TemplateProperties []types.TemplateRespGoodsProperty

	// 当前属性数据
	CurrentProperties []models.PropertyItem

	// 属性特征缓存（避免重复计算）
	Features map[int]PropertyFeature
	mutex    sync.RWMutex

	// 处理统计
	Statistics *ProcessingStats

	// 缓存管理
	Cache PropertyCache

	// 处理配置
	Config *ProcessingConfig
}

// PropertyFeature 属性特征信息 (别名，使用common包中的定义)
type PropertyFeature = common.PropertyFeature

// ProcessingStats 处理统计信息
type ProcessingStats struct {
	StartTime       time.Time
	TotalProperties int
	ProcessedCount  int
	FixedCount      int
	SkippedCount    int
	ErrorCount      int
	StageStats      map[string]*StageStats
	mutex           sync.RWMutex
}

// StageStats 阶段统计信息
type StageStats struct {
	StageName      string
	StartTime      time.Time
	EndTime        time.Time
	Duration       time.Duration
	ProcessedCount int
	SuccessCount   int
	ErrorCount     int
}

// ProcessingConfig 处理配置
type ProcessingConfig struct {
	EnableStrictMode bool          // 是否启用严格模式
	EnableCache      bool          // 是否启用缓存
	CacheTTL         time.Duration // 缓存TTL
	MaxRetryCount    int           // 最大重试次数
	EnableStatistics bool          // 是否启用统计
	EnableParallel   bool          // 是否启用并行处理
}

// NewPropertyContext 创建新的属性处理上下文
func NewPropertyContext(ctx context.Context, logger *logrus.Entry) *PropertyContext {
	return &PropertyContext{
		Context:    ctx,
		Logger:     logger,
		Features:   make(map[int]PropertyFeature),
		Statistics: NewProcessingStats(),
		Cache:      NewPropertyCache(),
		Config:     NewDefaultProcessingConfig(),
	}
}

// GetFeature 获取属性特征（线程安全）
func (pc *PropertyContext) GetFeature(pid int) (PropertyFeature, bool) {
	pc.mutex.RLock()
	defer pc.mutex.RUnlock()
	feature, exists := pc.Features[pid]
	return feature, exists
}

// SetFeature 设置属性特征（线程安全）
func (pc *PropertyContext) SetFeature(pid int, feature PropertyFeature) {
	pc.mutex.Lock()
	defer pc.mutex.Unlock()
	pc.Features[pid] = feature
}

// AddProperty 添加属性到当前属性列表
func (pc *PropertyContext) AddProperty(property models.PropertyItem) {
	pc.CurrentProperties = append(pc.CurrentProperties, property)
}

// RemoveProperty 从当前属性列表中移除属性
func (pc *PropertyContext) RemoveProperty(pid, refPid int) {
	filtered := make([]models.PropertyItem, 0, len(pc.CurrentProperties))
	for _, prop := range pc.CurrentProperties {
		if prop.Pid != pid || prop.RefPid != refPid {
			filtered = append(filtered, prop)
		}
	}
	pc.CurrentProperties = filtered
}

// GetPropertiesByPID 根据PID获取属性列表
func (pc *PropertyContext) GetPropertiesByPID(pid int) []models.PropertyItem {
	var properties []models.PropertyItem
	for _, prop := range pc.CurrentProperties {
		if prop.Pid == pid {
			properties = append(properties, prop)
		}
	}
	return properties
}

// NewProcessingStats 创建新的处理统计
func NewProcessingStats() *ProcessingStats {
	return &ProcessingStats{
		StartTime:  time.Now(),
		StageStats: make(map[string]*StageStats),
	}
}

// RecordStageStart 记录阶段开始
func (ps *ProcessingStats) RecordStageStart(stageName string) {
	ps.mutex.Lock()
	defer ps.mutex.Unlock()

	ps.StageStats[stageName] = &StageStats{
		StageName: stageName,
		StartTime: time.Now(),
	}
}

// RecordStageEnd 记录阶段结束
func (ps *ProcessingStats) RecordStageEnd(stageName string, processedCount, successCount, errorCount int) {
	ps.mutex.Lock()
	defer ps.mutex.Unlock()

	if stats, exists := ps.StageStats[stageName]; exists {
		stats.EndTime = time.Now()
		stats.Duration = stats.EndTime.Sub(stats.StartTime)
		stats.ProcessedCount = processedCount
		stats.SuccessCount = successCount
		stats.ErrorCount = errorCount
	}
}

// GetTotalDuration 获取总处理时长
func (ps *ProcessingStats) GetTotalDuration() time.Duration {
	return time.Since(ps.StartTime)
}

// NewDefaultProcessingConfig 创建默认处理配置
func NewDefaultProcessingConfig() *ProcessingConfig {
	return &ProcessingConfig{
		EnableStrictMode: false,
		EnableCache:      true,
		CacheTTL:         30 * time.Minute,
		MaxRetryCount:    3,
		EnableStatistics: true,
		EnableParallel:   false, // 暂时禁用并行处理
	}
}
