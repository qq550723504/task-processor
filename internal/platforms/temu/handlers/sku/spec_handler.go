package sku

import (
	"fmt"
	"strings"

	"task-processor/internal/domain/model"
	models "task-processor/internal/platforms/temu/api/product"
	"task-processor/internal/platforms/temu/types"

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
func (sh *SkuSpecHandler) collectNonColorSpecsForColor(aiMapping *types.AISkuMappingResponse, skuIndices []int, templateSpecs []types.TemplateRespGoodsSpecProperty) map[string]*SpecDimension {
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
func (sh *SkuSpecHandler) generateNonColorSpecCombinations(dimensions map[string]*SpecDimension) [][]models.SpecInfo {
	if len(dimensions) == 0 {
		// 如果没有非颜色规格，返回空切片（不是包含空数组的切片）
		sh.logger.Warn("没有非颜色规格维度，返回空组合列表")
		return [][]models.SpecInfo{}
	}

	// 将维度转换为切片以便处理
	var dimensionList []*SpecDimension
	for _, dimension := range dimensions {
		dimensionList = append(dimensionList, dimension)
	}

	// 生成笛卡尔积
	return sh.generateCartesianProduct(dimensionList, 0, []models.SpecInfo{})
}

// generateCartesianProduct 生成笛卡尔积
func (sh *SkuSpecHandler) generateCartesianProduct(dimensions []*SpecDimension, index int, current []models.SpecInfo) [][]models.SpecInfo {
	if index == len(dimensions) {
		// 复制当前组合
		combination := make([]models.SpecInfo, len(current))
		copy(combination, current)
		return [][]models.SpecInfo{combination}
	}

	var results [][]models.SpecInfo
	dimension := dimensions[index]

	for _, value := range dimension.Values {
		spec := models.SpecInfo{
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
func (sh *SkuSpecHandler) createSpecCombinationKey(specs []models.SpecInfo) string {
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
func (sh *SkuSpecHandler) createNonColorSpecKey(specs []models.SpecInfo) string {
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
func (sh *SkuSpecHandler) createSpecCombinationKeyFromSpecs(specs []models.SpecInfo) string {
	if len(specs) == 0 {
		return "empty_spec"
	}

	var parts []string
	for _, spec := range specs {
		parts = append(parts, fmt.Sprintf("%s:%s", spec.ParentSpecID, spec.SpecID))
	}
	return strings.Join(parts, "|")
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

// IsSizeSpec 判断是否为尺寸规格（实现 SpecHandlerInterface 接口）
func (sh *SkuSpecHandler) IsSizeSpec(specName string) bool {
	sizeKeywords := []string{
		"size", "尺寸", "尺码", "大小", "码数",
		"length", "width", "height", "长度", "宽度", "高度",
		"dimension", "dimensions", "规格",
	}

	specNameLower := strings.ToLower(specName)
	for _, keyword := range sizeKeywords {
		if strings.Contains(specNameLower, keyword) {
			return true
		}
	}
	return false
}

// convertUserInputSpecsToGoodsSpecProperties 转换用户输入规格为商品规格属性
func (sh *SkuSpecHandler) convertUserInputSpecsToGoodsSpecProperties(userInputSpecs []types.UserInputParentSpec) []types.TemplateRespGoodsSpecProperty {
	var specProperties []types.TemplateRespGoodsSpecProperty
	for i, userSpec := range userInputSpecs {
		specProperty := types.TemplateRespGoodsSpecProperty{
			PID:               i + 1000, // 使用临时ID
			TemplateModuleID:  0,
			TemplatePID:       0,
			RefPID:            0,
			Name:              userSpec.ParentSpecName,
			PropertyValueType: 1, // 假设为选择类型
			ValueUnit:         []string{},
			Values:            []types.PropertyValue{}, // 用户输入规格通常没有预定义值
			MaxValue:          "",
			MinValue:          "",
			ValuePrecision:    0,
			Required:          false,
			IsSale:            true,
			ParentSpecID:      userSpec.ParentSpecID,
			MainSale:          true,
			Feature:           0,
			ControlType:       1,
		}
		specProperties = append(specProperties, specProperty)
	}
	return specProperties
}

// ValidateSpecs 验证规格是否有效
func (sh *SkuSpecHandler) ValidateSpecs(specs []models.SpecInfo) error {
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
func (sh *SkuSpecHandler) CreateDefaultSpec(variant *model.Product) []models.SpecInfo {
	sh.logger.Error("❌ 尝试创建默认规格！这是不允许的，必须使用TEMU模板中的规格")
	sh.logger.Error("❌ 请检查AI映射是否正确生成了规格，或者模板规格是否正确配置")

	// 返回空切片，让调用方知道出错了
	return []models.SpecInfo{}
}
