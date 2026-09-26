# 硕米最终 UI / IA Authority

> 状态：Active product authority  
> 校准日期：2026-09-05；ListingKit 退休定位澄清：2026-09-26（不代表全部节点或功能状态重新验收）  
> Figma Authority：页面 `31:463`「硕米官网」  
> 适用范围：最终产品信息架构、导航层级、页面归属、用户可见命名、交互语义与产品投影

## 1. Authority 决策

硕米最终成品的 **UI / IA / 页面命名 / 用户交互语义**，以 Figma 页面 `31:463`「硕米官网」中 **当前可见、非归档** 的设计为准。

该 Figma 不再只作为视觉参考，而是最终产品的 Product Projection Authority。

它回答：

- 最终用户看到哪些一级/二级/三级模块；
- 一个领域能力最终投影到哪个产品页面；
- 用户使用什么业务语言理解任务、店铺、商品、智能体与企业资源；
- Human Review、异常、执行进度等状态如何进入用户体验。

它不回答：

- 当前代码是否已经实现；
- 某能力是否已经通过 production gate；
- canonical facts、状态机、权限、幂等、审计等领域 ownership；
- Agent 是否可以绕过 deterministic validator 或人工授权。

因此，**最终原型 ≠ 当前 release capability**。

### 1.1 ListingKit 不是目标产品组成（2026-09-26）

用户重申：ListingKit 需要退休，当前产品以 Figma 设计为准。**旧 ListingKit 产品投影、Task-first 工作台及 root 混合架构不能作为长期子产品、默认入口或永久执行引擎保留。** 这不只是“不让它决定导航”，也不能在能力图中换成“内部引擎”继续保留同一旧 owner。

有效行为按 [Legacy Policy](../refactoring/legacy-hard-cut-policy.md)、[Register](../refactoring/legacy-register.md) 归当前 Product / Listing / Marketplace / Integration / App 等 owner，调用方切换后退休原路径；不增加 wrapper、fallback 或双事实源。Listing 领域事实继续存在，不等于保留 ListingKit。

`web/listingkit-ui` 中已经符合 Figma 和当前领域合同的 Console / BFF / 共享组件继续复用；旧 Task-first 页面逐项退出。真实目录、命令或配置名称尚未修改只表示工程现状，不是产品保留决定，也不授权仅凭名称删除合格实现。本文不宣称物理退休已完成，不授权业务迁移、部署或操作真实数据。

## 2. 当前最终一级信息架构

以当前可见、非归档设计为准，一级模块为：

1. 运营驾驶舱
2. AI工作台
3. 供应市场
4. 智能市场
5. 工具市场
6. 生态服务
7. 数据服务
8. 店铺中心
9. 套餐与权益
10. 我的账户

当前设计中，历史「商品中心」已归档；不建立独立顶层 `Product Center` / `Listing Center` 作为最终 IA 前提。

`ProductSnapshot`、Platform Draft、Listing、ApprovedAsset 等仍然可以并且应该作为稳定领域事实存在，但 **领域对象不等于顶层导航菜单**。

## 3. 关键当前设计节点

以下节点用于固定当前产品语义：

- `393:321`：工作台状态 / 运营驾驶舱展开
- `427:2467`：二级页面 / AI工作台 / 硕米Chat
- `427:3005`：二级页面 / AI工作台 / 任务中心
- `428:323`：二级页面 / 智能市场 / 智能体市场
- `1560:359`：店铺中心 / 我的店铺 / 优化版
- `429:5323`：店铺中心 / 店铺商品
- `432:4483`：二级页面 / 我的账户 / 企业空间

若后续 Figma 明确更新为新的可见、非归档终稿，应同步更新本 Authority 文档与 GitHub Roadmap，而不是继续引用旧归档 Frame。

### 3.1 账户中心 v1 阶段边界（历史记录，2026-09-19，已被当前 Issue 范围取代）

该阶段继续使用 Figma `31:463` 约束“我的账户”的导航命名、页面层级、布局和交互语义，交付范围曾限定为真实本人资料、当前企业与导航、成员及源账号操作记录、个人推广码/链接和关系统计。该范围已由本 Issue 的 Account Center — Figma Parity Delivery Batch 取代，仅保留作历史证据。

当时未开放的组件不得据此推断当前范围；本批次应以 3.2 及 Issue #408 最新产品决定为准，仍然不得通过示例数字或前端假保存伪造事实。现有账户页面继续显式表达 loading、error、empty、denied、unavailable 和只读状态，且不得删除已确认的源账号操作记录。

相关设计组件分类以 `docs/engineering/issue348-account-ui.md` 的 Profile `432:534`、Enterprise `432:4483`、Management `1631:490`/`1631:497`/`1631:504`、Resources `1634:359`/`1636:371` 映射为当前 v1 的视觉与语义约束；成员、操作记录和个人推广组件按上述 v1/未来边界记录在 Issue #408。若无 Figma 编辑权限，不通过其他渠道修改原型，仅在 Issue 中保留待标注项。

### 3.2 Account Center — Figma Parity Delivery Batch（2026-09-20）

Issue #408 的最新产品范围取代 3.1 的缩减版 v1 阶段边界。本批次以 Figma `31:463` 当前可见、非归档账户中心为完成目标，连续交付账户总览、账户设置/经营画像/认证信息、成员与角色权限、资源/套餐/额度与成员资源分配、企业通用审计、推广收益与提现。Figma 仍只决定 UI/IA、命名、布局和交互语义；真实事实、权限、幂等、账本和提现状态必须由当前 owner/API/persistence 提供，不得用原型示例数字替代。

