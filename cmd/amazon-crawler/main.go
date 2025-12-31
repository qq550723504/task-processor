package main

import (
	"context"
	"log"

	"task-processor/internal/app/api"
	"task-processor/internal/app/service"
	"task-processor/internal/infra/repo"
	"task-processor/internal/utils"
)

// go run cmd/amazon-crawler/main.go -url "https://www.amazon.com.mx/dp/B07W4BKR1T" -region "mx" -zipcode "11000" -output "test_B07W4BKR1T_mx.json"
func main() {
	// 初始化依赖
	deps := initializeDependencies()

	// 解析命令行参数
	args := deps.cliHandler.ParseArgs()

	// 处理请求
	ctx := context.Background()
	if err := deps.cliHandler.HandleRequest(ctx, args); err != nil {
		log.Fatalf("处理请求失败: %v", err)
	}
}

// Dependencies 依赖容器
type Dependencies struct {
	cliHandler *api.CLIHandler
}

// initializeDependencies 初始化依赖注入
func initializeDependencies() *Dependencies {
	// 创建工具层
	helpPrinter := utils.NewHelpPrinter()
	urlBuilder := utils.NewURLBuilder()

	// 创建仓储层
	fileRepo := repo.NewFileRepository()

	// 创建服务层
	configService := service.NewConfigService()
	crawlerService := service.NewCrawlerService(configService, fileRepo, urlBuilder)

	// 创建API层
	cliHandler := api.NewCLIHandler(crawlerService, helpPrinter)

	return &Dependencies{
		cliHandler: cliHandler,
	}
}
