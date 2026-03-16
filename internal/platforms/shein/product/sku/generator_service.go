package sku

import (
	"crypto/md5"
	"fmt"
	"math/rand"
	"strconv"
	"time"
)

// SKU生成策略常量
const (
	StrategyASINOnly  = 0 // 仅使用ASIN
	StrategyRandom    = 1 // ASIN+随机数
	StrategyTimestamp = 2 // ASIN+时间戳
	StrategyHash      = 3 // ASIN+哈希
)

// GenerateSKU 根据不同策略生成SKU
func GenerateSKU(asin string, strategy int, prefix string, suffix string) string {
	switch strategy {
	case 0:
		return GenerateSKUByStrategy(asin, StrategyASINOnly, prefix, suffix)
	case 1:
		return GenerateSKUByStrategy(asin, StrategyRandom, prefix, suffix)
	case 2:
		return GenerateSKUByStrategy(asin, StrategyTimestamp, prefix, suffix)
	case 3:
		return GenerateSKUByStrategy(asin, StrategyHash, prefix, suffix)
	default:
		return asin
	}
}

// GenerateSKUByStrategy 根据指定策略生成SKU
func GenerateSKUByStrategy(asin string, strategy int, prefix string, suffix string) string {
	var sku string

	switch strategy {
	case StrategyASINOnly:
		sku = asin
	case StrategyRandom:
		// 生成随机数
		random := rand.Intn(999999)
		sku = asin + strconv.Itoa(random)
	case StrategyTimestamp:
		// 使用时间戳
		timestamp := time.Now().Unix()
		sku = asin + strconv.FormatInt(timestamp, 10)
	case StrategyHash:
		// 使用哈希，对于相同ASIN生成相同哈希值
		hash := md5.Sum([]byte(asin))
		// 取前8位作为哈希值
		hashStr := fmt.Sprintf("%x", hash)[:8]
		sku = hashStr
	default:
		sku = asin
	}

	// 添加前缀和后缀
	if prefix != "" {
		sku = prefix + sku
	}

	if suffix != "" {
		sku = sku + suffix
	}

	return sku
}
