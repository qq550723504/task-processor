## Task Processor Framework Phase 50 Scope Recommendation

### Recommendation

下一步更值得做的是 `ListingKit navigation follow-up read item clone aggregate ownership`。

也就是继续处理：

- [generation_navigation_followup_read_clone.go](/D:/code-task-processor/internal/listingkit/generation_navigation_followup_read_clone.go:1)

里现在还留着的 `top-level field copy + nested query clone delegation`。

### Why This Before Other Options

#### 1. Follow-up read slice clone 已经收干净了

现在：

- routing home 已只保留 pairing dispatch
- pairing home 已只保留 slice clone home dispatch
- item clone home 成了当前链路里下一个最明显的 aggregate home

继续在 slice orchestration 这一层追求更细对称性，收益已经明显下降。

#### 2. Item clone aggregate ownership 是下一个最自然的小切口

当前 item clone home 仍然同时知道：

- top-level field copy
- nested query clone delegation

这已经是很明确的 ownership hotspot，而且写面很小。

#### 3. 这个切片仍然足够 bounded

下一步可以只围绕：

- follow-up read item clone
- shared queue query clone helper
- current consumer-visible behavior stability

来做，不需要马上扩大到 broader descriptor clone entry 或 navigation dispatch flow 本身。

### Explicitly Not Recommended Next

#### 1. 不建议回头重开 follow-up read slice clone home

这个 ownership 问题已经解决了。

#### 2. 不建议直接重开 broader descriptor clone entry / navigation dispatch flow

那会显著扩大写面，超出当前最自然的小切片。

### Proposed Next Phase Name

建议下一阶段命名为：

`Phase 50: ListingKit navigation follow-up read item clone aggregate ownership`
