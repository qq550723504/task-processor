// Package utils 提供工具方法
package utils

import (
	"task-processor/internal/logger"

	"github.com/sirupsen/logrus"
)

// HelpPrinter 帮助信息打印器
type HelpPrinter struct {
	logger *logrus.Entry
}

// NewHelpPrinter 创建帮助信息打印器
func NewHelpPrinter() *HelpPrinter {
	return &HelpPrinter{
		logger: logger.GetGlobalLogger("help_printer"),
	}
}

// PrintHelp 打印帮助信息
func (h *HelpPrinter) PrintHelp() {
	h.logger.Info("Amazon爬虫工具 (Task Processor版本)")
	h.logger.Info("")
	h.logger.Info("用法:")
	h.logger.Info("  amazon-crawler -url=<Amazon产品页面URL> [-zipcode=<邮编>] [-region=<地区>] [-output=<输出文件路径>] [-config=<配置文件路径>]")
	h.logger.Info("")
	h.logger.Info("参数:")

	// 使用结构化日志记录参数信息
	h.logger.WithFields(logrus.Fields{
		"url":     "Amazon产品页面URL (必需)",
		"zipcode": "邮编 (可选)",
		"region":  "地区 (可选)",
		"output":  "输出文件路径 (可选)",
		"config":  "配置文件路径 (可选)",
	}).Info("可用参数")

	h.logger.Info("")
	h.logger.Info("示例:")

	examples := []string{
		"amazon-crawler -url=https://www.amazon.com/dp/B0F4X44ZRV -zipcode=94107",
		"amazon-crawler -region=jp -zipcode=100-0001",
		"amazon-crawler -url=https://www.amazon.co.jp/dp/B0F4X44ZRV -config=config/config-dev.yaml",
	}

	for i, example := range examples {
		h.logger.WithFields(logrus.Fields{
			"example_number": i + 1,
			"command":        example,
		}).Info("使用示例")
	}
}
