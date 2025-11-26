# Amazon 多站点价格提取器使用指南

## 概述

`PriceExtractor` 现已支持 Amazon 全球多个站点的价格提取，能够正确识别不同国家的货币符号、价格格式和语言。

## 支持的站点

### 美洲
- 🇺🇸 美国 (US/com) - USD ($)
- 🇨🇦 加拿大 (CA/ca) - CAD (C$)
- 🇲🇽 墨西哥 (MX/com.mx) - MXN ($)
- 🇧🇷 巴西 (BR/com.br) - BRL (R$)

### 欧洲
- 🇬🇧 英国 (UK/co.uk) - GBP (£)
- 🇩🇪 德国 (DE/de) - EUR (€)
- 🇫🇷 法国 (FR/fr) - EUR (€)
- 🇮🇹 意大利 (IT/it) - EUR (€)
- 🇪🇸 西班牙 (ES/es) - EUR (€)
- 🇵🇱 波兰 (PL/pl) - PLN (zł)
- 🇸🇪 瑞典 (SE/se) - SEK (kr)
- 🇹🇷 土耳其 (TR/com.tr) - TRY (₺)

### 亚太
- 🇯🇵 日本 (JP/co.jp) - JPY (¥)
- 🇨🇳 中国 (CN/cn) - CNY (¥)
- 🇦🇺 澳大利亚 (AU/com.au) - AUD (A$)
- 🇮🇳 印度 (IN/in) - INR (₹)
- 🇸🇬 新加坡 (SG/sg) - SGD (S$)
- 🇦🇪 阿联酋 (AE/ae) - AED (د.إ)

## 使用方法

```go
import (
    "github.com/playwright-community/playwright-go"
    "your-project/common/amazon"
)

// 创建提取器并指定站点
extractor := &amazon.PriceExtractor{
    Marketplace: "JP", // 日本站
}

// 或者
extractor := &amazon.PriceExtractor{
    Marketplace: "de", // 德国站
}

// 检查是否有有效价格
if extractor.HasValidPrice(page) {
    // 提取价格
    err := extractor.Extract(page, product)
    if err != nil {
        log.Error(err)
    }
}
```

## 关键特性

### 1. 货币符号自动识别

提取器能识别以下货币符号：
- `$` (美元、加元、澳元等，根据站点区分)
- `€` (欧元)
- `£` (英镑)
- `¥` (日元/人民币，根据站点区分)
- `₹` (印度卢比)
- `R$` (巴西雷亚尔)
- `kr` (瑞典克朗)
- `zł` (波兰兹罗提)
- 以及更多...

### 2. 价格格式自动适配

#### 美国/英国格式
```
$1,234.56  → 1234.56 USD
£999.99    → 999.99 GBP
```

#### 欧洲格式（德国、法国、意大利、西班牙）
```
1.234,56 € → 1234.56 EUR
999,99 €   → 999.99 EUR
```

#### 日本格式（通常无小数）
```
¥1,234     → 1234 JPY
¥999       → 999 JPY
```

### 3. 多语言不可用状态检测

提取器能识别多种语言的"不可用"状态：

- 英语: "Currently unavailable", "Out of stock"
- 日语: "在庫切れ", "取り扱い終了"
- 德语: "Derzeit nicht verfügbar"
- 法语: "Actuellement indisponible"
- 西班牙语: "Actualmente no disponible"
- 意大利语: "Attualmente non disponibile"

## 站点配置示例

### 示例 1: 日本站点
```go
extractor := &amazon.PriceExtractor{
    Marketplace: "JP",
}
// 自动识别: ¥ → JPY
// 价格格式: ¥1,234 (无小数)
```

### 示例 2: 德国站点
```go
extractor := &amazon.PriceExtractor{
    Marketplace: "DE",
}
// 自动识别: € → EUR
// 价格格式: 1.234,56 € (逗号为小数分隔符)
```

### 示例 3: 加拿大站点
```go
extractor := &amazon.PriceExtractor{
    Marketplace: "CA",
}
// 自动识别: $ 或 C$ → CAD
// 价格格式: C$1,234.56
```

## 注意事项

1. **必须设置 Marketplace 字段**
   ```go
   // ❌ 错误 - 未设置站点
   extractor := &amazon.PriceExtractor{}
   
   // ✅ 正确
   extractor := &amazon.PriceExtractor{
       Marketplace: "JP",
   }
   ```

2. **站点代码格式**
   - 支持大写: `"US"`, `"JP"`, `"DE"`
   - 支持小写: `"us"`, `"jp"`, `"de"`
   - 支持域名后缀: `"com"`, `"co.jp"`, `"co.uk"`

3. **货币符号冲突处理**
   - `¥` 符号会根据站点区分 JPY 和 CNY
   - `$` 符号会根据站点区分 USD、CAD、AUD、SGD 等

4. **价格格式检测**
   - 自动检测欧洲格式 (1.234,56) vs 美国格式 (1,234.56)
   - 基于逗号和点号的位置智能判断

## 测试建议

```go
// 测试不同站点
marketplaces := []string{"US", "JP", "DE", "UK", "FR"}

for _, marketplace := range marketplaces {
    extractor := &amazon.PriceExtractor{
        Marketplace: marketplace,
    }
    
    // 测试价格提取
    err := extractor.Extract(page, product)
    
    log.Printf("站点: %s, 货币: %s, 价格: %.2f", 
        marketplace, product.Currency, product.FinalPrice)
}
```

## 常见问题

### Q: 如何处理未知站点？
A: 如果站点未在映射表中，会默认使用 USD 作为货币。

### Q: 日本站的价格为什么没有小数？
A: 日元通常不使用小数，提取器会自动处理这种情况。

### Q: 欧洲站点的价格解析失败？
A: 确保 Marketplace 字段设置正确，提取器会自动使用逗号作为小数分隔符。

### Q: 如何添加新的站点支持？
A: 在 `getDefaultCurrencyByMarketplace()` 和 `getDefaultCurrencySymbol()` 方法中添加新的站点映射。
