package modules

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"task-processor/internal/common/shein/api/product"
	"time"
	"unicode"

	"github.com/sirupsen/logrus"
)

// ===== 配置结构 =====

// SensitiveWordConfig 敏感词配置结构（按语言分类）
type SensitiveWordConfig struct {
	StaticWords  map[string][]string `json:"static_words"`  // 按语言分类的静态敏感词
	DynamicWords map[string][]string `json:"dynamic_words"` // 按语言分类的动态敏感词
	LastUpdated  time.Time           `json:"last_updated"`
	Version      string              `json:"version"`
}

// ===== 服务结构 =====

// SensitiveWordService 基于JSON文件的敏感词处理服务
type SensitiveWordService struct {
	configPath string
	config     *SensitiveWordConfig
	mutex      sync.RWMutex
}

// ===== 构造函数 =====

// NewSensitiveWordService 创建敏感词服务
func NewSensitiveWordService() *SensitiveWordService {
	return NewSensitiveWordServiceWithPath("config/sensitive_words.json")
}

// NewSensitiveWordServiceWithPath 使用指定路径创建敏感词服务
func NewSensitiveWordServiceWithPath(configPath string) *SensitiveWordService {
	service := &SensitiveWordService{
		configPath: configPath,
	}

	if err := service.loadConfig(); err != nil {
		logrus.Errorf("加载敏感词配置失败: %v，使用默认配置", err)
		service.initDefaultConfig()
	}

	return service
}

// initDefaultConfig 初始化默认配置
func (s *SensitiveWordService) initDefaultConfig() {
	s.config = s.createDefaultConfig()
	logrus.Info("✅ 已初始化默认敏感词配置")
}

// ===== 核心处理方法 =====

// ProcessProductData 处理产品数据中的敏感词
func (s *SensitiveWordService) ProcessProductData(ctx *TaskContext) error {
	logrus.Info("🔍 开始敏感词处理（删除模式）...")

	if ctx.ProductData == nil {
		return fmt.Errorf("产品数据为空")
	}

	processedCount := 0
	processedCount += s.processMultiLanguageNames(ctx.ProductData.MultiLanguageNameList)
	processedCount += s.processMultiLanguageDescs(ctx.ProductData.MultiLanguageDescList)
	processedCount += s.processSKCData(ctx, ctx.ProductData.SKCList)

	s.logSensitiveWordStats()
	logrus.Infof("✅ 敏感词处理完成，共处理了 %d 个字段", processedCount)
	return nil
}

// HandleValidationErrors 处理验证错误中的敏感词
func (s *SensitiveWordService) HandleValidationErrors(ctx *TaskContext, validationResults []PreValidResult) bool {
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

// removeSensitiveWords 移除文本中的敏感词
func (s *SensitiveWordService) removeSensitiveWords(text string) string {
	if text == "" {
		return text
	}

	cleanedText := s.preprocessText(text)
	sensitiveWords := s.getAllSensitiveWords()

	for _, word := range sensitiveWords {
		cleanedText = s.removeWordFromText(cleanedText, word)
	}

	return s.cleanupText(cleanedText)
}

// removeSensitiveWordsAndBrandsWithContext 移除文本中的敏感词、Amazon品牌词和上下文中的品牌词
func (s *SensitiveWordService) removeSensitiveWordsAndBrandsWithContext(ctx *TaskContext, text string) string {
	if text == "" {
		return text
	}

	// 先移除敏感词
	cleanedText := s.removeSensitiveWords(text)

	// 再移除Amazon品牌词
	cleanedText = s.removeAmazonBrandWords(cleanedText)

	// 移除上下文中的品牌词
	cleanedText = s.removeContextBrandWords(ctx, cleanedText)

	// 为SHEIN平台进行最终清理（移除表情符号和特殊字符）
	cleanedText = s.cleanTextForSheinPlatform(cleanedText)

	return s.cleanupText(cleanedText)
}

// ===== 产品数据处理辅助方法 =====

// processMultiLanguageNames 处理多语言名称
func (s *SensitiveWordService) processMultiLanguageNames(nameList []product.LanguageContent) int {
	if nameList == nil {
		return 0
	}

	processedCount := 0
	for i, name := range nameList {
		if cleaned := s.removeSensitiveWords(name.Name); cleaned != name.Name {
			nameList[i].Name = cleaned
			processedCount++
		}
	}
	return processedCount
}

// processMultiLanguageNamesWithBrandsAndContext 处理多语言名称（包含Amazon品牌词和上下文品牌词移除）
func (s *SensitiveWordService) processMultiLanguageNamesWithBrandsAndContext(ctx *TaskContext, nameList []product.LanguageContent) int {
	if nameList == nil {
		return 0
	}

	processedCount := 0
	for i, name := range nameList {
		if cleaned := s.removeSensitiveWordsAndBrandsWithContext(ctx, name.Name); cleaned != name.Name {
			nameList[i].Name = cleaned
			processedCount++
		}
	}
	return processedCount
}

// processMultiLanguageDescs 处理多语言描述
func (s *SensitiveWordService) processMultiLanguageDescs(descList []product.LanguageContent) int {
	if descList == nil {
		return 0
	}

	processedCount := 0
	for i, desc := range descList {
		if cleaned := s.removeSensitiveWords(desc.Name); cleaned != desc.Name {
			descList[i].Name = cleaned
			processedCount++
		}
	}
	return processedCount
}

// processSKCData 处理SKC数据
func (s *SensitiveWordService) processSKCData(ctx *TaskContext, skcList []product.SKC) int {
	if skcList == nil {
		return 0
	}

	processedCount := 0
	for i, skc := range skcList {
		// 处理SKC多语言名称（包含Amazon品牌词移除）
		if cleaned := s.removeSensitiveWordsAndBrandsWithContext(ctx, skc.MultiLanguageName.Name); cleaned != skc.MultiLanguageName.Name {
			skcList[i].MultiLanguageName.Name = cleaned
			processedCount++
		}

		// 处理SKC多语言名称列表（包含Amazon品牌词移除）
		if skc.MultiLanguageNameList != nil {
			processedCount += s.processMultiLanguageNamesWithBrandsAndContext(ctx, skc.MultiLanguageNameList)
		}
	}
	return processedCount
}

// ===== 配置管理方法 =====

// loadConfig 加载敏感词配置文件
func (s *SensitiveWordService) loadConfig() error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// 检查文件是否存在
	if _, err := os.Stat(s.configPath); os.IsNotExist(err) {
		logrus.Warnf("敏感词配置文件不存在: %s，将自动创建默认配置", s.configPath)

		// 创建默认配置
		s.config = s.createDefaultConfig()

		// 保存到文件（不需要锁，因为已经在锁内）
		if err := s.saveConfigUnlocked(); err != nil {
			logrus.Errorf("创建默认敏感词配置文件失败: %v", err)
			// 即使保存失败，也使用默认配置
		} else {
			logrus.Infof("✅ 已自动创建默认敏感词配置文件: %s", s.configPath)
		}

		s.logConfigLoadStats()
		return nil
	}

	data, err := os.ReadFile(s.configPath)
	if err != nil {
		return fmt.Errorf("读取敏感词配置文件失败: %v", err)
	}

	var config SensitiveWordConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("解析敏感词配置文件失败: %v", err)
	}

	s.config = &config
	s.logConfigLoadStats()
	return nil
}

