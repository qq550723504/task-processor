package prompt

import (
	"context"
	"fmt"
	"time"

	"task-processor/internal/core/logger"

	"github.com/fsnotify/fsnotify"
	"github.com/sirupsen/logrus"
)

// FileWatcher 文件变更监听器
type FileWatcher interface {
	// Watch 开始监听目录，文件变更时调用 callback
	Watch(ctx context.Context, dir string, callback func(changedPath string)) error
	// Close 停止监听
	Close() error
}

// fileWatcher FileWatcher 的具体实现，基于 fsnotify
type fileWatcher struct {
	watcher *fsnotify.Watcher
	log     *logrus.Entry
}

// NewFileWatcher 创建一个新的 FileWatcher 实例
func NewFileWatcher() (FileWatcher, error) {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("创建 fsnotify watcher 失败: %w", err)
	}
	return &fileWatcher{
		watcher: w,
		log:     logger.GetGlobalLogger("prompt.watcher"),
	}, nil
}

// NewFileWatcherWithLogger 创建一个使用指定 logger 的 FileWatcher 实例
func NewFileWatcherWithLogger(log *logrus.Entry) (FileWatcher, error) {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("创建 fsnotify watcher 失败: %w", err)
	}
	return &fileWatcher{watcher: w, log: log}, nil
}

// Watch 开始监听目录，Write 和 Create 事件触发后延迟 200ms 再调用 callback
// ctx 取消时优雅退出 goroutine 并关闭 watcher
func (fw *fileWatcher) Watch(ctx context.Context, dir string, callback func(changedPath string)) error {
	if err := fw.watcher.Add(dir); err != nil {
		return fmt.Errorf("添加目录监听失败 %s: %w", dir, err)
	}

	go fw.loop(ctx, callback)
	return nil
}

// loop 是事件处理的主循环，在独立 goroutine 中运行
func (fw *fileWatcher) loop(ctx context.Context, callback func(changedPath string)) {
	defer fw.watcher.Close()

	for {
		select {
		case <-ctx.Done():
			return

		case event, ok := <-fw.watcher.Events:
			if !ok {
				return
			}
			// 只处理 Write 和 Create 事件
			if event.Op&(fsnotify.Write|fsnotify.Create) == 0 {
				continue
			}
			changedPath := event.Name
			// 延迟 200ms 再回调，防止文件写入未完成
			go func() {
				time.Sleep(200 * time.Millisecond)
				callback(changedPath)
			}()

		case err, ok := <-fw.watcher.Errors:
			if !ok {
				return
			}
			fw.log.WithError(err).Error("文件监听器发生错误")
		}
	}
}

// Close 停止监听
func (fw *fileWatcher) Close() error {
	return fw.watcher.Close()
}
