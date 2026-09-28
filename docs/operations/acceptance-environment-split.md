# 验收环境拆分：RUN-1 与跟随生产的完整环境

**状态**：Draft，待准入。本文只做拆分设计与边界裁定，不含实现。
**来源**：#541（四层漂移诊断）、#390（RUN-1 harness，原已关闭）
**取代**：`docs/operations/current-application-run1.md` 中关于「当前应用只装配哪些域」的隐含假设

## 1. 为什么要拆

RUN-1 自 2026-09-20 起无法启动，#541 逐层定位出**四个互相独立**的漂移：商业角色名、SA1 运行时授权、商业只读授权、以及一个 RUN-1 按定义**不应装配**的域（`accountallocation`）的表。

第 4 层是决定性的：它说明 **current-application 组合的域集合已经宽于 RUN-1 声明的范围**，而启动时每个组合模块都做 fail-closed 权限校验。继续逐层补 GRANT 会让 RUN-1 永久成为「生产的影子」——生产每加一个域，就要回来改一次，这正是本次连续暴露的成因。

因此把两类验收目标拆到两个环境，各自定义自己的 schema/授权范围。

## 2. 两个环境

| | **RUN-1（保留）** | **FULL（新增，跟随生产）** |
|---|---|---|
| 目的 | 身份 / 生效组织 / 账户路由的**隔离**验收 | 商业只读、SA1 + 账号中心 + 认证、**1688 采集**的整链验收 |
| 装配模块 | 仅其声明的四条身份/账户路由 | current-application 组合到的**全部**模块 |
| schema 范围 | 仅其装配模块的域 | 全部组合域，随生产演进 |
| 授权范围 | 同上 | 各域自己的 admitted boundary |
| 生命周期 | 保留现有 start/stop/restart/destroy 边界 | 同源复用，不另造一套 |
| 漂移触发 | 少 | 生产加域时同步（这就是它存在的意义） |
| 归属 | #541 | 本设计 |

## 3. 关键裁定

### 3.1（**提案，待相关域 owner 确认**）启动校验按「已组合模块」执行，而非全局边界

各域的 `VerifyRuntimePermissions` 断言的是**该域 admitted boundary 的全部表**。当一个进程只组合部分模块时，断言整域边界会要求它拥有用不到的权限 —— 这既是 RUN-1 装不下的原因，也是一个真实的最小权限缺口：进程持有同库其他域的表权限，就能在自己的 handler 里读到那些域的数据。

**提案**：权限校验以**实际组合的模块集合**为界。未组合的域不参与校验，其表也不授权。

> **这不是本文可以单方面裁定的事项。** 它会改变**商业、账号中心、认证三个域**的启动校验语义，属于它们的边界，需要这三个域的 owner 明确同意后才能作为实现依据。在得到确认前，RUN-1 的商业授权范围（见 §3.4 第 3 条）也随之待定。

### 3.2 FULL 环境的域清单必须显式列出

不写「装所有域」，而是列出 FULL 组合的域，并在每个域旁标注其 schema 安装入口与授权边界。生产新增域时，**由新增域的 PR 负责把 FULL 清单补齐**（用清单缺失测试守住），而不是靠事后有人想起来。

> 这一条是防复发的关键：把「漂移」变成「CI 失败」而不是「某人某天发现环境起不来」。

### 3.3 `accountallocation` 需要 schema-init 入口

该域有 `internal/app/schema/accountallocation`（含自带的 `commercial_runtime` 授权），但**没有 CLI**。FULL 环境需要安装它。

**裁定**：新增 `cmd/account-allocation-schema-init`，与 `source-account-registry-schema-init` 同构（私有 manifest + 连接上限 + 安装后自校验），而不是在环境装配的 Go 测试里直接调 `Migrate` —— 后者会把 DDL 藏进测试二进制，正是设计里禁止的「请求内 DDL」的变体。

### 3.4 RUN-1 的三层修复仍然要做

即使拆分，RUN-1 自身的漂移仍需修（否则它继续坏着）：

1. 商业角色 `commercial_reader` → `commercial_runtime`
2. SA1 运行时补账号中心/认证域授权（**更正**：这些表 SA1 schema 初始化本就安装，缺的只是授权）
3. 商业只读补 usage/audit 表授权

但**第 3 层要按 §3.1 收窄到 RUN-1 实际装配的范围**，而不是继续加到与生产一致 —— 否则拆分就没有意义。

## 4. 复用与不重复建设

FULL 复用 RUN-1 已有的：run-id/端口分配、私有目录与凭据、Zitadel 与代理、zero-write 基线、`stop`/`restart` 与 `destroy` 的边界语义。**不新建第二套生命周期实现**；差异只在「装哪些域、授什么权」这一个清单上。

## 4.1 交付状态：FULL 暂缓（用户决定 2026-09-27）

**FULL 环境本轮不建设。** #514 的能力已分别验证：真实 PostgreSQL 下的 schema/授权/持久化语义有真实 PG 证据，真实 1688 采集有成功样本。FULL 的增量价值是把两者串成一次点击，而这需要新增一整套共享验收基础设施。

因此本文当前的作用是**决策记录 + RUN-1 修复的依据**，不是 FULL 的实现基线。FULL 若将来要做，本文 §2/§3.2/§3.3 直接可用，但需重新确认其与最新生产的差异。

## 5. 对 #514 的影响

#514 的端到端验收（提交链接 → 发布 → `GET /product` 读回）挂在 **FULL** 上，RUN-1 不承载采集模块（它没有 `productAcquisitionDatabase`，也不该有）。

## 6. 验证

1. RUN-1：`start` 返回 `READY`，`check` 通过，`stop`/`restart` 往返保留事实，`destroy` 仍为独立破坏性边界
2. FULL：同样四项，且 `GET /api/v1/account/profile` 未认证返回 401（现有就绪探针）
3. **清单缺失测试**：FULL 声明的域若缺少 schema 安装入口或授权边界，CI 失败
4. 现有 lifecycle / contract / current-application 测试全绿

## 7. 未决

1. §3.1 改动各域启动校验的准入范围与排期
2. FULL 的域清单初始成员（除已知的 SA1/账号中心/认证/商业/account allocation 外是否还有）
3. `account-allocation-schema-init` 的 manifest 契约是否复用现有 DatabaseConfig
4. 谁负责在生产加域时同步 FULL 清单（建议做成 CI 门禁而非流程要求）

## 8. Legacy 决策

- **RETIRE**：不恢复任何 legacy crawler 的 browser/profile/service 路径；`url_helper.go GetMobileURL` 仍为死代码，不得作为既有能力引用。
- 本设计与实现不触碰 `internal/crawler/*`。
