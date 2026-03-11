# Amazon货币设置功能调查结果

## 问题描述
Amazon爬虫中的货币设置功能存在问题,无法正确设置目标货币。

## 调查过程

### 1. 实际页面结构分析
通过Chrome DevTools实时调试Amazon UK货币设置页面,发现:

- **货币设置页面URL**: `https://www.amazon.co.uk/customer-preferences/edit?ie=UTF8&preferencesReturnUrl=%2F&ref_=topnav_lang`
- **货币选择方式**: 使用单选按钮(radio buttons),而不是弹窗
- **单选按钮格式**: 例如 `radio "£ - GBP - British Pound (Default)"`
- **保存按钮**: `button "Save changes"`

### 2. 关键发现

#### 页面结构
1. 页面上有7个常用货币的单选按钮(GBP, EUR, USD, CNY, DKK, SEK, PLN)
2. 还有一个下拉框(combobox)包含更多货币选项
3. 单选按钮的文本格式: `符号 - 代码 - 全称`, 例如 `£ - GBP - British Pound (Default)`

#### 原代码问题
1. **导航方式错误**: 原代码尝试点击触发器打开弹窗,但实际应该直接导航到设置页面URL
2. **选择器不精确**: 使用的选择器无法准确匹配单选按钮
3. **保存按钮选择器错误**: 需要使用正确的按钮选择器

### 3. 解决方案

#### 更新的选择器策略
```go
// 货币单选按钮选择器 - 按优先级排序
currencyOptionSelectors := []string{
    // 方法1: 直接查找包含货币代码的单选按钮 (最精确)
    fmt.Sprintf("input[type='radio']:has-text('%s')", currency),
    // 方法2: 通过文本内容查找 (备用)
    fmt.Sprintf("text=/ - %s - /i >> input[type='radio']", currency),
    // 方法3: 查找包含货币代码的任何单选按钮 (兜底)
    fmt.Sprintf("input[type='radio']:has-text('- %s -')", currency),
}

// 保存按钮选择器
saveButtonSelectors := []string{
    "button:has-text('Save changes')",               // 主要的保存按钮
    "button:has-text('Save')",                       // 备用
    "input[type='submit']:has-text('Save')",         // 提交按钮
    "span.a-button-inner:has-text('Save') >> input", // 嵌套在span中的input
}
```

#### 完整流程
1. 直接导航到货币设置页面URL
2. 等待页面加载完成
3. 使用精确的选择器查找并点击目标货币的单选按钮
4. 点击"Save changes"按钮保存设置
5. 等待页面刷新
6. 按ESC键关闭可能存在的对话框

### 4. 测试验证

通过Chrome DevTools MCP工具进行的实时测试:
- ✅ 成功导航到货币设置页面
- ✅ 成功选择USD单选按钮
- ✅ 成功切换回GBP单选按钮
- ✅ 成功点击保存按钮
- ✅ 页面正确显示选中的货币

## 代码修改

已更新文件: `task-processor/internal/crawler/amazon/browser/currency_setter.go`

主要修改:
1. 改为直接导航到货币设置页面URL
2. 更新货币选择器为更精确的单选按钮选择器
3. 更新保存按钮选择器
4. 优化等待和错误处理逻辑

## 后续建议

1. **测试不同站点**: 验证代码在其他Amazon站点(如.com, .de, .jp)上的兼容性
2. **错误处理**: 考虑添加更多错误处理和重试逻辑
3. **日志优化**: 将部分Info级别日志改为Debug级别,减少日志噪音
4. **性能优化**: 考虑缓存货币设置,避免每次都重新设置

## 相关文件

- `task-processor/internal/crawler/amazon/browser/currency_setter.go` - 货币设置实现
- `task-processor/internal/crawler/amazon/browser/zipcode_input_handler.go` - 邮编设置实现(参考)
