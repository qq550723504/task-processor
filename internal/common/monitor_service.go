package common

import (
	"fmt"
	"time"

	"task-processor/internal/pkg/management/api"

	"github.com/sirupsen/logrus"
)

// MonitorService 监控服务基础接口
type MonitorService interface {
	// Start 启动监控服务
	Start() error
	// Stop 停止监控服务
	Stop()
	// GetPlatformName 获取平台名称
	GetPlatformName() string
}

// PriceMonitorService 价格监控服务接口
type PriceMonitorService interface {
	MonitorService
	// CheckPriceChanges 检查价格变化
	CheckPriceChanges(storeID, tenantID int64) error
}

// StockMonitorService 库存监控服务接口
type StockMonitorService interface {
	MonitorService
	// CheckStockChanges 检查库存变化
	CheckStockChanges(storeID, tenantID int64) error
}

// MonitorConfig 监控配置
type MonitorConfig struct {
	CheckInterval        time.Duration // 产品检查间隔（价格+库存）
	BatchSize            int           // 批量处理大小
	EnablePriceAlert     bool          // 启用价格告警
	EnableStockAlert     bool          // 启用库存告警
	PriceChangeThreshold float64       // 价格变化阈值（百分比）
	StockChangeThreshold int           // 库存变化阈值
}

// DefaultMonitorConfig 默认监控配置
func DefaultMonitorConfig() *MonitorConfig {
	return &MonitorConfig{
		CheckInterval:        24 * time.Hour, // 每24小时检查一次
		BatchSize:            50,             // 每批处理50个产品
		EnablePriceAlert:     true,
		EnableStockAlert:     true,
		PriceChangeThreshold: 5.0, // 价格变化超过5%告警
		StockChangeThreshold: 5,   // 库存变化超过5个告警
	}
}

// PriceChangeEvent 价格变化事件
type PriceChangeEvent struct {
	TenantID          int64
	StoreID           int64
	Platform          string
	ProductID         string // ASIN
	SKU               string
	OldPrice          float64
	NewPrice          float64
	ChangePercent     float64
	PlatformProductID string
	Timestamp         time.Time
}

// StockChangeEvent 库存变化事件
type StockChangeEvent struct {
	TenantID          int64
	StoreID           int64
	Platform          string
	ProductID         string // ASIN
	SKU               string
	OldStock          int
	NewStock          int
	ChangeAmount      int
	PlatformProductID string
	Timestamp         time.Time
}

// MonitorEventHandler 监控事件处理器
type MonitorEventHandler interface {
	// OnPriceChange 价格变化处理
	OnPriceChange(event *PriceChangeEvent) error
	// OnStockChange 库存变化处理
	OnStockChange(event *StockChangeEvent) error
}

// LogMonitorEventHandler 日志监控事件处理器
type LogMonitorEventHandler struct {
	logger *logrus.Entry
}

// NewLogMonitorEventHandler 创建日志监控事件处理器
func NewLogMonitorEventHandler() *LogMonitorEventHandler {
	return &LogMonitorEventHandler{
		logger: logrus.WithField("component", "MonitorEventHandler"),
	}
}

// OnPriceChange 价格变化处理
func (h *LogMonitorEventHandler) OnPriceChange(event *PriceChangeEvent) error {
	h.logger.WithFields(logrus.Fields{
		"tenant_id":           event.TenantID,
		"store_id":            event.StoreID,
		"platform":            event.Platform,
		"product_id":          event.ProductID,
		"sku":                 event.SKU,
		"old_price":           event.OldPrice,
		"new_price":           event.NewPrice,
		"change_percent":      fmt.Sprintf("%.2f%%", event.ChangePercent),
		"platform_product_id": event.PlatformProductID,
	}).Warn("检测到价格变化")
	return nil
}

// OnStockChange 库存变化处理
func (h *LogMonitorEventHandler) OnStockChange(event *StockChangeEvent) error {
	h.logger.WithFields(logrus.Fields{
		"tenant_id":           event.TenantID,
		"store_id":            event.StoreID,
		"platform":            event.Platform,
		"product_id":          event.ProductID,
		"sku":                 event.SKU,
		"old_stock":           event.OldStock,
		"new_stock":           event.NewStock,
		"change_amount":       event.ChangeAmount,
		"platform_product_id": event.PlatformProductID,
	}).Warn("检测到库存变化")
	return nil
}

// BaseMonitorService 基础监控服务
type BaseMonitorService struct {
	Config        *MonitorConfig
	MappingClient api.ProductImportMappingAPI
	EventHandler  MonitorEventHandler
	StopChan      chan struct{}
	Logger        *logrus.Entry
}

// NewBaseMonitorService 创建基础监控服务
func NewBaseMonitorService(
	config *MonitorConfig,
	mappingClient api.ProductImportMappingAPI,
	eventHandler MonitorEventHandler,
) *BaseMonitorService {
	if config == nil {
		config = DefaultMonitorConfig()
	}
	if eventHandler == nil {
		eventHandler = NewLogMonitorEventHandler()
	}
	return &BaseMonitorService{
		Config:        config,
		MappingClient: mappingClient,
		EventHandler:  eventHandler,
		StopChan:      make(chan struct{}),
		Logger:        logrus.WithField("component", "BaseMonitorService"),
	}
}

// Stop 停止监控服务
func (s *BaseMonitorService) Stop() {
	close(s.StopChan)
	s.Logger.Info("监控服务已停止")
}
