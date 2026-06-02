# SHEIN Sensitive Content Sanitizer Design

## Background

`listing kit pod -> shein` 目前存在两条并行的文案处理链路：

1. 预览/草稿链路：`internal/publishing/shein/listing_copy.go`、`assembler.go` 负责生成标题、描述、SKC 标题、草稿多语言文案。
2. 提交前链路：`internal/publishing/shein/submit_prep.go -> submitprep.CleanSensitiveWords() -> internal/shein/content/SensitiveWordService` 负责对最终提交的 `sheinproduct.Product` 做敏感词清洗。

这导致敏感词处理存在两个根因问题：

- 字段覆盖不一致。现有 `SensitiveWordService.ProcessProductData()` 只直接处理产品多语言标题、产品多语言描述、`SKC.MultiLanguageName`、`SKC.MultiLanguageNameList`，而 `listing_copy` 新生成的标题、描述、`SKCTitleBase`、草稿里的 `SKC` 文案并没有统一经过同一入口。
- 处理逻辑分散。当前是“在不同结构上分别遍历并写死字段”，每增加一个新字段都需要补一处逻辑，容易继续漏掉“产品资料”等自由文本属性。

用户要求的目标行为是：

- 预览阶段自动清洗敏感词
- 提交阶段再次统一兜底
- “产品资料”指 `ProductAttributeList` 中的自由文本属性值，也要纳入同一套规则

## Goals

- 为 `listing kit pod -> shein` 新增统一的敏感词过滤能力，覆盖标题、描述、SKC 标题、多语言标题、多语言描述、产品资料自由文本属性。
- 预览/草稿和提交前复用同一套清洗规则，避免用户看到的内容和最终提交内容不一致。
- 最大限度复用现有成熟能力，不重复造轮子：继续使用现有 `SensitiveWordService` 的词库、品牌词移除、上下文品牌词移除、SHEIN 平台文本清理。
- 将实现从“结构体字段硬编码遍历”收敛为“字段驱动”的 sanitizer，降低以后新增字段时的漏改风险。

## Non-Goals

- 不重写或替换现有敏感词词库来源。
- 不改变现有平台返回敏感词后 `RetrySensitiveWordCleanup()` 的兜底策略，只统一其底层清洗能力。
- 不处理纯结构化属性值，例如枚举 ID、数值、单位、仓库编码、条码等非自由文本字段。

## Current State Summary

### Existing reusable pieces

- `internal/shein/content/processor.go`
  - `removeSensitiveWordsAndBrandsWithContext()`
  - `processMultiLanguageNamesWithBrandsAndContext()`
  - 已具备敏感词、Amazon 品牌词、上下文品牌词、平台文本清理能力
- `internal/shein/submitprep/sensitive_words.go`
  - `CleanSensitiveWords()`
  - `RetrySensitiveWordCleanup()`
- `internal/publishing/shein/submit_prep.go`
  - `PrepareSubmitProductContent()`
  - 已经有提交前统一清洗时机

### Existing gaps

- `internal/publishing/shein/listing_copy.go`
  - `buildSheinListingCopy()` 只做 `cleanListingText()` / `sanitizeListingCopy()`，没有统一走敏感词逻辑
- `internal/publishing/shein/assembler.go`
  - 构造的 `DraftPayload.MultiLanguageNameList`、`DraftPayload.MultiLanguageDescList`、`DraftPayload.SKCList[*].MultiLanguageNameList`、`SkcName` 没有统一调用敏感词入口
- `internal/shein/content/word.go`
  - `ProcessProductData()` 仍是按固定字段写死遍历，扩展 `ProductAttributeList` 不自然，也不适合直接复用到预览侧的 `listingCopy`/`RequestDraft`

## Design Overview

新增一个统一的 `SHEIN content sanitizer`，职责只做“清洗会进入 SHEIN 的文案字段”，不负责生成文案。

设计分两层：

### 1. Base text sanitizer

输入：

- 文本内容
- `TaskContext`（可选，用于上下文品牌词）

处理逻辑：

- 复用 `SensitiveWordService.removeSensitiveWordsAndBrandsWithContext()`
- 对空文本直接返回
- 返回清洗后文本和是否发生变更

该层只关心“如何清洗一段文本”，不关心文本来自哪个结构。

### 2. Structure adapters

负责把不同结构中的目标字段映射成统一的文本项集合，然后批量调用 base sanitizer，再写回原结构。

需要支持三类结构：

- `listingCopy`
- `RequestDraft`
- `sheinproduct.Product`

## Field Coverage

### Preview / draft phase

以下字段必须在预览阶段自动清洗：

- `listingCopy.Title`
- `listingCopy.Description`
- `listingCopy.SKCTitleBase`
- `Package.DraftPayload.MultiLanguageNameList[*].Name`
- `Package.DraftPayload.MultiLanguageDescList[*].Name`
- `Package.DraftPayload.SKCList[*].SkcName`
- `Package.DraftPayload.SKCList[*].MultiLanguageNameList[*].Name`
- `Package.DraftPayload.ProductAttributeList[*].AttrValueName` 中的自由文本值

### Submit phase

以下字段必须在提交前再次兜底清洗：

- `sheinproduct.Product.MultiLanguageNameList[*].Name`
- `sheinproduct.Product.MultiLanguageDescList[*].Name`
- `sheinproduct.Product.ProductAttributeList[*].AttrValueName` 中的自由文本值
- `sheinproduct.Product.SKCList[*].MultiLanguageName.Name`
- `sheinproduct.Product.SKCList[*].MultiLanguageNameList[*].Name`

