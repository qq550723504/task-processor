# prohibited_items_detector.go 重构方案

## 当前状态

**文件：** `internal/platforms/temu/handlers/prohibited_items_detector.go`  
**行数：** 711行  
**严重程度：** 🔴 严重（项目中第2大文件）

## 复杂度分析

| 函数名 | 复杂度 | 说明 |
|--------|--------|------|
| calculateConfidence | 15 | 置信度计算逻辑复杂 |
| isWeaponContext | 13 | 武器上下文判断逻辑复杂 |

## 问题分析

### 1. 职责过多
文件包含了10种不同的检测逻辑：
1. 配置加载和管理
2. 武器类检测
3. 毒品类检测
4. 成人内容检测
5. 假冒伪劣检测
6. 危险品检测
7. 医疗器械检测
8. 烟草制品检测
9. 活体动物检测
10. 上下文验证和置信度计算

### 2. 代码重复
- 每个检测方法都有类似的关键词检查逻辑
- 白名单检查逻辑重复
- 模式匹配逻辑重复

### 3. 难以维护
- 添加新的违禁品类别需要修改大文件
- 修改某个类别的检测逻辑可能影响其他类别
- 测试困难，需要模拟整个检测器

## 重构方案

### 方案1：按检测类别拆分（推荐）

```
prohibited_items_detector.go (主检测器 - 150行)
    ├── prohibited_items_types.go (类型定义 - 50行)
    ├── prohibited_items_config.go (配置加载 - 100行)
    ├── prohibited_items_weapons.go (武器检测 - 150行)
    ├── prohibited_items_drugs.go (毒品检测 - 80行)
    ├── prohibited_items_adult.go (成人内容检测 - 80行)
    ├── prohibited_items_counterfeit.go (假冒伪劣检测 - 80行)
    ├── prohibited_items_dangerous.go (危险品检测 - 80行)
    ├── prohibited_items_medical.go (医疗器械检测 - 80行)
    ├── prohibited_items_tobacco.go (烟草检测 - 60行)
    ├── prohibited_items_animals.go (活体动物检测 - 80行)
    └── prohibited_items_utils.go (工具函数 - 100行)
```

**优点：**
- 每个文件职责单一
- 易于添加新的检测类别
- 易于测试每个检测器
- 代码复用性高

**缺点：**
- 文件数量较多（12个文件）

### 方案2：按功能模块拆分

```
prohibited_items_detector.go (主检测器 - 200行)
    ├── prohibited_items_types.go (类型定义 - 50行)
    ├── prohibited_items_config.go (配置加载 - 100行)
    ├── prohibited_items_detectors.go (所有检测器 - 400行)
    └── prohibited_items_utils.go (工具函数 - 150行)
```

**优点：**
- 文件数量适中（5个文件）
- 结构清晰

**缺点：**
- detectors.go 仍然较大（400行）
- 不如方案1灵活

## 推荐方案：方案1（按检测类别拆分）

### 文件拆分详情

#### 1. prohibited_items_detector.go (主检测器 - 150行)
- 主检测器结构
- HandleTemu 方法
- DetectProhibitedItems 主方法
- 协调各个子检测器

#### 2. prohibited_items_types.go (类型定义 - 50行)
- ProhibitedItemsConfig
- ProhibitedItemResult
- DetectorConfig

#### 3. prohibited_items_config.go (配置加载 - 100行)
- loadConfig
- loadDefaultConfig
- 配置文件解析

#### 4. prohibited_items_weapons.go (武器检测 - 150行)
- detectWeaponsWithContext
- isWeaponContext
- 武器相关的关键词和模式

#### 5. prohibited_items_drugs.go (毒品检测 - 80行)
- detectDrugs
- 毒品相关的关键词和模式

#### 6. prohibited_items_adult.go (成人内容检测 - 80行)
- detectAdultContent
- 成人内容相关的关键词和模式

#### 7. prohibited_items_counterfeit.go (假冒伪劣检测 - 80行)
- detectCounterfeit
- 假冒伪劣相关的关键词和模式

#### 8. prohibited_items_dangerous.go (危险品检测 - 80行)
- detectDangerous
- 危险品相关的关键词和模式

#### 9. prohibited_items_medical.go (医疗器械检测 - 80行)
- detectMedical
- 医疗器械相关的关键词和模式

#### 10. prohibited_items_tobacco.go (烟草检测 - 60行)
- detectTobacco
- 烟草相关的关键词和模式

#### 11. prohibited_items_animals.go (活体动物检测 - 80行)
- detectLiveAnimals
- 活体动物相关的关键词和模式

#### 12. prohibited_items_utils.go (工具函数 - 100行)
- checkKeywords
- checkPatterns
- calculateConfidence
- extractProductTexts
- extractProductCategories
- isLegitimateProductCategory

## 预期收益

### 代码质量提升
- 单文件最大行数：711行 → 150行（-79%）
- 平均文件行数：711行 → 90行（-87%）
- 最高函数复杂度：15 → <10（-33%）

### 可维护性提升
- 添加新检测类别：只需添加新文件
- 修改某个类别：只影响对应文件
- 测试覆盖：每个检测器可独立测试

### 扩展性提升
- 易于添加新的违禁品类别
- 易于调整检测逻辑
- 易于优化性能

## 实施计划

### 第一阶段：创建基础文件
1. ✅ prohibited_items_types.go - 类型定义
2. prohibited_items_config.go - 配置加载
3. prohibited_items_utils.go - 工具函数

### 第二阶段：拆分检测器
4. prohibited_items_weapons.go - 武器检测
5. prohibited_items_drugs.go - 毒品检测
6. prohibited_items_adult.go - 成人内容检测
7. prohibited_items_counterfeit.go - 假冒伪劣检测
8. prohibited_items_dangerous.go - 危险品检测
9. prohibited_items_medical.go - 医疗器械检测
10. prohibited_items_tobacco.go - 烟草检测
11. prohibited_items_animals.go - 活体动物检测

### 第三阶段：重构主检测器
12. prohibited_items_detector.go - 主检测器重构

### 第四阶段：验证和测试
13. 编译验证
14. 诊断检查
15. 业务逻辑验证
16. 生成重构文档

## 风险评估

### 低风险
- ✅ 业务逻辑清晰，易于拆分
- ✅ 每个检测器相对独立
- ✅ 测试验证简单

### 需要注意
- ⚠️ 确保所有关键词和模式完整迁移
- ⚠️ 确保白名单逻辑正确迁移
- ⚠️ 确保上下文验证逻辑正确

## 是否继续重构？

**建议：** 继续重构，因为：
1. 文件过大（711行），严重违反单一职责原则
2. 重构方案清晰，风险可控
3. 预期收益显著

**用户确认后开始实施。**
