// Package monitoring 提供进程信息管理
package monitoring

import (
	"sync"
	"time"
)

var (
	processStartTime int64
	processStartOnce sync.Once
)

// RecordProcessStartTime 记录进程启动时间
// 应该在main函数开始时调用
func RecordProcessStartTime() {
	processStartOnce.Do(func() {
		processStartTime = time.Now().Unix()
	})
}

// GetProcessStartTime 获取进程启动时间
func GetProcessStartTime() int64 {
	if processStartTime == 0 {
		// 如果没有显式记录，使用当前时间作为启动时间
		RecordProcessStartTime()
	}
	return processStartTime
}

// GetProcessUptime 获取进程运行时间（秒）
func GetProcessUptime() int64 {
	startTime := GetProcessStartTime()
	if startTime == 0 {
		return 0
	}
	return time.Now().Unix() - startTime
}
