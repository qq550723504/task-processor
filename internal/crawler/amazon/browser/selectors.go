// Package browser 提供浏览器相关的选择器工具
package browser

// GetContinueShoppingSelectors 获取"继续购物"按钮的多语言选择器
func GetContinueShoppingSelectors() []string {
	return []string{
		// 通用的Continue按钮（邮编设置后的确认按钮）
		"button:has-text('Continue'):not(:has-text('Cart')):not(:has-text('Buy'))",
		"input[type='submit']:has-text('Continue')",
		"span.a-button-text:has-text('Continue')",

		// 英语 - 只保留明确的Continue Shopping相关选择器
		"button:has-text('Continue Shopping')",
		"button:has-text('Continue shopping')",
		"a:has-text('Continue Shopping')",
		"a:has-text('Continue shopping')",
		// 中文
		"button:has-text('继续购物')",
		"a:has-text('继续购物')",
		// 日语
		"button:has-text('買い物を続ける')",
		"button:has-text('ショッピングを続ける')",
		"a:has-text('買い物を続ける')",
		"a:has-text('ショッピングを続ける')",
		"button:has-text('続ける')",
		"a:has-text('続ける')",
		// 西班牙语
		"button:has-text('Seguir comprando')",
		"button:has-text('Continuar comprando')",
		"a:has-text('Seguir comprando')",
		"a:has-text('Continuar comprando')",
		"button:has-text('Continuar')",
		"a:has-text('Continuar')",
		// 阿拉伯语
		"button:has-text('متابعة التسوق')",
		"a:has-text('متابعة التسوق')",
		// 德语
		"button:has-text('Weiter einkaufen')",
		"a:has-text('Weiter einkaufen')",
		"button:has-text('Weiter')",
		"a:has-text('Weiter')",
		// 法语
		"button:has-text('Continuer mes achats')",
		"a:has-text('Continuer mes achats')",
		"button:has-text('Continuer')",
		"a:has-text('Continuer')",
		// 意大利语
		"button:has-text('Continua lo shopping')",
		"a:has-text('Continua lo shopping')",
		"button:has-text('Continua')",
		"a:has-text('Continua')",
		// 葡萄牙语
		"button:has-text('Continuar comprando')",
		"a:has-text('Continuar comprando')",
		"button:has-text('Continuar')",
		"a:has-text('Continuar')",
		// 荷兰语
		"button:has-text('Verder winkelen')",
		"a:has-text('Verder winkelen')",
		"button:has-text('Verder')",
		"a:has-text('Verder')",
	}
}
