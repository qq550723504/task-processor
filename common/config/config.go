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
		case []any:
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
	OpenAI      OpenAIConfig
	Management  ManagementConfig
	AutoPricing AutoPricingConfig
	Amazon      AmazonConfig
	Updater     UpdaterConfig
	Sync        *SyncConfig    // 产品同步配置
	Monitor     *MonitorConfig // 产品监控配置
}

// ProcessorConfig 处理器配置
type ProcessorConfig struct {
	MaxRetries int
	Timeout    int // 单位：秒
}

// WorkerConfig 工作池配置
type WorkerConfig struct {
	Concurrency      int // 并发工作协程数
	BufferSize       int // 队列缓冲区大小
	TaskInterval     int // 任务获取间隔（秒）
	MaxFetchPerCycle int // 单次最多获取任务数
	QueueThreshold   int // 队列使用率阈值（%）
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
	TenantID     string // 租户ID
	UserID       int64
	StoreIDs     []int64
}

// AutoPricingConfig 自动核价配置
type AutoPricingConfig struct {
	Temu  PlatformAutoPricingConfig
	Shein PlatformAutoPricingConfig
}

// PlatformAutoPricingConfig 平台自动核价配置
type PlatformAutoPricingConfig struct {
	Enabled   bool
	Interval  int
	BatchSize int
}

// AmazonConfig Amazon爬虫配置
type AmazonConfig struct {
	Enabled           bool
	Headless          bool
	BrowserPath       string
	PoolSize          int
	Zipcodes          map[string]string
	ViewportWidth     int
	ViewportHeight    int
	ProxyServer       string
	DataFreshnessDays int // 数据新鲜度天数，默认7天
}

// UpdaterConfig 自动更新配置
type UpdaterConfig struct {
	Enabled            bool   `yaml:"enabled"`              // 是否启用自动更新
	UpdateURL          string `yaml:"update_url"`           // 版本检查地址
	CheckInterval      int    `yaml:"check_interval"`       // 检查间隔（秒）
	InsecureSkipVerify bool   `yaml:"insecure_skip_verify"` // 跳过TLS证书验证
	CurrentVersion     string `yaml:"-"`                    // 当前版本（从编译时注入）
}

// SyncConfig 产品同步配置
type SyncConfig struct {
	Enabled  bool    `yaml:"enabled"`   // 是否启用产品同步
	StoreIDs []int64 `yaml:"store_ids"` // 需要同步的店铺ID列表
}

// MonitorConfig 产品监控配置
type MonitorConfig struct {
	Enabled              bool    `yaml:"enabled"`                // 是否启用产品监控
	StoreIDs             []int64 `yaml:"store_ids"`              // 需要监控的店铺ID列表
	CheckInterval        int     `yaml:"check_interval"`         // 检查间隔（分钟）
	BatchSize            int     `yaml:"batch_size"`             // 批量处理大小
	EnablePriceAlert     bool    `yaml:"enable_price_alert"`     // 启用价格告警
	EnableStockAlert     bool    `yaml:"enable_stock_alert"`     // 启用库存告警
	PriceChangeThreshold float64 `yaml:"price_change_threshold"` // 价格变化阈值（百分比）
	StockChangeThreshold int     `yaml:"stock_change_threshold"` // 库存变化阈值
}

// PlatformConfig 平台特定配置
type PlatformConfig struct {
	Name string // "temu" 或 "shein"
	Type string // "web" 或 "cli"
}

