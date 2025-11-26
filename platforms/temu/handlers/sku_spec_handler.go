package handlers

import (
	"fmt"
	"strings"

	"task-processor/common/amazon"
	"task-processor/platforms/temu/types"

	"github.com/sirupsen/logrus"
)

// SpecDimension 规格维度
type SpecDimension struct {
	ParentSpecID   string
	ParentSpecName string
	Values         []SpecValue
	IsColor        bool
}

// SpecValue 规格值
type SpecValue struct {
	SpecID   string
	SpecName string
}

// SkuSpecHandler SKU规格处理器
type SkuSpecHandler struct {
	logger *logrus.Entry
}

// NewSkuSpecHandler 创建新的规格处理器
func NewSkuSpecHandler(logger *logrus.Entry) *SkuSpecHandler {
	return &SkuSpecHandler{
		logger: logger,
	}
}

// collectNonColorSpecsForColor 收集特定颜色下的非颜色规格
func (sh *SkuSpecHandler) collectNonColorSpecsForColor(aiMapping *AISkuMappingResponse, skuIndices []int, templateSpecs []GoodsSpecProperty) map[string]*SpecDimension {
	dimensions := make(map[string]*SpecDimension)

	// 从该颜色组的SKU中收集非颜色规格
	for _, skuIndex := range skuIndices {
		aiSku := aiMapping.SkuList[skuIndex]
		for _, spec := range aiSku.Spec {
			parentSpecID := spec.ParentSpecID

			// 跳过颜色规格
			isColor := false
			for _, templateSpec := range templateSpecs {
				if templateSpec.ParentSpecID == parentSpecID &&
					sh.isColorSpec(strings.ToLower(templateSpec.Name)) {
					isColor = true
					break
				}
			}
			if isColor {
				continue
			}

			if _, exists := dimensions[parentSpecID]; !exists {
				// 查找模板中的规格名称
				parentSpecName := spec.ParentSpecName
				for _, templateSpec := range templateSpecs {
					if templateSpec.ParentSpecID == parentSpecID {
						parentSpecName = templateSpec.Name
						break
					}
				}

				dimensions[parentSpecID] = &SpecDimension{
					ParentSpecID:   parentSpecID,
					ParentSpecName: parentSpecName,
					Values:         []SpecValue{},
					IsColor:        false,
				}
			}

			// 添加规格值（去重）
			dimension := dimensions[parentSpecID]
			found := false
			for _, value := range dimension.Values {
				if value.SpecID == spec.SpecID {
					found = true
					break
				}
			}
			if !found {
				dimension.Values = append(dimension.Values, SpecValue{
					SpecID:   spec.SpecID,
					SpecName: spec.SpecName,
				})
			}
		}
	}

	return dimensions
}

// generateNonColorSpecCombinations 生成非颜色规格的所有组合
func (sh *SkuSpecHandler) generateNonColorSpecCombinations(dimensions map[string]*SpecDimension) [][]types.SpecInfo {
	if len(dimensions) == 0 {
		// 如果没有非颜色规格，返回空切片（不是包含空数组的切片）
		sh.logger.Warn("没有非颜色规格维度，返回空组合列表")
		return [][]types.SpecInfo{}
	}

	// 将维度转换为切片以便处理
	var dimensionList []*SpecDimension
	for _, dimension := range dimensions {
		dimensionList = append(dimensionList, dimension)
	}

	// 生成笛卡尔积
	return sh.generateCartesianProduct(dimensionList, 0, []types.SpecInfo{})
}

// generateCartesianProduct 生成笛卡尔积
func (sh *SkuSpecHandler) generateCartesianProduct(dimensions []*SpecDimension, index int, current []types.SpecInfo) [][]types.SpecInfo {
	if index == len(dimensions) {
		// 复制当前组合
		combination := make([]types.SpecInfo, len(current))
		copy(combination, current)
		return [][]types.SpecInfo{combination}
	}

	var results [][]types.SpecInfo
	dimension := dimensions[index]

	for _, value := range dimension.Values {
		spec := types.SpecInfo{
			ParentSpecID:   dimension.ParentSpecID,
			ParentSpecName: dimension.ParentSpecName,
			SpecID:         value.SpecID,
			SpecName:       value.SpecName,
		}

		newCurrent := append(current, spec)
		subResults := sh.generateCartesianProduct(dimensions, index+1, newCurrent)
		results = append(results, subResults...)
	}

	return results
}