// saveConfig 保存敏感词配置到文件
func (s *SensitiveWordService) saveConfig() error {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	return s.saveConfigUnlocked()
}

// saveConfigUnlocked 保存敏感词配置到文件（不加锁版本，内部使用）
func (s *SensitiveWordService) saveConfigUnlocked() error {
	if s.config == nil {
		return fmt.Errorf("配置为空，无法保存")
	}

	s.config.LastUpdated = time.Now()

	if err := os.MkdirAll(filepath.Dir(s.configPath), 0755); err != nil {
		return fmt.Errorf("创建配置目录失败: %v", err)
	}

	data, err := json.MarshalIndent(s.config, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化配置失败: %v", err)
	}

	if err := os.WriteFile(s.configPath, data, 0644); err != nil {
		return fmt.Errorf("写入配置文件失败: %v", err)
	}

	logrus.Infof("💾 敏感词配置已保存到: %s", s.configPath)
	return nil
}

// createDefaultConfig 创建默认配置
func (s *SensitiveWordService) createDefaultConfig() *SensitiveWordConfig {
	return &SensitiveWordConfig{
		StaticWords: map[string][]string{
			"en": {
				// 品牌相关
				"brand", "trademark", "logo", "copyright",
				// 质量声明
				"best", "top", "number one", "#1", "world's best",
				// 医疗声明
				"cure", "treat", "medical", "therapeutic",
				// 绝对化用语
				"guarantee", "guaranteed", "100%", "perfect",
			},
			"zh": {
				// 中文敏感词（如果需要）
				"品牌", "商标", "版权",
			},
		},
		DynamicWords: map[string][]string{
			"en": {
				// 品牌名称模式
				`(?i)\b(nike|adidas|apple|samsung|sony)\b`,
				// 绝对化表述
				`(?i)\b(best|top|#1|number\s*one)\b`,
			},
		},
		LastUpdated: time.Now(),
		Version:     "1.0.0",
	}
}

// ReloadConfig 重新加载配置文件
func (s *SensitiveWordService) ReloadConfig() error {
	logrus.Info("🔄 重新加载敏感词配置...")
	return s.loadConfig()
}

// ===== 敏感词获取和管理方法 =====

// getAllSensitiveWords 获取所有敏感词列表（静态 + 动态）
func (s *SensitiveWordService) getAllSensitiveWords() []string {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	if s.config == nil {
		return []string{}
	}

	var allWords []string

	// 合并所有语言的静态和动态敏感词
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
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	if s.config == nil {
		return []string{}
	}

	var allWords []string
	for _, words := range s.config.StaticWords {
		allWords = append(allWords, words...)
	}
	return allWords
}

