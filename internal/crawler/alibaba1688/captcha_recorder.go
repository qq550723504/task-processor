package alibaba1688

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
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
	Track      []mousePoint `json:"track"`
	Distance   float64      `json:"distance"`
	TrackStartX float64     `json:"trackStartX"`
	TrackStartY float64     `json:"trackStartY"`
	StartX     float64      `json:"startX"`
	StartY     float64      `json:"startY"`
	EndX       float64      `json:"endX"`
	EndY       float64      `json:"endY"`
}

type trackRecorder struct {
	logger       *logrus.Entry
	savedTrack   *trackData
	mu           sync.Mutex
	recordCount  int
}

var globalRecorder *trackRecorder

func init() {
	globalRecorder = &trackRecorder{
		logger: logrus.WithFields(logrus.Fields{"component": "crawler/alibaba1688/track-recorder"}),
	}
}

// RecordAndPlay 录制并回放鼠标轨迹
func (tr *trackRecorder) RecordAndPlay(page playwright.Page, box *playwright.Rect, maxRetries int) error {
	startX := box.X + box.Width/2
	startY := box.Y + box.Height/2

	tr.logger.Info("==============================================")
	tr.logger.Info("  鼠标轨迹录制模式")
	tr.logger.Info("  请在浏览器中手动滑动验证码")
	tr.logger.Info("==============================================")

	// 尝试加载已保存的轨迹
	if tr.savedTrack != nil {
		tr.logger.Info("找到已保存的轨迹，是否直接使用? (y/n)")
		fmt.Print("> ")

		var input string
		fmt.Scanln(&input)

		if strings.ToLower(input) != "n" && strings.ToLower(input) != "no" {
			tr.logger.Infof("使用已保存的轨迹，滑动距离: %.1f px", tr.savedTrack.Distance)
			return tr.replayTrack(page, box, tr.savedTrack)
		}
	}

	// 开始录制
	tr.logger.Info("开始录制，请在5秒内手动滑动验证码...")

	result, err := page.Evaluate(`() => {
		return new Promise((resolve) => {
			const slider = document.querySelector('.nc_iconfont.btn_slide, #nc_1_n1z, .slider-button, .nc_wrapper .btn_slide, .nc-captcha-btn');
			if (!slider) {
				resolve({ success: false, message: '未找到滑块元素' });
				return;
			}

			const sliderRect = slider.getBoundingClientRect();
			const trackStartX = sliderRect.left + sliderRect.width / 2;
			const trackStartY = sliderRect.top + sliderRect.height / 2;

			const track = [];
			let isRecording = false;
			let startTime = 0;

			const onMouseDown = (e) => {
				if (!isRecording && e.clientX >= sliderRect.left - 30 && e.clientX <= sliderRect.right + 30 &&
					e.clientY >= sliderRect.top - 30 && e.clientY <= sliderRect.bottom + 30) {
					isRecording = true;
					startTime = performance.now();
					track.length = 0;
					track.push({ type: 'down', x: e.clientX, y: e.clientY, time: 0 });
				}
			};

			const onMouseMove = (e) => {
				if (isRecording) {
					const elapsed = performance.now() - startTime;
					track.push({ type: 'move', x: e.clientX, y: e.clientY, time: elapsed });
				}
			};

			const onMouseUp = (e) => {
				if (isRecording) {
					const elapsed = performance.now() - startTime;
					track.push({ type: 'up', x: e.clientX, y: e.clientY, time: elapsed });
					isRecording = false;
					resolve({
						success: true,
						message: '录制完成',
						track: track,
						trackStartX: trackStartX,
						trackStartY: trackStartY,
						distance: track.length > 0 ? track[track.length - 1].x - trackStartX : 0
					});
				}
			};

			document.addEventListener('mousedown', onMouseDown);
			document.addEventListener('mousemove', onMouseMove);
			document.addEventListener('mouseup', onMouseUp);

			setTimeout(() => {
				if (isRecording) {
					isRecording = false;
					resolve({
						success: true,
						message: '录制超时',
						track: track,
						trackStartX: trackStartX,
						trackStartY: trackStartY,
						distance: track.length > 0 ? track[track.length - 1].x - trackStartX : 0
					});
				} else {
					resolve({ success: false, message: '未检测到滑动' });
				}
				document.removeEventListener('mousedown', onMouseDown);
				document.removeEventListener('mousemove', onMouseMove);
				document.removeEventListener('mouseup', onMouseUp);
			}, 5000);
		});
	}`)

	if err != nil {
		return fmt.Errorf("录制失败: %w", err)
	}

	resultMap, ok := result.(map[string]interface{})
	if !ok || !resultMap["success"].(bool) {
		msg := "录制失败"
		if resultMap != nil {
			if m, ok := resultMap["message"].(string); ok {
				msg = m
			}
		}
		return errors.New(msg)
	}

	// 解析轨迹数据
	trackInterface, _ := resultMap["track"].([]interface{})
	track := make([]mousePoint, 0, len(trackInterface))
	for _, p := range trackInterface {
		if pm, ok := p.(map[string]interface{}); ok {
			pt := mousePoint{}
			if t, ok := pm["type"].(string); ok {
				pt.Type = t
			}
			if x, ok := pm["x"].(float64); ok {
				pt.X = x
			}
			if y, ok := pm["y"].(float64); ok {
				pt.Y = y
			}
			if tm, ok := pm["time"].(float64); ok {
				pt.Time = int64(tm)
			}
			track = append(track, pt)
		}
	}

	distance, _ := resultMap["distance"].(float64)
	trackStartX, _ := resultMap["trackStartX"].(float64)
	trackStartY, _ := resultMap["trackStartY"].(float64)

	if len(track) == 0 {
		return fmt.Errorf("录制的轨迹为空")
	}

	// 保存轨迹
	tr.mu.Lock()
	tr.savedTrack = &trackData{
		Track:       track,
		Distance:    distance,
		TrackStartX: trackStartX,
		TrackStartY: trackStartY,
		StartX:      startX,
		StartY:      startY,
		EndX:        startX + distance,
		EndY:        startY,
	}
	tr.recordCount++
	tr.mu.Unlock()

	tr.logger.Infof("录制成功! 共 %d 个轨迹点, 滑动距离: %.1f px", len(track), distance)

	// 保存到文件
	tr.saveTrackToFile(tr.savedTrack)

	// 回放轨迹
	return tr.replayTrack(page, box, tr.savedTrack)
}

