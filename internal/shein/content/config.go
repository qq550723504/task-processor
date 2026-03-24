// Package content 提供SHEIN平台的敏感词配置管理功能
package content

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"task-processor/internal/pkg/jsonx"
	"time"

	"github.com/sirupsen/logrus"
)

// SensitiveWordConfig 敏感词配置结构（按语言分类）
type SensitiveWordConfig struct {
	StaticWords  map[string][]string `json:"static_words"`  // 按语言分类的静态敏感词
	DynamicWords map[string][]string `json:"dynamic_words"` // 按语言分类的动态敏感词
	LastUpdated  time.Time           `json:"last_updated"`
	Version      string              `json:"version"`
}

// SensitiveWordService 基于JSON文件的敏感词处理服务
type SensitiveWordService struct {
	configPath string
	config     *SensitiveWordConfig
	mutex      sync.RWMutex
	ctx        context.Context
	saveQueue  chan struct{}
	stopSave   chan struct{}
	wg         sync.WaitGroup
	logger     *logrus.Entry
}

// loadConfig 加载敏感词配置文件
func (s *SensitiveWordService) loadConfig() error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// 检查文件是否存在
	if _, err := os.Stat(s.configPath); os.IsNotExist(err) {
		s.logger.Warnf("⚠️ 敏感词配置文件不存在: %s，将创建默认配置", s.configPath)
		s.config = s.createDefaultConfig()

		// 确保目录存在
		if err := os.MkdirAll(filepath.Dir(s.configPath), 0755); err != nil {
			return fmt.Errorf("创建配置目录失败: %w", err)
		}

		// 保存默认配置
		if err := s.saveConfigUnlocked(); err != nil {
			return fmt.Errorf("保存默认配置失败: %w", err)
		}

		s.logger.Info("✅ 已创建并保存默认敏感词配置")
		return nil
	}

	// 读取配置文件
	data, err := os.ReadFile(s.configPath)
	if err != nil {
		return fmt.Errorf("读取敏感词配置文件失败: %w", err)
	}

	// 解析JSON
	var config SensitiveWordConfig
	if err := jsonx.UnmarshalBytes(data, &config, "解析敏感词配置文件失败"); err != nil {
		return err
	}

	s.config = &config
	s.logConfigLoadStats()
	return nil
}

// saveConfig 保存敏感词配置到文件
func (s *SensitiveWordService) saveConfig() error {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	return s.saveConfigUnlocked()
}

// saveConfigUnlocked 保存敏感词配置到文件（不加锁版本，内部使用）
func (s *SensitiveWordService) saveConfigUnlocked() error {
	if s.config == nil {
		return fmt.Errorf("配置为空，无法保存")
	}

	s.config.LastUpdated = time.Now()
	s.config.Version = "1.0"

	data, err := json.MarshalIndent(s.config, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化配置失败: %w", err)
	}

	// 确保目录存在
	if err := os.MkdirAll(filepath.Dir(s.configPath), 0755); err != nil {
		return fmt.Errorf("创建配置目录失败: %w", err)
	}

	if err := os.WriteFile(s.configPath, data, 0644); err != nil {
		return fmt.Errorf("写入配置文件失败: %w", err)
	}

	return nil
}

// createDefaultConfig 创建默认配置
func (s *SensitiveWordService) createDefaultConfig() *SensitiveWordConfig {
	return &SensitiveWordConfig{
		StaticWords: map[string][]string{
			"en": {
				"amazon", "ebay", "alibaba", "aliexpress", "walmart", "target",
				"brand", "trademark", "copyright", "patent", "licensed",
			},
			"zh": {
				"亚马逊", "淘宝", "天猫", "京东", "拼多多",
				"品牌", "商标", "版权", "专利", "授权",
			},
			"ja": {
				"アマゾン", "楽天", "ヤフー",
				"ブランド", "商標", "著作権", "特許",
			},
			"ko": {
				"아마존", "쿠팡", "네이버",
				"브랜드", "상표", "저작권", "특허",
			},
		},
		DynamicWords: make(map[string][]string),
		LastUpdated:  time.Now(),
		Version:      "1.0",
	}
}

// ReloadConfig 重新加载配置文件
func (s *SensitiveWordService) ReloadConfig() error {
	s.logger.Info("🔄 重新加载敏感词配置...")
	return s.loadConfig()
}

// saveConfigAsync 异步保存配置（使用工作队列）
func (s *SensitiveWordService) saveConfigAsync() {
	select {
	case s.saveQueue <- struct{}{}:
	default:
		s.logger.Warn("保存队列已满，跳过本次保存请求")
	}
}

// Close 关闭服务，等待保存任务完成
func (s *SensitiveWordService) Close() {
	close(s.stopSave)
	s.wg.Wait()
	s.logger.Info("敏感词服务已关闭")
}