// GetDynamicSensitiveWords 获取当前动态敏感词列表（所有语言）
func (s *SensitiveWordService) GetDynamicSensitiveWords() []string {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	if s.config == nil {
		return []string{}
	}

	var allWords []string
	for _, words := range s.config.DynamicWords {
		allWords = append(allWords, words...)
	}
	return allWords
}

// GetSensitiveWordsByLanguage 按语言获取敏感词列表
func (s *SensitiveWordService) GetSensitiveWordsByLanguage(language string) (static []string, dynamic []string) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	if s.config == nil {
		return []string{}, []string{}
	}

	static = append([]string(nil), s.config.StaticWords[language]...)
	dynamic = append([]string(nil), s.config.DynamicWords[language]...)
	return static, dynamic
}

// ===== 敏感词添加和删除方法 =====

// AddDynamicSensitiveWords 添加动态敏感词（自动检测语言）
func (s *SensitiveWordService) AddDynamicSensitiveWords(words []string) {
	if len(words) == 0 {
		return
	}

	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.config == nil {
		logrus.Error("配置未初始化，无法添加动态敏感词")
		return
	}

	wordsByLang := s.classifyWordsByLanguage(words)
	totalAdded := s.addWordsToConfig(s.config.DynamicWords, wordsByLang, "动态")

	logrus.Infof("📝 总计添加动态敏感词: %d 个", totalAdded)
	s.saveConfigAsync()
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

	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.config == nil {
		logrus.Error("配置未初始化，无法添加静态敏感词")
		return
	}

	wordsByLang := s.classifyWordsByLanguage(words)
	totalAdded := s.addWordsToConfig(s.config.StaticWords, wordsByLang, "静态")

	logrus.Infof("📝 总计添加静态敏感词: %d 个", totalAdded)
	s.saveConfigAsync()
}

// ClearDynamicSensitiveWords 清空动态敏感词列表
func (s *SensitiveWordService) ClearDynamicSensitiveWords() {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.config == nil {
		logrus.Error("配置未初始化，无法清空动态敏感词")
		return
	}

	totalCleared := s.countWordsInConfig(s.config.DynamicWords)
	if totalCleared > 0 {
		logrus.Infof("🧹 清空动态敏感词列表 (共%d个)", totalCleared)
		s.config.DynamicWords = make(map[string][]string)
		s.saveConfigAsync()
	}
}

// ===== 语言检测和分类方法 =====

// classifyWordsByLanguage 按语言分类敏感词
func (s *SensitiveWordService) classifyWordsByLanguage(words []string) map[string][]string {
	result := make(map[string][]string)

	for _, word := range words {
		lang := s.detectLanguage(word)
		if result[lang] == nil {
			result[lang] = make([]string, 0)
		}
		result[lang] = append(result[lang], word)
	}

	return result
}

// detectLanguage 检测单词的语言
func (s *SensitiveWordService) detectLanguage(word string) string {
	word = strings.TrimSpace(strings.ToLower(word))

	// 检测特殊字符集
	if s.containsJapanese(word) {
		return "ja"
	}
	if s.containsChinese(word) {
		return "zh"
	}
	if s.containsCyrillic(word) {
		return "ru"
	}

	// 检测特定语言的特征词汇
	languagePatterns := map[string][]string{
		"de": {"lich", "ung", "keit", "isch", "sch", "tz", "ß"},
		"fr": {"ique", "tion", "eur", "eux", "ège", "ç", "é", "è", "à"},
		"es": {"ción", "dad", "oso", "ico", "ñ", "ó", "í", "á"},
		"it": {"zione", "oso", "ico", "ità", "ò", "ù", "à"},
		"pt": {"ção", "dade", "oso", "ico", "ã", "õ", "ç"},
		"nl": {"lijk", "heid", "isch", "ij", "oe", "aa"},
		"sv": {"lig", "het", "isk", "ä", "ö", "å"},
		"no": {"lig", "het", "isk", "ø", "å", "æ"},
		"da": {"lig", "hed", "isk", "ø", "å", "æ"},
		"fi": {"nen", "lla", "ssa", "ään", "ää", "öö", "yy"},
		"pl": {"owy", "icz", "ość", "ą", "ę", "ł", "ń", "ś", "ź", "ż"},
	}

	for lang, patterns := range languagePatterns {
		for _, pattern := range patterns {
			if strings.Contains(word, pattern) {
				return lang
			}
		}
	}

	return "en" // 默认为英文
}

// containsJapanese 检测是否包含日文字符
func (s *SensitiveWordService) containsJapanese(text string) bool {
	japanesePattern := regexp.MustCompile("[\u3040-\u309F\u30A0-\u30FF\u4E00-\u9FAF\u3000-\u303F]")
	return japanesePattern.MatchString(text)
}

// containsChinese 检测是否包含中文字符
func (s *SensitiveWordService) containsChinese(text string) bool {
	for _, r := range text {
		if unicode.Is(unicode.Scripts["Han"], r) {
			return true
		}
	}
	return false
}

// containsCyrillic 检测是否包含西里尔字符（俄文等）
func (s *SensitiveWordService) containsCyrillic(text string) bool {
	for _, r := range text {
		if unicode.Is(unicode.Scripts["Cyrillic"], r) {
			return true
		}
	}
	return false
}

