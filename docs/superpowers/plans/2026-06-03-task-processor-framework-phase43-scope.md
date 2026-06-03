## Task Processor Framework Phase 43 Scope Recommendation

### Recommendation

下一步更值得做的是 `ListingKit navigation descriptor residual shape ownership`。

也就是继续处理：

- [generation_navigation_descriptor_clone_shape.go](/D:/code/task-processor/internal/listingkit/generation_navigation_descriptor_clone_shape.go:1)

里还留着的 `Conditional / Invalidates / DispatchPlan` residual shape 决策。

### Why This Before Other Options

#### 1. Follow-up read clone 已经收干净了

现在：

- shared helper home 已独立
- action-target / review-navigation / descriptor / dispatch-plan aggregate clone 都已显式委托 nested clone shaping
- dispatch-plan step clone 和 descriptor follow-up read clone 也都已收成更窄的本地 seam

继续在 follow-up read 这层追求更细对称性，收益已经明显下降。

#### 2. Descriptor residual shape 是下一个明显的 aggregate hotspot

当前本地 descriptor shape seam 仍然同时知道：

- conditional clone
- invalidates slice clone
- dispatch-plan clone delegation

这已经是很明确的 residual ownership hotspot。

#### 3. 这个切片仍然足够 bounded

下一步可以只围绕：

- descriptor residual shape
- existing nested clone home dispatch
- current consumer-visible behavior stability

来做，不需要马上扩大到 descriptor builder 或 navigation dispatch flow 本身。

### Explicitly Not Recommended Next

#### 1. 不建议继续深挖 follow-up read clone home

那个 ownership 问题已经解决了。

#### 2. 不建议直接重开 broader descriptor builder / navigation dispatch flow

那会显著扩大写面，超出当前最自然的小切片。

### Proposed Next Phase Name

建议下一阶段命名为：

`Phase 43: ListingKit navigation descriptor residual shape ownership`
