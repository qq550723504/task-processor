package browser

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math/rand"
	"strconv"
	"time"
)

// FingerprintConfig 浏览器指纹配置（统一的类型定义）
type FingerprintConfig struct {
	Enable     bool              `json:"enable"`
	Canvas     map[string]any    `json:"canvas"`
	WebGL      map[string]string `json:"webgl"`
	WebRTC     map[string]string `json:"webrtc"`
	ClientRect float64           `json:"clientRect"`
	GPU        map[string]string `json:"gpu"`
	Languages  LanguageConfig    `json:"languages"`
	WebAudio   int               `json:"webaudio,omitempty"`
}

// GPUConfig GPU配置
type GPUConfig struct {
	Vendor      string `json:"vendor"`
	Renderer    string `json:"renderer"`
	Description string `json:"description"`
	Device      string `json:"device"`
}

// LanguageConfig 语言配置
type LanguageConfig struct {
	HTTP string `json:"http"`
	JS   string `json:"js"`
}

// IPConfig IP配置
type IPConfig struct {
	Public  string `json:"public"`
	Private string `json:"private"`
}

// FingerprintGenerator 指纹生成器
type FingerprintGenerator struct {
	gpuConfigs      []GPUConfig
	languageConfigs []LanguageConfig
	ipConfigs       []IPConfig
}

// NewFingerprintGenerator 创建指纹生成器实例
func NewFingerprintGenerator() *FingerprintGenerator {
	return &FingerprintGenerator{
		gpuConfigs: []GPUConfig{
			{
				Vendor:      "Google Inc. (NVIDIA)",
				Renderer:    "Google Inc. (NVIDIA) ANGLE (NVIDIA, NVIDIA GeForce RTX 3080 (0x00002684) Direct3D11 vs_5_0 ps_5_0, D3D11)",
				Description: "NVIDIA GeForce RTX 3080",
				Device:      "1.0003",
			},
			{
				Vendor:      "Google Inc. (NVIDIA)",
				Renderer:    "Google Inc. (NVIDIA) ANGLE (NVIDIA, NVIDIA GeForce RTX 4090 (0x00002684) Direct3D11 vs_5_0 ps_5_0, D3D11)",
				Description: "NVIDIA GeForce RTX 4090",
				Device:      "1.0004",
			},
			{
				Vendor:      "Google Inc. (AMD)",
				Renderer:    "Google Inc. (AMD) ANGLE (AMD, AMD Radeon RX 6800 XT (0x000073BF) Direct3D11 vs_5_0 ps_5_0, D3D11)",
				Description: "AMD Radeon RX 6800 XT",
				Device:      "2.0001",
			},
			{
				Vendor:      "Google Inc. (Intel)",
				Renderer:    "Google Inc. (Intel) ANGLE (Intel, Intel(R) UHD Graphics 630 (0x00003E9B) Direct3D11 vs_5_0 ps_5_0, D3D11)",
				Description: "Intel UHD Graphics 630",
				Device:      "3.0001",
			},
		},
		languageConfigs: []LanguageConfig{
			{HTTP: "en-US,en;q=0.9", JS: "en-US"},
			{HTTP: "zh-CN,zh;q=0.9,en;q=0.8", JS: "zh-CN"},
		},
		ipConfigs: []IPConfig{
			{Public: "170.106.72.204", Private: "192.168.1.1"},
			{Public: "203.208.60.1", Private: "192.168.0.100"},
			{Public: "8.8.8.8", Private: "10.0.0.1"},
			{Public: "1.1.1.1", Private: "172.16.0.1"},
			{Public: "114.114.114.114", Private: "192.168.2.1"},
		},
	}
}

// GenerateRandomFingerprint 生成完全随机的指纹（推荐使用）
func (fg *FingerprintGenerator) GenerateRandomFingerprint(publicIP string) *FingerprintConfig {
	// 如果没有提供公网IP，随机选择一个
	if publicIP == "" {
		defaultIPs := []string{
			"38.180.8.14",
			"170.106.72.204",
			"203.208.60.1",
			"8.8.8.8",
			"1.1.1.1",
		}
		publicIP = defaultIPs[rand.Intn(len(defaultIPs))]
	}

	// 创建临时随机数生成器，确保每次都是随机的
	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	// 生成随机的 canvas toDataUrl 数组（1-100，3个元素）
	canvasToDataURL := make([]int, 3)
	for i := range canvasToDataURL {
		canvasToDataURL[i] = r.Intn(100) + 1
	}

	// 生成随机NVIDIA GPU
	renderer := fg.generateRandomNvidiaGPU()

	// 随机生成 private IP 地址
	privateIP := fmt.Sprintf("192.168.%d.%d", r.Intn(255)+1, r.Intn(255)+1)

	// 随机生成 clientRect 浮点数（0-1之间，5位小数）
	clientRect := float64(r.Intn(100000)) / 100000.0

	// 随机生成 GPU description（5位随机大写字母+数字）
	chars := "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	gpuDescription := make([]byte, 5)
	for i := range gpuDescription {
		gpuDescription[i] = chars[r.Intn(len(chars))]
	}

	// 随机生成 GPU device（0-2之间，4位小数）
	gpuDevice := float64(r.Intn(20000)) / 10000.0

	// 随机生成 webaudio（1-500）
	webaudio := r.Intn(500) + 1

	return &FingerprintConfig{
		Enable: true,
		Canvas: map[string]any{
			"toDataUrl": canvasToDataURL,
		},
		WebGL: map[string]string{
			"vendor":   "Google Inc. (NVIDIA)",
			"renderer": renderer,
		},
		WebRTC: map[string]string{
			"public":  publicIP,
			"private": privateIP,
		},
		ClientRect: clientRect,
		GPU: map[string]string{
			"description": string(gpuDescription),
			"device":      fmt.Sprintf("%.4f", gpuDevice),
		},
		Languages: LanguageConfig{
			HTTP: "en-US,en;q=0.9",
			JS:   "en-US",
		},
		WebAudio: webaudio,
	}
}

