# 验收环境拆分：RUN-1 与跟随生产的完整环境

**状态**：**拆分决策已准入（IMPLEMENTATION_READY，2026-09-27）**；FULL 部分**经用户决定暂缓**；§3.1 为**待确认的提案**，未获裁定，且其**待裁定范围已于 2026-09-29 收窄**（见下）。本文不含实现。

**准入记录**：独立只读评审（chatgpt-codex-connector），9 条 finding 全部处理完毕（0 未解决）。其中多数指出的都是我文档里的**事实错误**，已逐条更正而非绕过：

- 装配面自述清单（写了两次都不完整）→ 改为「装配面由 `buildCurrentApplication` 决定，本文档不自述清单」
- 误判 `accountallocation` 无 schema-init 入口 → 实为 `cmd/listingkit-schema-migrate --scope commercial` 已存在，改为复用
- 误判「只读 `commercial_runtime` 会被启动拒绝」→ 预检只校验 ACL，不校验 `default_transaction_read_only`
- §4 与 §3.5 对 FULL 增量的表述自相矛盾 → 已统一
- 把 harness 实现混进设计 PR → 拆出至 #549
- 误述「启动时每个组合模块都做 fail-closed 权限校验」→ **对商业域不成立**，已按代码核实并在 §3.1 更正

**当前执行决定（用户 2026-09-29，#541）**：先在 #541 修复 current-application manifest，默认不提供 `commercialOwnerDatabase`，不把旧字段改名、升级角色或继续补 GRANT。保持现有 SA1／账号资料／主体认证 required + forbidden 校验及商业权限矩阵。A/B 独立推进 owner 确认，不作为本修复前置；FULL 继续暂缓，出口代理与真实采集成功路径不属于 #541 启动验收。

**未获裁定的部分**：§3.1 的长期提案仍需**账号中心与认证 owner**确认；商业域的独立问题是**测试绑定路径的目标和完整 runtime 权限合同**。当前 RUN-1 不连接商业 owner，保留既有商业授权，确认前不改矩阵。以下历史四层诊断不恢复已撤回的开工前置。
**来源**：#541（四层漂移诊断）、#390（RUN-1 harness，原已关闭）
**取代**：`docs/operations/current-application-run1.md` 中关于「当前应用只装配哪些域」的隐含假设

## 1. 为什么要拆

RUN-1 自 2026-09-20 起无法启动，#541 逐层定位出**四个互相独立**的漂移：商业角色名、SA1 运行时授权、商业只读授权、以及一个 RUN-1 按定义**不应装配**的域（`accountallocation`）的表。

这四层是历史诊断。第 3、4 层来自商业测试绑定路径，不能推定为当前生产二进制的启动链（见 §3.1.1）。当前确认的首个启动缺陷是 launcher 仍输出已删除的 `commercialDatabase`；先修该配置合同，再观察真实启动结果。拆分保留不同验收目标，不以逐层扩大 RUN-1 schema/GRANT 来追随生产。

因此把两类验收目标拆到两个环境，各自定义自己的 schema/授权范围。

## 2. 两个环境

| | **RUN-1（保留）** | **FULL（新增，跟随生产）** |
|---|---|---|
| 目的 | 身份 / 生效组织 / 账户路由的**隔离**验收 | 商业只读、SA1 + 账号中心 + 认证、**1688 采集**的整链验收 |
| 装配模块 | 由 `cmd/current-application` / `buildCurrentApplication` 的当前默认组合决定，包含账号资料、主体认证及统一商业概览；本 launcher 不提供商业 owner 或采集配置，不能用早期路由数量代替实际清单 | current-application 组合到的全部模块，外加独立的采集进程（见 §3.5） |
| schema 范围 | 仅其装配模块的域 | 全部组合域，随生产演进 |
| 授权范围 | 同上 | 各域自己的 admitted boundary |
| 生命周期 | 保留现有 start/stop/restart/destroy 边界 | 同源复用，不另造一套 |
| 漂移触发 | 少 | 生产加域时同步（这就是它存在的意义） |
| 归属 | #541 | 本设计 |

## 3. 关键裁定

### 3.1（**提案，待相关域 owner 确认**）启动校验按「已组合模块」执行，而非全局边界

source-account pool 的 `VerifyRuntimePermissions` 检查 required 和 forbidden privileges。当前默认组合**实际包含账号资料与主体认证**，共用 source-account pool；harness 已授予相关表和审计序列权限，不能把它们视作未组合或尚未授权的模块。长期若需支持部分模块组合，再由 owner 明确具体未组合模块及其权限合同。

**提案**：required privileges 可按实际组合模块集合计算；未组合模块不要求其必需权限，同时必须继续拒绝所有未准入的额外权限。未组合模块不要求 required privileges，与不检查越权是两回事。当前 #541 恢复保持现有组合、ACL 和启动校验，不实施本提案。

> **这不是本文可以单方面裁定的事项。** 它会改变相关域的启动校验语义，属于它们的边界，需要这些域的 owner 明确同意后才能作为实现依据。

#### 3.1.1（**2026-09-29 事实更正**）商业域没有启动校验，待裁定范围因此收窄

本文初版把四个漂移统一归因为「启动时每个组合模块都做 fail-closed 权限校验」。按代码核实后，**该表述对商业域不成立**：

| 域 | 生产组合路径是否校验 admitted boundary |
|---|---|
| source account（SA1） | **是** —— `current_application.go` 的 `buildSourceAccount` 工厂内调用 `sourceaccountstore.VerifyRuntimePermissions` |
| commercial read | **否** —— 生产组合走 `buildUnifiedCommercialRead`，**不调用任何校验** |

