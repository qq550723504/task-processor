// 独立录制工具 - 使用JavaScript注入监听用户操作
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

type trackData struct {
	Track       []mousePoint `json:"track"`
	Distance    float64      `json:"distance"`
	TrackStartX float64      `json:"trackStartX"`
	TrackStartY float64      `json:"trackStartY"`
}

func main() {
	logrus.SetLevel(logrus.InfoLevel)

	fmt.Println("==============================================")
	fmt.Println("  1688验证码 - 独立录制工具")
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
		ExecutablePath: playwright.String("./.local/chrome/chrome.exe"),
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
	fmt.Println("==============================================")
	fmt.Println("  操作说明")
	fmt.Println("==============================================")
	fmt.Println("1. 请在浏览器中找到验证码滑块")
	fmt.Println("2. 将鼠标移到滑块上，按下鼠标左键")
	fmt.Println("3. 拖动滑块到终点位置，然后松开鼠标")
	fmt.Println("4. 程序将自动录制您的滑动轨迹")
	fmt.Println("5. 录制完成后，轨迹会自动保存到文件")
	fmt.Println()
	fmt.Println("按 Enter 键开始录制...")
	fmt.Scanln()

	fmt.Println("⏳ 开始录制，请滑动验证码...")

	// 注入JavaScript监听器
	_, err = page.Evaluate(`() => {
		window.__recorderTrack = [];
		window.__recorderStartTime = 0;
		window.__recorderIsRecording = false;

		const getMousePos = (e) => {
			return {
				x: e.pageX || e.clientX || e.screenX || 0,
				y: e.pageY || e.clientY || e.screenY || 0
			};
		};

		const onMouseDown = (e) => {
			console.log('mousedown detected');
			if (!window.__recorderIsRecording) {
				const pos = getMousePos(e);
				window.__recorderIsRecording = true;
				window.__recorderStartTime = performance.now();
				window.__recorderTrack = [];
				window.__recorderTrack.push({
					type: 'down',
					x: pos.x,
					y: pos.y,
					time: 0
				});
			}
		};

		const onMouseMove = (e) => {
			if (window.__recorderIsRecording) {
				const pos = getMousePos(e);
				const elapsed = performance.now() - window.__recorderStartTime;
				window.__recorderTrack.push({
					type: 'move',
					x: pos.x,
					y: pos.y,
					time: elapsed
				});
			}
		};

		const onMouseUp = (e) => {
			if (window.__recorderIsRecording) {
				const pos = getMousePos(e);
				const elapsed = performance.now() - window.__recorderStartTime;
				window.__recorderTrack.push({
					type: 'up',
					x: pos.x,
					y: pos.y,
					time: elapsed
				});
				window.__recorderIsRecording = false;
				console.log('mouseup detected, track length:', window.__recorderTrack.length);
			}
		};

		document.addEventListener('mousedown', onMouseDown, true);
		document.addEventListener('mousemove', onMouseMove, true);
		document.addEventListener('mouseup', onMouseUp, true);

		// 找到滑块元素并高亮显示
		const sliderSelectors = [
			'.nc_iconfont.btn_slide',
			'#nc_1_n1z',
			'.slider-button',
			'.nc_wrapper .btn_slide',
			'.nc-captcha-btn'
		];

		let sliderFound = false;
		for (const selector of sliderSelectors) {
			const slider = document.querySelector(selector);
			if (slider) {
				slider.style.outline = '3px solid red';
				slider.style.zIndex = '9999';
				console.log('滑块元素找到:', selector);
				sliderFound = true;
				break;
			}
		}

		if (!sliderFound) {
			console.log('未找到滑块元素，请手动滑动验证码');
		}

		return { success: true, message: '监听器已挂载' };
	}`)

	if err != nil {
		fmt.Printf("⚠️ 注入监听器失败: %v\n", err)
	} else {
		fmt.Println("✅ JavaScript监听器已挂载，开始录制...")
	}

	// 等待用户操作
	fmt.Println("⏳ 等待您滑动验证码 (最多等待30秒)...")

	var recordedTrackData interface{}
	var hasTrack bool

	for i := 0; i < 30; i++ {
		time.Sleep(1 * time.Second)

		// 检查是否有录制数据
		result, err := page.Evaluate(`() => {
			if (window.__recorderTrack && window.__recorderTrack.length > 2) {
				return {
					hasTrack: true,
					track: window.__recorderTrack,
					startTime: window.__recorderStartTime
				};
			}
			return { hasTrack: false };
		}`)

		if err != nil {
			continue
		}

		if resultMap, ok := result.(map[string]interface{}); ok {
			if hasTrackVal, ok := resultMap["hasTrack"].(bool); ok && hasTrackVal {
				recordedTrackData = resultMap
				hasTrack = true
				fmt.Println("📝 检测到轨迹数据！")
				break
			}
		}

		if (i+1)%5 == 0 {
			fmt.Printf("⏳ 已等待 %d/30 秒...\n", i+1)
		}
	}

	if !hasTrack {
		fmt.Println("❌ 未录制到任何轨迹")
		fmt.Println()
		fmt.Println("可能的原因:")
		fmt.Println("  1. 页面中没有验证码显示")
		fmt.Println("  2. 您没有在浏览器中操作")
		fmt.Println("  3. 浏览器窗口没有获得焦点")
		return
	}

	// 解析录制数据
	resultMap := recordedTrackData.(map[string]interface{})
	trackInterface := resultMap["track"].([]interface{})

	var track []mousePoint
	var startX, startY float64

	for i, point := range trackInterface {
		if pointMap, ok := point.(map[string]interface{}); ok {
			p := mousePoint{
				Type: pointMap["type"].(string),
				X:    pointMap["x"].(float64),
				Y:    pointMap["y"].(float64),
				Time: int64(pointMap["time"].(float64)),
			}
			track = append(track, p)

			if i == 0 {
				startX = p.X
				startY = p.Y
			}
		}
	}

	distance := track[len(track)-1].X - startX

	fmt.Printf("✅ 录制成功！\n")
	fmt.Printf("   轨迹点数: %d\n", len(track))
	fmt.Printf("   滑动距离: %.1f px\n", distance)
	fmt.Printf("   起点位置: (%.1f, %.1f)\n", startX, startY)
	fmt.Println()

	// 保存到文件
	data := struct {
		Track       []mousePoint `json:"track"`
		Distance    float64      `json:"distance"`
		TrackStartX float64      `json:"trackStartX"`
		TrackStartY float64      `json:"trackStartY"`
	}{
		Track:       track,
		Distance:    distance,
		TrackStartX: startX,
		TrackStartY: startY,
	}

	filename := "captcha_track.json"
	file, err := os.Create(filename)
	if err != nil {
		fmt.Printf("❌ 保存文件失败: %v\n", err)
		return
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(data); err != nil {
		fmt.Printf("❌ 写入数据失败: %v\n", err)
		return
	}

	fmt.Printf("✅ 轨迹已保存到: %s\n", filename)
	fmt.Println()

	fmt.Println("轨迹预览（前10个点）：")
	for i := 0; i < len(track) && i < 10; i++ {
		p := track[i]
		fmt.Printf("  [%3d] %s: (%.1f, %.1f) @ %dms\n", i, p.Type, p.X, p.Y, p.Time)
	}
	if len(track) > 10 {
		fmt.Printf("  ... 共 %d 个点\n", len(track))
	}
	fmt.Println()

	// 询问是否回放
	fmt.Print("是否回放录制的轨迹? (y/n): ")
	var input string
	fmt.Scanln(&input)

	if strings.ToLower(input) == "y" || strings.ToLower(input) == "yes" {
		fmt.Println("⏳ 开始回放...")
		time.Sleep(2 * time.Second)

		// 刷新页面
		if _, err := page.Goto(targetURL); err != nil {
			fmt.Printf("❌ 刷新页面失败: %v\n", err)
			return
		}

		time.Sleep(5 * time.Second)

		// 回放轨迹
		for i, point := range track {
			if point.Type == "down" {
				page.Mouse().Move(point.X, point.Y)
				time.Sleep(100 * time.Millisecond)
				page.Mouse().Down()
			} else if point.Type == "move" {
				var delay time.Duration = 10
				if i > 0 {
					delay = time.Duration(point.Time-track[i-1].Time) * time.Millisecond
					if delay < 1 {
						delay = 1
					}
					if delay > 50 {
						delay = 50
					}
				}
				page.Mouse().Move(point.X, point.Y)
				time.Sleep(delay)
			} else if point.Type == "up" {
				page.Mouse().Move(point.X, point.Y)
				time.Sleep(10 * time.Millisecond)
				page.Mouse().Up()
			}
		}

		fmt.Println("✅ 回放完成！请查看浏览器中的验证码状态")
		time.Sleep(5 * time.Second)
	}

	fmt.Println()
	fmt.Println("录制工具退出")
}
