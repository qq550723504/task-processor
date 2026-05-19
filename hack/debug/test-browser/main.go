// 简单的浏览器测试程序，直接测试 Playwright 浏览器显示
package main

import (
	"fmt"
	"time"

	"github.com/playwright-community/playwright-go"
)

func main() {
	fmt.Println("=== 浏览器显示测试 ===")
	fmt.Println("此程序将直接测试 Playwright 浏览器是否可见")

	// 初始化 Playwright
	pw, err := playwright.Run()
	if err != nil {
		fmt.Printf("初始化 Playwright 失败: %v\n", err)
		return
	}
	defer pw.Stop()

	fmt.Println("Playwright 初始化成功")

	// 使用项目中的浏览器路径
	browserPath := "./.local/chrome/chrome.exe"
	fmt.Printf("使用浏览器路径: %s\n", browserPath)

	// 启动浏览器（非 headless 模式）
	fmt.Println("正在启动浏览器...")
	browser, err := pw.Chromium.Launch(playwright.BrowserTypeLaunchOptions{
		Headless:        playwright.Bool(false), // 非无头模式，应该显示窗口
		ExecutablePath:  playwright.String(browserPath),
		Args: []string{
			"--disable-infobars",
			"--start-maximized",
			"--no-sandbox",
		},
	})
	if err != nil {
		fmt.Printf("启动浏览器失败: %v\n", err)
		return
	}
	defer browser.Close()

	fmt.Println("浏览器启动成功！你应该能看到浏览器窗口")

	// 创建上下文
	context, err := browser.NewContext()
	if err != nil {
		fmt.Printf("创建上下文失败: %v\n", err)
		return
	}
	defer context.Close()

	// 创建页面
	page, err := context.NewPage()
	if err != nil {
		fmt.Printf("创建页面失败: %v\n", err)
		return
	}

	fmt.Println("正在导航到 1688 页面...")
	_, err = page.Goto("https://detail.1688.com/offer/722899324071.html")
	if err != nil {
		fmt.Printf("导航失败: %v\n", err)
	}

	fmt.Println("页面加载完成！浏览器窗口应该已经显示了")
	fmt.Println("等待 30 秒以便观察...")

	// 等待一段时间让用户观察
	for i := 30; i > 0; i-- {
		fmt.Printf("剩余 %d 秒...\r", i)
		time.Sleep(1 * time.Second)
	}

	fmt.Println("\n测试结束")
}