// GenerateStableFingerprint 为用户生成稳定的指纹（基于用户ID，每次生成相同）
func (fg *FingerprintGenerator) GenerateStableFingerprint(userID string) *FingerprintConfig {
	gpuConfig := fg.selectGPUConfig(userID)
	languageConfig := fg.selectLanguageConfig(userID)
	ipConfig := fg.selectIPConfig(userID)
	canvasFingerprint := fg.generateCanvasFingerprint(userID)

	return &FingerprintConfig{
		Enable: true,
		Canvas: map[string]any{
			"toDataUrl": canvasFingerprint,
		},
		WebGL: map[string]string{
			"vendor":   gpuConfig.Vendor,
			"renderer": gpuConfig.Renderer,
		},
		WebRTC: map[string]string{
			"public":  ipConfig.Public,
			"private": ipConfig.Private,
		},
		ClientRect: fg.generateClientRectNoise(userID),
		GPU: map[string]string{
			"description": gpuConfig.Description,
			"device":      gpuConfig.Device,
		},
		Languages: languageConfig,
	}
}

// 私有方法
func (fg *FingerprintGenerator) getStableSeed(userID, suffix string) int64 {
	hashInput := userID + suffix
	hash := sha256.Sum256([]byte(hashInput))
	hashHex := hex.EncodeToString(hash[:])
	seed, _ := strconv.ParseInt(hashHex[:8], 16, 64)
	return seed
}

func (fg *FingerprintGenerator) generateCanvasFingerprint(userID string) []int {
	seed := fg.getStableSeed(userID, "canvas")
	r := rand.New(rand.NewSource(seed))
	result := make([]int, 3)
	for i := range result {
		result[i] = r.Intn(5) + 1
	}
	return result
}

func (fg *FingerprintGenerator) generateClientRectNoise(userID string) float64 {
	seed := fg.getStableSeed(userID, "rect")
	r := rand.New(rand.NewSource(seed))
	noise := r.Float64()*(0.01-0.0001) + 0.0001
	return float64(int(noise*1000000)) / 1000000
}

func (fg *FingerprintGenerator) selectGPUConfig(userID string) GPUConfig {
	seed := fg.getStableSeed(userID, "gpu")
	r := rand.New(rand.NewSource(seed))
	return fg.gpuConfigs[r.Intn(len(fg.gpuConfigs))]
}

func (fg *FingerprintGenerator) selectLanguageConfig(userID string) LanguageConfig {
	seed := fg.getStableSeed(userID, "lang")
	r := rand.New(rand.NewSource(seed))
	return fg.languageConfigs[r.Intn(len(fg.languageConfigs))]
}

func (fg *FingerprintGenerator) selectIPConfig(userID string) IPConfig {
	seed := fg.getStableSeed(userID, "ip")
	r := rand.New(rand.NewSource(seed))
	return fg.ipConfigs[r.Intn(len(fg.ipConfigs))]
}

// generateRandomNvidiaGPU 生成随机NVIDIA GPU信息（与Python版本完全一致）
// Python格式: f"{vendor} ({gpu_vendor}) {engine} ({gpu_vendor}, {selected_model} ({device_id}) {render_api} {vs_version} {ps_version}, {render_api})"
func (fg *FingerprintGenerator) generateRandomNvidiaGPU() string {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	// 固定的厂商和引擎部分（与Python一致）
	vendor := "Google Inc."
	engine := "ANGLE"
	gpuVendor := "NVIDIA"

	rtxModels := []string{
		"NVIDIA GeForce RTX 4090", "NVIDIA GeForce RTX 4080", "NVIDIA GeForce RTX 4070",
		"NVIDIA GeForce RTX 4060", "NVIDIA GeForce RTX 3060", "NVIDIA GeForce RTX 3070",
	}
	gtxModels := []string{
		"NVIDIA GeForce GTX 1660 Ti", "NVIDIA GeForce GTX 1650",
		"NVIDIA GeForce GTX 1080", "NVIDIA GeForce GTX 1070",
		"NVIDIA GeForce GTX 1060", "NVIDIA GeForce GTX 1050 Ti",
	}

	allModels := append(rtxModels, gtxModels...)
	selectedModel := allModels[r.Intn(len(allModels))]

	// 随机生成设备ID（16进制格式，与Python一致）
	// Python: prefix = "0x0000", suffix = "{:04d}".format(random.randint(1514, 8978))
	deviceID := fmt.Sprintf("0x0000%04d", r.Intn(8978-1514+1)+1514)

	// 渲染API和着色器版本（与Python一致，注意Python的ps_version实际是"vs_5_0"）
	renderAPI := "D3D11"
	vsVersion := "vs_5_0"
	psVersion := "vs_5_0" // Python版本: ps_version = "vs_5_0"（可能是笔误但保持一致）

	// 构建最终的GPU信息字符串（与Python格式完全一致）
	// Python: f"{vendor} ({gpu_vendor}) {engine} ({gpu_vendor}, {selected_model} ({device_id}) {render_api} {vs_version} {ps_version}, {render_api})"
	return fmt.Sprintf("%s (%s) %s (%s, %s (%s) %s %s %s, %s)",
		vendor, gpuVendor, engine, gpuVendor, selectedModel, deviceID, renderAPI, vsVersion, psVersion, renderAPI)
}
