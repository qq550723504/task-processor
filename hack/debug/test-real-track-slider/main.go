// 测试基于真实轨迹的滑动算法
package main

import (
	"fmt"
	"task-processor/internal/core/config"
	alibaba1688 "task-processor/internal/crawler/alibaba1688"

	"github.com/sirupsen/logrus"
)

func main() {
	logrus.SetLevel(logrus.InfoLevel)

	fmt.Println("=== 测试基于真实轨迹的滑动算法 ===")
	fmt.Println()

	cfg, err := config.LoadConfig()
	if err != nil {
		logrus.Fatalf("load config failed: %v", err)
	}

	fmt.Println("强制设置 Headless 为 false...")
	cfg.Browser.Headless = false
	fmt.Println()

	processor := alibaba1688.NewAlibaba1688Processor(cfg)
	defer processor.Shutdown()

	fmt.Println("测试URL: https://detail.1688.com/offer/722899324071.html")
	fmt.Println()
	fmt.Println("说明：")
	fmt.Println("- 程序会访问页面，等待验证码出现")
	fmt.Println("- 使用基于真实轨迹的优化算法进行滑动")
	fmt.Println("- 观察是否能通过验证码")
	fmt.Println()

	processor.Process("https://detail.1688.com/offer/722899324071.html")

	fmt.Println()
	fmt.Println("测试完成")
}
