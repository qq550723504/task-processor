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

	tempIDCount := s.countTemporarySpecIDs(aiMapping)
	resolvedCount := 0
	failedCount := 0

	s.logger.Infof("🔍 发现 %d 个临时规格ID需要解析", tempIDCount)

	var resolveErr error
	aiMapping.ForEachSKUIndexed(func(i int, sku *temucontext.AIGeneratedSku) {
		if resolveErr != nil {
			return
		}

		for j := range sku.Spec {
			spec := &sku.Spec[j]

			if strings.HasPrefix(spec.SpecID, "TEMP_") {
				s.logger.Infof("Resolving temporary spec ID: %s -> %s/%s", spec.SpecID, spec.ParentSpecName, spec.SpecName)

				realSpecID, err := s.apiClient.QuerySpecID(runtime, spec.ParentSpecID, spec.SpecName)
				if err != nil {
					s.logger.Errorf("Spec lookup failed [%s/%s]: %v", spec.ParentSpecName, spec.SpecName, err)
					resolveErr = fmt.Errorf("spec lookup failed [%s/%s]: %w", spec.ParentSpecName, spec.SpecName, err)
					return
				}

				s.logger.Infof("Resolved spec ID: %s -> %s", spec.SpecID, realSpecID)
				spec.SpecID = realSpecID
				resolvedCount++
			}
		}

		s.refreshResolvedSKUIdentifiers(i, sku)
	})
	if resolveErr != nil {
		return resolveErr
	}

	s.logger.Infof("✅ 临时规格ID解析完成: 总计=%d, 成功=%d, 失败=%d", tempIDCount, resolvedCount, failedCount)
	return nil
}

func (s *SpecResolverService) countTemporarySpecIDs(aiMapping *temucontext.AISkuMappingResponse) int {
	tempIDCount := 0
	aiMapping.ForEachSKU(func(sku *temucontext.AIGeneratedSku) {
		for _, spec := range sku.Spec {
			if strings.HasPrefix(spec.SpecID, "TEMP_") {
				tempIDCount++
			}
		}
	})

	return tempIDCount
}

func (s *SpecResolverService) refreshResolvedSKUIdentifiers(index int, sku *temucontext.AIGeneratedSku) {
	if len(sku.Spec) >= 2 {
		sku.UniqueID = fmt.Sprintf("%s_%s", sku.Spec[0].SpecID, sku.Spec[1].SpecID)
	} else if len(sku.Spec) == 1 {
		sku.UniqueID = sku.Spec[0].SpecID
	}

	if len(sku.Spec) > 0 {
		sku.SpecID = sku.Spec[len(sku.Spec)-1].SpecID
		s.logger.Debugf("SKU[%d] set spec_id: %s", index, sku.SpecID)
	}
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
