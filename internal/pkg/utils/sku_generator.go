package utils

import (
	"crypto/md5"
	"fmt"
	"math/rand"
	"strconv"
	"sync"
	"time"
)

// SKU生成策略常量
const (
	StrategyASINOnly  = 0 // 仅使用ASIN
	StrategyRandom    = 1 // ASIN+随机数
	StrategyTimestamp = 2 // ASIN+时间戳
	StrategyHash      = 3 // ASIN+哈希
)

var (
	// 使用独立的随机数生成器，避免全局rand的竞争
	rng  *rand.Rand
	once sync.Once
)

// initRand 初始化随机数生成器（只执行一次）
func initRand() {
	once.Do(func() {
		rng = rand.New(rand.NewSource(time.Now().UnixNano()))
	})
}

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
		// 初始化随机数生成器
		initRand()
		// 生成6位随机数（000000-999999）
		random := rng.Intn(1000000)
		sku = fmt.Sprintf("%s%06d", asin, random)
	case StrategyTimestamp:
		// 使用时间戳（取后10位，避免过长）
		timestamp := time.Now().Unix()
		sku = asin + strconv.FormatInt(timestamp%10000000000, 10)
	case StrategyHash:
		// 使用MD5哈希，对于相同ASIN生成相同哈希值
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
