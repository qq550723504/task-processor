package common

import (
	"fmt"
	"os"

	"github.com/sirupsen/logrus"
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

// GetInstanceID 获取实例ID（统一实现）
func GetInstanceID() string {
	hostname, err := os.Hostname()
	if err != nil {
		return "unknown"
	}
	return hostname
}

// LogWithInstance 带实例ID的日志记录
func LogWithInstance(format string, args ...interface{}) {
	logrus.Infof("[实例%s] "+format, append([]interface{}{GetInstanceID()}, args...)...)
}

// GetPodForTenantShop 计算指定租户和店铺应该分配到哪个Pod
func GetPodForTenantShop(tenantID, shopID string, totalShards int) int {
	// 使用相同的哈希算法将租户和店铺映射到分片
	key := fmt.Sprintf("%s:%s", tenantID, shopID)
	return SimpleHash(key, totalShards)
}
