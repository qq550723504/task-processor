# 硕米最终 UI / IA Authority

> 状态：Active product authority  
> 校准日期：2026-09-05  
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
   决定业务规则、canonical facts、领域 ownership、安全边界、Tool/Agent contract、权限、幂等与审计。
3. **`docs/refactoring/current-refactoring-status.md`**  
   决定当前 repository implementation reality、Now / Next / Later 与已验证证据。
4. **GitHub Roadmap Authority（#137）**  
   决定当前工程执行顺序与 backlog 映射。

冲突处理规则：

- 用户可见模块名称、导航归属与交互语义：优先遵循最终 Figma；
- canonical facts、状态机、安全、权限与副作用控制：优先遵循领域/架构 contract；
- Figma 中的「已开放」「可用」等产品状态不得覆盖 production capability gate；
- repository implemented 不等于 production-ready；
- 旧归档 Frame、旧 Listing Workspace/Task-first 文档不得覆盖当前最终 IA。

## 8. 产品定位

硕米智能引擎的最终定位为：

> **以 AI 工作台和专业智能体为核心，以供应链、店铺、数据、工具和生态能力为上下文的 AI 电商经营平台。**

ListingKit、Product、Image、SHEIN/TEMU/Amazon、Store Center、Resource Ledger 等是该产品下的领域能力与执行引擎，不应反向决定最终导航结构。
