// Package main 提供产品获取功能测试入口
package main

import (
	"fmt"
	"log"
	"task-processor/internal/core/system"
)

func main() {
	fmt.Println("🧪 ProductFetcher 真实依赖测试")
	fmt.Println("================================")

	// 设置日志格式
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	// 初始化系统
	systemConfig := &system.SystemConfig{
		AppName: "product-fetcher-test",
		Version: "1.0.0",
	}

	systemInit := system.NewSystemInitializer(systemConfig)
	if err := systemInit.Initialize(); err != nil {
		log.Fatalf("❌ 系统初始化失败: %v", err)
	}
	defer systemInit.Shutdown()

}
