// Package rabbitmq 提供配置管理功能
package rabbitmq

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
)

// ConfigManager 配置管理器
type ConfigManager struct {
	configPath  string
	config      *RabbitMQFullConfig
	configMutex sync.RWMutex
	logger      *logrus.Logger

	// 热更新
	watcher    *FileWatcher
	updateChan chan *RabbitMQFullConfig
	ctx        context.Context
	cancel     context.CancelFunc
	wg         sync.WaitGroup

	// 回调函数
	updateCallbacks []ConfigUpdateCallback
	callbackMutex   sync.RWMutex
}

// RabbitMQFullConfig 完整的RabbitMQ配置
type RabbitMQFullConfig struct {
	RabbitMQ       RabbitMQConfig `yaml:"rabbitmq"`
	ResultReporter ReporterConfig `yaml:"result_reporter"`
	LoadMonitor    MonitorConfig  `yaml:"load_monitor"`
	NodeConfig     NodeConfig     `yaml:"node"`
}

// NodeConfig 节点配置
type NodeConfig struct {
	NodeID          string        `yaml:"node_id"`
	MaxConcurrency  int           `yaml:"max_concurrency"`
	HealthCheckPort int           `yaml:"health_check_port"`
	MetricsPort     int           `yaml:"metrics_port"`
	LogLevel        string        `yaml:"log_level"`
	ShutdownTimeout time.Duration `yaml:"shutdown_timeout"`
}

// ConfigUpdateCallback 配置更新回调函数
type ConfigUpdateCallback func(oldConfig, newConfig *RabbitMQFullConfig) error

// FileWatcher 文件监控器
type FileWatcher struct {
	filePath    string
	lastModTime time.Time
	logger      *logrus.Logger
}

// NewConfigManager 创建配置管理器
func NewConfigManager(configPath string, logger *logrus.Logger) *ConfigManager {
	return &ConfigManager{
		configPath:      configPath,
		logger:          logger,
		updateChan:      make(chan *RabbitMQFullConfig, 10),
		updateCallbacks: make([]ConfigUpdateCallback, 0),
	}
}

// LoadConfig 加载配置
func (cm *ConfigManager) LoadConfig() error {
	cm.configMutex.Lock()
	defer cm.configMutex.Unlock()

	cm.logger.Infof("加载配置文件: %s", cm.configPath)

	// 读取配置文件
	data, err := os.ReadFile(cm.configPath)
	if err != nil {
		return fmt.Errorf("读取配置文件失败: %w", err)
	}

	// 解析YAML配置
	var config RabbitMQFullConfig
	err = yaml.Unmarshal(data, &config)
	if err != nil {
		return fmt.Errorf("解析配置文件失败: %w", err)
	}

	// 设置默认值
	cm.setDefaultValues(&config)

	// 验证配置
	if err := cm.validateConfig(&config); err != nil {
		return fmt.Errorf("配置验证失败: %w", err)
	}

	cm.config = &config
	cm.logger.Info("配置加载完成")
	return nil
}

// setDefaultValues 设置默认值
func (cm *ConfigManager) setDefaultValues(config *RabbitMQFullConfig) {
	// RabbitMQ默认值
	if config.RabbitMQ.URL == "" {
		config.RabbitMQ.URL = "amqp://guest:guest@localhost:5672/"
	}
	if config.RabbitMQ.ReconnectInterval == 0 {
		config.RabbitMQ.ReconnectInterval = 5 * time.Second
	}
	if config.RabbitMQ.MaxReconnectTries == 0 {
		config.RabbitMQ.MaxReconnectTries = 10
	}
	if config.RabbitMQ.Consumer.PrefetchCount == 0 {
		config.RabbitMQ.Consumer.PrefetchCount = 1
	}
	if config.RabbitMQ.Consumer.RetryDelay == 0 {
		config.RabbitMQ.Consumer.RetryDelay = 5 * time.Second
	}
	if config.RabbitMQ.Consumer.MaxRetries == 0 {
		config.RabbitMQ.Consumer.MaxRetries = 3
	}

	// 结果上报器默认值
	if config.ResultReporter.ReportURL == "" {
		config.ResultReporter.ReportURL = "http://localhost:8080/api/task/result"
	}
	if config.ResultReporter.Timeout == 0 {
		config.ResultReporter.Timeout = 30 * time.Second
	}
	if config.ResultReporter.BufferSize == 0 {
		config.ResultReporter.BufferSize = 1000
	}
	if config.ResultReporter.NodeID == "" {
		config.ResultReporter.NodeID = fmt.Sprintf("node-%d", time.Now().Unix())
	}

	// 负载监控默认值
	if config.LoadMonitor.UpdateInterval == 0 {
		config.LoadMonitor.UpdateInterval = 30 * time.Second
	}
	config.LoadMonitor.EnableCPU = true
	config.LoadMonitor.EnableMemory = true
	config.LoadMonitor.EnableTasks = true

	// 节点配置默认值
	if config.NodeConfig.NodeID == "" {
		config.NodeConfig.NodeID = config.ResultReporter.NodeID
	}
	if config.NodeConfig.MaxConcurrency == 0 {
		config.NodeConfig.MaxConcurrency = 5
	}
	if config.NodeConfig.HealthCheckPort == 0 {
		config.NodeConfig.HealthCheckPort = 8081
	}
	if config.NodeConfig.MetricsPort == 0 {
		config.NodeConfig.MetricsPort = 8082
	}
	if config.NodeConfig.LogLevel == "" {
		config.NodeConfig.LogLevel = "info"
	}
	if config.NodeConfig.ShutdownTimeout == 0 {
		config.NodeConfig.ShutdownTimeout = 30 * time.Second
	}
}

