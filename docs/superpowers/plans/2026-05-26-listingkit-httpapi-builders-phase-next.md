# ListingKit HTTPAPI Builders Next Phase

## 推荐结论

先把 `internal/listingkit/httpapi/builders.go` 停在当前状态，优先切回别的热点，除非接下来还要继续扩 builder 数量或调整 tenant resolver/image upload 规则。

原因：

1. 最明显的 fallback 模板重复已经收平
2. 剩余重复更多集中在 `newDB...Repository(...)` 这类更高副作用、更大验证面的代码
3. 继续抽象如果没有明确目标，容易把简单 builder 变成过度封装

## 如果继续，推荐顺序

### 方向 1：收紧 DB repository builder 模板

目标：

- 统一 `newDB...Repository(...)` 里重复的“连库 + migrate + repo + closer”模式

建议步骤：

1. 先挑 2-3 个最同质的 `newDB...Repository(...)` 做试点
2. 只抽最小共性，例如 shared database open/close
3. 不要一口气全文件模板化

适用前提：

- 团队准备接受更高一点的验证成本

### 方向 2：把 non-repository builder 分文件

目标：

- 降低 `builders.go` 的主题混杂程度

建议步骤：

1. 把 image upload / pricing / tenant resolver 移到 `builder_runtime.go` 或类似文件
2. 保留 repository builder 在原文件
3. 不改导出 API，只做文件边界整理

适用前提：

- 当前团队更在意文件可读性而不是继续抽象 helper

### 方向 3：继续细化 legacy tenant resolver 探测

目标：

- 让 `ConfigureLegacyTenantResolver(...)` 更像纯编排

建议步骤：

1. 提取单个候选数据库 probe helper
2. 明确 “连上但 metadata 不存在” 与 “连库失败” 的分支语义
3. 只在确实要继续演进 legacy bridge 时推进

适用前提：

- 未来还会继续维护这条 legacy tenant bridge

## 不推荐的方向

### 1. 继续把所有 builder 都抽成独立单函数

原因：

- 现在最明显的收益已经拿到了
- 再继续切碎，阅读体验可能不升反降

### 2. 现在就引入完整 DI 框架

原因：

- 当前 builder 层复杂度还没到必须引入更重工具的程度
- 会放大验证面，不符合这轮“小步、行为不变”的节奏

## 下一步建议

更推荐的下一步是：

1. 切回别的高收益热点继续降复杂度
2. 或先做更大范围回归验证，准备合并这一轮重构
