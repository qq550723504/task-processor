// Package skugen 提供 SKU 生成工具
package skugen

import (
	"crypto/md5"
	"fmt"
	"math/rand"
	"strconv"
	"sync"
	"time"
)

// 生成策略常量
const (
	StrategyASINOnly  = 0 // 仅使用 ASIN
	StrategyRandom    = 1 // ASIN + 随机数
	StrategyTimestamp = 2 // ASIN + 时间戳
	StrategyHash      = 3 // ASIN + MD5 哈希
)

var (
	rng  *rand.Rand
	once sync.Once
)

func initRand() {
	once.Do(func() {
		rng = rand.New(rand.NewSource(time.Now().UnixNano()))
	})
}

// Generate 根据策略生成 SKU
func Generate(asin string, strategy int, prefix, suffix string) string {
	return GenerateByStrategy(asin, strategy, prefix, suffix)
}

// GenerateByStrategy 根据指定策略生成 SKU
func GenerateByStrategy(asin string, strategy int, prefix, suffix string) string {
	var sku string

	switch strategy {
	case StrategyASINOnly:
		sku = asin
	case StrategyRandom:
		initRand()
		sku = fmt.Sprintf("%s%06d", asin, rng.Intn(1000000))
	case StrategyTimestamp:
		sku = asin + strconv.FormatInt(time.Now().Unix()%10000000000, 10)
	case StrategyHash:
		hash := md5.Sum([]byte(asin))
		sku = fmt.Sprintf("%x", hash)[:8]
	default:
		sku = asin
	}

	if prefix != "" {
		sku = prefix + sku
	}
	if suffix != "" {
		sku = sku + suffix
	}

	return sku
}
