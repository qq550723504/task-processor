# SHEIN 登录可靠取消设计

日期：2026-08-02

## 目标

第一期只解决 SHEIN 登录任务的可靠取消，不改变 Worker 的并发模型。用户在登录队列、浏览器启动、页面操作或等待验证码时点击“取消登录”，系统都应：

- 尽快停止对应的浏览器流程；
- 将登录 attempt 稳定地置为 `cancelled`；
- 不让 Worker 后续把已取消任务覆盖成成功或失败；
- 取消登录时保留原有 Cookie；
- 让前端显示最终状态，并允许再次登录。

## 当前问题

- 后端已有 `DELETE /api/v1/shein-login/accounts/:store_id/verify-code-wait`，可以通过 Redis 控制队列通知拥有浏览器会话的 Worker。
- 前端验证码弹窗的“取消”只执行本地 `setVerifyStoreID(null)`，没有调用后端取消接口。
- Worker 的取消信号目前主要在等待验证码阶段被消费。
- 浏览器启动和页面操作使用 Worker 的父 context，没有按 attempt 建立可取消的子 context。
- 取消接口会把 attempt 标记为终态，但 Worker 后续的完成逻辑仍可能无条件写入成功或失败。
- 当前 Worker 进程逐条同步消费登录消息；本期保持这一行为。

## 设计

### 1. 状态与取消竞态

保留现有状态集合，不新增公开状态。取消操作仍将 active attempt 置为 `cancelled`，并写入 `LOGIN_CANCELLED` 及用户可读消息。

在 Worker 完成 attempt 时增加 active-state 保护：只有当前持久化状态仍为 `queued`、`launching` 或 `waiting_verify_code` 时，才能写入 `succeeded` 或 `failed`。如果 API 已经写入 `cancelled`，Worker 必须保留 `cancelled`，关闭本地会话并确认消息。

持久化更新应使用 Redis 原子条件更新或等价的 compare-and-set 语义，避免“读取 active、用户取消、Worker 再写成功”的竞态。

### 2. Worker attempt context

Worker 处理每条消息时创建独立的 `attemptCtx` 和 `cancelAttempt`，并在 attempt 结束时释放。

Worker 同时启动一个轻量取消监听：监听该 attempt 的 Redis control key；收到 `cancel` 后调用 `cancelAttempt`。等待验证码阶段继续复用同一控制通道，但必须避免监听 goroutine 和验证码等待逻辑同时消费同一条控制消息。推荐由一个 attempt-level control loop 统一接收取消和验证码事件，再分别触发 context cancel 或验证码提交。

浏览器启动、页面操作、验证码提交和等待登录结果都使用 `attemptCtx`。取消后应执行：

1. cancel context；
2. 关闭当前 VerifySession/浏览器；
3. 保留 `cancelled` 终态；
4. acknowledge Redis stream message。

如果浏览器库在某个底层操作中不能立即响应 context，系统仍必须保证最终状态不会被覆盖，并在 session close 后释放资源。

### 3. API 语义

保留现有取消路由及租户校验。取消接口应对以下情况保持幂等：

- active attempt：发送取消信号并返回 `cancelled: true`；
- 已经 `cancelled`：返回成功，不重复创建控制消息；
- 已完成或没有 attempt：返回成功但 `cancelled: false`；
- attempt 不属于当前租户/店铺：继续返回现有错误。

取消登录不清理 Cookie。清理 Cookie 仍由现有 DELETE cookie 接口负责；它可以同时取消 active login，但两种操作的语义和 UI 文案必须保持区分。

### 4. 前端交互

新增 `cancelSheinLogin` API 函数和 `useCancelSheinLogin` mutation。

当店铺的 `login_in_progress` 为 true，或 latest attempt 状态为 `queued`、`launching`、`waiting_verify_code` 时显示“取消登录”。

- 队列/启动阶段：直接显示取消按钮；成功后刷新账户状态。
- 验证码弹窗：保留“关闭”作为仅关闭弹窗的动作，另提供明确的“取消登录”动作；取消请求成功后关闭弹窗。
- 取消请求进行中禁用相关登录操作，并显示“正在取消登录”。
- 取消失败时保留当前界面和 attempt 信息，显示错误并允许重试。
- 取消完成后显示“已取消”，待状态刷新确认后允许再次登录。

### 5. 并发边界

本期不修改 `RunWorker` 的串行消息消费，也不把 `maxConcurrentLogins` 直接改大。Redis 仍然保证同一店铺最多一个 active attempt。

并发扩展另立任务，必须同时验证浏览器 profile 隔离、验证码会话隔离、消息确认、Worker 重启恢复和资源上限。

## 数据流

```text
UI 点击取消
  -> DELETE verify-code-wait
  -> Redis: attempt=cancelled + control=cancel
  -> Worker attempt control loop
  -> cancel(attemptCtx)
  -> browser/session close
  -> CAS 保留 cancelled
  -> acknowledge stream message
  -> UI 轮询显示已取消
```

## 测试要求

### 后端单元测试

- 队列中取消：Worker 取到消息后跳过已取消 attempt。
- 启动浏览器期间取消：automation 收到 context cancellation，attempt 最终为 `cancelled`。
- 页面操作期间取消：session 被关闭，不能写入成功。
- 等待验证码期间取消：Worker 被唤醒并完成 `cancelled`。
- 取消与成功同时发生：使用 active-state CAS，取消结果不能被成功覆盖。
- 重复取消：幂等且不会产生多余控制消息。
- 取消后重新登录：旧 attempt 不阻塞新 attempt。

### 前端测试

- active login 显示“取消登录”。
- 点击后调用 DELETE endpoint。
- 验证码弹窗中的“关闭”不取消登录，“取消登录”才调用 mutation。
- mutation pending、成功、失败状态分别正确显示。

### 验证

- `go test ./internal/sheinlogin/...`
- ListingKit UI 相关 Vitest 测试
- `git diff --check`
- 手工验证至少覆盖队列态和验证码等待态；浏览器启动态使用可控 automation stub 验证。

## 非目标

- 本期不实现多登录并发。
- 本期不改变 SHEIN 登录 Cookie 生命周期。
- 本期不引入新的消息中间件或替换 Redis Streams。
- 本期不重构整个登录状态机。

## 验收标准

用户可以从 UI 取消任意 active SHEIN 登录；浏览器会话最终被关闭；attempt 状态稳定为 `cancelled`；原有 Cookie 不被删除；取消后的店铺可以再次发起登录；现有验证码提交、Worker 重启恢复和清理 Cookie 流程不回归。
