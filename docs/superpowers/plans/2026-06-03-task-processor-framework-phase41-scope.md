## Task Processor Framework Phase 41 Scope Recommendation

### Recommendation

下一步更值得做的是 `ListingKit navigation dispatch-plan step clone ownership`。

也就是继续处理：

- [generation_navigation_dispatch_plan_clone_shape.go](/D:/code/task-processor/internal/listingkit/generation_navigation_dispatch_plan_clone_shape.go:1)

里 still inlined 的 step-level clone shaping。

### Why This Before Other Options

#### 1. Dispatch-plan aggregate clone 已经收干净了

现在：

- shared helper home 已独立
- action-target / review-navigation / descriptor aggregate clone 都已显式委托 nested clone shaping
- dispatch-plan aggregate clone 也已显式委托 nested clone shaping

继续在 aggregate owner 这一层追求更细对称性，收益已经明显下降。

#### 2. Dispatch-plan step clone 是下一个明显的 aggregate hotspot

当前本地 shape seam 仍然同时知道：

- step slice clone
- step-level query clone

这已经是很明确的 residual ownership hotspot。

#### 3. 这个切片仍然足够 bounded

下一步可以只围绕：

- dispatch-plan step clone
- step query clone delegation
- current consumer-visible behavior stability

来做，不需要马上扩大到 dispatch-plan execution flow 本身。

### Explicitly Not Recommended Next

#### 1. 不建议继续深挖 dispatch-plan aggregate clone home

那个 ownership 问题已经解决了。

#### 2. 不建议直接重开 broader dispatch-plan execution flow

那会显著扩大写面，超出当前最自然的小切片。

### Proposed Next Phase Name

建议下一阶段命名为：

`Phase 41: ListingKit navigation dispatch-plan step clone ownership`
