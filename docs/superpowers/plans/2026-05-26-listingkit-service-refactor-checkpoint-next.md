# ListingKit Service Refactor Checkpoint Next

## 推荐结论

把 `listingkit` 的 service 重构先停在当前 checkpoint，优先做更大范围回归验证，或者切去别的复杂度热点。

原因：

1. 高收益的子域边界已经基本拆出来了
2. 再继续拆下去，收益会从“降复杂度”逐渐转向“结构精修”
3. 当前工作区里已经有一批独立的 `web/listingkit-ui/...` 改动在推进，继续深挖后端容易让两个方向互相干扰

## 如果仍然继续留在 `listingkit`

### 方向 1：SHEIN 运行时辅助层独立建模

目标：

- 收紧 store selection / API client / attribute API 这组运行时 helper 的边界

建议步骤：

1. 提取 `sheinRuntimeService` 或 `sheinStoreRoutingService`
2. 让 `resolveSheinStoreSelection(...)`、`resolveSheinStoreID(...)`、`resolveSheinStoreProfile(...)` 共用统一落点
3. 再决定 `buildSheinAttributeAPI(...)` 与 `newSheinAPIClient(...)` 是否一起归并

适用前提：

- 后续仍有较多 SHEIN store routing 或 category/attribute 管理需求

### 方向 2：workflow/process 层阶段化

目标：

- 把 `ProcessListingKit(...)` 这类厚执行链从“顺序脚本”推进到更清楚的 phase model

建议步骤：

1. 先提炼 `standard workflow state` 的阶段对象
2. 再看 `platform adaptation` 是否适合做对应的 phase helper
3. 每次只动一个执行层，避免把 generation/submit/admin 一起卷进来

适用前提：

- 团队接下来要继续投入 listingkit workflow 本身，而不是 settings/admin 或 studio

### 方向 3：为协作者补更聚焦的单测

目标：

- 让新拆出的协作者不只依赖历史集成测试兜底

建议步骤：

1. 给 `sheinAdminService` 增补 category search / final draft 的聚焦测试
2. 继续为 `settingsAdminService` 补更明确的 store option / tenant fallback 场景
3. 只补关键边界，不做测试数量驱动

适用前提：

- 准备长期维护这条后端重构线，而不是短期合并收口

## 更推荐的下一步

当前更推荐：

1. 跑一轮更大范围验证：
   `go test ./internal/listingkit/... ./internal/listingkit/httpapi/... ./internal/app/consumer/... ./internal/listingadmin/... -count=1`
2. 或切回别的热点，例如当前工作区里正在演进的 `web/listingkit-ui` studio 相关前端改动

## 不推荐的方向

### 1. 继续机械拆更多 facade/helper

原因：

- 当前根 `service` 已经明显瘦下来了
- 继续零散切更多薄 facade，阅读成本会开始超过结构收益

### 2. 同时继续深拆后端和大改前端

原因：

- 工作区当前已经有一批独立前端改动
- 两条线同时大步推进，会放大回归面，也不利于后续分批合并
