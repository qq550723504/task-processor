package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

// getInt64Slice 从viper获取int64切片的辅助函数
func getInt64Slice(key string) []int64 {
	if ifaceSlice := viper.Get(key); ifaceSlice != nil {
		switch v := ifaceSlice.(type) {
		case []interface{}:
			result := make([]int64, len(v))
			for i, val := range v {
				switch val := val.(type) {
				case int64:
					result[i] = val
				case int:
					result[i] = int64(val)
				case float64:
					result[i] = int64(val)
				case string:
					if intVal, err := strconv.ParseInt(val, 10, 64); err == nil {
						result[i] = intVal
					}
				}
			}
			return result
		case []int64:
			return v
		case []int:
			result := make([]int64, len(v))
			for i, val := range v {
				result[i] = int64(val)
			}
			return result
		case string:
			if v != "" {
				parts := strings.Split(v, ",")
				result := make([]int64, 0, len(parts))
				for _, part := range parts {
					part = strings.TrimSpace(part)
					if part != "" {
						if intVal, err := strconv.ParseInt(part, 10, 64); err == nil {
							result = append(result, intVal)
						}
					}
				}
				return result
			}
		}
	}
	return []int64{}
}

// Config 配置结构体
type Config struct {
	Processor   ProcessorConfig
	Worker      WorkerConfig
	Server      ServerConfig
	OpenAI      OpenAIConfig
	Management  ManagementConfig
	AutoPricing AutoPricingConfig
	Amazon      AmazonConfig
	Updater     UpdaterConfig
	Platform    PlatformConfig // 平台特定配置
}

// ProcessorConfig 处理器配置
type ProcessorConfig struct {
	MaxRetries int
	Timeout    int // 单位：秒
}

// WorkerConfig 工作池配置
type WorkerConfig struct {
	Concurrency  int
	BufferSize   int
	TaskInterval int // 任务提交间隔（秒）
}

// ServerConfig 服务器配置
type ServerConfig struct {
	Port int
}

// OpenAIConfig OpenAI配置
type OpenAIConfig struct {
	APIKey  string
	Model   string
	BaseURL string
	Timeout int // 单位：秒
}

// ManagementConfig 管理系统配置
type ManagementConfig struct {
	BaseURL      string
	ClientID     string
	ClientSecret string
	TokenURL     string
	Scopes       []string
	UserID       int64
	StoreIDs     []int64
}

// AutoPricingConfig 自动核价配置
type AutoPricingConfig struct {
	Enabled   bool
	Interval  int
	BatchSize int
}

// AmazonConfig Amazon爬虫配置
type AmazonConfig struct {
	Enabled        bool
	Headless       bool
	BrowserPath    string
	PoolSize       int
	Zipcodes       map[string]string
	ViewportWidth  int
	ViewportHeight int
	ProxyServer    string
}

// UpdaterConfig 自动更新配置
type UpdaterConfig struct {
	Enabled       bool
	ServerURL     string
	CheckInterval int
	AutoApply     bool
	SkipChecksum  bool
}

// PlatformConfig 平台特定配置
type PlatformConfig struct {
	Name string // "temu" 或 "shein"
	Type string // "web" 或 "cli"
}

