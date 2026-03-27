package rules

import (
	"fmt"
	"regexp"
	"strings"

		"task-processor/internal/core/logger"
	"github.com/sirupsen/logrus"
)

// BulletPointsOptimizer 产品要点优化器
type BulletPointsOptimizer struct {
	logger *logrus.Entry
}

// OptimizationStrategy 优化策略
type OptimizationStrategy struct {
	MaxPoints      int      `json:"max_points"`       // 最大要点数
	MaxTotalLength int      `json:"max_total_length"` // 最大总长度
	MaxPointLength int      `json:"max_point_length"` // 单个要点最大长度
	MinPointLength int      `json:"min_point_length"` // 单个要点最小长度
	PriorityWords  []string `json:"priority_words"`   // 优先关键词
}

// NewBulletPointsOptimizer 创建新的要点优化器
func NewBulletPointsOptimizer() *BulletPointsOptimizer {
	return &BulletPointsOptimizer{
		logger: logger.GetGlobalLogger("BulletPointsOptimizer"),
	}
}

// GetDefaultStrategy 获取默认优化策略
func (o *BulletPointsOptimizer) GetDefaultStrategy() *OptimizationStrategy {
	return &OptimizationStrategy{
		MaxPoints:      6,
		MaxTotalLength: 700,
		MaxPointLength: 120,
		MinPointLength: 15,
		PriorityWords: []string{
			"ergonomic", "comfortable", "durable", "high-quality", "premium",
			"adjustable", "easy", "professional", "safe", "reliable",
			"lightweight", "portable", "waterproof", "breathable", "efficient",
		},
	}
}

// OptimizeForTemu 为TEMU平台优化要点
func (o *BulletPointsOptimizer) OptimizeForTemu(points []string, productName string) ([]string, []string) {
	strategy := o.GetDefaultStrategy()
	var optimizations []string

	// 1. 分析产品类型和关键特性
	productType := o.analyzeProductType(productName)
	keyFeatures := o.extractKeyFeatures(productName)

	// 2. 优化每个要点
	optimizedPoints := []string{}
	for i, point := range points {
		if i >= strategy.MaxPoints {
			optimizations = append(optimizations, fmt.Sprintf("截断超出的要点: 保留前%d个", strategy.MaxPoints))
			break
		}

		optimized := o.optimizeSinglePoint(point, productType, keyFeatures, strategy)
		if optimized != point {
			optimizations = append(optimizations, fmt.Sprintf("优化要点[%d]: 提升表达效果", i+1))
		}
		optimizedPoints = append(optimizedPoints, optimized)
	}

	// 3. 确保包含关键卖点
	optimizedPoints = o.ensureKeySellingPoints(optimizedPoints, productType, keyFeatures, &optimizations)

	// 4. 优化要点顺序
	optimizedPoints = o.optimizePointOrder(optimizedPoints, &optimizations)

	// 5. 控制总长度
	optimizedPoints = o.controlTotalLength(optimizedPoints, strategy.MaxTotalLength, &optimizations)

	return optimizedPoints, optimizations
}

// analyzeProductType 分析产品类型
func (o *BulletPointsOptimizer) analyzeProductType(productName string) string {
	nameLower := strings.ToLower(productName)

	productTypes := map[string][]string{
		"chair":       {"chair", "seat", "seating"},
		"desk":        {"desk", "table", "workstation"},
		"electronics": {"computer", "pc", "electronic", "device"},
		"gaming":      {"gaming", "game", "gamer"},
		"office":      {"office", "work", "professional", "business"},
		"furniture":   {"furniture", "home", "decor"},
	}

	for productType, keywords := range productTypes {
		for _, keyword := range keywords {
			if strings.Contains(nameLower, keyword) {
				return productType
			}
		}
	}

	return "general"
}

// extractKeyFeatures 提取关键特性
func (o *BulletPointsOptimizer) extractKeyFeatures(productName string) []string {
	nameLower := strings.ToLower(productName)
	var features []string

	featureKeywords := map[string][]string{
		"ergonomic":    {"ergonomic", "comfort", "comfortable"},
		"adjustable":   {"adjustable", "height", "tilt"},
		"durable":      {"durable", "strong", "sturdy", "heavy-duty"},
		"premium":      {"premium", "high-quality", "quality"},
		"portable":     {"portable", "lightweight", "compact"},
		"professional": {"professional", "executive", "business"},
	}

	for feature, keywords := range featureKeywords {
		for _, keyword := range keywords {
			if strings.Contains(nameLower, keyword) {
				features = append(features, feature)
				break
			}
		}
	}

	return features
}

