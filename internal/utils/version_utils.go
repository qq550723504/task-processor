// Package utils 提供工具方法
package utils

import "github.com/sirupsen/logrus"

// VersionInfo 版本信息
type VersionInfo struct {
	Version   string
	BuildTime string
}

// PrintVersionInfo 打印版本信息
func PrintVersionInfo(logger *logrus.Logger, version VersionInfo) {
	logger.Infof("========================================")
	logger.Infof("Task Processor 启动")
	logger.Infof("版本: %s", version.Version)
	logger.Infof("构建时间: %s", version.BuildTime)
	logger.Infof("========================================")
}
