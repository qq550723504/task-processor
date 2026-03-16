// Package helpers 提供配置辅助工具
package helpers

import (
	"encoding/json"
	"fmt"
)

// MergeConfigs 合并两个配置(后者覆盖前者)
func MergeConfigs(base, override any) error {
	// 将base序列化为JSON
	baseData, err := json.Marshal(base)
	if err != nil {
		return fmt.Errorf("序列化基础配置失败: %w", err)
	}

	// 将override序列化为JSON
	overrideData, err := json.Marshal(override)
	if err != nil {
		return fmt.Errorf("序列化覆盖配置失败: %w", err)
	}

	// 合并JSON
	var baseMap, overrideMap map[string]any
	if err2 := json.Unmarshal(baseData, &baseMap); err2 != nil {
		return fmt.Errorf("解析基础配置失败: %w", err2)
	}
	if err2 := json.Unmarshal(overrideData, &overrideMap); err2 != nil {
		return fmt.Errorf("解析覆盖配置失败: %w", err2)
	}

	// 递归合并
	mergedMap := MergeMaps(baseMap, overrideMap)

	// 将合并后的结果反序列化回base
	mergedData, err := json.Marshal(mergedMap)
	if err != nil {
		return fmt.Errorf("序列化合并配置失败: %w", err)
	}

	if err2 := json.Unmarshal(mergedData, base); err2 != nil {
		return fmt.Errorf("反序列化合并配置失败: %w", err2)
	}

	return nil
}

// MergeMaps 递归合并两个map
func MergeMaps(base, override map[string]any) map[string]any {
	result := make(map[string]any)

	// 复制base
	for k, v := range base {
		result[k] = v
	}

	// 覆盖或合并override
	for k, v := range override {
		if baseVal, exists := result[k]; exists {
			// 如果两者都是map,递归合并
			if baseMap, ok := baseVal.(map[string]any); ok {
				if overrideMap, ok := v.(map[string]any); ok {
					result[k] = MergeMaps(baseMap, overrideMap)
					continue
				}
			}
		}
		// 否则直接覆盖
		result[k] = v
	}

	return result
}
