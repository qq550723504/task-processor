// Package content 提供SHEIN平台的敏感词服务核心功能
package content

import (
	"context"
	"task-processor/internal/pkg/recovery"
	"task-processor/internal/platforms/shein"

	"github.com/sirupsen/logrus"
)

// NewSensitiveWordService 创建敏感词服务
func NewSensitiveWordService() *SensitiveWordService {
	return NewSensitiveWordServiceWithPath("data/sensitive_words.json")
}

// NewSensitiveWordServiceWithPath 使用指定路径创建敏感词服务
func NewSensitiveWordServiceWithPath(configPath string) *SensitiveWordService {
	service := &SensitiveWordService{
		configPath: configPath,
		ctx:        context.Background(),
		saveQueue:  make(chan struct{}, 10), // 缓冲队列，最多10个待保存请求
		stopSave:   make(chan struct{}),
	}

	// 启动保存工作协程
	service.startSaveWorker()

	if err := service.loadConfig(); err != nil {
		logrus.Errorf("加载敏感词配置失败: %v，使用默认配置", err)
		service.initDefaultConfig()
	}

	return service
}

// startSaveWorker 启动保存工作协程
func (s *SensitiveWordService) startSaveWorker() {
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		defer recovery.Recover("保存工作协程", logrus.StandardLogger().WithField("component", "SensitiveWordService"))

		for {
			select {
			case <-s.saveQueue:
				if err := s.saveConfig(); err != nil {
					logrus.Errorf("保存敏感词配置失败: %v", err)
				} else {
					logrus.Debug("敏感词配置已保存")
				}
			case <-s.stopSave:
				// 处理剩余的保存请求
				for len(s.saveQueue) > 0 {
					<-s.saveQueue
					if err := s.saveConfig(); err != nil {
						logrus.Errorf("保存敏感词配置失败: %v", err)
					}
				}
				logrus.Info("保存工作协程已停止")
				return
			}
		}
	}()
}

// initDefaultConfig 初始化默认配置
func (s *SensitiveWordService) initDefaultConfig() {
	s.config = s.createDefaultConfig()
	logrus.Info("✅ 已初始化默认敏感词配置")
}

// ProcessProductData 处理产品数据中的敏感词
func (s *SensitiveWordService) ProcessProductData(ctx *shein.TaskContext) error {
	logrus.Info("🔍 开始敏感词处理（删除模式）...")

	if ctx == nil || ctx.ProductData == nil {
		logrus.Warn("⚠️ 上下文或产品数据为空，跳过敏感词处理")
		return nil
	}

	processedCount := 0

	// 处理产品名称 - 使用原始的processMultiLanguageNames方法
	if ctx.ProductData.MultiLanguageNameList != nil {
		processedCount += s.processMultiLanguageNames(ctx.ProductData.MultiLanguageNameList)
	}

	// 处理产品描述
	if ctx.ProductData.MultiLanguageDescList != nil {
		processedCount += s.processMultiLanguageDescs(ctx.ProductData.MultiLanguageDescList)
	}

	// 处理SKC数据
	if ctx.ProductData.SKCList != nil {
		processedCount += s.processSKCData(ctx, ctx.ProductData.SKCList)
	}

	s.logSensitiveWordStats()
	logrus.Infof("✅ 敏感词处理完成，共处理了 %d 个字段", processedCount)
	return nil
}

// HandleValidationErrors 处理验证错误中的敏感词
func (s *SensitiveWordService) HandleValidationErrors(ctx *shein.TaskContext, validationResults []shein.PreValidResult) bool {
	logrus.Info("🔍 开始处理验证错误中的敏感词（按语言分类模式）...")

	extractedWords := s.extractSensitiveWordsFromValidation(validationResults)
	if len(extractedWords) == 0 {
		logrus.Info("未发现敏感词错误，无需重试")
		return false
	}

	logrus.Infof("从验证错误中提取到敏感词: %v", extractedWords)
	s.AddDynamicSensitiveWords(extractedWords)

	if err := s.ProcessProductData(ctx); err != nil {
		logrus.Errorf("重新处理产品数据失败: %v", err)
		return false
	}

	logrus.Infof("✅ 敏感词处理完成，发现 %d 个新敏感词", len(extractedWords))
	return true
}