// optimizeSinglePoint 优化单个要点
func (o *BulletPointsOptimizer) optimizeSinglePoint(point, productType string, keyFeatures []string, strategy *OptimizationStrategy) string {
	// 清理和格式化
	optimized := strings.TrimSpace(point)

	// 确保首字母大写
	if len(optimized) > 0 {
		if len(optimized) == 1 {
			optimized = strings.ToUpper(optimized)
		} else {
			optimized = strings.ToUpper(string(optimized[0])) + optimized[1:]
		}
	}

	// 移除末尾句号
	optimized = strings.TrimSuffix(optimized, ".")

	// 增强表达效果
	optimized = o.enhanceExpression(optimized, productType, keyFeatures)

	// 控制长度
	if len(optimized) > strategy.MaxPointLength {
		optimized = o.truncatePoint(optimized, strategy.MaxPointLength)
	}

	return optimized
}

// enhanceExpression 增强表达效果
func (o *BulletPointsOptimizer) enhanceExpression(point, productType string, keyFeatures []string) string {
	_ = productType // 预留参数，用于未来根据产品类型定制增强逻辑
	_ = keyFeatures // 预留参数，用于未来根据关键特性定制增强逻辑

	// 替换通用词汇为更具体的描述
	replacements := map[string]string{
		"good quality": "premium quality",
		"nice":         "excellent",
		"great":        "outstanding",
		"comfortable":  "ergonomically comfortable",
		"strong":       "durable and robust",
		"easy to use":  "user-friendly design",
		"looks good":   "stylish appearance",
	}

	pointLower := strings.ToLower(point)
	for old, new := range replacements {
		if strings.Contains(pointLower, old) {
			re := regexp.MustCompile(`(?i)\b` + regexp.QuoteMeta(old) + `\b`)
			point = re.ReplaceAllString(point, new)
		}
	}

	return point
}

// ensureKeySellingPoints 确保包含关键卖点
func (o *BulletPointsOptimizer) ensureKeySellingPoints(points []string, productType string, keyFeatures []string, optimizations *[]string) []string {
	allText := strings.ToLower(strings.Join(points, " "))

	// 根据产品类型添加必要的卖点
	requiredPoints := o.getRequiredSellingPoints(productType, keyFeatures)

	for _, requiredPoint := range requiredPoints {
		if !o.containsSellingPoint(allText, requiredPoint) && len(points) < 6 {
			points = append(points, requiredPoint)
			*optimizations = append(*optimizations, "添加关键卖点: "+requiredPoint)
		}
	}

	return points
}

// getRequiredSellingPoints 获取必需的卖点
func (o *BulletPointsOptimizer) getRequiredSellingPoints(productType string, keyFeatures []string) []string {
	var points []string

	switch productType {
	case "chair":
		if !o.containsFeature(keyFeatures, "ergonomic") {
			points = append(points, "Ergonomic design for optimal comfort")
		}
		if !o.containsFeature(keyFeatures, "adjustable") {
			points = append(points, "Adjustable features for personalized use")
		}
	case "electronics":
		points = append(points, "Reliable performance and quality")
	case "gaming":
		points = append(points, "Designed for extended gaming sessions")
	}

	return points
}

// containsSellingPoint 检查是否包含特定卖点
func (o *BulletPointsOptimizer) containsSellingPoint(text, sellingPoint string) bool {
	sellingPointWords := strings.Fields(strings.ToLower(sellingPoint))

	// 检查是否包含卖点的关键词
	matchCount := 0
	for _, word := range sellingPointWords {
		if strings.Contains(text, word) {
			matchCount++
		}
	}

	// 如果匹配超过一半的关键词，认为已包含该卖点
	return matchCount >= len(sellingPointWords)/2
}

// containsFeature 检查是否包含特定特性
func (o *BulletPointsOptimizer) containsFeature(features []string, feature string) bool {
	for _, f := range features {
		if f == feature {
			return true
		}
	}
	return false
}