// ===== 文本处理辅助方法 =====

// preprocessText 预处理文本
func (s *SensitiveWordService) preprocessText(text string) string {
	text = s.filterEmojis(text)
	text = s.normalizeSpecialCharacters(text)
	return strings.TrimSpace(regexp.MustCompile(`\s+`).ReplaceAllString(text, " "))
}

// removeWordFromText 从文本中移除指定单词
func (s *SensitiveWordService) removeWordFromText(text, word string) string {
	if s.containsJapanese(word) {
		return strings.ReplaceAll(text, word, "")
	}

	re := regexp.MustCompile(`(?i)\b` + regexp.QuoteMeta(word) + `\b`)
	return re.ReplaceAllString(text, "")
}

// cleanupText 清理文本中的多余空格
func (s *SensitiveWordService) cleanupText(text string) string {
	text = regexp.MustCompile(`\s+`).ReplaceAllString(text, " ")
	return strings.TrimSpace(text)
}

// filterEmojis 过滤表情符号
func (s *SensitiveWordService) filterEmojis(text string) string {
	// 更全面的表情符号正则表达式，覆盖所有Unicode表情符号范围
	emojiRegex := regexp.MustCompile(`[\x{1F600}-\x{1F64F}]|` + // 表情符号 (Emoticons)
		`[\x{1F300}-\x{1F5FF}]|` + // 杂项符号和象形文字 (Miscellaneous Symbols and Pictographs)
		`[\x{1F680}-\x{1F6FF}]|` + // 交通和地图符号 (Transport and Map Symbols)
		`[\x{1F1E0}-\x{1F1FF}]|` + // 区域指示符号 (Regional Indicator Symbols)
		`[\x{2600}-\x{26FF}]|` + // 杂项符号 (Miscellaneous Symbols)
		`[\x{2700}-\x{27BF}]|` + // 装饰符号 (Dingbats)
		`[\x{1F900}-\x{1F9FF}]|` + // 补充符号和象形文字 (Supplemental Symbols and Pictographs)
		`[\x{1F018}-\x{1F270}]|` + // 封闭式字母数字补充 (Enclosed Alphanumeric Supplement)
		`[\x{2300}-\x{25FF}]|` + // 杂项技术符号 (Miscellaneous Technical)
		`[\x{2000}-\x{206F}]|` + // 一般标点符号 (General Punctuation)
		`[\x{FE00}-\x{FE0F}]|` + // 变体选择器 (Variation Selectors)
		`[\x{1F004}]|` + // 麻将牌
		`[\x{1F0CF}]|` + // 扑克牌
		`[\x{1F170}-\x{1F251}]|` + // 封闭式字母数字
		`[\x{1F700}-\x{1F77F}]|` + // 炼金术符号
		`[\x{1F780}-\x{1F7FF}]|` + // 几何形状扩展A
		`[\x{1F800}-\x{1F8FF}]|` + // 补充箭头C
		`[\x{1FA00}-\x{1FA6F}]|` + // 象棋符号
		`[\x{1FA70}-\x{1FAFF}]|` + // 符号和象形文字扩展A
		`[\x{2B50}]|` + // 星星
		`[\x{2B55}]|` + // 圆圈
		`[\x{231A}-\x{231B}]|` + // 手表
		`[\x{23E9}-\x{23EC}]|` + // 播放按钮
		`[\x{23F0}]|` + // 闹钟
		`[\x{23F3}]|` + // 沙漏
		`[\x{25AA}-\x{25AB}]|` + // 方块
		`[\x{25B6}]|` + // 播放按钮
		`[\x{25C0}]|` + // 倒退按钮
		`[\x{25FB}-\x{25FE}]|` + // 方块
		`[\x{2934}-\x{2935}]|` + // 箭头
		`[\x{2B05}-\x{2B07}]|` + // 箭头
		`[\x{2B1B}-\x{2B1C}]|` + // 方块
		`[\x{3030}]|` + // 波浪线
		`[\x{303D}]|` + // 部分交替标记
		`[\x{3297}]|` + // 表意文字优势符号
		`[\x{3299}]`) // 表意文字秘密符号

	// 第一次过滤
	cleanedText := emojiRegex.ReplaceAllString(text, "")

	// 第二次过滤：使用更激进的方法移除所有可能的表情符号
	cleanedText = s.removeAllEmojisAggressively(cleanedText)

	return cleanedText
}