// getAllSensitiveWords 获取所有敏感词列表（静态 + 动态）
func (s *SensitiveWordService) getAllSensitiveWords() []string {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	var allWords []string

	// 添加静态敏感词
	for _, words := range s.config.StaticWords {
		allWords = append(allWords, words...)
	}

	// 添加动态敏感词
	for _, words := range s.config.DynamicWords {
		allWords = append(allWords, words...)
	}

	return s.deduplicateWords(allWords)
}

// GetStaticSensitiveWords 获取当前静态敏感词列表（所有语言）
func (s *SensitiveWordService) GetStaticSensitiveWords() []string {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	var words []string
	for _, langWords := range s.config.StaticWords {
		words = append(words, langWords...)
	}

	return s.deduplicateWords(words)
}

// GetDynamicSensitiveWords 获取当前动态敏感词列表（所有语言）
func (s *SensitiveWordService) GetDynamicSensitiveWords() []string {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	var words []string
	for _, langWords := range s.config.DynamicWords {
		words = append(words, langWords...)
	}

	return s.deduplicateWords(words)
}

// GetSensitiveWordsByLanguage 按语言获取敏感词列表
func (s *SensitiveWordService) GetSensitiveWordsByLanguage(language string) (static []string, dynamic []string) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	static = make([]string, len(s.config.StaticWords[language]))
	copy(static, s.config.StaticWords[language])

	dynamic = make([]string, len(s.config.DynamicWords[language]))
	copy(dynamic, s.config.DynamicWords[language])

	return static, dynamic
}

// AddDynamicSensitiveWords 添加动态敏感词（自动检测语言）
func (s *SensitiveWordService) AddDynamicSensitiveWords(words []string) {
	if len(words) == 0 {
		return
	}

	// 按语言分类
	wordsByLang := s.classifyWordsByLanguage(words)

	s.mutex.Lock()
	defer s.mutex.Unlock()

	// 添加到配置中
	totalAdded := s.addWordsToConfig(s.config.DynamicWords, wordsByLang, "动态")

	if totalAdded > 0 {
		logrus.Infof("✅ 成功添加 %d 个动态敏感词", totalAdded)
		s.saveConfigAsync()
	}
}

// AddDynamicSensitiveWordsByLanguage 按指定语言添加动态敏感词
func (s *SensitiveWordService) AddDynamicSensitiveWordsByLanguage(language string, words []string) {
	s.addWordsByLanguage(s.config.DynamicWords, language, words, "动态")
}

// AddStaticSensitiveWords 添加静态敏感词（自动检测语言）
func (s *SensitiveWordService) AddStaticSensitiveWords(words []string) {
	if len(words) == 0 {
		return
	}

	// 按语言分类
	wordsByLang := s.classifyWordsByLanguage(words)

	s.mutex.Lock()
	defer s.mutex.Unlock()

	// 添加到配置中
	totalAdded := s.addWordsToConfig(s.config.StaticWords, wordsByLang, "静态")

	if totalAdded > 0 {
		logrus.Infof("✅ 成功添加 %d 个静态敏感词", totalAdded)
		s.saveConfigAsync()
	}
}

// ClearDynamicSensitiveWords 清空动态敏感词列表
func (s *SensitiveWordService) ClearDynamicSensitiveWords() {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	oldCount := s.countWordsInConfig(s.config.DynamicWords)
	s.config.DynamicWords = make(map[string][]string)

	logrus.Infof("✅ 已清空 %d 个动态敏感词", oldCount)

	// 异步保存配置
	s.saveConfigAsync()
}
