// Package utils 提供Amazon产品标识符生成工具
package utils

import (
	"crypto/md5"
	"fmt"
	"strconv"
	"strings"
)

// IdentifierGenerator 标识符生成器
type IdentifierGenerator struct{}

// NewIdentifierGenerator 创建标识符生成器
func NewIdentifierGenerator() *IdentifierGenerator {
	return &IdentifierGenerator{}
}

// GenerateUPC 为测试目的生成一个有效的UPC码
func (g *IdentifierGenerator) GenerateUPC(sku string) string {
	// 使用SKU的MD5哈希值生成一个测试用的UPC码
	// 注意：在生产环境中应该使用真实的UPC码
	hash := md5.Sum([]byte(sku))
	hashStr := fmt.Sprintf("%x", hash)

	// 取前11位数字
	var digits []rune
	for _, char := range hashStr {
		if len(digits) >= 11 {
			break
		}
		if char >= '0' && char <= '9' {
			digits = append(digits, char)
		} else {
			// 将字母转换为数字
			digits = append(digits, rune('0'+int(char-'a')%10))
		}
	}

	// 补齐到11位
	for len(digits) < 11 {
		digits = append(digits, '0')
	}

	base := string(digits)
	checksum := g.calculateUPCChecksum(base)

	return base + fmt.Sprintf("%d", checksum)
}

// calculateUPCChecksum 计算UPC校验位
func (g *IdentifierGenerator) calculateUPCChecksum(base string) int {
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
func (g *IdentifierGenerator) GenerateEAN13(sku string) string {
	// 使用SKU的MD5哈希值生成一个测试用的EAN13码
	// 注意：在生产环境中应该使用真实的EAN13码
	hash := md5.Sum([]byte(sku))
	hashStr := fmt.Sprintf("%x", hash)

	// 取前12位数字
	var digits []rune
	for _, char := range hashStr {
		if len(digits) >= 12 {
			break
		}
		if char >= '0' && char <= '9' {
			digits = append(digits, char)
		} else {
			// 将字母转换为数字
			digits = append(digits, rune('0'+int(char-'a')%10))
		}
	}

	// 补齐到12位
	for len(digits) < 12 {
		digits = append(digits, '0')
	}

	base := string(digits)
	checksum := g.calculateEAN13Checksum(base)

	return base + fmt.Sprintf("%d", checksum)
}

// calculateEAN13Checksum 计算EAN13校验位
func (g *IdentifierGenerator) calculateEAN13Checksum(base string) int {
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

// GenerateASIN 生成测试用的ASIN码
func (g *IdentifierGenerator) GenerateASIN(sku string) string {
	// ASIN是10位字符，包含字母和数字
	hash := md5.Sum([]byte("ASIN_" + sku))
	hashStr := fmt.Sprintf("%x", hash)

	// 取前10位，确保第一位是字母
	asin := "B" + strings.ToUpper(hashStr[:9])
	return asin
}

// GenerateSKU 生成标准化的SKU
func (g *IdentifierGenerator) GenerateSKU(prefix, productName string) string {
	// 清理产品名称
	cleanName := g.cleanProductName(productName)

	// 生成哈希后缀
	hash := md5.Sum([]byte(productName))
	hashStr := fmt.Sprintf("%x", hash)
	suffix := strings.ToUpper(hashStr[:6])

	// 组合SKU
	sku := fmt.Sprintf("%s-%s-%s", strings.ToUpper(prefix), cleanName, suffix)

	// 确保SKU长度不超过40个字符
	if len(sku) > 40 {
		sku = sku[:40]
	}

	return sku
}

// cleanProductName 清理产品名称用于SKU生成
func (g *IdentifierGenerator) cleanProductName(name string) string {
	// 转换为大写
	name = strings.ToUpper(name)

	// 移除特殊字符，只保留字母和数字
	var cleaned []rune
	for _, char := range name {
		if (char >= 'A' && char <= 'Z') || (char >= '0' && char <= '9') {
			cleaned = append(cleaned, char)
		}
	}

	result := string(cleaned)

	// 限制长度
	if len(result) > 15 {
		result = result[:15]
	}

	// 如果为空，使用默认值
	if result == "" {
		result = "PRODUCT"
	}

	return result
}

// ValidateUPC 验证UPC码是否有效
func (g *IdentifierGenerator) ValidateUPC(upc string) bool {
	if len(upc) != 12 {
		return false
	}

	// 检查是否全为数字
	for _, char := range upc {
		if char < '0' || char > '9' {
			return false
		}
	}

	// 验证校验位
	base := upc[:11]
	expectedChecksum := g.calculateUPCChecksum(base)
	actualChecksum, err := strconv.Atoi(string(upc[11]))
	if err != nil {
		return false
	}

	return expectedChecksum == actualChecksum
}

// ValidateEAN13 验证EAN13码是否有效
func (g *IdentifierGenerator) ValidateEAN13(ean13 string) bool {
	if len(ean13) != 13 {
		return false
	}

	// 检查是否全为数字
	for _, char := range ean13 {
		if char < '0' || char > '9' {
			return false
		}
	}

	// 验证校验位
	base := ean13[:12]
	expectedChecksum := g.calculateEAN13Checksum(base)
	actualChecksum, err := strconv.Atoi(string(ean13[12]))
	if err != nil {
		return false
	}

	return expectedChecksum == actualChecksum
}

// ValidateASIN 验证ASIN码是否有效
func (g *IdentifierGenerator) ValidateASIN(asin string) bool {
	if len(asin) != 10 {
		return false
	}

	// ASIN通常以B开头
	if asin[0] != 'B' {
		return false
	}

	// 检查其余字符是否为字母或数字
	for i := 1; i < len(asin); i++ {
		char := asin[i]
		if !((char >= 'A' && char <= 'Z') || (char >= '0' && char <= '9')) {
			return false
		}
	}

	return true
}

// GenerateBatchIdentifiers 批量生成标识符
func (g *IdentifierGenerator) GenerateBatchIdentifiers(skus []string) map[string]map[string]string {
	results := make(map[string]map[string]string)

	for _, sku := range skus {
		results[sku] = map[string]string{
			"upc":   g.GenerateUPC(sku),
			"ean13": g.GenerateEAN13(sku),
			"asin":  g.GenerateASIN(sku),
		}
	}

	return results
}
