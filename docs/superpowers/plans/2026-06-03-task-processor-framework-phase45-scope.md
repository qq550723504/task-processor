## Task Processor Framework Phase 45 Scope Recommendation

### Recommendation

下一步更值得做的是 `ListingKit navigation descriptor dispatch-plan delegation ownership`。

也就是继续处理：

- [generation_navigation_descriptor_residual_shape.go](/D:/code-task-processor/internal/listingkit/generation_navigation_descriptor_residual_shape.go:1)

里现在唯一剩下的 `DispatchPlan` clone delegation。

### Why This Before Other Options

#### 1. Residual pairing 已经收干净了

现在：

- follow-up read clone 已有独立 home
- residual pairing 也已有独立 home
- residual shape home 现在主要只剩 dispatch-plan clone delegation

继续在 pairing 这层追求更细对称性，收益已经明显下降。

#### 2. Dispatch-plan delegation 是下一个最自然的小切口

当前 residual shape home 只剩一个显眼职责：

- dispatch-plan clone delegation

这已经是很明确的 ownership hotspot，而且写面很小。

#### 3. 这个切片仍然足够 bounded

下一步可以只围绕：

- descriptor residual dispatch-plan delegation
- existing dispatch-plan clone home
- current consumer-visible behavior stability

来做，不需要马上扩大到 broader descriptor builder 或 navigation dispatch flow 本身。

### Explicitly Not Recommended Next

#### 1. 不建议回头重开 residual pairing seam

那个 ownership 问题已经解决了。

#### 2. 不建议直接重开 broader descriptor builder / navigation dispatch flow

那会显著扩大写面，超出当前最自然的小切片。

### Proposed Next Phase Name

建议下一阶段命名为：

`Phase 45: ListingKit navigation descriptor dispatch-plan delegation ownership`