// replayTrack 回放鼠标轨迹
func (tr *trackRecorder) replayTrack(page playwright.Page, box *playwright.Rect, data *trackData) error {
	if data == nil || len(data.Track) == 0 {
		return fmt.Errorf("轨迹数据为空")
	}

	tr.logger.Infof("开始回放轨迹，共 %d 个点", len(data.Track))

	// 移动到起点
	page.Mouse().Move(data.Track[0].X, data.Track[0].Y)
	time.Sleep(100 * time.Millisecond)

	// 按下鼠标
	page.Mouse().Down()
	time.Sleep(50 * time.Millisecond)

	// 回放轨迹
	for i, point := range data.Track {
		if point.Type == "down" {
			page.Mouse().Move(point.X, point.Y)
		} else if point.Type == "move" {
			// 计算与上一个点的时间差
			var delay time.Duration = 10
			if i > 0 {
				delay = time.Duration(point.Time-data.Track[i-1].Time) * time.Millisecond
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

	tr.logger.Info("轨迹回放完成")
	return nil
}

// saveTrackToFile 保存轨迹到文件
func (tr *trackRecorder) saveTrackToFile(data *trackData) error {
	filename := "captcha_track.json"
	file, err := os.Create(filename)
	if err != nil {
		tr.logger.Warnf("保存轨迹文件失败: %v", err)
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(data); err != nil {
		tr.logger.Warnf("保存轨迹数据失败: %v", err)
		return err
	}

	tr.logger.Infof("轨迹已保存到: %s", filename)
	return nil
}

// LoadTrackFromFile 从文件加载轨迹
func (tr *trackRecorder) LoadTrackFromFile(filename string) error {
	file, err := os.Open(filename)
	if err != nil {
		return fmt.Errorf("打开轨迹文件失败: %w", err)
	}
	defer file.Close()

	var data trackData
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&data); err != nil {
		return fmt.Errorf("解析轨迹文件失败: %w", err)
	}

	tr.mu.Lock()
	tr.savedTrack = &data
	tr.mu.Unlock()

	tr.logger.Infof("从文件加载轨迹成功，共 %d 个点, 距离: %.1f px", len(data.Track), data.Distance)
	return nil
}

// GetSavedTrack 获取已保存的轨迹
func (tr *trackRecorder) GetSavedTrack() *trackData {
	tr.mu.Lock()
	defer tr.mu.Unlock()
	return tr.savedTrack
}

// HasTrack 是否有保存的轨迹
func (tr *trackRecorder) HasTrack() bool {
	tr.mu.Lock()
	defer tr.mu.Unlock()
	return tr.savedTrack != nil && len(tr.savedTrack.Track) > 0
}