// removeAllEmojisAggressively 激进地移除所有表情符号
func (s *SensitiveWordService) removeAllEmojisAggressively(text string) string {
	// 先使用字符串替换移除常见的表情符号
	commonEmojis := []string{
		"🌈", "🩷", "😀", "😃", "😄", "😁", "😆", "😅", "😂", "🤣", "☺️", "😊", "😇", "🙂", "🙃", "😉", "😌", "😍", "🥰", "😘", "😗", "😙", "😚", "😋", "😛", "😝", "😜", "🤪", "🤨", "🧐", "🤓", "😎", "🤩", "🥳", "😏", "😒", "😞", "😔", "😟", "😕", "🙁", "☹️", "😣", "😖", "😫", "😩", "🥺", "😢", "😭", "😤", "😠", "😡", "🤬", "🤯", "😳", "🥵", "🥶", "😱", "😨", "😰", "😥", "😓", "🤗", "🤔", "🤭", "🤫", "🤥", "😶", "😐", "😑", "😬", "🙄", "😯", "😦", "😧", "😮", "😲", "🥱", "😴", "🤤", "😪", "😵", "🤐", "🥴", "🤢", "🤮", "🤧", "😷", "🤒", "🤕", "🤑", "🤠", "😈", "👿", "👹", "👺", "🤡", "💩", "👻", "💀", "☠️", "👽", "👾", "🤖", "🎃", "😺", "😸", "😹", "😻", "😼", "😽", "🙀", "😿", "😾",
		"❤️", "🧡", "💛", "💚", "💙", "💜", "🖤", "🤍", "🤎", "💔", "❣️", "💕", "💞", "💓", "💗", "💖", "💘", "💝", "💟", "♥️", "💯", "💢", "💥", "💫", "💦", "💨", "🕳️", "💣", "💬", "👁️‍🗨️", "🗨️", "🗯️", "💭", "💤",
		"👋", "🤚", "🖐️", "✋", "🖖", "👌", "🤏", "✌️", "🤞", "🤟", "🤘", "🤙", "👈", "👉", "👆", "🖕", "👇", "☝️", "👍", "👎", "👊", "✊", "🤛", "🤜", "👏", "🙌", "👐", "🤲", "🤝", "🙏", "✍️", "💅", "🤳", "💪", "🦾", "🦿", "🦵", "🦶", "👂", "🦻", "👃", "🧠", "🦷", "🦴", "👀", "👁️", "👅", "👄", "💋",
		"🔥", "⭐", "🌟", "💫", "⚡", "☄️", "💥", "🔴", "🟠", "🟡", "🟢", "🔵", "🟣", "⚫", "⚪", "🟤", "🔺", "🔻", "🔸", "🔹", "🔶", "🔷", "🔳", "🔲", "▪️", "▫️", "◾", "◽", "◼️", "◻️", "⬛", "⬜", "🟥", "🟧", "🟨", "🟩", "🟦", "🟪", "🟫",
		"🎉", "🎊", "🎈", "🎁", "🎀", "🎂", "🍰", "🧁", "🍭", "🍬", "🍫", "🍩", "🍪", "🎃", "🎄", "🎆", "🎇", "🧨", "✨", "🎋", "🎍", "🎎", "🎏", "🎐", "🎑", "🧧", "🎗️", "🎟️", "🎫", "🎖️", "🏆", "🏅", "🥇", "🥈", "🥉",
	}

	cleanedText := text
	for _, emoji := range commonEmojis {
		cleanedText = strings.ReplaceAll(cleanedText, emoji, "")
	}

	// 然后移除所有非基本字符中的表情符号
	result := []rune{}
	for _, r := range cleanedText {
		// 保留基本的拉丁字符、数字、标点符号和空格
		if (r >= 0x0020 && r <= 0x007E) || // 基本ASCII
			(r >= 0x00A0 && r <= 0x00FF) || // 拉丁补充
			(r >= 0x0100 && r <= 0x017F) || // 拉丁扩展A
			(r >= 0x0180 && r <= 0x024F) || // 拉丁扩展B
			(r >= 0x1E00 && r <= 0x1EFF) || // 拉丁扩展附加
			(r >= 0x0400 && r <= 0x04FF) || // 西里尔字母
			(r >= 0x0370 && r <= 0x03FF) || // 希腊字母
			(r >= 0x0600 && r <= 0x06FF) || // 阿拉伯语
			(r >= 0x0750 && r <= 0x077F) || // 阿拉伯语补充
			(r >= 0x08A0 && r <= 0x08FF) || // 阿拉伯语扩展A
			(r >= 0xFB50 && r <= 0xFDFF) || // 阿拉伯语表现形式A
			(r >= 0xFE70 && r <= 0xFEFF) || // 阿拉伯语表现形式B
			(r >= 0x4E00 && r <= 0x9FFF) || // CJK统一汉字
			(r >= 0x3040 && r <= 0x309F) || // 平假名
			(r >= 0x30A0 && r <= 0x30FF) || // 片假名
			(r >= 0xAC00 && r <= 0xD7AF) || // 韩文音节
			r == 0x000A || r == 0x000D || // 换行符
			r == 0x0009 { // 制表符
			result = append(result, r)
		}
		// 跳过所有其他字符（包括表情符号）
	}
	return string(result)
}

