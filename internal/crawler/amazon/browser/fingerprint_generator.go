// Package browser 提供浏览器指纹生成功能
package browser

import (
	"fmt"
	"math/rand"
	"task-processor/internal/crawler/amazon/antidetection"
	"time"
)

// FingerprintGenerator 指纹生成器，整合了antidetection包的功能
type FingerprintGenerator struct {
	antiFingerprintManager *antidetection.FingerprintManager
	antiGenerator          *antidetection.FingerprintGenerator
	random                 *rand.Rand
}

// NewFingerprintGenerator 创建指纹生成器
func NewFingerprintGenerator() *FingerprintGenerator {
	return &FingerprintGenerator{
		antiFingerprintManager: antidetection.NewFingerprintManager(),
		antiGenerator:          antidetection.NewFingerprintGenerator(),
		random:                 rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// GenerateFingerprint 生成指纹，转换为browser包的FingerprintConfig格式
func (fg *FingerprintGenerator) GenerateFingerprint(userID string) *FingerprintConfig {
	// 使用antidetection包生成完整的高级指纹
	advancedFingerprint := fg.antiGenerator.GenerateAdvancedFingerprint(userID)

	// 转换为browser包的格式，包含完整的指纹信息
	return &FingerprintConfig{
		Enable: advancedFingerprint.Enable,
		GPU: map[string]any{
			"vendor":      advancedFingerprint.WebGL["vendor"],
			"renderer":    advancedFingerprint.WebGL["renderer"],
			"description": advancedFingerprint.GPU["description"],
			"device":      advancedFingerprint.GPU["device"],
			// 添加完整的指纹信息
			"canvas":     advancedFingerprint.Canvas,
			"webrtc":     advancedFingerprint.WebRTC,
			"clientRect": advancedFingerprint.ClientRect,
			"languages":  advancedFingerprint.Languages,
		},
	}
}

// GenerateRandomFingerprint 生成随机指纹
func (fg *FingerprintGenerator) GenerateRandomFingerprint() *FingerprintConfig {
	userID := fmt.Sprintf("random_%d", time.Now().UnixNano())
	return fg.GenerateFingerprint(userID)
}

// GenerateUniqueFingerprint 为实例生成唯一指纹
func (fg *FingerprintGenerator) GenerateUniqueFingerprint(instanceID int) *FingerprintConfig {
	userID := fmt.Sprintf("instance_%d_%d", instanceID, time.Now().UnixNano())
	return fg.GenerateFingerprint(userID)
}

// GenerateAdvancedFingerprint 生成高级指纹配置，包含更多反检测特征
func (fg *FingerprintGenerator) GenerateAdvancedFingerprint(instanceID int) *FingerprintConfig {
	userID := fmt.Sprintf("advanced_instance_%d_%d", instanceID, time.Now().UnixNano())

	// 使用antidetection包生成完整的高级指纹
	advancedFingerprint := fg.antiGenerator.GenerateAdvancedFingerprint(userID)

	return &FingerprintConfig{
		Enable: advancedFingerprint.Enable,
		GPU: map[string]any{
			"vendor":      advancedFingerprint.WebGL["vendor"],
			"renderer":    advancedFingerprint.WebGL["renderer"],
			"description": advancedFingerprint.GPU["description"],
			"device":      advancedFingerprint.GPU["device"],
			// 完整的高级指纹信息
			"canvas":     advancedFingerprint.Canvas,
			"webrtc":     advancedFingerprint.WebRTC,
			"clientRect": advancedFingerprint.ClientRect,
			"languages":  advancedFingerprint.Languages,
			// 添加额外的反检测特征
			"platform":            "Win32",            // 固定为Windows平台
			"hardwareConcurrency": 8,                  // 硬件并发数
			"deviceMemory":        8,                  // 设备内存GB
			"colorDepth":          24,                 // 颜色深度
			"pixelDepth":          24,                 // 像素深度
			"timezone":            "America/New_York", // 时区
		},
	}
}

// GenerateStableFingerprint 为用户生成稳定的指纹（基于用户ID，每次生成相同）
func (fg *FingerprintGenerator) GenerateStableFingerprint(userID string) *FingerprintConfig {
	// 直接使用用户ID，不添加时间戳，确保每次生成相同的指纹
	advancedFingerprint := fg.antiGenerator.GenerateAdvancedFingerprint(userID)

	return &FingerprintConfig{
		Enable: advancedFingerprint.Enable,
		GPU: map[string]any{
			"vendor":      advancedFingerprint.WebGL["vendor"],
			"renderer":    advancedFingerprint.WebGL["renderer"],
			"description": advancedFingerprint.GPU["description"],
			"device":      advancedFingerprint.GPU["device"],
			"canvas":      advancedFingerprint.Canvas,
			"webrtc":      advancedFingerprint.WebRTC,
			"clientRect":  advancedFingerprint.ClientRect,
			"languages":   advancedFingerprint.Languages,
		},
	}
}

// ValidateFingerprint 验证指纹配置
func (fg *FingerprintGenerator) ValidateFingerprint(config *FingerprintConfig) bool {
	if config == nil {
		return false
	}

	if !config.Enable {
		return true // 禁用状态也是有效的
	}

	// 检查GPU配置
	if config.GPU == nil {
		return false
	}

	// 检查必要的GPU字段
	requiredFields := []string{"vendor", "renderer", "description"}
	for _, field := range requiredFields {
		if _, exists := config.GPU[field]; !exists {
			return false
		}
	}

	return true
}

// GetRandomGPUConfig 获取随机GPU配置
func (fg *FingerprintGenerator) GetRandomGPUConfig() map[string]any {
	userID := fmt.Sprintf("gpu_random_%d", time.Now().UnixNano())
	gpuConfig := fg.antiGenerator.SelectGPUConfig(userID)

	return map[string]any{
		"vendor":      gpuConfig.Vendor,
		"renderer":    gpuConfig.Renderer,
		"description": gpuConfig.Description,
		"device":      gpuConfig.Device,
	}
}

// GeneratePythonStyleFingerprint 生成完全匹配Python版本的随机指纹
func (fg *FingerprintGenerator) GeneratePythonStyleFingerprint(publicIP string) *FingerprintConfig {
	// 如果没有提供公网IP，使用一个默认的或随机生成
	if publicIP == "" {
		// 使用一些常见的公网IP作为默认值
		defaultIPs := []string{
			"38.180.8.14",
			"170.106.72.204",
			"203.208.60.1",
			"8.8.8.8",
			"1.1.1.1",
		}
		publicIP = defaultIPs[fg.random.Intn(len(defaultIPs))]
	}

	// 使用antidetection包生成Python风格的指纹
	pythonFingerprint := fg.antiGenerator.GeneratePythonStyleFingerprint(publicIP)

	// 转换为browser包的格式
	return &FingerprintConfig{
		Enable: true,
		GPU:    pythonFingerprint,
	}
}

// GenerateRandomFingerprintForInstance 为实例生成完全随机的指纹（匹配Python行为）
func (fg *FingerprintGenerator) GenerateRandomFingerprintForInstance(instanceID int, publicIP string) *FingerprintConfig {
	// 每次都生成完全随机的指纹，不基于实例ID
	return fg.GeneratePythonStyleFingerprint(publicIP)
}

// GenerateCompletelyRandomFingerprint 生成完全随机的指纹（每次都不同，不需要IP参数）
func (fg *FingerprintGenerator) GenerateCompletelyRandomFingerprint() *FingerprintConfig {
	// 使用空字符串，让方法内部随机选择IP
	return fg.GeneratePythonStyleFingerprint("")
}
