// Package browser 提供Amazon浏览器自动化的邮编验证功能
package browser

import (
	"fmt"

	"github.com/playwright-community/playwright-go"
)

// ZipcodeValidator 邮编验证器
type ZipcodeValidator struct {
	getter *ZipcodeGetter
}

// NewZipcodeValidator 创建邮编验证器实例
func NewZipcodeValidator() *ZipcodeValidator {
	return &ZipcodeValidator{
		getter: NewZipcodeGetter(),
	}
}

// VerifyZipcode 验证邮编是否设置成功
func (zv *ZipcodeValidator) VerifyZipcode(page playwright.Page, expectedZipcode string) (bool, error) {
	// 获取当前邮编并验证
	currentZipcode, err := zv.getter.GetCurrentZipcode(page)
	if err != nil {
		return false, fmt.Errorf("获取当前邮编失败: %w", err)
	}

	// 直接匹配邮编
	if currentZipcode == expectedZipcode {
		return true, nil
	}

	// 对于某些站点(如沙特),页面显示的是城市名称而非邮编
	// 尝试将邮编映射到城市名称进行验证
	expectedCity := mapZipcodeToCity(expectedZipcode)
	if expectedCity != "" && currentZipcode == expectedCity {
		return true, nil
	}

	return false, nil
}

// mapZipcodeToCity 将邮编映射到城市名称(用于验证)
func mapZipcodeToCity(zipcode string) string {
	// 沙特城市映射
	saudiCityMap := map[string]string{
		"11564": "Riyadh",   // 利雅得
		"21432": "Jeddah",   // 吉达
		"23218": "Dammam",   // 达曼
		"31952": "Mecca",    // 麦加
		"24231": "Medina",   // 麦地那
		"32272": "Khobar",   // 胡拜尔
		"13521": "Buraidah", // 布赖代
		"51431": "Abha",     // 艾卜哈
		"82723": "Tabuk",    // 塔布克
		"41311": "Hail",     // 哈伊勒
	}

	// 阿联酋城市映射
	uaeCityMap := map[string]string{
		"00000": "Dubai",     // 迪拜
		"00001": "Abu Dhabi", // 阿布扎比
		"00002": "Sharjah",   // 沙迦
		"00003": "Ajman",     // 阿治曼
	}

	// 先尝试沙特映射
	if city, exists := saudiCityMap[zipcode]; exists {
		return city
	}

	// 再尝试阿联酋映射
	if city, exists := uaeCityMap[zipcode]; exists {
		return city
	}

	return ""
}