## Free-Text Attribute Rules

“产品资料”按 `ProductAttributeList` 中的自由文本属性值理解。

纳入清洗的条件：

- 值字段是自然语言文本
- 非空
- 不是纯数字/单位/ID/布尔/固定枚举编码

不纳入清洗的条件：

- 仅用于平台映射的属性 ID
- 数值型规格
- 单位类字段
- 结构化编码

实现上建议优先复用现有属性模型中的值字段，不引入新 schema。必要时可增加一个小型判定函数来跳过明显的结构化值。

## Integration Points

### 1. listing copy generation

在 `buildSheinListingCopy()` 生成出 `Title`、`Description`、`SKCTitleBase` 后，立即调用 sanitizer。

目标：

- 确保 preview 基础文案源头就是干净的
- `SKCTitleBase` 后续参与变体组装时不再传播敏感词

### 2. package / draft assembly

在 `assembler.Build()` 完成 `DraftPayload` 和 `SKCList` 组装后，对整个草稿执行一次 sanitizer。

目标：

- 覆盖 `DraftPayload` 中的多语言标题、描述、`SKC` 文案、产品资料属性
- 确保用户在 review / preview 中看到的内容和最终提交策略一致

### 3. submit preparation

保留 `PrepareSubmitProductContent()` 中的清洗时机，但底层改为调用统一 sanitizer 的 `sheinproduct.Product` 适配器。

目标：

- 对最终提交模型统一兜底
- 防止中间编辑、翻译、属性修订重新引入敏感词

### 4. retry cleanup

保留 `RetrySensitiveWordCleanup()` 作为平台返回新敏感词时的二次兜底。

要求：

- 尽量复用同一个 base sanitizer
- 避免形成第三套规则

## Refactor Plan

### Step 1

新增统一 sanitizer 模块，抽出“批量文本项清洗”的公共能力。

### Step 2

为 `listingCopy`、`RequestDraft`、`sheinproduct.Product` 分别增加结构适配器。

### Step 3

将 `listing_copy.go`、`assembler.go`、`submit_prep.go` 接到统一 sanitizer。

### Step 4

适度收敛 `SensitiveWordService.ProcessProductData()`，让其内部尽量复用统一 sanitizer，而不是继续维护一套只针对 `Product` 的手写遍历逻辑。

这里不要求一次性大改所有旧入口，但新实现必须让新增字段扩展以统一 sanitizer 为中心。

## Test Strategy

严格按 TDD 执行，先补失败测试再改实现。

新增或补强以下测试：

- `listing_copy` 生成的 `Title` / `Description` / `SKCTitleBase` 含敏感词时，预览阶段已自动清洗
- `assembler.Build()` 生成的 `DraftPayload.MultiLanguageNameList`、`MultiLanguageDescList`、`SKCList[*].SkcName`、`SKCList[*].MultiLanguageNameList` 已清洗
- `ProductAttributeList` 中自由文本属性值会被清洗，结构化属性值不会误伤
- `PrepareSubmitProductContent()` 提交前再次执行统一清洗，确保最终 `sheinproduct.Product` 不漏字段
- 品牌词、Amazon 品牌词、上下文品牌词行为不回退
- 平台敏感词重试逻辑仍可工作

优先落在现有相关测试文件中，避免新建过多分散测试入口：

- `internal/publishing/shein/listing_copy_test.go`
- `internal/publishing/shein/submit_prep_test.go`
- 如有必要，为统一 sanitizer 新建小型单元测试文件

## Risks and Mitigations

### Risk: over-cleaning attributes

问题：

- 产品资料里可能混有结构化值，误清洗会影响属性映射

缓解：

- 只处理自由文本值
- 为结构化值增加跳过判定
- 为典型属性映射场景补回归测试

### Risk: preview and submit diverge

问题：

- 如果 preview 和 submit 没有完全走同一规则，仍会出现用户看到的和提交的不同

缓解：

- 统一通过同一 sanitizer base 处理
- preview 与 submit 都增加覆盖测试

### Risk: duplicate cleanup side effects

问题：

- 预览阶段清洗后，提交前再清洗一次，可能引发重复格式化差异

缓解：

- sanitizer 设计为幂等
- 为重复调用补幂等测试

## Recommended Implementation Shape

推荐将统一 sanitizer 放在 SHEIN 现有内容准备相关包附近，便于复用并避免循环依赖。

建议优先选择能同时被：

- `internal/publishing/shein`
- `internal/shein/submitprep`
- `internal/shein/content`

复用的位置。

若直接放入 `internal/shein/content`，需要确保不会让发布侧产生不合理的包耦合；若放在 `internal/publishing/shein`，则要避免 submitprep 反向依赖 publishing。最终以“依赖方向最干净”为准。

## Acceptance Criteria

- `listing kit pod -> shein` 预览中的标题、描述、`SKC` 标题、多语言标题、多语言描述、产品资料自由文本属性都会自动做敏感词清洗
- 提交前会对最终 `sheinproduct.Product` 再执行同一套规则的统一兜底清洗
- 平台返回新增敏感词时，现有 retry cleanup 仍能工作
- 预览内容与提交内容的敏感词处理策略一致
- 新增字段扩展只需要在统一 sanitizer 注册字段，不需要到多处复制遍历逻辑
