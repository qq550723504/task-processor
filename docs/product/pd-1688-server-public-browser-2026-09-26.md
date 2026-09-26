# Product Decision: 1688 服务端匿名浏览器采集（允许处理验证码）

Product Decision: **PD-1688-SERVER-PUBLIC-BROWSER-2026-09-26**<br>
Status: **APPROVED / ACTIVE**<br>
Authority: 用户 2026-09-26 在实现会话中的明确决定（本决定由用户本人作出，非 Agent 自行放行）<br>
Supersedes: 下文列出的既有禁令条款

## 决定

用户明确决定：**允许服务端直接进行 1688 匿名公开商品页的浏览器采集，并允许自动处理站点验证码/风控挑战页**，以在不依赖用户 1688 登录态、不依赖本地执行器的前提下获得公开商品信息。

本决定**推翻**以下既有条款中被本决定直接覆盖的部分：

| 来源 | 被覆盖条款 | 状态 |
| --- | --- | --- |
| [Issue #398](https://github.com/qq550723504/task-processor/issues/398)「明确不做」 | 「不绕过站点访问控制、验证码或权限限制；获取失败必须如实失败/提示其他批准入口」 | `SUPERSEDED`（仅该条；#398 的其余匿名/无登录前提、SourceEnvelope→SRC-1→Catalog 契约不变） |
| `PD-1688-BATCH-VERIFY-ON-CAPTCHA-2026-09-21`（[batch design](../superpowers/specs/2026-09-21-1688-batch-local-agent-design.md) §1.6/§1.3） | 「不自动识别或绕过验证码」「不恢复服务端匿名裸 HTTP」 | `SUPERSEDED`（仅这两条；本地执行器路线本身仍有效，不是取消它） |

**未被本决定改变**的边界（继续有效）：

- 不使用、不接收、不持久化任何用户 1688 登录态、Cookie、密码、浏览器 profile、session token（本决定仍是**匿名**路径，不引入 SourceAccount/Connection）。
- SourceIdentity / SourceEnvelope / SRC-1 / Catalog 的当前 owner 与事实规则不变；采集结果仍是**不可信 acquisition evidence**，由当前 sourcing mapper/validator 构造 SourceEnvelope。
- 幂等、重放、COMMIT unknown、授权/租户隔离、资源上限等既有合同不变。
- 旧代码仍按 Hard-Cut 只做 `EXTRACT | RETIRE`：把仍有效的浏览器采集/提取行为抽取到**当前 owner**，不把旧 Service 直接接进新路径、不引入 public→account 回退或第二事实源。

## 已明确的已知风险（用户已知并接受）

用户在被明确告知以下事实后仍选择继续：

1. **站点条款与访问控制**：自动处理验证码属于规避站点反自动化/风控措施，可能违反 1688 服务条款，并存在法律与账号/来源风控层面的不确定性。
2. **匿名路径实测可靠性低**：[batch design](../superpowers/specs/2026-09-21-1688-batch-local-agent-design.md) §1.4 实测：同一 IP 被污染后，匿名非持久化配置 **0/8 成功**；真正 5/5 成功的是**登录 profile**。本决定不引入登录态，因此**服务端匿名采集预计会频繁遇到挑战页且可能不稳定**。
3. **服务端出口 IP 风控**：服务端出口 IP 为共享且易被标记；持续高频匿名采集可能导致该 IP 被 1688 风控，影响其它 1688 相关功能。
4. **验证码自动处理成功率有限**：验证码形态会变化，自动处理不保证长期有效，失败必须如实失败，不得伪造成功。

上述风险属于 `ACCEPTED_RISK`，依据为本决定（用户 2026-09-26）。

## 影响与后续准入

- 本决定改变的是**产品能力 / 外部副作用 / 访问控制边界**：新增真实浏览器执行、反自动化规避、验证码自动处理、以及新增的服务端出网副作用。
- 因此后续实现属于 **Independent Architecture**：必须先在当前 owner 名下产出独立架构文档（`internal/integration/acquisition/a1688` 的浏览器采集叶能力、隔离边界、验证码处理边界、失败/unknown、资源上限、Legacy `EXTRACT` 处置），完成独立 Architecture Review 并达到 `IMPLEMENTATION_READY` 后，Writer 才能修改生产业务路径。
- 本决定**不授权**：合并、部署、真实环境数据访问/迁移/删除、使用共享或他人账号、修改保护规则、对真实 1688 账号做自动化登录。

## 直接冲突引用处理

- `docs/engineering/src2b-public-acquisition.md` 中「not Browser Capture…」与「不绕过访问控制」相关表述，由架构阶段一并给出**最小局部**修订，不整篇改写历史。
- `docs/superpowers/specs/2026-09-21-1688-batch-local-agent-design.md` 的 §1.6/§1.3/§12 中与本决定冲突的两条，标注 `SUPERSEDED`（仅那两条），其余设计继续有效。