// LoadConfig 加载配置
func LoadConfig(platform string) *Config {
	env := os.Getenv("TASK_PROCESSOR_ENV")
	if env == "" {
		env = "dev"
	}

	configName := fmt.Sprintf("config-%s-%s", platform, env)
	logrus.Printf("加载配置文件: %s.yaml", configName)

	viper.SetConfigName(configName)
	viper.SetConfigType("yaml")

	// 获取可执行文件所在目录
	exePath, err := os.Executable()
	if err == nil {
		exeDir := filepath.Dir(exePath)
		viper.AddConfigPath(filepath.Join(exeDir, "config"))
		viper.AddConfigPath(exeDir)
	}

	viper.AddConfigPath("./config")
	viper.AddConfigPath(".")
	viper.AddConfigPath("/etc/task-processor/")

	viper.SetEnvPrefix("TASK_PROCESSOR")
	viper.AutomaticEnv()

	// 设置默认值
	setDefaults(platform)

	if err := viper.ReadInConfig(); err != nil {
		logrus.Printf("警告: 无法读取配置文件: %v", err)
		logrus.Printf("使用默认配置和环境变量")
	} else {
		logrus.Printf("成功加载配置文件: %s", viper.ConfigFileUsed())
	}

	return &Config{
		Processor: ProcessorConfig{
			MaxRetries: viper.GetInt("processor.maxRetries"),
			Timeout:    viper.GetInt("processor.timeout"),
		},
		Worker: WorkerConfig{
			Concurrency:  viper.GetInt("worker.concurrency"),
			BufferSize:   viper.GetInt("worker.bufferSize"),
			TaskInterval: viper.GetInt("worker.taskInterval"),
		},
		Server: ServerConfig{
			Port: viper.GetInt("server.port"),
		},
		OpenAI: OpenAIConfig{
			APIKey:  viper.GetString("openai.apiKey"),
			Model:   viper.GetString("openai.model"),
			BaseURL: viper.GetString("openai.baseURL"),
			Timeout: viper.GetInt("openai.timeout"),
		},
		Management: ManagementConfig{
			BaseURL:      viper.GetString("management.baseURL"),
			ClientID:     viper.GetString("management.clientID"),
			ClientSecret: viper.GetString("management.clientSecret"),
			TokenURL:     viper.GetString("management.tokenURL"),
			Scopes:       viper.GetStringSlice("management.scopes"),
			UserID:       viper.GetInt64("management.userID"),
			StoreIDs:     getInt64Slice("management.storeIDs"),
		},
		AutoPricing: AutoPricingConfig{
			Enabled:   viper.GetBool("autoPricing.enabled"),
			Interval:  viper.GetInt("autoPricing.interval"),
			BatchSize: viper.GetInt("autoPricing.batchSize"),
		},
		Amazon: AmazonConfig{
			Enabled:        viper.GetBool("amazon.enabled"),
			Headless:       viper.GetBool("amazon.headless"),
			BrowserPath:    viper.GetString("amazon.browserPath"),
			PoolSize:       viper.GetInt("amazon.poolSize"),
			Zipcodes:       viper.GetStringMapString("amazon.zipcodes"),
			ViewportWidth:  viper.GetInt("amazon.viewportWidth"),
			ViewportHeight: viper.GetInt("amazon.viewportHeight"),
			ProxyServer:    viper.GetString("amazon.proxyServer"),
		},
		Updater: UpdaterConfig{
			Enabled:       viper.GetBool("updater.enabled"),
			ServerURL:     viper.GetString("updater.serverURL"),
			CheckInterval: viper.GetInt("updater.checkInterval"),
			AutoApply:     viper.GetBool("updater.autoApply"),
			SkipChecksum:  viper.GetBool("updater.skipChecksum"),
		},
		Platform: PlatformConfig{
			Name: platform,
			Type: viper.GetString("platform.type"),
		},
	}
}

func setDefaults(platform string) {
	viper.SetDefault("processor.maxRetries", 3)
	viper.SetDefault("processor.timeout", 300)

	viper.SetDefault("worker.concurrency", 10)
	viper.SetDefault("worker.bufferSize", 100)
	viper.SetDefault("worker.taskInterval", 60)

	// 根据平台设置不同的默认端口
	if platform == "temu" {
		viper.SetDefault("server.port", 8081)
	} else {
		viper.SetDefault("server.port", 8080)
	}

	viper.SetDefault("openai.apiKey", "sk-qns4hBrljHkJ520vwwA2508c7Dj3Oe0zGlh7oq7FWkcWXkx4")
	viper.SetDefault("openai.model", "gemini-2.0-flash")
	viper.SetDefault("openai.baseURL", "https://yunwu.ai/v1")
	viper.SetDefault("openai.timeout", 120)

	viper.SetDefault("management.baseURL", "http://getway.linkcloudai.com")
	viper.SetDefault("management.clientID", "go-listing")
	viper.SetDefault("management.clientSecret", "go-listing-secret")
	viper.SetDefault("management.tokenURL", "http://getway.linkcloudai.com/admin-api/system/oauth2/token")
	viper.SetDefault("management.scopes", []string{"user.read"})

	viper.SetDefault("autoPricing.enabled", true)
	viper.SetDefault("autoPricing.interval", 300)
	viper.SetDefault("autoPricing.batchSize", 100)

	viper.SetDefault("amazon.enabled", true)
	viper.SetDefault("amazon.headless", true)
	viper.SetDefault("amazon.browserPath", "./chrome/chrome.exe")
	viper.SetDefault("amazon.poolSize", 3)
	viper.SetDefault("amazon.viewportWidth", 1920)
	viper.SetDefault("amazon.viewportHeight", 1080)
	viper.SetDefault("amazon.proxyServer", "")

	viper.SetDefault("updater.enabled", true)
	viper.SetDefault("updater.serverURL", "https://files.linkcloudai.com/api/version")
	viper.SetDefault("updater.checkInterval", 24)
	viper.SetDefault("updater.autoApply", true)
	viper.SetDefault("updater.skipChecksum", false)

	viper.SetDefault("platform.type", "web")
}