// validateConfig 验证配置
func (cm *ConfigManager) validateConfig(config *RabbitMQFullConfig) error {
	// 验证RabbitMQ配置
	if config.RabbitMQ.URL == "" {
		return fmt.Errorf("RabbitMQ URL不能为空")
	}

	if config.RabbitMQ.Consumer.PrefetchCount < 1 {
		return fmt.Errorf("Consumer PrefetchCount必须大于0")
	}

	// 验证结果上报器配置
	if config.ResultReporter.ReportURL == "" {
		return fmt.Errorf("结果上报URL不能为空")
	}

	if config.ResultReporter.BufferSize < 1 {
		return fmt.Errorf("结果上报缓冲区大小必须大于0")
	}

	// 验证节点配置
	if config.NodeConfig.MaxConcurrency < 1 {
		return fmt.Errorf("最大并发数必须大于0")
	}

	if config.NodeConfig.HealthCheckPort < 1 || config.NodeConfig.HealthCheckPort > 65535 {
		return fmt.Errorf("健康检查端口必须在1-65535范围内")
	}

	return nil
}

// GetConfig 获取配置
func (cm *ConfigManager) GetConfig() *RabbitMQFullConfig {
	cm.configMutex.RLock()
	defer cm.configMutex.RUnlock()

	if cm.config == nil {
		return nil
	}

	// 返回配置的深拷贝
	configCopy := *cm.config
	return &configCopy
}

// StartWatching 开始监控配置文件变化
func (cm *ConfigManager) StartWatching(ctx context.Context) error {
	cm.ctx, cm.cancel = context.WithCancel(ctx)

	// 创建文件监控器
	cm.watcher = &FileWatcher{
		filePath: cm.configPath,
		logger:   cm.logger,
	}

	// 获取初始修改时间
	if err := cm.watcher.updateModTime(); err != nil {
		return fmt.Errorf("获取配置文件修改时间失败: %w", err)
	}

	// 启动监控goroutine
	cm.wg.Add(2)
	go cm.watchConfigFile()
	go cm.handleConfigUpdates()

	cm.logger.Info("配置文件监控启动完成")
	return nil
}

// StopWatching 停止监控配置文件变化
func (cm *ConfigManager) StopWatching(ctx context.Context) error {
	cm.logger.Info("停止配置文件监控...")

	// 取消上下文
	if cm.cancel != nil {
		cm.cancel()
	}

	// 关闭更新通道
	close(cm.updateChan)

	// 等待goroutine完成
	done := make(chan struct{})
	go func() {
		defer close(done)
		cm.wg.Wait()
	}()

	select {
	case <-done:
		cm.logger.Info("配置文件监控停止完成")
	case <-ctx.Done():
		cm.logger.Warn("等待配置文件监控停止超时")
		return fmt.Errorf("停止配置文件监控超时")
	}

	return nil
}

// watchConfigFile 监控配置文件
func (cm *ConfigManager) watchConfigFile() {
	defer cm.wg.Done()
	defer func() {
		if r := recover(); r != nil {
			cm.logger.Errorf("配置文件监控发生panic: %v", r)
		}
	}()

	ticker := time.NewTicker(5 * time.Second) // 每5秒检查一次
	defer ticker.Stop()

	for {
		select {
		case <-cm.ctx.Done():
			cm.logger.Info("配置文件监控停止")
			return
		case <-ticker.C:
			if cm.watcher.hasChanged() {
				cm.logger.Info("检测到配置文件变化，重新加载...")
				if err := cm.reloadConfig(); err != nil {
					cm.logger.Errorf("重新加载配置失败: %v", err)
				}
			}
		}
	}
}

