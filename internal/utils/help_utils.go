// Package utils 提供工具方法
package utils

import (
	"flag"
	"fmt"
)

// HelpPrinter 帮助信息打印器
type HelpPrinter struct{}

// NewHelpPrinter 创建帮助信息打印器
func NewHelpPrinter() *HelpPrinter {
	return &HelpPrinter{}
}

// PrintHelp 打印帮助信息
func (h *HelpPrinter) PrintHelp() {
	fmt.Println("Amazon爬虫工具 (Task Processor版本)")
	fmt.Println()
	fmt.Println("用法:")
	fmt.Println("  amazon-crawler -url=<Amazon产品页面URL> [-zipcode=<邮编>] [-region=<地区>] [-output=<输出文件路径>] [-config=<配置文件路径>]")
	fmt.Println()
	fmt.Println("参数:")
	flag.PrintDefaults()
	fmt.Println()
	fmt.Println("示例:")
	fmt.Println("  amazon-crawler -url=https://www.amazon.com/dp/B0F4X44ZRV -zipcode=94107")
	fmt.Println("  amazon-crawler -region=jp -zipcode=100-0001")
	fmt.Println("  amazon-crawler -url=https://www.amazon.co.jp/dp/B0F4X44ZRV -config=config/config-dev.yaml")
}
