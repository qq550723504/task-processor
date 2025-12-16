// Package main 提供Amazon平台演示工具
package main

import (
	"fmt"
	"os"
	"task-processor/platforms/amazon"

	"github.com/sirupsen/logrus"
)

func main() {
	// 设置日志格式
	logrus.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
		ForceColors:   true,
	})

	logrus.Info("🎯 Amazon平台演示工具启动")

	// 打印架构信息
	amazon.PrintAmazonArchitecture()

	// 运行功能演示
	if err := amazon.DemoAmazonPlatform(); err != nil {
		logrus.Errorf("❌ 演示失败: %v", err)
		os.Exit(1)
	}

	// 导出演示数据
	demoData, err := amazon.ExportDemoData()
	if err != nil {
		logrus.Errorf("❌ 导出数据失败: %v", err)
		os.Exit(1)
	}

	fmt.Println("\n📊 演示数据:")
	fmt.Println(demoData)

	logrus.Info("🎉 Amazon平台演示完成")
}
