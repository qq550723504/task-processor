# Amazon Zipcode 功能问题记录

## 问题 1: 页面刷新时机问题

### 问题描述
邮编设置成功后,页面不会立即刷新,而是在关闭确认对话框后才会刷新页面。

### 日志证据
```
time="2026-03-11 12:58:37" level=info msg="成功点击Apply按钮"
time="2026-03-11 12:58:39" level=info msg="未找到Done按钮,可能已自动关闭"
time="2026-03-11 12:58:39" level=info msg="开始查找邮编确认对话框的Continue按钮..."
time="2026-03-11 12:58:39" level=info msg="邮编设置操作完成"
time="2026-03-11 12:58:40" level=info msg="验证邮编 - 期望: 'SW1A 1AA', 当前: 'Hong Kong'"  ❌
time="2026-03-11 12:58:42" level=info msg="第二次尝试前刷新页面"
time="2026-03-11 12:58:48" level=info msg="验证邮编 - 期望: 'SW1A 1AA', 当前: 'London SW1A 1'" ✅
```

### 问题分析
1. 点击 Apply 按钮后,会出现确认对话框 "You're now shopping for delivery to: SW1A 1AA"
2. 需要点击 Continue 按钮关闭对话框
3. 只有关闭对话框后,页面才会刷新并显示新的邮编
4. 当前代码没有找到并点击 Continue 按钮,导致验证失败

### 最新日志分析 (2026-03-11 13:08)
```
time="2026-03-11 13:08:00" level=info msg="等待Apply按钮处理完成，准备查找Continue按钮..."
time="2026-03-11 13:08:02" level=info msg="未找到Done按钮，可能已自动关闭"
time="2026-03-11 13:08:02" level=info msg="开始查找邮编确认对话框的Continue按钮..."
time="2026-03-11 13:08:02" level=info msg="选择器 div[role='dialog'] button:has-text('Continue') 找到 0 个元素"
time="2026-03-11 13:08:02" level=info msg="选择器 dialog button:has-text('Continue') 找到 0 个元素"
time="2026-03-11 13:08:02" level=info msg="选择器 button:has-text('Continue'):not(:has-text('Shopping')) 找到 0 个元素"
time="2026-03-11 13:08:02" level=info msg="选择器 button:has-text('Continue') 找到 0 个元素"
```

**根本原因**：
- 等待时间只有 2 秒，可能不够对话框完全加载
- 没有使用 Playwright 的 WaitForSelector 等待对话框出现
- 选择器可能在对话框出现前就执行完毕了

### MCP 工具验证 (2026-03-11)
通过 Chrome DevTools MCP 工具验证：
1. ✅ 点击 Apply 按钮后，确认对话框会出现
2. ✅ 对话框显示 "You're now shopping for delivery to: SW1A 1AA"
3. ✅ 对话框中有 Continue 按钮（uid=24_8）
4. ✅ 点击 Continue 按钮后，对话框关闭，页面刷新

### 修复方案 ✅

**已实施的修复**：
1. ✅ 增加对话框等待时间：从 5 秒增加到 8 秒
2. ✅ 增加对话框内容加载等待：从 1 秒增加到 2 秒
3. ✅ 优化 Continue 按钮选择器顺序：优先查找对话框中的按钮
4. ✅ 增加 Continue 按钮等待时间：从 3 秒增加到 5 秒
5. ✅ 增加点击后的页面刷新等待：从 2 秒增加到 3 秒

**代码修改位置**：
- `internal/crawler/amazon/browser/zipcode_input_handler.go` - `submitZipcodeChange` 函数

**修改内容**：
```go
// 1. 等待对话框出现（8秒超时）
if err := page.Locator(selector).First().WaitFor(playwright.LocatorWaitForOptions{
    State:   playwright.WaitForSelectorStateVisible,
    Timeout: playwright.Float(8000), // 从 5000 增加到 8000
}); err == nil {
    logrus.Infof("确认对话框已出现: %s", selector)
    dialogAppeared = true
    break
}

// 2. 对话框出现后等待内容加载（2秒）
time.Sleep(2 * time.Second) // 从 1 秒增加到 2 秒

// 3. 优化选择器顺序
zipcodeConfirmSelectors := []string{
    "div[role='dialog'] button:has-text('Continue')",         // 最优先
    "[role='dialog'] button:has-text('Continue')",            
    "button:has-text('Continue'):not(:has-text('Shopping'))", 
    "button:has-text('Continue')",                            // 兜底
}

// 4. 等待 Continue 按钮出现（5秒超时）
if err := locator.WaitFor(playwright.LocatorWaitForOptions{
    State:   playwright.WaitForSelectorStateVisible,
    Timeout: playwright.Float(5000), // 从 3000 增加到 5000
}); err != nil {
    continue
}

// 5. 点击后等待页面刷新（3秒）
time.Sleep(3 * time.Second) // 从 2 秒增加到 3 秒
```

### 相关代码
- `internal/crawler/amazon/browser/zipcode_input_handler.go` - `submitZipcodeChange` 函数

---

## 问题 2: 货币解析错误

### 问题描述
英国站需要抓取的是 GBP,虽然日志提示解析到的价格是 GBP,但实际抓取到的是 HKD。

