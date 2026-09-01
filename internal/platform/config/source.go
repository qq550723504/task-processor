// Package config provides generic runtime configuration loading mechanisms.
package config

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/fsnotify/fsnotify"
)

// Source supplies configuration data and optional change notifications.
type Source interface {
	Read() ([]byte, error)
	Watch(context.Context, func([]byte)) error
	Name() string
}

type fileSource struct {
	filePath string
}

// NewFileSource creates a configuration source backed by path.
func NewFileSource(path string) Source {
	return &fileSource{filePath: path}
}

func (f *fileSource) Read() ([]byte, error) {
	if f.filePath == "" {
		return []byte{}, nil
	}

	if _, err := os.Stat(f.filePath); os.IsNotExist(err) {
		return nil, fmt.Errorf("配置文件不存在: %s", f.filePath)
	}

	data, err := os.ReadFile(f.filePath)
	if err != nil {
		return nil, fmt.Errorf("读取配置文件失败: %w", err)
	}
	return data, nil
}

func (f *fileSource) Watch(ctx context.Context, callback func([]byte)) error {
	if f.filePath == "" {
		return nil
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("创建文件监听器失败: %w", err)
	}

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
				if event.Name == f.filePath && event.Op&fsnotify.Write == fsnotify.Write {
					time.Sleep(100 * time.Millisecond)
					data, err := f.Read()
					if err != nil {
						continue
					}
					callback(data)
				}
			case _, ok := <-watcher.Errors:
				if !ok {
					return
				}
			}
		}
	}()

	return nil
}

func (f *fileSource) Name() string {
	if f.filePath == "" {
		return "default"
	}
	return fmt.Sprintf("file:%s", f.filePath)
}

type memorySource struct {
	name string
	data []byte
}

// NewMemorySource creates an in-memory configuration source.
func NewMemorySource(name string, data []byte) Source {
	return &memorySource{name: name, data: data}
}

func (m *memorySource) Read() ([]byte, error) {
	return m.data, nil
}

func (m *memorySource) Watch(context.Context, func([]byte)) error {
	return nil
}

func (m *memorySource) Name() string {
	return fmt.Sprintf("memory:%s", m.name)
}
