# Shein 只读诊断 BFF 与隔离联调

Issue #323 的专用入口为 `GET /api/listing/shein-records/{record_id}/offline-diagnostic`。它只消费 #322 的显式 Go 应用，不修改默认 Go 生产装配。

## 请求与响应合同

- `action` 必填且仅 `save_draft | publish`；`expected_digest` 可选，若出现须为 `sha256:` 加 64 位小写十六进制。不接受 body、重复或未知 query、非 UUID record ID。原始 query 最多 1,024 字节。
- 浏览器 client 是 `src/lib/api/shein-diagnostic-client.ts` 的 `fetchSheinDiagnostic({recordId, action, organizationId, expectedDigest?, signal?})`。只发同源 GET、same-origin credentials、no-store 和 `X-Expected-Organization-ID`。类型在 `src/lib/api/shein-diagnostic.ts`：`SheinDiagnosticAction`、`SheinDiagnostic`、`SheinDiagnosticFailure`。
- 失败抛 `SheinDiagnosticError`，含 `status`、`code`、`payload`。Abort/网络异常保持原生；不自动重试，不保存结果。组织/用户/record/action 切换的取消及迟到结果隔离由页面 owner 负责。
- BFF 从既有 `serverAuth` / `readZitadelServerAccessToken` 取得 token，对照现有 `shuomi_effective_organization` cookie 和预期组织 header。缺 token 返回 401 `AUTHENTICATION_REQUIRED`；组织缺失/不匹配返回 409 `ORGANIZATION_CONTEXT_CHANGED`，均无上游请求。
- 服务端配置 `SHEIN_RECORDS_API_ORIGIN` 只允许一个完整 http/https origin（可末尾 `/`），禁止凭据、路径、query、fragment，无默认值或 `/api/v1` 回退。它不是 `NEXT_PUBLIC_*`。仅转发重新构造的 Accept、服务端 Authorization、X-Requested-Organization-ID；禁重定向及透传浏览器身份头/cookie。
- 成功字段保持 Go DTO 的 snake_case、版本、时间精度、digest、freshness、未评估范围和原因、checks/blockers/warnings，不重算规则/hash/TTL。必需数组拒绝 null/缺失。`diagnostic_only=true` 和 200 都不是发布许可。
- Go 错误保留 `{error,freshness?}`：400 invalid_request/unsupported_action/unsupported_target；403 permission_denied；404 not_found；409 stale_input；413 input_too_large；422 invalid_input；500 evaluation_failed/unsupported_rule_version；503 unavailable；504 deadline_exceeded。freshness 仅 stale_input 可带，含 status/coverage/causes。
- 身份/组织错误保留既有 Workbench `{code,message,requestId,fieldErrors}`。BFF 另有 502 DEPENDENCY_UNAVAILABLE（配置/连接），502 INVALID_UPSTREAM_RESPONSE（无效合同），504 DEADLINE_EXCEEDED（超时/取消）。错误 code 大小写保留。撤销/拒绝组织访问清理当前选择 cookie，沿用既有行为。
- 成功和错误均 private/no-store、nosniff。实际响应流最多 2 MiB，不信 Content-Length；拒绝重复 JSON key、无效 UTF-8、意外字段、错误 HTTP status/code 组合。15 秒 BFF 总预算为 Go 5 秒请求预算及响应留空间；入站 abort 终止上游 fetch/读取。

## 独占文件与 #324

#323 拥有 `src/lib/api/shein-diagnostic*.ts`、`src/lib/server/shein-diagnostic-proxy*.ts`、`src/app/api/listing/shein-records/[record_id]/offline-diagnostic/route.ts`、本 fixture 和文档。#324 只消费这些文件，实现页面/组件/hook/UI 测试。不得在 UI 新造 schema/client 或修改原有 catch-all。

## 启动隔离 fixture

需要本地 Go、Docker、Node、已执行 `pnpm install --frozen-lockfile` 的前端依赖。从仓库根目录执行：