// cleanTextForSheinPlatform 为SHEIN平台清理文本
func (s *SensitiveWordService) cleanTextForSheinPlatform(text string) string {
	if text == "" {
		return text
	}

	// 第一步：移除表情符号
	cleanedText := s.filterEmojis(text)

	// 第二步：移除特殊符号和装饰字符
	specialChars := []string{
		"【", "】", "『", "』", "「", "」", "〖", "〗", "〔", "〕", "｛", "｝", "（", "）", "［", "］",
		"★", "☆", "♪", "♫", "♬", "♩", "♭", "♮", "♯", "♠", "♣", "♥", "♦", "♤", "♧", "♡", "♢",
		"※", "§", "¶", "†", "‡", "•", "‰", "′", "″", "‴", "‵", "‶", "‷", "‸", "‹", "›", "‼", "‽",
		"⁇", "⁈", "⁉", "⁏", "⁐", "⁑", "⁒", "⁓", "⁔", "⁕", "⁖", "⁗", "⁘", "⁙", "⁚", "⁛", "⁜", "⁝", "⁞",
		"▲", "▼", "◆", "◇", "○", "●", "◎", "◉", "◈", "◊", "□", "■", "▢", "▣", "▤", "▥", "▦", "▧", "▨", "▩",
		"✓", "✔", "✕", "✖", "✗", "✘", "✚", "✛", "✜", "✝", "✞", "✟", "✠", "✡", "✢", "✣", "✤", "✥", "✦", "✧",
		"➤", "➥", "➦", "➧", "➨", "➩", "➪", "➫", "➬", "➭", "➮", "➯", "➰", "➱", "➲", "➳", "➴", "➵", "➶", "➷",
	}

	for _, char := range specialChars {
		cleanedText = strings.ReplaceAll(cleanedText, char, "")
	}

	// 第三步：清理多余的空格和换行
	cleanedText = regexp.MustCompile(`\s+`).ReplaceAllString(cleanedText, " ")
	cleanedText = strings.TrimSpace(cleanedText)

	return cleanedText
}

// normalizeSpecialCharacters 标准化特殊字符
func (s *SensitiveWordService) normalizeSpecialCharacters(input string) string {
	result := []rune{}

	for _, r := range input {
		switch {
		case r >= 0x1D400 && r <= 0x1D419: // 数学粗体大写
			r = 'A' + (r - 0x1D400)
		case r >= 0x1D434 && r <= 0x1D44D: // 数学斜体大写
			r = 'A' + (r - 0x1D434)
		case r >= 0xFF21 && r <= 0xFF3A: // 全角大写
			r = 'A' + (r - 0xFF21)
		case r >= 0x1D41A && r <= 0x1D433: // 数学粗体小写
			r = 'a' + (r - 0x1D41A)
		case r >= 0x1D44E && r <= 0x1D467: // 数学斜体小写
			r = 'a' + (r - 0x1D44E)
		case r >= 0xFF41 && r <= 0xFF5A: // 全角小写
			r = 'a' + (r - 0xFF41)
		case r >= 0xFF10 && r <= 0xFF19: // 全角数字
			r = '0' + (r - 0xFF10)
		case r == 0x2013 || r == 0x2014: // 各种横线
			r = '-'
		case r == 0x2018 || r == 0x2019: // 各种单引号
			r = '\''
		case r == 0x201C || r == 0x201D: // 各种双引号
			r = '"'
		case r == 0x2026: // 省略号
			r = '.'
		case r == 0x00A0: // 不间断空格
			r = ' '
		}
		result = append(result, r)
	}

	return string(result)
}

// ===== 验证错误处理方法 =====

// extractSensitiveWordsFromValidation 从验证结果中提取敏感词
func (s *SensitiveWordService) extractSensitiveWordsFromValidation(results []PreValidResult) []string {
	var sensitiveWords []string

	for _, result := range results {
		sensitiveWords = append(sensitiveWords, s.extractWordsFromMessages(result.Messages)...)

		for _, messages := range result.OtherLanguageMessageMap {
			sensitiveWords = append(sensitiveWords, s.extractWordsFromMessages(messages)...)
		}

		for _, skcError := range result.SkcErrorMessageMap {
			sensitiveWords = append(sensitiveWords, s.extractWordsFromMessages(skcError.Messages)...)

			for _, messages := range skcError.OtherLanguageMessageMap {
				sensitiveWords = append(sensitiveWords, s.extractWordsFromMessages(messages)...)
			}
		}
	}

	return s.deduplicateWords(sensitiveWords)
}

// extractWordsFromMessages 从消息中提取敏感词
func (s *SensitiveWordService) extractWordsFromMessages(messages []string) []string {
	var words []string

	patterns := []string{
		`敏感词[：:]\s*\[?([^\]\s]+)\]?`,
		`包含敏感词[：:]\s*\[([^\]]+)\]`,
		`敏感词[：:]\s*\[([^\]]+)\]`,
		`敏感词[：:]\s*([^，,\[\]]+(?:[，,][^，,\[\]]+)*)`,
		`contains?\s+sensitive\s+words?\s*[：:]\s*\[?([^\]]+)\]?`,
		`sensitive\s+words?\s*[：:]\s*\[?([^\]]+)\]?`,
		`违禁词[：:]\s*\[?([^\]]+)\]?`,
		`禁用词[：:]\s*\[?([^\]]+)\]?`,
		`不当词汇[：:]\s*\[?([^\]]+)\]?`,
	}

	for _, message := range messages {
		for _, pattern := range patterns {
			re := regexp.MustCompile(pattern)
			matches := re.FindAllStringSubmatch(message, -1)

			for _, match := range matches {
				if len(match) > 1 {
					wordsStr := strings.TrimSpace(match[1])
					extractedWords := s.splitWords(wordsStr)
					words = append(words, extractedWords...)
				}
			}
		}
	}

	return s.deduplicateWords(words)
}

