# ZITADEL 多 Organization 授权验证

```yaml
real_environment_status: pending
observation_time: pending
issuer_host: pending
http_status: pending
identifier_suffixes:
  subject: pending
  home_organization: pending
  project: pending
  organization_a: pending
  organization_b: pending
organizations:
  - name: ListingKit Acceptance Organization A
    role_keys: [listingkit_admin]
  - name: ListingKit Acceptance Organization B
    role_keys: [listingkit_viewer]
revocation_propagation_status: pending
```

本轮只完成了 loopback-only、显式确认、幂等 provision 的代码和合成测试；没有运行
`provision-multi-org-acceptance`，也没有创建、修改、删除或恢复真实 ZITADEL 资源。
因此不能声称真实双 Organization 验收通过。

获得修改本地可重置 ZITADEL 测试数据的明确批准后，按本地验收文档运行 opt-in
命令。随后通过现有 Auth.js 流程登录，服务端使用其持有的浏览器 bearer token 调用：

```text
POST /zitadel.authorization.v2.AuthorizationService/ListAuthorizations
Connect-Protocol-Version: 1
```

只记录脱敏后的 issuer host、标识符后缀、Organization 名称、role key、HTTP 状态和
观察时间。验收必须观察到两个 active 项，并逐项确认：用户 ID 等于 introspection
得到的 subject；两个项目 ID 都等于 runtime 中的 ListingKit Project；用户的 Home
Organization 保持相同；授权 Organization ID 在 A、B 之间不同；A 仅有
`listingkit_admin`，B 仅有 `listingkit_viewer`。

角色撤销传播是另一项外部变更。只有再次获得明确批准后才能撤销和恢复授权；在此之前
保持 `revocation_propagation_status: pending`。

## local_verification（2026-08-31）

```yaml
observed_at: 2026-08-31T07:18:11+08:00
commit_range: 131438de02804f214b8ee7ae1f5a7315448eb2dc..57c28c78e95426ea97741a3949777e6780e44e23
head: 57c28c78e95426ea97741a3949777e6780e44e23
local_test_status: pass
api_runtime_smoke_status: pass
ui_runtime_smoke_status: pass
enabled_workbench_runtime_status: pending
real_environment_status: pending
revocation_propagation_status: pending
```

本节只记录本机合成测试和不含秘密的禁用 Workbench 运行态尝试，不构成真实 ZITADEL
验收，也不能据此称该切片 production-ready。

### 新鲜命令证据

- `go test ./internal/zitadelprovision ./internal/zitadelprovision/cmd ./internal/authidentity ./internal/authruntime/zitadel ./internal/workbenchcontext/... ./internal/httproute ./internal/app/httpapi -count=1 -race`：退出码 0，8 个包通过。
- 聚焦 Vitest 命令（Task 10 简报列出的 10 个文件，包含 `application-frame.test.tsx` 和 `zitadel-auth.test.ts`）：退出码 0，10/10 文件、120/120 测试通过。
- `npm.cmd test -- --maxWorkers=50%`：退出码 0，286/286 文件、1868/1868 测试通过；输出同时包含 12 条 CSS 解析告警，没有测试失败。
- `npm.cmd run typecheck`：退出码 0。
- `npm.cmd run lint`：退出码 0，0 error、14 warning；告警位于既有 SHEIN/preview 文件，不在本切片变更范围。
- `npm.cmd run build`：退出码 0；Next.js 16.3.0 production build 编译成功并生成 46/46 个静态页面单元。

补充的具名 Go race 运行也以退出码 0 验证了：伪造 Organization 目标在 grant lookup
前被拒绝；`live_write`/`live_switch` 绕过缓存且失败后失效；缓存键不含 bearer、TTL
上限为 60 秒并受 token 到期时间约束；中间件顺序为 authentication → Organization
resolution → scoped role → handler；A-admin 不会泄漏到 B-viewer；Workbench 不走 legacy
allowlist/数值 tenant fallback。聚焦前端命令覆盖了不可信 header/cookie 拒绝、撤销/拒绝时
清 cookie，以及切换成功或失败时先清 Organization-scoped query。

### 运行态冒烟

- 冒烟前 `127.0.0.1:58085` 和 `127.0.0.1:53000` 均无监听，也没有属于该 worktree
  的既有 Node/Go 进程。
- API 二进制构建退出码 0。首次直接使用 `config/config-test.yaml` 时，配置中的 shared
  OpenAI 与 image client API key 为空，进程在监听前以退出码 1 结束。随后仅用
  `apply_patch` 临时把 shared key 改为明显的非秘密占位值、把 shared base URL 改为
  `http://127.0.0.1:9/v1`，并关闭 API/ListingKit auto-migration；image client 通过继承
  shared 配置通过校验。禁用 Workbench 的 API 以本轮 PID `18528` 在
  `127.0.0.1:58085` 建立唯一 task-owned listener，`GET /health` 返回 200 和
  `{"status":"ok"}`。进程停止后监听数为 0；临时配置两行已通过 `apply_patch` 逐字恢复，
  `git diff -- config/config-test.yaml` 为空，index/worktree 规范化 hash 相同。
- 已构建 UI 使用 `LISTINGKIT_SERVICE_API_BASE=http://127.0.0.1:58085/api/v1` 在
  `127.0.0.1:53000` 启动；`GET /healthz` 返回 200 和 `{"ok":true}`，`GET /` 返回
  200，`<title>` 为“硕米智能引擎 | 新一代 AI 电商智能操作系统”，页面包含产品标题。
- 只停止了本轮启动的进程；结束后两个端口的监听数均为 0。启用 Workbench 的运行态会
  需要真实 issuer/凭据，未启用并保持 pending。

### 日志与秘密审计

审计范围为上述 commit range 的 74 个变更文件（其中 41 个生产文件）、Task 1–9
报告/审查记录和本轮临时运行日志。变更生产代码中没有新增显式日志发射调用；错误路径不
序列化 bearer、cookie、client secret、credential 或原始授权响应。测试中的
`server-token` 等值仅为合成夹具；扫描未发现 JWT 形态值或真实凭据赋值。本轮 API 日志
首次只出现空 API-key 字段的校验说明；成功冒烟的 632 行 API stdout 中敏感词和外部
HTTP URL 命中数均为 0，UI 日志也没有授权敏感词，均未打印值。

### 仍待外部门禁

没有运行 `provision-multi-org-acceptance`，没有读取验收凭据，没有调用真实 issuer，
也没有创建、修改、撤销或恢复任何 ZITADEL grant。真实同一 subject 的双 Organization
A-admin/B-viewer、真实撤销后的即时 write/switch 拒绝、60 秒内 cached-read 失效和恢复
传播仍全部 pending。禁用 Workbench 的本地 API/UI 冒烟已经通过，但启用 Workbench 的
运行态仍需要真实 issuer/凭据；在这些外部门禁完成前仍不能声称 production-ready。
