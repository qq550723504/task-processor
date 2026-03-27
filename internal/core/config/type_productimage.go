package config

// ProductImageConfig 定义 productimage 流水线的运行配置。
type ProductImageConfig struct {
	WorkDir         string                      `yaml:"workDir"`
	Segmenter       ProductImageModelConfig     `yaml:"segmenter"`
	WhiteBackground ProductImageModelConfig     `yaml:"whiteBackground"`
	Publisher       ProductImagePublisherConfig `yaml:"publisher"`
	Lifecycle       ProductImageLifecycleConfig `yaml:"lifecycle"`
}

// ProductImageModelConfig 定义外部图像模型服务配置。
type ProductImageModelConfig struct {
	Enabled  bool   `yaml:"enabled"`
	Endpoint string `yaml:"endpoint"`
	APIKey   string `yaml:"apiKey"`
	Timeout  int    `yaml:"timeout"`
}

// ProductImagePublisherConfig 定义图片产物发布配置。
type ProductImagePublisherConfig struct {
	Enabled    bool   `yaml:"enabled"`
	Provider   string `yaml:"provider"`
	OutputDir  string `yaml:"outputDir"`
	PublicBase string `yaml:"publicBase"`
}

// ProductImageLifecycleConfig 定义图片产物生命周期策略。
type ProductImageLifecycleConfig struct {
	CleanupTemporaryFiles bool `yaml:"cleanupTemporaryFiles"`
	ReuseExistingAssets   bool `yaml:"reuseExistingAssets"`
}