// handleConfigUpdates 处理配置更新
func (cm *ConfigManager) handleConfigUpdates() {
	defer cm.wg.Done()
	defer func() {
		if r := recover(); r != nil {
			cm.logger.Errorf("配置更新处理发生panic: %v", r)
		}
	}()

	for {
		select {
		case <-cm.ctx.Done():
			cm.logger.Info("配置更新处理停止")
			return
		case newConfig, ok := <-cm.updateChan:
			if !ok {
				cm.logger.Info("配置更新通道已关闭")
				return
			}

			cm.processConfigUpdate(newConfig)
		}
	}
}

// reloadConfig 重新加载配置
func (cm *ConfigManager) reloadConfig() error {
	// 读取新配置
	data, err := os.ReadFile(cm.configPath)
	if err != nil {
		return fmt.Errorf("读取配置文件失败: %w", err)
	}

	var newConfig RabbitMQFullConfig
	err = yaml.Unmarshal(data, &newConfig)
	if err != nil {
		return fmt.Errorf("解析配置文件失败: %w", err)
	}

	// 设置默认值和验证
	cm.setDefaultValues(&newConfig)
	if err := cm.validateConfig(&newConfig); err != nil {
		return fmt.Errorf("新配置验证失败: %w", err)
	}

	// 发送配置更新
	select {
	case cm.updateChan <- &newConfig:
		cm.logger.Info("配置更新已发送")
	case <-cm.ctx.Done():
		return fmt.Errorf("配置更新被取消")
	default:
		cm.logger.Warn("配置更新通道已满，跳过此次更新")
	}

	return nil
}

// processConfigUpdate 处理配置更新
func (cm *ConfigManager) processConfigUpdate(newConfig *RabbitMQFullConfig) {
	cm.configMutex.Lock()
	oldConfig := cm.config
	cm.config = newConfig
	cm.configMutex.Unlock()

	cm.logger.Info("配置已更新")

	// 调用回调函数
	cm.callbackMutex.RLock()
	callbacks := make([]ConfigUpdateCallback, len(cm.updateCallbacks))
	copy(callbacks, cm.updateCallbacks)
	cm.callbackMutex.RUnlock()

	for _, callback := range callbacks {
		if err := callback(oldConfig, newConfig); err != nil {
			cm.logger.Errorf("配置更新回调失败: %v", err)
		}
	}
}

// RegisterUpdateCallback 注册配置更新回调
func (cm *ConfigManager) RegisterUpdateCallback(callback ConfigUpdateCallback) {
	cm.callbackMutex.Lock()
	defer cm.callbackMutex.Unlock()

	cm.updateCallbacks = append(cm.updateCallbacks, callback)
}

// updateModTime 更新修改时间
func (fw *FileWatcher) updateModTime() error {
	info, err := os.Stat(fw.filePath)
	if err != nil {
		return err
	}

	fw.lastModTime = info.ModTime()
	return nil
}

// hasChanged 检查文件是否已更改
func (fw *FileWatcher) hasChanged() bool {
	info, err := os.Stat(fw.filePath)
	if err != nil {
		fw.logger.Errorf("检查文件状态失败: %v", err)
		return false
	}

	if info.ModTime().After(fw.lastModTime) {
		fw.lastModTime = info.ModTime()
		return true
	}

	return false
}

// SaveConfig 保存配置到文件
func (cm *ConfigManager) SaveConfig(config *RabbitMQFullConfig) error {
	cm.configMutex.Lock()
	defer cm.configMutex.Unlock()

	// 创建目录
	dir := filepath.Dir(cm.configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("创建配置目录失败: %w", err)
	}

	// 序列化配置
	data, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("序列化配置失败: %w", err)
	}

	// 写入文件
	err = os.WriteFile(cm.configPath, data, 0644)
	if err != nil {
		return fmt.Errorf("写入配置文件失败: %w", err)
	}

	cm.config = config
	cm.logger.Infof("配置已保存到: %s", cm.configPath)
	return nil
}

// GetConfigPath 获取配置文件路径
func (cm *ConfigManager) GetConfigPath() string {
	return cm.configPath
}