// splitWords 分割单词字符串
func (s *SensitiveWordService) splitWords(wordsStr string) []string {
	var words []string

	switch {
	case strings.Contains(wordsStr, "、"):
		words = strings.Split(wordsStr, "、")
	case strings.Contains(wordsStr, ","):
		words = strings.Split(wordsStr, ",")
	case strings.Contains(wordsStr, "，"):
		words = strings.Split(wordsStr, "，")
	default:
		words = strings.Fields(wordsStr)
	}

	var cleanWords []string
	for _, word := range words {
		if word = strings.TrimSpace(strings.Trim(word, "[]")); word != "" {
			cleanWords = append(cleanWords, word)
		}
	}

	return cleanWords
}

// ===== 内部辅助方法 =====

// addWordsByLanguage 按指定语言添加敏感词
func (s *SensitiveWordService) addWordsByLanguage(configMap map[string][]string, language string, words []string, wordType string) {
	if len(words) == 0 {
		return
	}

	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.config == nil {
		logrus.Errorf("配置未初始化，无法添加%s敏感词", wordType)
		return
	}

	if configMap[language] == nil {
		configMap[language] = make([]string, 0)
	}

	configMap[language] = append(configMap[language], words...)
	configMap[language] = s.deduplicateWords(configMap[language])

	logrus.Infof("📝 添加%s敏感词 [%s]: %v (当前该语言%s列表共%d个)",
		wordType, language, words, wordType, len(configMap[language]))

	s.saveConfigAsync()
}

// addWordsToConfig 将分类后的敏感词添加到配置中
func (s *SensitiveWordService) addWordsToConfig(configMap map[string][]string, wordsByLang map[string][]string, wordType string) int {
	totalAdded := 0

	for lang, langWords := range wordsByLang {
		if len(langWords) == 0 {
			continue
		}

		if configMap[lang] == nil {
			configMap[lang] = make([]string, 0)
		}

		configMap[lang] = append(configMap[lang], langWords...)
		configMap[lang] = s.deduplicateWords(configMap[lang])

		totalAdded += len(langWords)
		logrus.Infof("📝 添加%s敏感词 [%s]: %v (当前该语言%s列表共%d个)",
			wordType, lang, langWords, wordType, len(configMap[lang]))
	}

	return totalAdded
}

// countWordsInConfig 统计配置中的敏感词数量
func (s *SensitiveWordService) countWordsInConfig(configMap map[string][]string) int {
	total := 0
	for _, words := range configMap {
		total += len(words)
	}
	return total
}

// saveConfigAsync 异步保存配置
func (s *SensitiveWordService) saveConfigAsync() {
	go func() {
		if err := s.saveConfig(); err != nil {
			logrus.Errorf("保存敏感词配置失败: %v", err)
		}
	}()
}

// deduplicateWords 去重单词列表
func (s *SensitiveWordService) deduplicateWords(words []string) []string {
	seen := make(map[string]bool)
	var result []string

	for _, word := range words {
		if !seen[word] {
			seen[word] = true
			result = append(result, word)
		}
	}

	return result
}

// ===== 日志和统计方法 =====

// logConfigLoadStats 记录配置加载统计信息
func (s *SensitiveWordService) logConfigLoadStats() {
	staticTotal := s.countWordsInConfig(s.config.StaticWords)
	dynamicTotal := s.countWordsInConfig(s.config.DynamicWords)

	for lang, words := range s.config.StaticWords {
		if len(words) > 0 {
			logrus.Debugf("加载静态敏感词 [%s]: %d 个", lang, len(words))
		}
	}

	for lang, words := range s.config.DynamicWords {
		if len(words) > 0 {
			logrus.Debugf("加载动态敏感词 [%s]: %d 个", lang, len(words))
		}
	}

	logrus.Infof("✅ 成功加载敏感词配置: 静态(%d) + 动态(%d) = 总计(%d)",
		staticTotal, dynamicTotal, staticTotal+dynamicTotal)
}

// logSensitiveWordStats 记录敏感词统计信息
func (s *SensitiveWordService) logSensitiveWordStats() {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	if s.config == nil {
		logrus.Warn("敏感词配置未初始化")
		return
	}

	staticTotal := s.countWordsInConfig(s.config.StaticWords)
	dynamicTotal := s.countWordsInConfig(s.config.DynamicWords)
	amazonBrandWordsCount := len(s.getAmazonBrandWords())

	logrus.Infof("📊 敏感词统计:")

	// 显示各语言的敏感词数量
	for lang, words := range s.config.StaticWords {
		if count := len(words); count > 0 {
			logrus.Infof("   静态敏感词 [%s]: %d 个", lang, count)
		}
	}

	for lang, words := range s.config.DynamicWords {
		if count := len(words); count > 0 {
			logrus.Infof("   动态敏感词 [%s]: %d 个", lang, count)
		}
	}

	logrus.Infof("   Amazon品牌词: %d 个", amazonBrandWordsCount)
	logrus.Infof("   总计: 静态(%d) + 动态(%d) + 品牌词(%d) = %d 个",
		staticTotal, dynamicTotal, amazonBrandWordsCount, staticTotal+dynamicTotal+amazonBrandWordsCount)
	logrus.Infof("   配置文件: %s", s.configPath)
	logrus.Infof("   最后更新: %s", s.config.LastUpdated.Format("2006-01-02 15:04:05"))
}

