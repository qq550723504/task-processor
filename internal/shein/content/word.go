// Package content 提供SHEIN平台的敏感词服务核心功能
package content

import (
	"context"
	corelogger "task-processor/internal/core/logger"
	"task-processor/internal/pkg/recovery"
	"task-processor/internal/shein"
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
		saveQueue:  make(chan struct{}, 10),
		stopSave:   make(chan struct{}),
		logger:     corelogger.GetGlobalLogger("shein.sensitive_word"),
	}

	service.startSaveWorker()

	if err := service.loadConfig(); err != nil {
		service.logger.Errorf("加载敏感词配置失败: %v，使用默认配置", err)
		service.initDefaultConfig()
	}

	return service
}

// startSaveWorker 启动保存工作协程
func (s *SensitiveWordService) startSaveWorker() {
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		defer recovery.Recover("保存工作协程", s.logger)

		for {
			select {
			case <-s.saveQueue:
				if err := s.saveConfig(); err != nil {
					s.logger.Errorf("保存敏感词配置失败: %v", err)
				} else {
					s.logger.Debug("敏感词配置已保存")
				}
			case <-s.stopSave:
				for len(s.saveQueue) > 0 {
					<-s.saveQueue
					if err := s.saveConfig(); err != nil {
						s.logger.Errorf("保存敏感词配置失败: %v", err)
					}
				}
				s.logger.Info("保存工作协程已停止")
				return
			}
		}
	}()
}

// initDefaultConfig 初始化默认配置
func (s *SensitiveWordService) initDefaultConfig() {
	s.config = s.createDefaultConfig()
	s.logger.Info("✅ 已初始化默认敏感词配置")
}

// ProcessProductData 处理产品数据中的敏感词
func (s *SensitiveWordService) ProcessProductData(ctx *shein.TaskContext) error {
	s.logger.Info("🔍 开始敏感词处理（删除模式）...")

	if ctx == nil || ctx.ProductData == nil {
		s.logger.Warn("⚠️ 上下文或产品数据为空，跳过敏感词处理")
		return nil
	}

	processedCount := 0

	if ctx.ProductData.MultiLanguageNameList != nil {
		processedCount += s.processMultiLanguageNames(ctx.ProductData.MultiLanguageNameList)
	}
	if ctx.ProductData.MultiLanguageDescList != nil {
		processedCount += s.processMultiLanguageDescs(ctx.ProductData.MultiLanguageDescList)
	}
	if ctx.ProductData.SKCList != nil {
		processedCount += s.processSKCData(ctx, ctx.ProductData.SKCList)
	}

	s.logSensitiveWordStats()
	s.logger.Infof("✅ 敏感词处理完成，共处理了 %d 个字段", processedCount)
	return nil
}

// HandleValidationErrors 处理验证错误中的敏感词
func (s *SensitiveWordService) HandleValidationErrors(ctx *shein.TaskContext, validationResults []shein.PreValidResult) bool {
	s.logger.Info("🔍 开始处理验证错误中的敏感词（按语言分类模式）...")

	extractedWords := s.extractSensitiveWordsFromValidation(validationResults)
	if len(extractedWords) == 0 {
		s.logger.Info("未发现敏感词错误，无需重试")
		return false
	}

	s.logger.Infof("从验证错误中提取到敏感词: %v", extractedWords)
	s.AddDynamicSensitiveWords(extractedWords)

	if err := s.ProcessProductData(ctx); err != nil {
		s.logger.Errorf("重新处理产品数据失败: %v", err)
		return false
	}

	s.logger.Infof("✅ 敏感词处理完成，发现 %d 个新敏感词", len(extractedWords))
	return true
}

// getAllSensitiveWords 获取所有敏感词列表（静态 + 动态）
func (s *SensitiveWordService) getAllSensitiveWords() []string {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	var allWords []string
	for _, words := range s.config.StaticWords {
		allWords = append(allWords, words...)
	}
	for _, words := range s.config.DynamicWords {
		allWords = append(allWords, words...)
	}
	return s.deduplicateWords(allWords)
}

// GetStaticSensitiveWords 获取当前静态敏感词列表（所有语言）
func (s *SensitiveWordService) GetStaticSensitiveWords() []string {
	return s.getWordsFromConfig(func() map[string][]string { return s.config.StaticWords })
}

// GetDynamicSensitiveWords 获取当前动态敏感词列表（所有语言）
func (s *SensitiveWordService) GetDynamicSensitiveWords() []string {
	return s.getWordsFromConfig(func() map[string][]string { return s.config.DynamicWords })
}

// getWordsFromConfig 通用：从配置中收集所有语言的词并去重
func (s *SensitiveWordService) getWordsFromConfig(getMap func() map[string][]string) []string {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	var words []string
	for _, langWords := range getMap() {
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
	s.addSensitiveWords(words, s.config.DynamicWords, "动态")
}

// AddStaticSensitiveWords 添加静态敏感词（自动检测语言）
func (s *SensitiveWordService) AddStaticSensitiveWords(words []string) {
	s.addSensitiveWords(words, s.config.StaticWords, "静态")
}

// addSensitiveWords 通用：按语言分类后添加到指定词表
func (s *SensitiveWordService) addSensitiveWords(words []string, target map[string][]string, kind string) {
	if len(words) == 0 {
		return
	}
	wordsByLang := s.classifyWordsByLanguage(words)
	s.mutex.Lock()
	defer s.mutex.Unlock()
	if totalAdded := s.addWordsToConfig(target, wordsByLang, kind); totalAdded > 0 {
		s.logger.Infof("✅ 成功添加 %d 个%s敏感词", totalAdded, kind)
		s.saveConfigAsync()
	}
}

// AddDynamicSensitiveWordsByLanguage 按指定语言添加动态敏感词
func (s *SensitiveWordService) AddDynamicSensitiveWordsByLanguage(language string, words []string) {
	s.addWordsByLanguage(s.config.DynamicWords, language, words, "动态")
}

// ClearDynamicSensitiveWords 清空动态敏感词列表
func (s *SensitiveWordService) ClearDynamicSensitiveWords() {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	oldCount := s.countWordsInConfig(s.config.DynamicWords)
	s.config.DynamicWords = make(map[string][]string)
	s.logger.Infof("✅ 已清空 %d 个动态敏感词", oldCount)
	s.saveConfigAsync()
}
