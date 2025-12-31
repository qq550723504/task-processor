// Package main 提供产品获取功能测试入口
package main

import (
	"fmt"
	"log"
	"task-processor/internal/app/bootstrap"
)

func main() {
	fmt.Println("🧪 ProductFetcher 真实依赖测试")
	fmt.Println("================================")

	// 设置日志格式
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	// 初始化系统
	systemConfig := &bootstrap.SystemConfig{
		AppName: "product-fetcher-test",
		Version: "1.0.0",
	}

	systemInit := bootstrap.NewSystemInitializer(systemConfig)
	if err := systemInit.Initialize(); err != nil {
		log.Fatalf("❌ 系统初始化失败: %v", err)
	}
	defer systemInit.Shutdown()

	// 加载配置
	cfg, err := LoadConfig("config/config-dev.yaml")
	if err != nil {
		log.Fatalf("❌ 加载配置失败: %v", err)
	}

	// 创建测试服务
	testService, err := NewTestService(systemInit.GetContext(), cfg, systemInit.GetLogger("test"))
	if err != nil {
		log.Fatalf("❌ 创建测试服务失败: %v", err)
	}
	defer testService.Cleanup()
	testService.RunBasicTest()

}
