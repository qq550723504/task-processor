## Task Processor Framework Phase 40 Scope Recommendation

### Recommendation

下一步更值得做的是 `ListingKit navigation dispatch-plan clone aggregate ownership`。

也就是继续处理：

- [generation_navigation_target_conditional.go](/D:/code/task-processor/internal/listingkit/generation_navigation_target_conditional.go:1) 里的 `cloneGenerationNavigationDispatchPlan(...)`

而不是继续回头深挖已经稳定下来的 descriptor aggregate clone。

### Why This Before Other Options

#### 1. Descriptor aggregate clone 已经收干净了

现在：

- shared helper home 已独立
- action-target aggregate clone 已显式委托 nested clone shaping
- review-navigation target aggregate clone 已显式委托 nested clone shaping
- descriptor aggregate clone 也已显式委托 nested clone shaping

继续在 descriptor 这层追求更细对称性，收益已经明显下降。

#### 2. Dispatch-plan clone 是下一个明显的 aggregate hotspot

当前 `cloneGenerationNavigationDispatchPlan(...)` 仍然同时知道：

- step slice clone
- step-level query clone

这已经是很明确的 aggregate clone owner。

#### 3. 这个切片仍然足够 bounded

下一步可以只围绕：

- dispatch-plan aggregate clone
- step clone / query clone nested delegation
- current consumer-visible behavior stability

来做，不需要马上扩大到 navigation dispatch execution flow 本身。

### Explicitly Not Recommended Next

#### 1. 不建议继续深挖 descriptor aggregate clone home

那个 ownership 问题已经解决了。

#### 2. 不建议直接重开 broader navigation dispatch execution flow

那会显著扩大写面，超出当前最自然的小切片。

### Proposed Next Phase Name

建议下一阶段命名为：

`Phase 40: ListingKit navigation dispatch-plan clone aggregate ownership`
