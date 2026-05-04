package category

import (
	"regexp"
	"strconv"
	"task-processor/internal/core/logger"

	"task-processor/internal/shein"
)

// CollectCategoryRestrictionsHandler 错误时收集分类限制及敏感词处理器
type CollectCategoryRestrictionsHandler struct {
}

// NewCollectCategoryRestrictionsHandler 创建新的错误时收集分类限制及敏感词处理器
func NewCollectCategoryRestrictionsHandler() *CollectCategoryRestrictionsHandler {
	return &CollectCategoryRestrictionsHandler{}
}

// Name 返回处理器名称
func (h *CollectCategoryRestrictionsHandler) Name() string {
	return "错误时收集分类限制及敏感词"
}

// Handle 执行错误时收集分类限制及敏感词处理
func (h *CollectCategoryRestrictionsHandler) Handle(ctx *shein.TaskContext) error {

	// 收集分类限制信息
	if err := h.collectCategoryRestrictions(ctx); err != nil {
		logger.GetGlobalLogger("shein/category").Warnf("收集分类限制信息时出错: %v", err)
		// 收集分类限制信息失败，但不影响主流程，记录警告日志即可
	}

	// 检查是否真的存在主规格相关的错误
	hasMainSpecError := h.hasMainSpecificationError(ctx)
	if hasMainSpecError {
		logger.GetGlobalLogger("shein/category").Warnf("检测到主规格配置错误，终止处理流程")
		return shein.NewNonRetryableError("主规格错误", nil)
	}

	// 如果没有主规格错误，处理成功
	logger.GetGlobalLogger("shein/category").Info("分类限制信息收集完成，无主规格错误")
	return nil
}

// collectCategoryRestrictions 收集分类限制信息
func (h *CollectCategoryRestrictionsHandler) collectCategoryRestrictions(ctx *shein.TaskContext) error {
	errorResults := ctx.SpecificationErrors
	if len(errorResults) == 0 {
		logger.GetGlobalLogger("shein/category").Debug("没有检测到规格配置错误信息")
		return nil
	}
	logger.GetGlobalLogger("shein/category").Infof("category restriction collection disabled, skipped %d specification errors", len(errorResults))
	return nil
}

// parseSpecificationError 解析规格配置错误信息
func (h *CollectCategoryRestrictionsHandler) parseSpecificationError(ctx *shein.TaskContext, messages []string) (int, string, int, string) {
	forbiddenAttrID := 0
	forbiddenAttrName := ""
	defaultAttrID := 27
	defaultAttrName := "颜色"

	for _, message := range messages {
		// 示例：解析"[尺寸]不可以作为主规格，请重新选择"
		// 或者"属性[123]名称不能作为主规格，建议使用属性[456]名称"

		// 匹配"[属性名称]不可以作为主规格"的模式
		re1 := regexp.MustCompile(`\[([^\]]+)\]不可以作为主规格`)
		matches1 := re1.FindStringSubmatch(message)
		if len(matches1) > 1 {
			forbiddenAttrName = matches1[1]
		}

		// 匹配"属性[123]名称不能作为主规格，建议使用属性[456]名称"的模式
		re2 := regexp.MustCompile(`属性\[(\d+)\]([^\s不能]+)不能作为主规格，建议使用属性\[(\d+)\]([^\s名称]+)名称`)
		matches2 := re2.FindStringSubmatch(message)
		if len(matches2) > 4 {
			forbiddenAttrID, _ = strconv.Atoi(matches2[1])
			forbiddenAttrName = matches2[2]
			defaultAttrID, _ = strconv.Atoi(matches2[3])
			defaultAttrName = matches2[4]
		}

		// 匹配"属性[123]不能作为主规格，建议使用属性[456]"的模式
		re3 := regexp.MustCompile(`属性\[(\d+)\]不能作为主规格，建议使用属性\[(\d+)\]`)
		matches3 := re3.FindStringSubmatch(message)
		if len(matches3) > 2 {
			forbiddenAttrID, _ = strconv.Atoi(matches3[1])
			defaultAttrID, _ = strconv.Atoi(matches3[2])
		}

		// 匹配"属性名称不能作为主规格，建议使用其他属性"的模式
		re4 := regexp.MustCompile(`([^\s不能]+)不能作为主规格，建议使用([^\s属性]+)属性`)
		matches4 := re4.FindStringSubmatch(message)
		if len(matches4) > 2 {
			forbiddenAttrName = matches4[1]
			defaultAttrName = matches4[2]
		}
	}

	if forbiddenAttrID == 0 {
		if len(ctx.ProductData.SKCList) == 0 {
			logger.GetGlobalLogger("shein/category").Warnf("没有找到主规格信息")
			return 0, "", 0, ""
		}
		forbiddenAttrID = ctx.ProductData.SKCList[0].SaleAttribute.AttributeID
	}

	return forbiddenAttrID, forbiddenAttrName, defaultAttrID, defaultAttrName
}

// hasMainSpecificationError 检查是否存在主规格相关的错误
func (h *CollectCategoryRestrictionsHandler) hasMainSpecificationError(ctx *shein.TaskContext) bool {
	// 检查规格配置错误信息
	errorResults := ctx.SpecificationErrors

	// 如果没有规格配置错误信息，则没有主规格错误
	if len(errorResults) == 0 {
		logger.GetGlobalLogger("shein/category").Debug("没有检测到规格配置错误信息")
		return false
	}

	// 检查是否存在主规格相关的错误
	for _, result := range errorResults {
		if result.Module == "specification_info" && result.Form == "main_specification" {
			logger.GetGlobalLogger("shein/category").Warnf("发现主规格配置错误: Module=%s, Form=%s, Messages=%v",
				result.Module, result.Form, result.Messages)
			return true
		}
	}

	logger.GetGlobalLogger("shein/category").Debug("没有发现主规格相关的错误")
	return false
}
