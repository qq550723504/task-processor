package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"

	"task-processor/common/amazon"
	"task-processor/common/config"
)

func main() {
	// 定义命令行参数
	url := flag.String("url", "", "Amazon产品页面URL")
	zipcode := flag.String("zipcode", "", "邮编")
	region := flag.String("region", "us", "地区代码 (us, jp, uk, de, fr, ca, it, es, in, mx, br, au)")
	output := flag.String("output", "output.json", "输出文件路径")
	configFile := flag.String("config", "", "配置文件路径（可选）")
	help := flag.Bool("help", false, "显示帮助信息")

	flag.Parse()

	// 显示帮助信息
	if *help {
		printHelp()
		return
	}

	// 地区到域名的映射
	domainMap := getDomainMap()

	// 地区到默认邮编的映射
	zipcodeMap := getZipcodeMap()

	// 检查必需参数
	if *url == "" {
		// 根据地区构建默认URL
		domain := domainMap[*region]
		if domain == "" {
			domain = "amazon.com" // 默认使用美国站
		}

		// 设置默认邮编
		if *zipcode == "" {
			*zipcode = zipcodeMap[*region]
			if *zipcode == "" {
				*zipcode = "94107" // 默认使用美国邮编
			}
		}

		// 默认产品URL
		languageParam := "en_US"
		*url = fmt.Sprintf("https://www.%s/dp/B0DF49ML4P?language=%s", domain, languageParam)
	} else {
		// 如果提供了URL但没有提供邮编，则使用默认邮编
		if *zipcode == "" {
			*zipcode = zipcodeMap[*region]
			if *zipcode == "" {
				*zipcode = "94107" // 默认使用美国邮编
			}
		}
	}

	// 加载配置
	cfg := loadConfig(*configFile)

	// 创建处理器
	processor := amazon.NewAmazonProcessor(&cfg.Amazon)
	defer processor.Shutdown()

	// 处理页面
	log.Printf("开始处理Amazon产品: %s", *url)
	product, err := processor.Process(*url, *zipcode)
	if err != nil {
		log.Fatalf("处理页面失败: %v", err)
	}

	// 将结果保存到文件
	if err := saveToFile(product, *output); err != nil {
		log.Fatalf("保存文件失败: %v", err)
	}

	log.Printf("成功保存结果到: %s", *output)
	log.Printf("产品标题: %s", product.Title)
	log.Printf("产品价格: %.2f %s", product.FinalPrice, product.Currency)
}

// printHelp 打印帮助信息
func printHelp() {
	fmt.Println("Amazon爬虫工具 (Task Processor版本)")
	fmt.Println()
	fmt.Println("用法:")
	fmt.Println("  amazon-crawler -url=<Amazon产品页面URL> [-zipcode=<邮编>] [-region=<地区>] [-output=<输出文件路径>] [-config=<配置文件路径>]")
	fmt.Println()
	fmt.Println("参数:")
	flag.PrintDefaults()
	fmt.Println()
	fmt.Println("示例:")
	fmt.Println("  amazon-crawler -url=https://www.amazon.com/dp/B0F4X44ZRV -zipcode=94107")
	fmt.Println("  amazon-crawler -region=jp -zipcode=100-0001")
	fmt.Println("  amazon-crawler -url=https://www.amazon.co.jp/dp/B0F4X44ZRV -config=config/config-dev.yaml")
}

// loadConfig 加载配置
func loadConfig(configFile string) *config.Config {
	if configFile != "" {
		// 使用指定的配置文件
		os.Setenv("CONFIG_FILE", configFile)
	}

	cfg := config.LoadConfig()
	if cfg == nil {
		log.Println("警告: 配置加载失败，使用默认配置")
		cfg = getDefaultConfig()
	}

	return cfg
}

// getDefaultConfig 获取默认配置
func getDefaultConfig() *config.Config {
	return &config.Config{
		Amazon: config.AmazonConfig{
			Enabled:        true,
			Headless:       true,
			BrowserPath:    "",
			PoolSize:       1,
			ViewportWidth:  1920,
			ViewportHeight: 1080,
			ProxyServer:    "",
		},
	}
}

// saveToFile 将产品信息保存到文件
func saveToFile(product *amazon.Product, filename string) error {
	// 创建文件
	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("创建文件失败: %w", err)
	}
	defer file.Close()

	// 创建产品数组（与示例文件格式匹配）
	products := []amazon.Product{*product}

	// 序列化为JSON
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(products); err != nil {
		return fmt.Errorf("序列化JSON失败: %w", err)
	}

	return nil
}

// getDomainMap 获取地区到域名的映射
func getDomainMap() map[string]string {
	return map[string]string{
		"us": "amazon.com",
		"uk": "amazon.co.uk",
		"de": "amazon.de",
		"fr": "amazon.fr",
		"jp": "amazon.co.jp",
		"ca": "amazon.ca",
		"it": "amazon.it",
		"es": "amazon.es",
		"in": "amazon.in",
		"mx": "amazon.com.mx",
		"br": "amazon.com.br",
		"au": "amazon.com.au",
	}
}

// getZipcodeMap 获取地区到默认邮编的映射
func getZipcodeMap() map[string]string {
	return map[string]string{
		"us": "94107",     // 旧金山
		"uk": "SW1A 1AA",  // 伦敦
		"de": "10115",     // 柏林
		"fr": "75001",     // 巴黎
		"jp": "153-0064",  // 东京
		"ca": "M5H 2N2",   // 多伦多
		"it": "00118",     // 罗马
		"es": "28001",     // 马德里
		"in": "110001",    // 新德里
		"mx": "11000",     // 墨西哥城
		"br": "01310-100", // 圣保罗
		"au": "2000",      // 悉尼
	}
}
