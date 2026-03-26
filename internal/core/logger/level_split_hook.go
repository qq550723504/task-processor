// Package logger 提供统一的日志管理功能
package logger

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/sirupsen/logrus"
)

// LevelSplitHook 按日志级别将日志写入不同文件的 Hook
type LevelSplitHook struct {
	writers   map[logrus.Level]io.Writer
	formatter logrus.Formatter
	mu        sync.Mutex
}

// LevelFileConfig 单个级别的文件配置
type LevelFileConfig struct {
	// Levels 该文件接收的日志级别，如 ["error", "fatal", "panic"]
	Levels []string `yaml:"levels" json:"levels"`
	// File 输出文件路径
	File string `yaml:"file" json:"file"`
}

// NewLevelSplitHook 创建按级别分文件的 Hook。
// configs 中每条规则将指定的多个级别写入同一个文件。
// formatter 为 nil 时使用 logrus 默认格式。
func NewLevelSplitHook(configs []LevelFileConfig, formatter logrus.Formatter) (*LevelSplitHook, error) {
	h := &LevelSplitHook{
		writers:   make(map[logrus.Level]io.Writer),
		formatter: formatter,
	}

	for _, cfg := range configs {
		if cfg.File == "" {
			continue
		}

		// 确保目录存在
		dir := filepath.Dir(cfg.File)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("创建日志目录失败 %s: %w", dir, err)
		}

		f, err := os.OpenFile(cfg.File, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return nil, fmt.Errorf("打开日志文件失败 %s: %w", cfg.File, err)
		}

		for _, lvlStr := range cfg.Levels {
			lvl, err := logrus.ParseLevel(strings.ToLower(lvlStr))
			if err != nil {
				return nil, fmt.Errorf("无效的日志级别 '%s': %w", lvlStr, err)
			}
			h.writers[lvl] = f
		}
	}

	return h, nil
}

// Levels 实现 logrus.Hook 接口，返回该 Hook 关注的所有级别
func (h *LevelSplitHook) Levels() []logrus.Level {
	h.mu.Lock()
	defer h.mu.Unlock()

	levels := make([]logrus.Level, 0, len(h.writers))
	for lvl := range h.writers {
		levels = append(levels, lvl)
	}
	return levels
}

// Fire 实现 logrus.Hook 接口，将日志条目写入对应文件
func (h *LevelSplitHook) Fire(entry *logrus.Entry) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	w, ok := h.writers[entry.Level]
	if !ok {
		return nil
	}

	formatter := h.formatter
	if formatter == nil {
		formatter = entry.Logger.Formatter
	}

	b, err := formatter.Format(entry)
	if err != nil {
		return fmt.Errorf("格式化日志失败: %w", err)
	}

	_, err = w.Write(b)
	return err
}
