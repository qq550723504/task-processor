// package hashx 提供哈希与分片工具方法
package hashx

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
)

// MD5 计算字符串的 MD5 哈希值，返回十六进制字符串
func MD5(s string) string {
	h := md5.Sum([]byte(s))
	return hex.EncodeToString(h[:])
}

// SimpleHash 简单哈希函数，将 key 映射到 [0, totalShards) 区间
func SimpleHash(key string, totalShards int) int {
	hash := 0
	for _, char := range key {
		hash = (hash*97 + int(char)) % totalShards
	}
	if hash < 0 {
		hash = -hash
	}
	return hash
}

// ShardForTenantShop 计算指定租户和店铺应分配到哪个分片
func ShardForTenantShop(tenantID, shopID string, totalShards int) int {
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

// ConsistentHash 一致性哈希，将 key 映射到 nodes 中的某个节点
func ConsistentHash(key string, nodes []string) string {
	if len(nodes) == 0 {
		return ""
	}
	hash := HashString(key)
	return nodes[int(hash)%len(nodes)]
}