`VerifyCommercialReadSchema` 在全仓库只有一个调用方：`buildCommercialReadModuleFromDatabase`。而该函数的**非测试调用方为零**，只有 `commercial_read_schema_binding_postgres_test.go` 使用；生产组合使用的是另一个函数（`composition_builder.go` 里的 `buildCommercialReadModule`），**不校验**。`VerifyStoreQuotaRuntime` 同样只有测试调用方。

**因此**：

1. **基线 `569555f3…` 的首个缺陷是 manifest 解码失败。** PR #568 查出：launcher 仍发送被 `f49a58be3` 移除的 `commercialDatabase`，`LoadConfig` 的 `DisallowUnknownFields` 因而拒绝配置。该缺陷在 #541 内修复：current-application 不再发送旧字段，也不自动补 `commercialOwnerDatabase`；后者要求 `commercial_owner_runtime` 并接入另一组能力，不是机械字段／角色改名。其他模式配置保持原状。商业概览可认证访问，缺失资源／店铺事实为 `unavailable`，不记作商业成功路径通过。解码通过也不预先承诺没有其他启动缺陷。
2. 第 3、4 层的**来源判定**仍然成立：它们当时来自 `buildCommercialReadModuleFromDatabase` 这条只在测试里走的绑定路径（它要求 `accountallocation` 的表），生产二进制启动不经过它。只是它们已不是当前的阻塞点。
3. 商业域**不存在**「启动校验语义」可收窄，因此**不需要商业域 owner 就 §3.1 表态**，否则会让 owner 面对一个不存在的问题。待裁定范围收窄为**账号中心与认证两个域**的启动校验语义。
4. 商业域的 B 问题须结合实际依据确认：`accountallocation` installer 显式向 `commercial_runtime` 授予 member-token 四表 SELECT/INSERT/UPDATE，`listingsubscription/ai_usage.go` 消费 allocation 和 lock 表。因此不能因表名就认定权限误写。请 owner 确认完整 runtime 权限合同是否保留，以及辅助函数的测试目标是「商业概览读取」还是「完整商业 runtime 角色」；若为前者，明确如何分离且保留有效额度／计量安全测试。**确认前不改权限矩阵**，也不反向推定生产应新增启动校验。正常应用走 `buildUnifiedCommercialRead`，B 不作为 manifest 修复前置。

> **这构成对已准入设计的事实更正，不构成裁定。** 更正只缩小待裁定范围，不新增任何实现授权。

### 3.2 FULL 环境的域清单必须显式列出

不写「装所有域」，而是列出 FULL 组合的域，并在每个域旁标注其 schema 安装入口与授权边界。生产新增域时，**由新增域的 PR 负责把 FULL 清单补齐**（用清单缺失测试守住），而不是靠事后有人想起来。

> 这一条是防复发的关键：把「漂移」变成「CI 失败」而不是「某人某天发现环境起不来」。

### 3.3 `accountallocation` 已有 schema-init 入口（**更正**）

本文早期版本称该域「没有 CLI」，这是**错的**：`cmd/listingkit-schema-migrate` 早已提供 `--scope commercial`，并由 `internal/app/runtime/listingkitschemamigrate/runtime.go` 调用 `accountallocationschema.Migrate`。

**裁定**：FULL 直接复用该入口，**不新增**第二个 schema-init 命令——否则同一 schema 会有两个 owner 并各自漂移。若后续发现它无法满足需要（例如 manifest 契约、连接上限、安装后自校验），应先记录它具体不满足什么，而不是假定它不存在。

### 3.4 RUN-1 当前修复范围

前三层角色／授权对齐已由 #549 合入，作为现状保留：

1. 商业角色 `commercial_reader` → `commercial_runtime`
2. SA1 运行时补账号中心/认证域授权（**更正**：这些表 SA1 schema 初始化本就安装，缺的只是授权）
3. 商业只读补 usage/audit 表授权

本次仅修过期 manifest 和直接相关说明／测试断言；先验证真实 `LoadConfig`，再验证 READY、check、stop/start/restart 的事实保留。A/B 征询独立推进，现有权限合同保持不变，不继续补 GRANT。出口代理和挑战墙只影响另一路真实采集验收，不决定 #541 启动修复是否完成；RUN-1 不承载采集配置，FULL 暂缓。

### 3.5 FULL 还需要独立的采集进程

FULL 的目标是覆盖 1688 采集的端到端，而采集**不是一个模块**：按 D13，它是**独立的、无数据库凭据的 Chromium 进程**，有自己的调用方准入（服务凭据）与网络边界，且 `current-application` 明确**不**启动浏览器。

因此仅做 schema/授权的增量**不足以**让 FULL 覆盖采集路径。FULL 还必须包含：采集进程的启动、就绪、监管、停止与销毁，以及它自己的凭据生成与私密处理。若不做，FULL 只能验证到「应用能起来」，验证不了「提交链接 → 读回商品」。

> 这也印证了 §4.1 的判断：FULL 不是「RUN-1 加点 schema」，而是一套新的共享验收基础设施。

## 4. 复用与不重复建设

FULL 复用 RUN-1 已有的：run-id/端口分配、私有目录与凭据、Zitadel 与代理、zero-write 基线、`stop`/`restart` 与 `destroy` 的边界语义。**不新建第二套生命周期实现**。

差异**不止** schema/授权清单：还包括 §3.5 的采集进程（启动、就绪、监管、停止、销毁，以及它自己的凭据生成与私密处理）。只做 schema/授权增量得到的 FULL 只能证明应用起得来，证明不了「提交链接 → 读回商品」。

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
