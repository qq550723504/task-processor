// Package utils 提供Amazon产品标识符生成工具
package utils

import "fmt"

// GenerateUPC 为测试目的生成一个有效的UPC码
func GenerateUPC(sku string) string {
	// 使用SKU的哈希值生成一个测试用的UPC码
	// 注意：在生产环境中应该使用真实的UPC码
	hash := 0
	for _, char := range sku {
		hash = (hash*31 + int(char)) % 100000000000
	}

	// 确保是11位数字，然后计算校验位
	base := fmt.Sprintf("%011d", hash)
	checksum := calculateUPCChecksum(base)

	return base + fmt.Sprintf("%d", checksum)
}

// calculateUPCChecksum 计算UPC校验位
func calculateUPCChecksum(base string) int {
	sum := 0
	for i, char := range base {
		digit := int(char - '0')
		if i%2 == 0 {
			sum += digit * 3
		} else {
			sum += digit
		}
	}
	return (10 - (sum % 10)) % 10
}

// GenerateEAN13 为测试目的生成一个有效的EAN13码
func GenerateEAN13(sku string) string {
	// 使用SKU的哈希值生成一个测试用的EAN13码
	// 注意：在生产环境中应该使用真实的EAN13码
	hash := 0
	for _, char := range sku {
		hash = (hash*31 + int(char)) % 1000000000000
	}

	// 确保是12位数字，然后计算校验位
	base := fmt.Sprintf("%012d", hash)
	checksum := calculateEAN13Checksum(base)

	return base + fmt.Sprintf("%d", checksum)
}

// calculateEAN13Checksum 计算EAN13校验位
func calculateEAN13Checksum(base string) int {
	sum := 0
	for i, char := range base {
		digit := int(char - '0')
		if i%2 == 0 {
			sum += digit
		} else {
			sum += digit * 3
		}
	}
	return (10 - (sum % 10)) % 10
}
