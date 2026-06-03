## Task Processor Framework Phase 33 Scope Recommendation

### Candidate Directions

`Phase 32` 收口之后，ListingKit execute/action 邻域里至少还有两个可继续推进的方向：

#### 方向一：ListingKit action execute handoff mode-pairing normalization ownership

当前最集中的 residual hotspot 在：

- [task_generation_action_execute_request_handoff_mode_pairing.go](/D:/code-task-processor/internal/listingkit/task_generation_action_execute_request_handoff_mode_pairing.go:1)

也就是 mode-pairing seam 仍然保留：

- `runRetryable`
- `runQueue`

两条几乎镜像的 orchestration 路径。

#### 方向二：继续深挖 shared `queue/retry` clone helper ownership

例如继续围绕：

- [cloneGenerationQueueQuery(...)](/D:/code-task-processor/internal/listingkit/service_generation_actions.go:15)
- [cloneRetryGenerationTasksRequest(...)](/D:/code-task-processor/internal/listingkit/service_generation_actions.go:23)

去讨论它们是否应该继续离开 shared helper home。

这两个方向都合理，但优先级并不相同。

### Recommendation

`Phase 33` 应先聚焦 **ListingKit action execute handoff mode-pairing normalization ownership**。

也就是优先处理：

- [task_generation_action_execute_request_handoff_mode_pairing.go](/D:/code-task-processor/internal/listingkit/task_generation_action_execute_request_handoff_mode_pairing.go:1)

而不是现在就把 shared `queue/retry` clone helper 的更宽归属问题重新全面讨论一遍。

### Why This Is The Right Next Slice

#### 1. `Phase 32` 已把 unified handoff result normalization 收干净

现在已经明确落地了：

- [task_generation_action_execute_request_handoff_result_normalization.go](/D:/code-task-processor/internal/listingkit/task_generation_action_execute_request_handoff_result_normalization.go:1)
- [task_generation_action_execute_request_handoff_result_shape.go](/D:/code-task-processor/internal/listingkit/task_generation_action_execute_request_handoff_result_shape.go:1)
- [phase32_action_execute_handoff_result_normalization_boundary_test.go](/D:/code-task-processor/internal/listingkit/phase32_action_execute_handoff_result_normalization_boundary_test.go:1)

这意味着：

- unified result normalization 责任已经从 broad result-shape owner 中被剥离
- 再围着 result normalization 本身做对称性打磨，收益已经下降

#### 2. 下一块真实压力已经转移到 mode-pairing seam 的镜像双轨

当前 mode-pairing seam 里最明显的 mixed concern 是：

- `runRetryable`
- `runQueue`

这说明根因不再是“谁拥有 unified handoff result normalization”，而是：

- 为什么一个 mode-pairing seam 还要沿 retry/queue 两条镜像路径同步演进

这比直接重开 shared helper owner 更像下一块真正的 bounded ownership hotspot。

#### 3. 这条线比 shared helper relocation 更适合做小切片

shared `queue/retry` clone helper 仍然被很多路径使用：

- action execute handoff
- navigation dispatch
- temporal result
- descriptor / conditional shaping

如果现在直接重开它们的定义位置，会很容易变成一个多 consumer 的大问题。

相比之下，mode-pairing normalization：

- 影响范围更集中
- 行为语义更清晰
- 可以先在 pairing layer 局部收一刀，而不碰别的 consumer

#### 4. 这条线还能顺着 `Phase 31/32` 刚建立的 seams 继续推进

当前 execute 邻域已经形成了清晰的局部结构：

- top-level handoff entry seam
- mode-routing seam
- mode-pairing seam
- branch-local invocation seams
- branch-local result seams
- result-normalization seam
- result-shape seam
- adaptation seam

所以下一刀继续收 mode-pairing mirror orchestration，会和前面几轮的 ownership 收口风格非常一致，不需要引入新的抽象风格。

### Why Not Reopen Shared Queue / Retry Clone Helpers First

不建议 `Phase 33` 继续优先追 shared `queue/retry` clone helper ownership，原因有三个：

#### 1. 共享性证据仍然很强

当前这些 helper 仍在多个路径里被直接使用，说明它们暂时仍然更像 shared utility，而不是已经明显该继续搬家的本地 seam。

#### 2. 现在去动 helper 定义位置，会把 slice 变大

一旦下刀，很容易把：

- action execute handoff
- navigation dispatch
- temporal result
- descriptor / conditional

这些路径都卷进来。

#### 3. mode-pairing mirror orchestration 更像眼前的根因

既然 mode-pairing seam 已经明确保留两条几乎同形的路径，就没必要跳过这个更近、更具体的 residual hotspot，直接去碰更宽的 shared helper 问题。

### Proposed Phase Shape

一个合适的 `Phase 33` 形状应该是：

1. 保持 `Phase 32` 的 result-normalization / result-shape / adaptation split 不动
2. 不重开 shared `queue/retry` helper 的 broader multi-consumer owner
3. 只围绕 mode-pairing mirror orchestration 下刀
4. 先明确哪些 retry/queue 镜像职责可以封装成更明确的 local pairing-normalization home
5. 把 outward retry/queue behavior 保持不变
6. 用 focused tests 和 ownership guardrail 锁住“harness 更清晰、outward behavior 不变”的方向

### Guardrails

`Phase 33` 建议保持以下 guardrails：

1. 不要回头重开 `Phase 32` 已稳定的 result-normalization / result-shape / adaptation split
2. 不要把范围扩成 shared `queue/retry` helper 的全面大改
3. 不要改变 action execute outward behavior
4. 不要引入新的 generic mirror-normalization framework
5. 优先把 mode-pairing 自己的 local mirror orchestration ownership 收清楚

### Recommended Next Step

下一步建议直接写一个 `Phase 33` implementation plan，主题明确限定为：

**ListingKit action execute handoff mode-pairing normalization ownership**

计划锚点应放在：

- [task_generation_action_execute_request_handoff_mode_pairing.go](/D:/code-task-processor/internal/listingkit/task_generation_action_execute_request_handoff_mode_pairing.go:1)

结论需要明确：

- `Phase 33` 应先聚焦 mode-pairing mirror orchestration ownership
- 不建议现在就全面重开 shared `queue/retry` clone helper ownership
- 不建议把这一刀扩大成 generic mirror-normalization abstraction
