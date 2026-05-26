# ListingKit HTTPAPI Builders Refactor Review

## 背景

`internal/listingkit/httpapi/builders.go` 原先主要承载两类职责：

1. repository builder：根据是否配置数据库，决定创建 DB repository、in-memory fallback 或 disabled fallback
2. 小型装配分支：例如 image upload store 选择、legacy tenant resolver 探测

这类文件最容易出现的问题不是“某一段特别复杂”，而是：

- 同一模板复制很多次，后续改一处容易漏另一处
- 小型分支逻辑夹杂在一起，虽然都不长，但边界不清楚

## 本轮已完成的结构收敛

### 1. repository fallback 模板统一

已新增：

- `buildRepositoryWithFallback(...)`

当前已经接入的 builder 包括：

- `BuildListingKitTaskRepository(...)`
- `BuildListingKitStudioAsyncJobRepository(...)`
- `BuildListingAdminStoreRepository(...)`
- `BuildListingAdminStoreStatisticsRepository(...)`
- `BuildListingKitStoreProfileRepository(...)`
- `BuildListingKitStoreRoutingSettingsRepository(...)`
- `BuildListingAdminImportTaskRepository(...)`
- `BuildListingAdminFilterRuleRepository(...)`
- `BuildListingAdminProfitRuleRepository(...)`
- `BuildListingAdminPricingRuleRepository(...)`
- `BuildListingAdminOperationStrategyRepository(...)`
- `BuildListingAdminSensitiveWordRepository(...)`
- `BuildListingAdminProductImportMappingRepository(...)`
- `BuildListingAdminCategoryRepository(...)`
- `BuildListingAdminProductDataRepository(...)`
- `BuildListingSubscriptionRepository(...)`
- `BuildAssetRepository(...)`
- `BuildListingKitReviewRepository(...)`
- `BuildListingKitStudioSessionRepository(...)`
- `BuildListingKitUploadedImageRepository(...)`
- `BuildSheinResolutionCacheStore(...)`

当前效果：

- “有 DB 就建 repo，否则走 fallback” 的模板已经从一大片手写重复收成统一 helper
- 以后如果新增同类 builder，不需要再复制一整段样板

### 2. legacy tenant resolver 探测拆分

已新增：

- `shouldDisableLegacyTenantResolver(...)`
- `legacyTenantResolverDatabaseConfigs(...)`

当前效果：

- `ConfigureLegacyTenantResolver(...)` 不再自己同时承担禁用判定、候选库枚举和 probe 编排
- 至少前两层已经变成纯规则函数，可单测约束

### 3. image upload store 选择拆分

已新增：

- `shouldUseS3ImageUploadStore(...)`
- `localImageUploadRootDir(...)`

当前效果：

- `BuildImageUploadStore(...)` 只保留 provider 分支编排
- provider 判定和本地目录构造不再硬编码在同一个函数体里

### 4. builders 专属护栏测试建立

已新增：

- [internal/listingkit/httpapi/builders_test.go](/D:/code/task-processor/internal/listingkit/httpapi/builders_test.go)

当前覆盖重点：

- repository in-memory fallback
- repository disabled fallback
- legacy tenant resolver 禁用规则
- legacy tenant resolver 候选库枚举
- image upload store provider 选择
- image upload 本地路径构造

当前效果：

- `builders.go` 不再完全依赖更大的 `bootstrap` 测试兜底
- 后续继续重构时，有了更贴近文件职责的护栏

## 当前结构现状

现在 `builders.go` 已经从“长串 builder + 分支拼在一起”的状态，开始变成：

- 统一的 repository fallback 模板
- 少量独立的小型规则 helper
- 更明确的编排函数

这意味着这个文件虽然仍然大，但重复模板密度已经明显下降。

## 仍然集中的剩余职责

### 1. DB builder 本身仍然是一大片

例如：

- `newDBListingKitTaskRepository(...)`
- `newDBListingAdminStoreRepository(...)`
- 以及一整批 `newDB...Repository(...)`

它们虽然不再在上层重复 fallback 模板，但仍各自手写：

- `database.NewSharedDatabaseFromConfig(...)`
- logging
- AutoMigrate
- repository creation
- closer 构造

这类重复目前还在，但验证面更大，继续抽象要更谨慎。

### 2. legacy tenant resolver 仍有真实探测副作用

当前虽然禁用判定和候选枚举已经拆开，但：

- 连库
- metadata table 探测
- resolver 接线

仍然集中在 `ConfigureLegacyTenantResolver(...)` 本体里。

### 3. builders.go 仍混合 repository builder 和非-repository builder

比如：

- repository builders
- pricing policy builder
- image upload store builder
- legacy tenant resolver configurator

从文件组织上看，后续仍可能值得按主题分文件。

## 结论

这一轮 `builders.go` 收敛已经拿到了最直接的收益：

- repository fallback 模板不再大面积重复
- branching helper 开始可测
- 后续再加 builder 时，样板扩张速度会明显下降

继续往下做当然可以，但收益已经从“去掉明显重复”转向“进一步组织文件边界”。
下一阶段如果继续，不应再无差别提 helper，而应该围绕更明确的目标推进。