// LoadConfig 加载配置
func LoadConfig() *Config {
	env := os.Getenv("TASK_PROCESSOR_ENV")
	if env == "" {
		env = "dev"
	}

	// 加载统一配置文件 config-dev.yaml
	configName := fmt.Sprintf("config-%s", env)
	logrus.Infof("加载配置文件: %s.yaml", configName)

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
	setDefaults()

	// 读取配置文件
	if err := viper.ReadInConfig(); err != nil {
		logrus.Warnf("无法读取配置文件: %v", err)
		logrus.Info("使用默认配置和环境变量")
	} else {
		logrus.Infof("成功加载配置文件: %s", viper.ConfigFileUsed())
	}

	cfg := &Config{
		Processor: ProcessorConfig{
			MaxRetries: viper.GetInt("processor.maxRetries"),
			Timeout:    viper.GetInt("processor.timeout"),
		},
		Worker: WorkerConfig{
			Concurrency:      viper.GetInt("worker.concurrency"),
			BufferSize:       viper.GetInt("worker.bufferSize"),
			TaskInterval:     viper.GetInt("worker.taskInterval"),
			MaxFetchPerCycle: viper.GetInt("worker.maxFetchPerCycle"),
			QueueThreshold:   viper.GetInt("worker.queueThreshold"),
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
			TenantID:     viper.GetString("management.tenantID"),
			UserID:       viper.GetInt64("management.userID"),
			StoreIDs:     getInt64Slice("management.storeIDs"),
		},
		AutoPricing: AutoPricingConfig{
			Temu: PlatformAutoPricingConfig{
				Enabled:   viper.GetBool("autoPricing.temu.enabled"),
				Interval:  viper.GetInt("autoPricing.temu.interval"),
				BatchSize: viper.GetInt("autoPricing.temu.batchSize"),
			},
			Shein: PlatformAutoPricingConfig{
				Enabled:   viper.GetBool("autoPricing.shein.enabled"),
				Interval:  viper.GetInt("autoPricing.shein.interval"),
				BatchSize: viper.GetInt("autoPricing.shein.batchSize"),
			},
		},
		Amazon: AmazonConfig{
			Enabled:           viper.GetBool("amazon.enabled"),
			Headless:          viper.GetBool("amazon.headless"),
			BrowserPath:       viper.GetString("amazon.browserPath"),
			PoolSize:          viper.GetInt("amazon.poolSize"),
			Zipcodes:          viper.GetStringMapString("amazon.zipcodes"),
			ViewportWidth:     viper.GetInt("amazon.viewportWidth"),
			ViewportHeight:    viper.GetInt("amazon.viewportHeight"),
			ProxyServer:       viper.GetString("amazon.proxyServer"),
			DataFreshnessDays: viper.GetInt("amazon.dataFreshnessDays"),
		},
		Updater: UpdaterConfig{
			Enabled:            viper.GetBool("updater.enabled"),
			UpdateURL:          viper.GetString("updater.updateURL"),
			CheckInterval:      viper.GetInt("updater.checkInterval"),
			InsecureSkipVerify: viper.GetBool("updater.insecureSkipVerify"),
		},
	}

	// 加载同步配置（如果存在）
	if viper.IsSet("sync") {
		cfg.Sync = &SyncConfig{
			Enabled:  viper.GetBool("sync.enabled"),
			StoreIDs: getInt64Slice("sync.storeIDs"),
		}
	}

	// 加载监控配置（如果存在）
	if viper.IsSet("monitor") {
		cfg.Monitor = &MonitorConfig{
			Enabled:              viper.GetBool("monitor.enabled"),
			StoreIDs:             getInt64Slice("monitor.storeIDs"),
			CheckInterval:        viper.GetInt("monitor.checkInterval"),
			BatchSize:            viper.GetInt("monitor.batchSize"),
			EnablePriceAlert:     viper.GetBool("monitor.enablePriceAlert"),
			EnableStockAlert:     viper.GetBool("monitor.enableStockAlert"),
			PriceChangeThreshold: viper.GetFloat64("monitor.priceChangeThreshold"),
			StockChangeThreshold: viper.GetInt("monitor.stockChangeThreshold"),
		}
	}

	return cfg
}

func setDefaults() {
	viper.SetDefault("processor.maxRetries", 3)
	viper.SetDefault("processor.timeout", 300)

	viper.SetDefault("worker.concurrency", 10)
	viper.SetDefault("worker.bufferSize", 100)
	viper.SetDefault("worker.taskInterval", 60)
	viper.SetDefault("worker.maxFetchPerCycle", 5)
	viper.SetDefault("worker.queueThreshold", 75)

	viper.SetDefault("openai.apiKey", "sk-qns4hBrljHkJ520vwwA2508c7Dj3Oe0zGlh7oq7FWkcWXkx4")
	viper.SetDefault("openai.model", "gemini-2.0-flash")
	viper.SetDefault("openai.baseURL", "https://yunwu.ai/v1")
	viper.SetDefault("openai.timeout", 120)

	viper.SetDefault("management.baseURL", "http://getway.linkcloudai.com")
	viper.SetDefault("management.clientID", "go-listing")
	viper.SetDefault("management.clientSecret", "go-listing-secret")
	viper.SetDefault("management.tokenURL", "http://getway.linkcloudai.com/admin-api/system/oauth2/token")
	viper.SetDefault("management.scopes", []string{"user.read"})
	viper.SetDefault("management.tenantID", "1") // 默认租户ID为1

	// TEMU 自动核价默认配置
	viper.SetDefault("autoPricing.temu.enabled", true)
	viper.SetDefault("autoPricing.temu.interval", 300)
	viper.SetDefault("autoPricing.temu.batchSize", 100)

	// SHEIN 自动核价默认配置
	viper.SetDefault("autoPricing.shein.enabled", false)
	viper.SetDefault("autoPricing.shein.interval", 300)
	viper.SetDefault("autoPricing.shein.batchSize", 100)

	viper.SetDefault("amazon.enabled", true)
	viper.SetDefault("amazon.headless", true)
	viper.SetDefault("amazon.browserPath", "./chrome/chrome.exe")
	viper.SetDefault("amazon.poolSize", 3)
	viper.SetDefault("amazon.viewportWidth", 1920)
	viper.SetDefault("amazon.viewportHeight", 1080)
	viper.SetDefault("amazon.proxyServer", "")
	viper.SetDefault("amazon.dataFreshnessDays", 7) // 默认7天

	viper.SetDefault("updater.enabled", true) // 默认禁用，生产环境手动启用
	viper.SetDefault("updater.updateURL", "https://auto-update-1303159911.cos.ap-shanghai.myqcloud.com/task-processor/version.json")
	viper.SetDefault("updater.checkInterval", 300) // 5分钟
	viper.SetDefault("updater.insecureSkipVerify", false)

	// 产品同步默认配置
	viper.SetDefault("sync.enabled", false)
	viper.SetDefault("sync.storeIDs", []int64{})

	// 产品监控默认配置
	viper.SetDefault("monitor.enabled", false)
	viper.SetDefault("monitor.storeIDs", []int64{})
	viper.SetDefault("monitor.checkInterval", 1440) // 默认24小时（1440分钟）
	viper.SetDefault("monitor.batchSize", 50)
	viper.SetDefault("monitor.enablePriceAlert", true)
	viper.SetDefault("monitor.enableStockAlert", true)
	viper.SetDefault("monitor.priceChangeThreshold", 5.0)
	viper.SetDefault("monitor.stockChangeThreshold", 5)
}