### 日志证据
```
time="2026-03-11 12:58:29" level=info msg="从选择器 #glow-ingress-line2 获取到文本: Hong Kong"
time="2026-03-11 12:58:40" level=info msg="验证邮编失败 - 期望: 'SW1A1AA', 当前: 'HONGKONG'"
time="2026-03-11 12:58:48" level=info msg="当前邮编已经是目标邮编 SW1A 1AA,无需设置"
time="2026-03-11 12:58:48" level=info msg="解析到价格: 473.16 GBP"  ⚠️ 日志说是GBP
```

### 问题分析
1. 第一次验证时,显示的是 "Hong Kong",说明之前的位置设置是香港
2. 香港的货币是 HKD,不是 GBP
3. 虽然后来设置成功了 "London SW1A 1",但页面显示的价格仍然是 HKD473.16
4. 确认的问题:
   - ✅ 页面在关闭对话框后才刷新
   - ✅ 刷新后货币仍然显示 HKD 而不是 GBP
   - ❌ 货币符号解析逻辑可能有问题,或者页面没有完全刷新

### 最新日志分析 (2026-03-11 13:08)
```
time="2026-03-11 13:08:12" level=info msg="当前邮编已经是目标邮编 SW1A 1AA，无需设置"
time="2026-03-11 13:08:12" level=info msg="解析到价格: 473.16 HKD"
```

**根本原因**：
- 邮编设置成功显示 "London SW1A 1"
- 但价格仍然显示 HKD 而不是 GBP
- 这是因为 Amazon 账户的默认货币设置为港币
- 即使设置了英国邮编，页面底部仍显示 "HK$ HKD - Hong Kong Dollar"
- 需要在邮编设置后，额外处理货币切换

### 相关代码
- `internal/crawler/amazon/browser/price_extractor.go` - 价格提取逻辑
- 需要检查价格提取的时机和货币解析逻辑

---

## 修复方案

### 问题 1: Continue 按钮点击失败

**根本原因**:
- 代码中已经有查找和点击 Continue 按钮的逻辑
- 但选择器可能不够准确,导致找不到按钮

**修复方案**:
1. 在 `submitZipcodeChange` 函数中,增加等待时间
2. 添加更多的 Continue 按钮选择器
3. 确保在点击 Continue 后等待页面刷新完成

### 问题 2: 货币解析错误 (HKD 显示为 GBP)

**根本原因**:
1. 货币管理器的 `currencyCodes` 映射中缺少 "HKD" (港币)
2. 页面显示的是 "HKD473.16",但货币管理器无法识别
3. 可能回退到默认货币或根据站点返回 GBP

**修复方案**:
1. 在 `CurrencyManager` 的 `currencyCodes` 映射中添加 "HKD"
2. 添加港币符号 "HK$" 的识别
3. 确保价格提取在页面完全刷新后进行


---

## 问题 2 修复记录 (2026-03-11)

### 最终修复方案 ✅

**问题根源**：
- Amazon 账户的默认货币设置为港币（HKD）
- 即使设置了英国邮编，页面仍然显示 HKD 价格
- 需要在页面底部的货币选择器中手动修改货币设置

**修复方案**：
创建了完整的货币设置模块，类似邮编设置器的结构：

1. **创建货币设置器** (`currency_setter.go`)
   - `SetAndVerifyCurrency()`: 设置并验证货币
   - `getCurrentCurrency()`: 获取当前页面的货币设置
   - `setCurrency()`: 在页面底部修改货币设置
   - 支持最多 3 次重试

2. **集成到产品处理流程** (`instance_processor.go`)
   - 在邮编设置成功后，立即设置货币
   - 从 URL 中提取期望的货币代码
   - 调用货币设置器进行设置和验证
   - 货币设置失败只记录警告，不终止抓取

3. **价格提取器验证** (`price_extractor.go`)
   - 提取价格后验证货币是否匹配
   - 如果不匹配，记录警告日志
   - 使用页面实际显示的货币（因为这是真实价格）

**代码修改位置**：
- 新增文件：`internal/crawler/amazon/browser/currency_setter.go`
- 修改文件：`internal/crawler/amazon/instance_processor.go`
- 修改文件：`internal/crawler/amazon/extractor/price_extractor.go`

**处理流程**：
```
1. 设置邮编 (SW1A 1AA)
   ↓
2. 验证邮编 (London SW1A 1)
   ↓
3. 设置货币 (GBP)
   ↓
4. 验证货币 (GBP)
   ↓
5. 提取价格 (473.16 GBP)
   ↓
6. 验证货币匹配
```

**货币设置器工作原理**：
1. 点击页面底部的货币选择器（`#icp-nav-flyout`）
2. 等待货币选择弹窗出现
3. 选择目标货币（如 GBP）
4. 点击保存按钮
5. 等待页面刷新
6. 验证货币是否设置成功
7. 如果失败，重试最多 3 次

**预期日志**：
```
尝试设置货币 (第 1/3 次): GBP
当前货币: HKD, 目标货币: GBP，需要设置
成功点击货币选择器: #icp-nav-flyout
成功选择货币: GBP
成功点击保存按钮
等待页面刷新...
成功设置并验证货币: GBP
解析到价格: 473.16 GBP
```

**注意事项**：
- 货币设置器的选择器可能需要根据实际页面结构调整
- 如果货币设置失败，不会终止抓取，但会记录警告
- 价格提取时会验证货币是否匹配，如果不匹配会记录警告
