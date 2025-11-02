package antidetection

import (
	"math/rand"
	"time"
)

// FingerprintManager 浏览器指纹管理器
type FingerprintManager struct {
	random    *rand.Rand
	generator *FingerprintGenerator
}

// Fingerprint 浏览器指纹配置
type Fingerprint struct {
	Platform            string
	HardwareConcurrency int
	DeviceMemory        int
	ColorDepth          int
	PixelDepth          int
	Timezone            string
	WebGLVendor         string
	WebGLRenderer       string
}

// NewFingerprintManager 创建指纹管理器
func NewFingerprintManager() *FingerprintManager {
	return &FingerprintManager{
		random:    rand.New(rand.NewSource(time.Now().UnixNano())),
		generator: NewFingerprintGenerator(),
	}
}

// GetRandomFingerprint 获取随机指纹配置
func (fm *FingerprintManager) GetRandomFingerprint() *Fingerprint {
	platforms := []string{"Win32", "MacIntel", "Linux x86_64"}
	hardwareConcurrencies := []int{4, 8, 12, 16}
	deviceMemories := []int{4, 8, 16}
	timezones := []string{
		"America/New_York",
		"America/Los_Angeles",
		"America/Chicago",
		"America/Denver",
	}
	webglVendors := []string{
		"Intel Inc.",
		"Google Inc.",
		"NVIDIA Corporation",
	}
	webglRenderers := []string{
		"Intel Iris OpenGL Engine",
		"ANGLE (Intel, Intel(R) UHD Graphics 630, OpenGL 4.1)",
		"ANGLE (NVIDIA, NVIDIA GeForce GTX 1060, OpenGL 4.5)",
	}

	return &Fingerprint{
		Platform:            platforms[fm.random.Intn(len(platforms))],
		HardwareConcurrency: hardwareConcurrencies[fm.random.Intn(len(hardwareConcurrencies))],
		DeviceMemory:        deviceMemories[fm.random.Intn(len(deviceMemories))],
		ColorDepth:          24,
		PixelDepth:          24,
		Timezone:            timezones[fm.random.Intn(len(timezones))],
		WebGLVendor:         webglVendors[fm.random.Intn(len(webglVendors))],
		WebGLRenderer:       webglRenderers[fm.random.Intn(len(webglRenderers))],
	}
}
