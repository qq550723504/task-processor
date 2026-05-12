// 简化版录制工具 - 用于测试
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/playwright-community/playwright-go"
	"github.com/sirupsen/logrus"
)

type mousePoint struct {
	Type string  `json:"type"`
	X    float64 `json:"x"`
	Y    float64 `json:"y"`
	Time int64   `json:"time"`
}

func main() {
	logrus.SetLevel(logrus.InfoLevel)

	fmt.Println("==============================================")
	fmt.Println("  1688验证码 - 简单录制工具")
	fmt.Println("==============================================")
	fmt.Println()

	pw, err := playwright.Run()
	if err != nil {
		fmt.Printf("❌ 初始化 Playwright 失败: %v\n", err)
		return
	}
	defer pw.Stop()

	fmt.Println("✅ Playwright 初始化成功")

	browser, err := pw.Chromium.Launch(playwright.BrowserTypeLaunchOptions{
		Headless:       playwright.Bool(false),
		ExecutablePath: playwright.String("./chrome/chrome.exe"),
	})
	if err != nil {
		fmt.Printf("❌ 启动浏览器失败: %v\n", err)
		return
	}
	defer browser.Close()

	fmt.Println("✅ 浏览器启动成功！")

	context, err := browser.NewContext()
	if err != nil {
		fmt.Printf("❌ 创建上下文失败: %v\n", err)
		return
	}
	defer context.Close()

	page, err := context.NewPage()
	if err != nil {
		fmt.Printf("❌ 创建页面失败: %v\n", err)
		return
	}

	if err := page.SetViewportSize(1920, 1080); err != nil {
		fmt.Printf("⚠️ 设置视口失败: %v\n", err)
	}

	targetURL := "https://detail.1688.com/offer/722899324071.html"
	fmt.Printf("正在导航到: %s\n", targetURL)

	if _, err := page.Goto(targetURL); err != nil {
		fmt.Printf("❌ 导航失败: %v\n", err)
		return
	}

	fmt.Println("✅ 页面加载完成！")
	fmt.Println()

	// 检查是否有验证码
	fmt.Println("🔍 检查页面状态...")
	title, _ := page.Title()
	fmt.Printf("📄 页面标题: %s\n", title)

	// 检查滑块元素
	sliderSelectors := []string{
		".nc_iconfont.btn_slide",
		"#nc_1_n1z",
		".slider-button",
		".nc_wrapper .btn_slide",
		".nc-captcha-btn",
	}

	var sliderFound bool

	for _, sel := range sliderSelectors {
		el, _ := page.QuerySelector(sel)
		if el != nil {
			sliderFound = true
			fmt.Printf("✅ 找到滑块元素: %s\n", sel)

			// 高亮显示
			page.Evaluate(`(selector) => {
				const el = document.querySelector(selector);
				if (el) {
					el.style.outline = '4px solid red';
					el.style.backgroundColor = 'rgba(255, 0, 0, 0.2)';
				}
			}`, sel)
			break
		}
	}

	if !sliderFound {
		fmt.Println("⚠️ 页面中未检测到验证码滑块")
		fmt.Println("   可能原因：")
		fmt.Println("   1. 1688未检测到爬虫行为")
		fmt.Println("   2. 需要刷新页面或访问其他链接")
		fmt.Println()
	}

	fmt.Println()
	fmt.Println("==============================================")
	fmt.Println("  操作步骤：")
	fmt.Println("==============================================")
	fmt.Println("1. 保持此终端窗口在前台")
	fmt.Println("2. 在浏览器中找到红色边框的滑块")
	fmt.Println("3. 将鼠标移到滑块上，按下左键")
	fmt.Println("4. 拖动滑块到终点，松开鼠标")
	fmt.Println("5. 等待5秒让程序检测到轨迹")
	fmt.Println("6. 程序会自动保存轨迹到 captcha_track.json")
	fmt.Println()
	fmt.Println("按 Enter 键开始监听...")
	fmt.Scanln()

	// 注入JavaScript监听器
	fmt.Println("🔧 注入监听器...")
	_, err = page.Evaluate(`() => {
		window.__captchaTrack = [];
		window.__captchaStartTime = 0;
		window.__captchaRecording = false;

		console.log('Recorder: Initialized');

		const onMouseDown = (e) => {
			if (!window.__captchaRecording) {
				console.log('Recorder: mousedown at', e.clientX, e.clientY);
				window.__captchaRecording = true;
				window.__captchaStartTime = Date.now();
				window.__captchaTrack = [];
				window.__captchaTrack.push({
					type: 'down',
					x: e.clientX || e.pageX || e.screenX,
					y: e.clientY || e.pageY || e.screenY,
					time: 0
				});
			}
		};

		const onMouseMove = (e) => {
			if (window.__captchaRecording) {
				window.__captchaTrack.push({
					type: 'move',
					x: e.clientX || e.pageX || e.screenX,
					y: e.clientY || e.pageY || e.screenY,
					time: Date.now() - window.__captchaStartTime
				});
			}
		};

		const onMouseUp = (e) => {
			if (window.__captchaRecording) {
				console.log('Recorder: mouseup, track length:', window.__captchaTrack.length);
				window.__captchaTrack.push({
					type: 'up',
					x: e.clientX || e.pageX || e.screenX,
					y: e.clientY || e.pageY || e.screenY,
					time: Date.now() - window.__captchaStartTime
				});
				window.__captchaRecording = false;
			}
		};

		window.addEventListener('mousedown', onMouseDown, true);
		window.addEventListener('mousemove', onMouseMove, true);
		window.addEventListener('mouseup', onMouseUp, true);

		return { status: 'ready' };
	}`)

	if err != nil {
		fmt.Printf("⚠️ 注入失败: %v\n", err)
	} else {
		fmt.Println("✅ 监听器已就绪")
	}

	fmt.Println()
	fmt.Println("⏳ 请在浏览器中滑动验证码（5秒后检测）...")
	time.Sleep(5 * time.Second)

	// 获取录制数据
	fmt.Println("🔍 检测录制数据...")
	result, err := page.Evaluate(`() => {
		if (window.__captchaTrack && window.__captchaTrack.length > 0) {
			return {
				success: true,
				track: window.__captchaTrack,
				length: window.__captchaTrack.length
			};
		}
		return { success: false, length: 0 };
	}`)

	if err != nil {
		fmt.Printf("❌ 获取数据失败: %v\n", err)
		return
	}

	resultMap, ok := result.(map[string]interface{})
	if !ok {
		fmt.Println("❌ 数据解析失败")
		return
	}

	success, _ := resultMap["success"].(bool)
	if !success {
		fmt.Println("❌ 未录制到轨迹")
		fmt.Println()
		fmt.Println("💡 提示：")
		fmt.Println("   1. 确认浏览器窗口是活动窗口")
		fmt.Println("   2. 直接在浏览器中操作，不要通过远程桌面")
		fmt.Println("   3. 可以刷新页面后重试")
		return
	}

	trackLenVal, _ := resultMap["length"]
	var trackLen int
	switch v := trackLenVal.(type) {
	case float64:
		trackLen = int(v)
	case int:
		trackLen = v
	}
	fmt.Printf("✅ 录制到 %d 个轨迹点\n", trackLen)

	trackInterface, _ := resultMap["track"].([]interface{})
	var track []mousePoint

	for i, p := range trackInterface {
		if pointMap, ok := p.(map[string]interface{}); ok {
			pt := mousePoint{}

			if typeVal, ok := pointMap["type"].(string); ok {
				pt.Type = typeVal
			}

			switch v := pointMap["x"].(type) {
			case float64:
				pt.X = v
			case int:
				pt.X = float64(v)
			}

			switch v := pointMap["y"].(type) {
			case float64:
				pt.Y = v
			case int:
				pt.Y = float64(v)
			}

			switch v := pointMap["time"].(type) {
			case float64:
				pt.Time = int64(v)
			case int:
				pt.Time = int64(v)
			}

			track = append(track, pt)

			if i < 3 {
				fmt.Printf("  [%d] %s: (%.1f, %.1f) @ %dms\n", i, pt.Type, pt.X, pt.Y, pt.Time)
			}
		}
	}

	// 保存文件
	filename := "captcha_track.json"
	file, err := os.Create(filename)
	if err != nil {
		fmt.Printf("❌ 创建文件失败: %v\n", err)
		return
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(track); err != nil {
		fmt.Printf("❌ 写入失败: %v\n", err)
		return
	}

	fmt.Printf("✅ 轨迹已保存到: %s\n", filename)
	fmt.Println()

	// 询问是否回放
	fmt.Print("是否回放轨迹? (y/n): ")
	var input string
	fmt.Scanln(&input)

	if strings.ToLower(input) == "y" || strings.ToLower(input) == "yes" {
		fmt.Println("⏳ 准备回放...")
		time.Sleep(2 * time.Second)

		if _, err := page.Goto(targetURL); err != nil {
			fmt.Printf("❌ 刷新失败: %v\n", err)
			return
		}

		time.Sleep(3 * time.Second)

		fmt.Println("🎬 开始回放...")
		for i, pt := range track {
			if pt.Type == "down" {
				page.Mouse().Move(pt.X, pt.Y)
				time.Sleep(100 * time.Millisecond)
				page.Mouse().Down()
			} else if pt.Type == "move" {
				delay := 10 * time.Millisecond
				if i > 0 {
					delay = time.Duration(pt.Time-track[i-1].Time) * time.Millisecond
					if delay < 5 {
						delay = 5
					}
					if delay > 30 {
						delay = 30
					}
				}
				page.Mouse().Move(pt.X, pt.Y)
				time.Sleep(delay)
			} else if pt.Type == "up" {
				page.Mouse().Move(pt.X, pt.Y)
				time.Sleep(20 * time.Millisecond)
				page.Mouse().Up()
			}
		}

		fmt.Println("✅ 回放完成！")
		fmt.Println("⏳ 等待3秒查看结果...")
		time.Sleep(3 * time.Second)
	}

	fmt.Println()
	fmt.Println("程序退出")
}
