// Package api 提供API处理层
package api

import (
	"context"
	"flag"

	"task-processor/internal/service"
	"task-processor/internal/utils"
)

// CLIHandler 命令行处理器
type CLIHandler struct {
	crawlerService *service.CrawlerService
	helpPrinter    *utils.HelpPrinter
}

// NewCLIHandler 创建命令行处理器
func NewCLIHandler(
	crawlerService *service.CrawlerService,
	helpPrinter *utils.HelpPrinter,
) *CLIHandler {
	return &CLIHandler{
		crawlerService: crawlerService,
		helpPrinter:    helpPrinter,
	}
}

// CLIArgs 命令行参数
type CLIArgs struct {
	URL        *string
	Zipcode    *string
	Region     *string
	Output     *string
	ConfigFile *string
	Help       *bool
}

// ParseArgs 解析命令行参数
func (h *CLIHandler) ParseArgs() *CLIArgs {
	return &CLIArgs{
		URL:        flag.String("url", "", "Amazon产品页面URL"),
		Zipcode:    flag.String("zipcode", "", "邮编"),
		Region:     flag.String("region", "us", "地区代码 (us, jp, uk, de, fr, ca, it, es, in, mx, br, au)"),
		Output:     flag.String("output", "output.json", "输出文件路径"),
		ConfigFile: flag.String("config", "", "配置文件路径（可选）"),
		Help:       flag.Bool("help", false, "显示帮助信息"),
	}
}

// HandleRequest 处理请求
func (h *CLIHandler) HandleRequest(ctx context.Context, args *CLIArgs) error {
	// 解析命令行参数
	flag.Parse()

	// 显示帮助信息
	if *args.Help {
		h.helpPrinter.PrintHelp()
		return nil
	}

	// 构建请求
	req := &service.CrawlerRequest{
		URL:        *args.URL,
		Zipcode:    *args.Zipcode,
		Region:     *args.Region,
		Output:     *args.Output,
		ConfigFile: *args.ConfigFile,
	}

	// 处理爬虫请求
	return h.crawlerService.ProcessProduct(ctx, req)
}
