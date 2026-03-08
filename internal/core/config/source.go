// Package config 提供配置源实现
package config

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/fsnotify/fsnotify"
)

// FileConfigSource 文件配置源
type FileConfigSource struct {
	filePath string
}

// NewFileConfigSource 创建文件配置源
func NewFileConfigSource(filePath string) ConfigSource {
	return &FileConfigSource{
		filePath: filePath,
	}
}

// Read 读取配置文件
func (f *FileConfigSource) Read() ([]byte, error) {
	if f.filePath == "" {
		// 如果没有指定配置文件，使用默认配置
		return []byte{}, nil
	}

	// 检查文件是否存在
	if _, err := os.Stat(f.filePath); os.IsNotExist(err) {
		return nil, fmt.Errorf("配置文件不存在: %s", f.filePath)
	}

	// 读取文件内容
	data, err := os.ReadFile(f.filePath)
	if err != nil {
		return nil, fmt.Errorf("读取配置文件失败: %w", err)
	}

	return data, nil
}

// Watch 监听配置文件变化
func (f *FileConfigSource) Watch(ctx context.Context, callback func([]byte)) error {
	if f.filePath == "" {
		// 没有配置文件，不需要监听
		return nil
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("创建文件监听器失败: %w", err)
	}

	// 监听配置文件目录
	dir := filepath.Dir(f.filePath)
	if err := watcher.Add(dir); err != nil {
		watcher.Close()
		return fmt.Errorf("添加文件监听失败: %w", err)
	}

	go func() {
		defer watcher.Close()

		for {
			select {
			case <-ctx.Done():
				return

			case event, ok := <-watcher.Events:
				if !ok {
					return
				}

				// 只处理配置文件的写入事件
				if event.Name == f.filePath && (event.Op&fsnotify.Write == fsnotify.Write) {
					// 延迟一下，避免文件还在写入中
					time.Sleep(100 * time.Millisecond)

					data, err := f.Read()
					if err != nil {
						continue
					}

					callback(data)
				}

			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				// 这里可以记录错误日志
				_ = err
			}
		}
	}()

	return nil
}

// Name 返回配置源名称
func (f *FileConfigSource) Name() string {
	if f.filePath == "" {
		return "default"
	}
	return fmt.Sprintf("file:%s", f.filePath)
}

// MemoryConfigSource 内存配置源（用于测试）
type MemoryConfigSource struct {
	name string
	data []byte
}

// NewMemoryConfigSource 创建内存配置源
func NewMemoryConfigSource(name string, data []byte) ConfigSource {
	return &MemoryConfigSource{
		name: name,
		data: data,
	}
}

// Read 读取内存中的配置数据
func (m *MemoryConfigSource) Read() ([]byte, error) {
	return m.data, nil
}

// Watch 内存配置源不支持监听
func (m *MemoryConfigSource) Watch(ctx context.Context, callback func([]byte)) error {
	// 内存配置源不支持监听，直接返回
	return nil
}

// Name 返回配置源名称
func (m *MemoryConfigSource) Name() string {
	return fmt.Sprintf("memory:%s", m.name)
}