```powershell
# 自动联调、Go 回归、退出验证和清理
node web/listingkit-ui/scripts/shein-diagnostic-fixture.mjs

# 供 #324 浏览器使用；保留进程，最长约 30 分钟
node web/listingkit-ui/scripts/shein-diagnostic-fixture.mjs --serve

# 用另一个已包含 #323 提交的独立 UI worktree 启动 Next
node web/listingkit-ui/scripts/shein-diagnostic-fixture.mjs --serve --web-dir C:/path/to/ui-worktree/web/listingkit-ui
```

Launcher 每次构建 Go **测试二进制**，创建随机命名的 `postgres:17.2-alpine --rm` 容器，端口只绑定 `127.0.0.1`，库固定为这个新容器内的 `issue323_fixture`，并在其中创建随机独占 schema。没有读取共享 DSN 或历史数据的入口。临时容器采用 trust 认证，只承载本次合成数据。

Go fixture 使用实际 Catalog Publisher，然后通过实际 HTTP POST 得到 `record_id`，不插入 Listing 成功行。实际 `NewSheinRecordApplication`、路由注册、身份/Organization 中间件、权限代码、record reader 和 v2 evaluator 保持原样。额外 context 服务器注册已有 Workbench context/switch module 与同一真实中间件，使实际 shell 能切换组织。

外部边界仅替换两处：测试二进制内一次性 token 的 verifier/grant loader；launcher 用一次性 AUTH_SECRET 和既有 Auth.js encode 发行合成 session cookie。实际 Next serverAuth 解密、session 回调、page proxy、BFF 路由及 HTTP 转发均未替换。生产文件无 fixture flag、自签 token 入口或默认管理员。**这不是实际 ZITADEL 登录验收。**

输出只打印 origin、record ID 和临时 `fixture.json` 路径；该文件含合成 session cookie，浏览器测试通过 `context.addCookies(manifest.sessions.owner)` 使用。不要把 cookie 值写入证据或提交。

- `origin`：实际 Next；`goOrigin`：诊断实际 Go；`contextOrigin`：实际 context Go。
- `recordId`：owner 的实际 POST 记录；`readonlyRecordId`：先 POST 再在外部 grant 替身撤销写权限的记录。
- `sessions.owner/other/admin/readonly/store/revoked/slow/unavailable`：独立合成身份。`store` 只有 Store read；`readonly` 有 Listing read 无 write；`slow/unavailable` 用于真实中间件超时/依赖不可用响应。
- 组织 `200`（Fixture A）与 `100`（Fixture B），既有 OrganizationSwitcher 可通过实际 BFF/context PUT 切换。记录属于 200；切到 100 再读同 ID 返回 404。
- `controlDirectory`：写入空文件 `restart` 后，等待 `restarted`，应用/repository 重建且 record ID 保留。写入 `stop-fixture` 让 launcher 正常退出，或前台 Ctrl+C。请等待进程结束及清理成功消息。

退出时 Go fixture 比较所有业务表内容及 xmin 行版本，确保 GET/浏览器诊断没有业务写入，然后删除自己的随机 schema。Launcher 停止自己的 Next/Go 进程，并按创建时记录的容器 ID 停止自己的 `--rm` 容器；不操作其他容器。临时 evidence.json、go.log、go-regression.log、next.log 留在控制目录供追溯。异常退出若来不及清理，可使用 manifest 中的自有容器 ID 执行 `docker stop <containerId>`，先核对 ID，勿批量清理。

自动模式验证实际 BFF GET、两 action、expected digest、一致性重读、非 owner/跨组织隔离、Listing read 与 Store read 区别、撤销、Go 超时、组织断言、缺 session、输入和 method 边界、真实 context、public session 不暴露 token。正常 Go suite 仅跳过这个显式启动 fixture；自动模式额外对同一隔离 PostgreSQL 执行相关 Go 回归，其测试使用各自随机 schema。