// ===== Amazon品牌词处理方法 =====

// removeAmazonBrandWords 移除Amazon品牌词
func (s *SensitiveWordService) removeAmazonBrandWords(text string) string {
	if text == "" {
		return text
	}

	originalText := text
	cleanedText := text
	amazonBrandWords := s.getAmazonBrandWords()
	removedWords := []string{}

	for _, brandWord := range amazonBrandWords {
		beforeRemoval := cleanedText
		cleanedText = s.removeWordFromText(cleanedText, brandWord)

		// 记录被移除的品牌词
		if beforeRemoval != cleanedText {
			removedWords = append(removedWords, brandWord)
		}
	}

	// 记录品牌词移除统计
	if len(removedWords) > 0 {
		logrus.Debugf("🏷️ 移除Amazon品牌词: %v", removedWords)
		logrus.Debugf("🏷️ 品牌词清理: %s -> %s", originalText, cleanedText)
	}

	return cleanedText
}

// removeContextBrandWords 移除上下文中的品牌词（从AmazonProduct.Brand字段）
func (s *SensitiveWordService) removeContextBrandWords(ctx *TaskContext, text string) string {
	if text == "" || ctx == nil || ctx.AmazonProduct == nil {
		return text
	}

	brandWord := strings.TrimSpace(ctx.AmazonProduct.Brand)
	if brandWord == "" {
		return text
	}

	originalText := text
	cleanedText := s.removeWordFromText(text, brandWord)

	// 记录品牌词移除统计
	if originalText != cleanedText {
		logrus.Debugf("🏷️ 移除上下文品牌词: %s", brandWord)
		logrus.Debugf("🏷️ 上下文品牌词清理: %s -> %s", originalText, cleanedText)
	}

	return cleanedText
}

// getAmazonBrandWords 获取Amazon品牌词列表
func (s *SensitiveWordService) getAmazonBrandWords() []string {
	return []string{
		// Amazon自有品牌
		"Amazon", "amazon", "AMAZON",
		"Amazon Basics", "AmazonBasics", "Amazon basics",
		"Amazon Essentials", "AmazonEssentials", "Amazon essentials",
		"Amazon Choice", "Amazon's Choice", "Amazon choice",
		"Solimo", "SOLIMO", "solimo",
		"Goodthreads", "GOODTHREADS", "goodthreads",
		"Daily Ritual", "DAILY RITUAL", "daily ritual",
		"Core 10", "CORE 10", "core 10",
		"Lark & Ro", "LARK & RO", "lark & ro",
		"28 Palms", "28 PALMS", "28 palms",
		"Buttoned Down", "BUTTONED DOWN", "buttoned down",
		"Brand - ", "Brand: ", "brand - ", "brand: ",

		// 常见的Amazon产品标识词
		"Prime", "PRIME", "prime",
		"Prime Eligible", "Prime eligible", "prime eligible",
		"Free Shipping", "FREE SHIPPING", "free shipping",
		"Best Seller", "BEST SELLER", "best seller",
		"#1 Best Seller", "#1 BEST SELLER", "#1 best seller",
		"Amazon's", "amazon's", "AMAZON'S",

		// 其他Amazon相关词汇
		"Fulfillment by Amazon", "FBA", "fba",
		"Ships from Amazon", "ships from amazon",
		"Sold by Amazon", "sold by amazon",
		"Amazon Warehouse", "amazon warehouse",

		// 品牌标识符
		"Brand New", "BRAND NEW", "brand new",
		"Official", "OFFICIAL", "official",
		"Authentic", "AUTHENTIC", "authentic",
		"Original", "ORIGINAL", "original",
		"Genuine", "GENUINE", "genuine",
	}
}

// TestEmojiFiltering 测试表情符号过滤功能（用于调试）
func (s *SensitiveWordService) TestEmojiFiltering(text string) string {
	logrus.Infof("🧪 测试表情符号过滤: %s", text)

	result := s.filterEmojis(text)
	logrus.Infof("🧪 过滤结果: %s -> %s", text, result)

	return result
}

// TestSheinPlatformCleaning 测试SHEIN平台文本清理功能（用于调试）
func (s *SensitiveWordService) TestSheinPlatformCleaning(text string) string {
	logrus.Infof("🧪 测试SHEIN平台文本清理: %s", text)

	result := s.cleanTextForSheinPlatform(text)
	logrus.Infof("🧪 清理结果: %s -> %s", text, result)

	return result
}
