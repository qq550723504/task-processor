package antidetection

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

// FingerprintConfig 指纹配置
type FingerprintConfig struct {
	Enable     bool                   `json:"enable"`
	Canvas     map[string]interface{} `json:"canvas"`
	WebGL      map[string]string      `json:"webgl"`
	WebRTC     map[string]string      `json:"webrtc"`
	ClientRect float64                `json:"clientRect"`
	GPU        map[string]string      `json:"gpu"`
	Languages  LanguageConfig         `json:"languages"`
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
				Renderer:    "ANGLE (NVIDIA, NVIDIA GeForce RTX 3080 Direct3D11 vs_5_0 ps_5_0)",
				Description: "NVIDIA GeForce RTX 3080",
				Device:      "1.0003",
			},
			{
				Vendor:      "Google Inc. (NVIDIA)",
				Renderer:    "ANGLE (NVIDIA, NVIDIA GeForce RTX 4090 Direct3D11 vs_5_0 ps_5_0)",
				Description: "NVIDIA GeForce RTX 4090",
				Device:      "1.0004",
			},
			{
				Vendor:      "Google Inc. (AMD)",
				Renderer:    "ANGLE (AMD, AMD Radeon RX 6800 XT Direct3D11 vs_5_0 ps_5_0)",
				Description: "AMD Radeon RX 6800 XT",
				Device:      "2.0001",
			},
			{
				Vendor:      "Google Inc. (Intel)",
				Renderer:    "ANGLE (Intel, Intel(R) UHD Graphics 630 Direct3D11 vs_5_0 ps_5_0)",
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
