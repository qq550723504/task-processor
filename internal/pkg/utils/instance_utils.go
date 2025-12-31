// Package utils 提供工具方法
package utils

import (
	"os"

	"github.com/sirupsen/logrus"
)

// GetInstanceID 获取实例ID（统一实现）
func GetInstanceID() string {
	hostname, err := os.Hostname()
	if err != nil {
		return "unknown"
	}
	return hostname
}

// LogWithInstance 带实例ID的日志记录
func LogWithInstance(logger *logrus.Logger, format string, args ...interface{}) {
	instanceID := GetInstanceID()
	logger.Infof("[实例%s] "+format, append([]interface{}{instanceID}, args...)...)
}

// GetPodName 获取Pod名称（Kubernetes环境）
func GetPodName() string {
	podName := os.Getenv("POD_NAME")
	if podName == "" {
		return GetInstanceID()
	}
	return podName
}

// GetNamespace 获取命名空间（Kubernetes环境）
func GetNamespace() string {
	namespace := os.Getenv("POD_NAMESPACE")
	if namespace == "" {
		return "default"
	}
	return namespace
}

// InstanceInfo 实例信息
type InstanceInfo struct {
	ID        string
	PodName   string
	Namespace string
	Hostname  string
}

// GetInstanceInfo 获取完整的实例信息
func GetInstanceInfo() InstanceInfo {
	hostname, _ := os.Hostname()

	return InstanceInfo{
		ID:        GetInstanceID(),
		PodName:   GetPodName(),
		Namespace: GetNamespace(),
		Hostname:  hostname,
	}
}
