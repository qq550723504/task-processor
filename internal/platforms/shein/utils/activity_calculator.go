// Package utils 提供活动相关的计算工具方法
package utils

// CalculateActivityStock 计算活动库存（默认为总库存的80%）
func CalculateActivityStock(totalStock int) int {
	if totalStock <= 0 {
		return 0
	}
	actStock := int(float64(totalStock) * 0.8)
	return max(actStock, 1)
}

// GetDefaultDropRate 获取默认降价幅度（20%）
func GetDefaultDropRate() int {
	return 20
}

// CalculateReservedStock 计算预留库存（默认为总库存的10%）
func CalculateReservedStock(totalStock int) int {
	if totalStock <= 0 {
		return 0
	}
	reservedStock := int(float64(totalStock) * 0.1)
	return max(reservedStock, 1)
}
