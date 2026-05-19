// 录制回放测试程序
package main

import (
	"fmt"
	"log"
	"task-processor/internal/core/config"
	alibaba1688 "task-processor/internal/crawler/alibaba1688"

	"github.com/sirupsen/logrus"
)

func main() {
	logrus.SetLevel(logrus.DebugLevel)

	fmt.Println("=== 1688验证码录制回放测试程序 ===")
	fmt.Println("此程序会录制您的手动滑动轨迹，然后自动回放")
	fmt.Println()

	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("load config failed: %v", err)
	}

	fmt.Println("=== 配置信息 ===")
	fmt.Printf("Headless: %v\n", cfg.Browser.Headless)
	fmt.Printf("BrowserPath: %s\n", cfg.Browser.BrowserPath)
	fmt.Printf("Viewport: %dx%d\n", cfg.Browser.ViewportWidth, cfg.Browser.ViewportHeight)
	fmt.Println()

	fmt.Println("强制设置 Headless 为 false...")
	cfg.Browser.Headless = false
	fmt.Printf("修改后 Headless: %v\n", cfg.Browser.Headless)
	fmt.Println()

	processor := alibaba1688.NewAlibaba1688Processor(cfg)
	defer processor.Shutdown()

	fmt.Println("测试URL: https://detail.1688.com/offer/722899324071.html")
	fmt.Println()
	fmt.Println("调试信息说明：")
	fmt.Println("- 程序会检测验证码并尝试录制您的滑动轨迹")
	fmt.Println("- 录制完成后会自动回放")
	fmt.Println("- 您可以在浏览器中观察录制和回放过程")
	fmt.Println()

	processor.Process("https://detail.1688.com/offer/722899324071.html")

	fmt.Println()
	fmt.Println("测试完成")
}
