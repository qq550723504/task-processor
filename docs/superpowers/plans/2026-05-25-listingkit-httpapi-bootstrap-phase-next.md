# ListingKit HTTPAPI Bootstrap Next Phase

## 推荐结论

先把 `internal/listingkit/httpapi/bootstrap.go` 停在当前状态，优先做阶段盘点和更大范围回归验证，而不是继续机械细拆。

原因：

1. 这轮已经把最大的集中职责拆掉了。
2. 剩余热点更多是“未来可能重新膨胀的入口”，而不是眼前已经失控的复杂度块。
3. 再继续细拆，收益会开始低于验证和维护成本。

## 如果继续，推荐顺序

### 方向 1：收紧 `buildListingKitServiceConfig(...)`

目标：

- 避免 `ServiceConfig` 重新成为新的装配热点

建议步骤：

1. 拆出 `buildListingKitCoreDependencies(...)`
2. 拆出 `buildListingKitAssetDependencies(...)`
3. 拆出 `buildListingKitSheinDependencies(...)`
4. 保留 workflow defaults 在顶层或单独 helper

适用前提：

- 团队确认 `listingkit.ServiceConfig` 还会继续扩展

### 方向 2：明确 runtime payload 边界

目标：

- 减少 `BuildModule(...)` 对 `ServiceBundle.runtime` 私有字段的隐式耦合

建议步骤：

1. 提取独立 `moduleRuntimePayload`
2. 由 `buildServiceRuntime(...)` 显式产出 payload
3. 让 `buildModuleRuntime(...)` 只依赖 payload，而不是读取 bundle 私有内部

适用前提：

- 未来需要支持更多 module runtime 变体

### 方向 3：补更明确的 phase 顺序护栏

目标：

- 防止后续插入新 phase 时无意改变当前阶段语义

建议步骤：

1. 为 repository assembly 的 closer 顺序补更直接测试
2. 为 module environment 的 phase 顺序补更直接测试
3. 为 service runtime modules 的组合边界补更精确断言

适用前提：

- 当前结构先不继续拆，但希望降低未来回归风险

## 不推荐的方向

### 1. 继续无差别拆 helper

原因：

- 现在最明显的收益已经拿到了
- 再拆下去很容易进入“函数更碎，但模型没更清楚”的状态

### 2. 直接重写为完整 DI 容器

原因：

- 当前复杂度还没到必须引入更重工具的程度
- 会扩大验证面，也不符合这轮“行为不变、小步收敛”的节奏

## 下一步建议

推荐先做两件事中的一个：

1. 先停在这里，转去别的热点继续降复杂度
2. 如果还要留在 `httpapi`，优先做 `buildListingKitServiceConfig(...)` 的子域拆分，而不是继续打散别的 helper