本批次按 M1–M5 推进，一个主要分支和一个主要 PR；M3 使用 Token set-target allocation、企业 entitlement window 和 version/idempotency；M5 使用个人 referral、10% minor-unit immutable ledger、14 日结算、退款 adjustment、¥100 人工提现和 version/idempotency 状态机。支付事实仍以 commercial/payment owner 的真实 settled cash payment、refund 和 chargeback 为唯一来源，不得由 referral 或前端推造。当前已接入的 M1 经营画像由账户中心持久化；身份认证状态仍以 ZITADEL 与当前组织授权事实为准。

M5 提现申请还必须消费现有 canonical payout-method owner 的有效收款方式事实；渠道枚举本身不构成收款方式。当前仓库尚无该 owner，因此申请提现接口在 owner 接入前 fail closed，不能把任意 `ALIPAY` / `BANK_TRANSFER` 值当作已验证收款方式。

## 4. AI工作台的产品对象

AI工作台当前产品结构至少包括：

- 硕米Chat
- 任务中心
- 项目中心
- 知识库
- 我的报告

其中「任务中心」是 **用户可见的业务任务中心**，不是内部 Workflow / Queue / Temporal Task Dashboard。

推荐对象关系：

```text
BusinessTask                 # 用户可理解、可跟踪、可决策的业务任务
  -> AgentRun                # 单次 Agent 运行
    -> AgentStep
      -> ToolCall / ModelCall
        -> Temporal / Queue / Internal Task
```

因此：

- BusinessTask 可以是长期产品对象；
- AgentRun 只拥有单次 Agent 推理/执行状态；
- Temporal / Queue / internal task 仍是执行基础设施；
- 不允许把内部 task ID、workflow 状态或 retry owner 直接提升为主要产品导航。

任务中心当前用户语义包括：

- 执行中
- 待确认 / 待你处理
- 已完成
- 异常任务
- 已暂停
- 工作范围
- 硕米建议
- Human Review / 用户显式决策

## 5. Product / Listing 的产品投影

共享 Product / Listing 领域模型继续保留，但最终 UI 不要求独立 Listing Center。

当前推荐产品路径为：

```text
供应市场
  -> 获取 / 接入 / 选择供给
AI工作台
  -> 分析 / 生成 / 优化 / 决策 / 执行
店铺中心
  -> 我的店铺
  -> 店铺商品
  -> 订单履约
```

Marketplace 平台（SHEIN / TEMU / Amazon）应作为共享 marketplace/listing capability 与店铺商品投影的维度扩展，不复制平台专属产品孤岛，也不为了共享领域模型重新建立顶层 Listing Center。

## 6. Multi-Agent 的最终方向与当前执行顺序

Figma「智能市场 / 智能体市场」明确表达了最终多专业智能体产品方向。

因此：

- Multi-Agent / 多专业智能体是最终产品方向；
- 当前 Roadmap 的「不先做 Multi-Agent」仅表示工程执行顺序；
- 首期仍应先证明单个 bounded Agent + Commerce Tools + Human Review 的安全性和收益；
- 不因为最终产品有多个智能体，就提前建立第二套 Runtime、Tool Contract、权限、事实源或状态机。

## 7. Authority 层级

发生冲突时按以下职责判断：

1. **Figma `31:463` 当前可见、非归档终稿 + 本文档**  
   决定最终 UI / IA / 页面命名 / 用户交互语义 / Product Projection。
2. **产品战略与 Architecture Specs**  
   决定业务规则、canonical facts、领域 ownership、安全边界、Tool/Agent contract、权限、幂等与审计；不能恢复 §1.1 已明确退休的旧产品或架构。
3. **`docs/refactoring/current-refactoring-status.md`**  
   提供注明基线的 implementation / 成熟度记录；今天的代码、接线和验收以准确 HEAD 的 Issue / PR / runtime 证据为准，不把历史状态当实时状态。
4. **GitHub Roadmap Authority（#137）与具体执行 Issue**  
   决定当前工程执行顺序、任务范围、依赖与操作权限。

冲突处理规则：

- 用户可见模块名称、导航归属与交互语义：优先遵循最终 Figma；
- canonical facts、状态机、安全、权限与副作用控制：优先遵循领域/架构 contract；
- Figma 中的「已开放」「可用」等产品状态不得覆盖 production capability gate；
- repository implemented 不等于 production-ready；
- 旧归档 Frame、旧 Listing Workspace/Task-first 文档不得覆盖当前最终 IA；
- 历史 ListingKit 战略或执行计划不得以“内部执行引擎”为理由覆盖既有 EXTRACT / RETIRE 决定。

## 8. 产品定位

硕米智能引擎的最终定位为：

> **以 AI 工作台和专业智能体为核心，以供应链、店铺、数据、工具和生态能力为上下文的 AI 电商经营平台。**

Product、Asset、Listing、Marketplace、Store、Resource 等是支撑该产品的当前领域能力；它们不反向决定最终导航结构。**ListingKit 不再列为目标架构中的领域能力或长期执行引擎**：其有效行为由当前 owner 承接，旧产品投影和混合装配按 EXTRACT → RETIRE 收口。
