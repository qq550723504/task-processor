package browser

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math/rand"
	"strconv"
	"time"
)

// FingerprintConfig 浏览器指纹配置（简化版，适配fingerprint-chromium）
type FingerprintConfig struct {
	Enable    bool              `json:"enable"`
	GPU       map[string]string `json:"gpu"`       // GPU信息，用于生成--fingerprint-gpu-vendor参数
	WebRTC    map[string]string `json:"webrtc"`    // WebRTC IP信息
	Languages LanguageConfig    `json:"languages"` // 语言配置
}

// LanguageConfig 语言配置
type LanguageConfig struct {
	HTTP string `json:"http"`
	JS   string `json:"js"`
}

// FingerprintGenerator 指纹生成器（简化版）
type FingerprintGenerator struct {
	gpuModels []string
}

// NewFingerprintGenerator 创建指纹生成器实例
func NewFingerprintGenerator() *FingerprintGenerator {
	return &FingerprintGenerator{
		gpuModels: []string{
			"NVIDIA GeForce RTX 4090",
			"NVIDIA GeForce RTX 4080",
			"NVIDIA GeForce RTX 4070",
			"NVIDIA GeForce RTX 4060",
			"NVIDIA GeForce RTX 3080",
			"NVIDIA GeForce RTX 3070",
			"NVIDIA GeForce RTX 3060",
			"NVIDIA GeForce GTX 1660 Ti",
			"NVIDIA GeForce GTX 1650",
			"NVIDIA GeForce GTX 1080",
			"NVIDIA GeForce GTX 1070",
			"NVIDIA GeForce GTX 1060",
		},
	}
}

// GenerateRandomFingerprint 生成随机指纹（简化版，适配fingerprint-chromium）
func (fg *FingerprintGenerator) GenerateRandomFingerprint(publicIP string) *FingerprintConfig {
	// 如果没有提供公网IP，使用默认IP
	if publicIP == "" {
		publicIP = "8.8.8.8"
	}

	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	// 随机选择GPU型号
	gpuModel := fg.gpuModels[r.Intn(len(fg.gpuModels))]

	// 生成随机私有IP
	privateIP := fmt.Sprintf("192.168.%d.%d", r.Intn(255)+1, r.Intn(255)+1)

	return &FingerprintConfig{
		Enable: true,
		GPU: map[string]string{
			"description": gpuModel,
		},
		WebRTC: map[string]string{
			"public":  publicIP,
			"private": privateIP,
		},
		Languages: LanguageConfig{
			HTTP: "en-US,en;q=0.9",
			JS:   "en-US",
		},
	}
}

// GenerateStableFingerprint 生成稳定指纹（简化版，基于用户ID）
func (fg *FingerprintGenerator) GenerateStableFingerprint(userID string) *FingerprintConfig {
	// 基于用户ID生成稳定的种子
	seed := fg.getStableSeed(userID, "main")
	r := rand.New(rand.NewSource(seed))

	// 稳定选择GPU型号
	gpuModel := fg.gpuModels[r.Intn(len(fg.gpuModels))]

	// 生成稳定的私有IP
	privateIP := fmt.Sprintf("192.168.%d.%d", r.Intn(255)+1, r.Intn(255)+1)

	// 稳定的公网IP
	publicIPs := []string{"8.8.8.8", "1.1.1.1", "203.208.60.1"}
	publicIP := publicIPs[r.Intn(len(publicIPs))]

	return &FingerprintConfig{
		Enable: true,
		GPU: map[string]string{
			"description": gpuModel,
		},
		WebRTC: map[string]string{
			"public":  publicIP,
			"private": privateIP,
		},
		Languages: LanguageConfig{
			HTTP: "en-US,en;q=0.9",
			JS:   "en-US",
		},
	}
}

// getStableSeed 基于用户ID生成稳定的种子
func (fg *FingerprintGenerator) getStableSeed(userID, suffix string) int64 {
	hashInput := userID + suffix
	hash := sha256.Sum256([]byte(hashInput))
	hashHex := hex.EncodeToString(hash[:])
	seed, _ := strconv.ParseInt(hashHex[:8], 16, 64)
	return seed
}