// createSpecCombinationKey 创建规格组合的唯一键
func (sh *SkuSpecHandler) createSpecCombinationKey(specs []types.SpecInfo) string {
	if len(specs) == 0 {
		return "empty_spec"
	}

	var parts []string
	for _, spec := range specs {
		parts = append(parts, fmt.Sprintf("%s:%s", spec.ParentSpecID, spec.SpecID))
	}
	return strings.Join(parts, "|")
}

// createNonColorSpecKey 创建非颜色规格的键
func (sh *SkuSpecHandler) createNonColorSpecKey(specs []types.SpecInfo) string {
	var parts []string
	for _, spec := range specs {
		// 跳过颜色规格
		if !sh.isColorSpec(strings.ToLower(spec.ParentSpecName)) {
			parts = append(parts, fmt.Sprintf("%s:%s", spec.ParentSpecID, spec.SpecID))
		}
	}
	if len(parts) == 0 {
		return "no_non_color_spec"
	}
	return strings.Join(parts, "|")
}

// createSpecCombinationKeyFromSpecs 从规格列表创建组合键
func (sh *SkuSpecHandler) createSpecCombinationKeyFromSpecs(specs []types.SpecInfo) string {
	if len(specs) == 0 {
		return "empty_spec"
	}

	var parts []string
	for _, spec := range specs {
		parts = append(parts, fmt.Sprintf("%s:%s", spec.ParentSpecID, spec.SpecID))
	}
	return strings.Join(parts, "|")
}

// addColorSpecToCombination 将颜色规格添加到组合中
func (sh *SkuSpecHandler) addColorSpecToCombination(nonColorSpecs []types.SpecInfo, colorSpecID string, aiMapping *AISkuMappingResponse, skuIndices []int) []types.SpecInfo {
	// 找到颜色规格信息
	var colorSpec types.SpecInfo
	for _, skuIndex := range skuIndices {
		aiSku := aiMapping.SkuList[skuIndex]
		for _, spec := range aiSku.Spec {
			if spec.SpecID == colorSpecID {
				colorSpec = spec
				break
			}
		}
		if colorSpec.SpecID != "" {
			break
		}
	}

	// 将颜色规格添加到组合的开头
	fullSpecs := []types.SpecInfo{colorSpec}
	fullSpecs = append(fullSpecs, nonColorSpecs...)

	return fullSpecs
}

// isColorSpec 判断是否为颜色规格
func (sh *SkuSpecHandler) isColorSpec(specName string) bool {
	colorKeywords := []string{"color", "colour", "颜色", "色"}
	lowerName := strings.ToLower(specName)
	for _, keyword := range colorKeywords {
		if strings.Contains(lowerName, keyword) {
			return true
		}
	}
	return false
}

// convertUserInputSpecsToGoodsSpecProperties 转换用户输入规格为商品规格属性
func (sh *SkuSpecHandler) convertUserInputSpecsToGoodsSpecProperties(userInputSpecs []UserInputParentSpec) []GoodsSpecProperty {
	var templateSpecs []GoodsSpecProperty
	for _, userSpec := range userInputSpecs {
		templateSpecs = append(templateSpecs, GoodsSpecProperty{
			ParentSpecID: userSpec.ParentSpecID,
			Name:         userSpec.ParentSpecName,
		})
	}
	return templateSpecs
}

// ValidateSpecs 验证规格是否有效
func (sh *SkuSpecHandler) ValidateSpecs(specs []types.SpecInfo) error {
	if len(specs) == 0 {
		return fmt.Errorf("规格列表为空")
	}

	// TEMU规则：销售规格最多只能有2种
	if len(specs) > 2 {
		return fmt.Errorf("规格数量超过限制：当前有%d个规格，但TEMU最多只允许2个销售规格", len(specs))
	}

	for i, spec := range specs {
		if spec.ParentSpecID == "" {
			return fmt.Errorf("规格[%d]的ParentSpecID为空", i)
		}
		if spec.SpecID == "" {
			return fmt.Errorf("规格[%d]的SpecID为空", i)
		}
	}

	return nil
}

// CreateDefaultSpec 创建默认规格（当没有规格时使用）
// 注意：不应该使用默认规格，必须从模板中选择规格
// 这个方法现在返回空切片并记录错误
func (sh *SkuSpecHandler) CreateDefaultSpec(variant *amazon.Product) []types.SpecInfo {
	sh.logger.Error("❌ 尝试创建默认规格！这是不允许的，必须使用TEMU模板中的规格")
	sh.logger.Error("❌ 请检查AI映射是否正确生成了规格，或者模板规格是否正确配置")

	// 返回空切片，让调用方知道出错了
	return []types.SpecInfo{}
}
