// Package spec 提供TEMU平台的规格解析服务
package spec

import (
	"fmt"
	"strings"
	temucontext "task-processor/internal/temu/context"

	"github.com/sirupsen/logrus"
	"task-processor/internal/core/logger"
)

// SpecResolverService 规格解析服务
type SpecResolverService struct {
	logger    *logrus.Entry
	apiClient SpecQueryAPI
}

// SpecQueryAPI 规格查询API接口
type SpecQueryAPI interface {
	QuerySpecID(runtime *ResolveSpecRuntimeInput, parentSpecID, specName string) (string, error)
}

// NewSpecResolverService 创建规格解析服务
func NewSpecResolverService(apiClient SpecQueryAPI) *SpecResolverService {
	return &SpecResolverService{
		logger:    logger.GetGlobalLogger("SpecResolver"),
		apiClient: apiClient,
	}
}

// ResolveTemporarySpecIDs 解析AI映射中的临时规格ID
func (s *SpecResolverService) ResolveTemporarySpecIDs(runtime *ResolveSpecRuntimeInput, aiMapping *temucontext.AISkuMappingResponse) error {
	s.logger.Info("🔍 开始解析临时规格ID")

	tempIDCount := 0
	resolvedCount := 0
	failedCount := 0

	// 统计临时ID数量
	aiMapping.ForEachSKU(func(sku *temucontext.AIGeneratedSku) {
		for _, spec := range sku.Spec {
			if strings.HasPrefix(spec.SpecID, "TEMP_") {
				tempIDCount++
			}
		}
	})

	s.logger.Infof("🔍 发现 %d 个临时规格ID需要解析", tempIDCount)

	for i := range aiMapping.SkuList {
		sku := &aiMapping.SkuList[i]

		// 解析每个规格的临时ID
		for j := range sku.Spec {
			spec := &sku.Spec[j]

			// 检查是否为临时ID
			if strings.HasPrefix(spec.SpecID, "TEMP_") {
				s.logger.Infof("🔍 解析临时规格ID: %s -> %s/%s", spec.SpecID, spec.ParentSpecName, spec.SpecName)

				// 调用规格查询API获取真实的spec_id
				realSpecID, err := s.apiClient.QuerySpecID(runtime, spec.ParentSpecID, spec.SpecName)
				if err != nil {
					s.logger.Errorf("❌ 规格查询失败 [%s/%s]: %v", spec.ParentSpecName, spec.SpecName, err)
					return fmt.Errorf("规格查询失败 [%s/%s]: %w", spec.ParentSpecName, spec.SpecName, err)
				}

				s.logger.Infof("✅ 成功解析规格ID: %s -> %s", spec.SpecID, realSpecID)
				spec.SpecID = realSpecID
				resolvedCount++
			}
		}

		// 重新生成unique_id（因为spec_id可能已更改）
		if len(sku.Spec) >= 2 {
			sku.UniqueID = fmt.Sprintf("%s_%s", sku.Spec[0].SpecID, sku.Spec[1].SpecID)
		} else if len(sku.Spec) == 1 {
			sku.UniqueID = sku.Spec[0].SpecID
		}

		// 更新spec_id - 使用最后一个规格作为主要spec_id
		if len(sku.Spec) > 0 {
			sku.SpecID = sku.Spec[len(sku.Spec)-1].SpecID
			s.logger.Debugf("🔄 SKU[%d] 设置spec_id: %s", i, sku.SpecID)
		}
	}

	s.logger.Infof("✅ 临时规格ID解析完成: 总计=%d, 成功=%d, 失败=%d", tempIDCount, resolvedCount, failedCount)
	return nil
}

// HasTemporaryIDs 检查是否还有未解析的临时规格ID
func (s *SpecResolverService) HasTemporaryIDs(aiMapping *temucontext.AISkuMappingResponse) bool {
	hasTemporaryIDs := false
	aiMapping.ForEachSKU(func(sku *temucontext.AIGeneratedSku) {
		if hasTemporaryIDs {
			return
		}
		for _, spec := range sku.Spec {
			if strings.HasPrefix(spec.SpecID, "TEMP_") || strings.HasPrefix(spec.ParentSpecID, "TEMP_") {
				hasTemporaryIDs = true
				return
			}
		}
	})
	return hasTemporaryIDs
}
