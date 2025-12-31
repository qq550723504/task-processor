// Package utils 提供工具方法
package utils

import (
	"fmt"
)

// SimpleHash 简单的哈希函数（统一实现）
func SimpleHash(key string, totalShards int) int {
	hash := 0
	// 使用更大的质数97作为乘数，以获得更好的分布
	for _, char := range key {
		hash = (hash*97 + int(char)) % totalShards
	}
	if hash < 0 {
		hash = -hash
	}
	return hash
}

// GetPodForTenantShop 计算指定租户和店铺应该分配到哪个Pod
func GetPodForTenantShop(tenantID, shopID string, totalShards int) int {
	// 使用相同的哈希算法将租户和店铺映射到分片
	key := fmt.Sprintf("%s:%s", tenantID, shopID)
	return SimpleHash(key, totalShards)
}

// HashString 对字符串进行哈希
func HashString(s string) uint32 {
	hash := uint32(0)
	for _, c := range s {
		hash = hash*31 + uint32(c)
	}
	return hash
}

// ConsistentHash 一致性哈希
func ConsistentHash(key string, nodes []string) string {
	if len(nodes) == 0 {
		return ""
	}

	hash := HashString(key)
	index := int(hash) % len(nodes)
	return nodes[index]
}
