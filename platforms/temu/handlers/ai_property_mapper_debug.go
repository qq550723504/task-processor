package handlers

import (
	"fmt"
	"task-processor/common/pipeline"

	"github.com/sirupsen/logrus"
)

// DebugPropertyMapping 调试属性映射，输出详细信息
func DebugPropertyMapping(ctx *pipeline.TaskContext, logger *logrus.Entry) {
	if logger == nil {
		logger = logrus.WithField("debug", "property_mapping")
	}

	logger.Info("========== 开始调试属性映射 ==========")

	// 获取模板信息
	templateInfo, exists := GetTemplateInfoFromContext(ctx)
	if !exists {
		logger.Error("❌ 未找到模板信息")
		return
	}

	logger.Infof("📋 模板ID: %d", templateInfo.TemplateID)
	logger.Infof("📋 模板属性总数: %d", len(templateInfo.GoodsProperties))

	// 统计必填属性
	requiredCount := 0
	optionalCount := 0
	for _, prop := range templateInfo.GoodsProperties {
		if prop.Required {
			requiredCount++
			logger.Infof("  ✅ 必填属性: %s (PID=%d, RefPID=%d, Type=%d, Values=%d个)",
				prop.Name, prop.PID, prop.RefPID, prop.PropertyValueType, len(prop.Values))

			// 如果是选择类型，显示前3个可选值
			if prop.PropertyValueType == 1 && len(prop.Values) > 0 {
				maxShow := 3
				if len(prop.Values) < maxShow {
					maxShow = len(prop.Values)
				}
				for i := 0; i < maxShow; i++ {
					logger.Infof("    - 可选值[%d]: %s (VID=%d)", i, prop.Values[i].Value, prop.Values[i].VID)
				}
				if len(prop.Values) > maxShow {
					logger.Infof("    - ... 还有 %d 个可选值", len(prop.Values)-maxShow)
				}
			}
		} else {
			optionalCount++
		}
	}

	logger.Infof("📊 必填属性: %d 个", requiredCount)
	logger.Infof("📊 可选属性: %d 个", optionalCount)

	// 检查产品数据
	if ctx.TemuProduct != nil {
		logger.Infof("📦 产品名称: %s", ctx.TemuProduct.GoodsBasic.GoodsName)
		logger.Infof("📦 已填充属性数量: %d", len(ctx.TemuProduct.GoodsExtensionInfo.GoodsProperty.GoodsProperties))

		if len(ctx.TemuProduct.GoodsExtensionInfo.GoodsProperty.GoodsProperties) > 0 {
			logger.Info("📝 已填充的属性列表:")
			for i, prop := range ctx.TemuProduct.GoodsExtensionInfo.GoodsProperty.GoodsProperties {
				logger.Infof("  [%d] PID=%d, VID=%d, Value=%s, RefPid=%d, TemplatePid=%d",
					i, prop.Pid, prop.Vid, prop.Value, prop.RefPid, prop.TemplatePid)
			}
		}

		// 检查哪些必填属性缺失（使用 PID+RefPID 匹配）
		filledMap := make(map[string]bool)
		for _, prop := range ctx.TemuProduct.GoodsExtensionInfo.GoodsProperty.GoodsProperties {
			key := fmt.Sprintf("%d_%d", prop.Pid, prop.RefPid)
			filledMap[key] = true
		}

		missingRequired := []string{}
		for _, templateProp := range templateInfo.GoodsProperties {
			if templateProp.Required {
				key := fmt.Sprintf("%d_%d", templateProp.PID, templateProp.RefPID)
				if !filledMap[key] {
					missingRequired = append(missingRequired, templateProp.Name)
					logger.Errorf("  ❌ 缺失必填属性: %s (PID=%d, RefPID=%d)", templateProp.Name, templateProp.PID, templateProp.RefPID)
				}
			}
		}

		if len(missingRequired) == 0 {
			logger.Info("✅ 所有必填属性都已填充")
		} else {
			logger.Errorf("❌ 缺失 %d 个必填属性: %v", len(missingRequired), missingRequired)
		}
	} else {
		logger.Error("❌ 产品数据为空")
	}

	logger.Info("========== 属性映射调试结束 ==========")
}