// optimizePointOrder 优化要点顺序
func (o *BulletPointsOptimizer) optimizePointOrder(points []string, optimizations *[]string) []string {
	if len(points) <= 1 {
		return points
	}

	// 按重要性排序要点
	priorityScores := make([]int, len(points))

	for i, point := range points {
		priorityScores[i] = o.calculatePriorityScore(point)
	}

	// 简单的冒泡排序（按优先级降序）
	for i := 0; i < len(points)-1; i++ {
		for j := 0; j < len(points)-i-1; j++ {
			if priorityScores[j] < priorityScores[j+1] {
				// 交换要点和分数
				points[j], points[j+1] = points[j+1], points[j]
				priorityScores[j], priorityScores[j+1] = priorityScores[j+1], priorityScores[j]
			}
		}
	}

	*optimizations = append(*optimizations, "优化要点顺序: 按重要性排列")
	return points
}

// calculatePriorityScore 计算要点优先级分数
func (o *BulletPointsOptimizer) calculatePriorityScore(point string) int {
	score := 0
	pointLower := strings.ToLower(point)

	// 高优先级关键词
	highPriorityWords := []string{"ergonomic", "comfortable", "quality", "durable", "premium"}
	for _, word := range highPriorityWords {
		if strings.Contains(pointLower, word) {
			score += 10
		}
	}

	// 中优先级关键词
	mediumPriorityWords := []string{"adjustable", "easy", "professional", "safe", "reliable"}
	for _, word := range mediumPriorityWords {
		if strings.Contains(pointLower, word) {
			score += 5
		}
	}

	// 长度奖励（适中长度的要点更好）
	if len(point) >= 30 && len(point) <= 80 {
		score += 3
	}

	return score
}

// controlTotalLength 控制总长度
func (o *BulletPointsOptimizer) controlTotalLength(points []string, maxLength int, optimizations *[]string) []string {
	totalLength := 0
	for _, point := range points {
		totalLength += len(point)
	}

	if totalLength <= maxLength {
		return points
	}

	// 需要缩减长度
	*optimizations = append(*optimizations, fmt.Sprintf("控制总长度: %d -> %d字符", totalLength, maxLength))

	// 逐步缩减要点
	for totalLength > maxLength && len(points) > 0 {
		// 移除最后一个要点或缩短最长的要点
		longestIndex := 0
		longestLength := len(points[0])

		for i, point := range points {
			if len(point) > longestLength {
				longestIndex = i
				longestLength = len(point)
			}
		}

		if longestLength > 50 {
			// 缩短最长的要点
			points[longestIndex] = o.truncatePoint(points[longestIndex], longestLength-20)
		} else {
			// 移除最后一个要点
			points = points[:len(points)-1]
		}

		// 重新计算总长度
		totalLength = 0
		for _, point := range points {
			totalLength += len(point)
		}
	}

	return points
}

// truncatePoint 截断要点
func (o *BulletPointsOptimizer) truncatePoint(point string, maxLength int) string {
	if len(point) <= maxLength {
		return point
	}

	// 在单词边界截断
	words := strings.Fields(point)
	result := ""

	for _, word := range words {
		if len(result+" "+word) <= maxLength-3 { // 为"..."预留空间
			if result == "" {
				result = word
			} else {
				result += " " + word
			}
		} else {
			break
		}
	}

	if result != point {
		result += "..."
	}

	return result
}

// GetOptimizationTips 获取优化建议
func (o *BulletPointsOptimizer) GetOptimizationTips(productType string) []string {
	baseTips := []string{
		"突出产品的独特卖点和优势",
		"使用具体而有说服力的词汇",
		"保持要点简洁明了",
		"按重要性排序要点",
		"避免重复相似内容",
	}

	switch productType {
	case "chair":
		baseTips = append(baseTips, []string{
			"强调舒适性和人体工学设计",
			"突出材质和耐用性",
			"说明调节功能和适用场景",
		}...)
	case "electronics":
		baseTips = append(baseTips, []string{
			"强调性能和可靠性",
			"突出技术特性和兼容性",
			"说明易用性和安全性",
		}...)
	}

	return baseTips
}
