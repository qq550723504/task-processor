# Timestamp空字符串解析错误修复（最终方案）

## 问题描述

从服务器获取的历史产品数据中，`timestamp` 字段为空字符串 `""`，导致JSON解析失败：

```
WARN 解析为单个对象失败: parsing time "" as "2006-01-02T15:04:05Z07:00": cannot parse "" as "2006"
INFO JSON对象包含的字段: [title asin final_price ... timestamp ...]
INFO title字段存在，值类型: string
INFO asin字段存在，值: B0C4WXBBSY
INFO final_price字段存在，值: 6.99, 类型: float64
ERRO 解析为数组也失败: json: cannot unmarshal object into Go value of type []amazon.Product
```

**问题原因：**
- 服务器返回的JSON中 `timestamp` 字段为空字符串 `""`
- Go的 `time.Time` 类型无法解析空字符串
- 导致整个产品对象解析失败

## 根本原因

**重要发现：** 即使将 `Timestamp` 改为 `*time.Time`，空字符串 `""` 仍然会导致解析失败！

这是因为：
1. JSON解析器看到 `"timestamp": ""` 时
2. 识别出字段类型是 `*time.Time`
3. 尝试将空字符串解析为时间
4. 解析失败：`parsing time "" as "2006-01-02T15:04:05Z07:00": cannot parse "" as "2006"`

**正确的空值应该是：** `"timestamp": null` 或完全省略该字段

## 解决方案

### 方案1: 使用 `*time.Time`（失败❌）

```go
Timestamp *time.Time `json:"timestamp,omitempty"`
```

**问题：** 空字符串 `""` 仍然会导致解析失败

### 方案2: 自定义 `NullableTime` 类型（成功✅）

#### 1. 创建 `NullableTime` 类型

在 `common/amazon/models.go` 中添加：

```go
// NullableTime 可空时间类型，支持空字符串解析为nil
type NullableTime struct {
	*time.Time
}

// UnmarshalJSON 自定义JSON解析，将空字符串解析为nil
func (nt *NullableTime) UnmarshalJSON(data []byte) error {
	// 去除引号
	str := string(data)
	if str == "null" || str == `""` || str == "" {
		nt.Time = nil
		return nil
	}

	// 解析时间
	var t time.Time
	if err := json.Unmarshal(data, &t); err != nil {
		return err
	}
	nt.Time = &t
	return nil
}

// MarshalJSON 自定义JSON序列化
func (nt NullableTime) MarshalJSON() ([]byte, error) {
	if nt.Time == nil {
		return []byte("null"), nil
	}
	return json.Marshal(*nt.Time)
}
```

#### 2. 修改 `Product` 结构体

**修改前：**
```go
type Product struct {
    // ...
    Timestamp time.Time `json:"timestamp"`
    // ...
}
```

**修改后：**
```go
type Product struct {
    // ...
    Timestamp NullableTime `json:"timestamp,omitempty"` // 使用NullableTime支持空字符串
    // ...
}
```

**优势：**
- ✅ 空字符串 `""` 会被解析为 `nil`
- ✅ `null` 会被解析为 `nil`
- ✅ 有效时间字符串正常解析
- ✅ 序列化时 `nil` 输出为 `null`

#### 3. 修改 `common/amazon/processor.go`

更新所有设置 `Timestamp` 的代码：

**修改前：**
```go
product := &Product{
    URL:       url,
    Zipcode:   zipcode,
    Asin:      ap.extractASINFromURL(url),
    Currency:  ap.getCurrencyFromURL(url),
    Timestamp: time.Now(),
}
```

**修改后：**
```go
now := time.Now()
product := &Product{
    URL:       url,
    Zipcode:   zipcode,
    Asin:      ap.extractASINFromURL(url),
    Currency:  ap.getCurrencyFromURL(url),
    Timestamp: NullableTime{Time: &now},
}
```

**说明：**
- 创建 `NullableTime` 实例
- 将 `time.Now()` 的地址赋值给 `Time` 字段

## 效果对比

### 修改前
```json
// 服务器返回的JSON
{
    "title": "Product Title",
    "asin": "B0C4WXBBSY",
    "final_price": 6.99,
    "timestamp": ""  // 空字符串
}
```
```
❌ 解析失败: parsing time "" as "2006-01-02T15:04:05Z07:00": cannot parse "" as "2006"
```

### 修改后
```json
// 服务器返回的JSON
{
    "title": "Product Title",
    "asin": "B0C4WXBBSY",
    "final_price": 6.99,
    "timestamp": ""  // 空字符串
}
```
```
✅ 解析成功: product.Timestamp = nil
```

## 使用注意事项

### 1. 检查 Timestamp 是否为 nil

**修改前：**
```go
fmt.Printf("时间: %v", product.Timestamp)
```

**修改后：**
```go
if product.Timestamp.Time != nil {
    fmt.Printf("时间: %v", *product.Timestamp.Time)
} else {
    fmt.Println("时间: 未设置")
}
```

### 2. 设置 Timestamp

**修改前：**
```go
product.Timestamp = time.Now()
```

**修改后：**
```go
now := time.Now()
product.Timestamp = NullableTime{Time: &now}
```

### 3. 比较 Timestamp

**修改前：**
```go
if product.Timestamp.After(someTime) {
    // ...
}
```

**修改后：**
```go
if product.Timestamp.Time != nil && product.Timestamp.Time.After(someTime) {
    // ...
}
```

## 其他可能需要空值支持的字段

如果遇到类似问题，可以考虑将以下字段也改为指针类型：

```go
type Product struct {
    // 数值类型（区分0和未设置）
    InitialPrice    *float64 `json:"initial_price,omitempty"`
    FinalPrice      *float64 `json:"final_price,omitempty"`
    Rating          *float64 `json:"rating,omitempty"`
    ReviewsCount    *int     `json:"reviews_count,omitempty"`
    
    // 布尔类型（区分false和未设置）
    IsAvailable     *bool    `json:"is_available,omitempty"`
    Video           *bool    `json:"video,omitempty"`
    
    // 时间类型
    Timestamp       *time.Time `json:"timestamp,omitempty"`
}
```

## 相关问题

如果遇到其他字段的类似错误：
1. 检查日志中的字段类型
2. 确认服务器返回的实际值
3. 考虑使用指针类型或自定义 `UnmarshalJSON` 方法

## 测试建议

测试不同的timestamp值：
```go
// 测试用例
testCases := []string{
    `{"timestamp": ""}`,                           // 空字符串 → nil
    `{"timestamp": "2024-01-01T00:00:00Z"}`,      // 有效时间 → 解析成功
    `{}`,                                          // 缺少字段 → nil
}
```
