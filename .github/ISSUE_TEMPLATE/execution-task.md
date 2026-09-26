---
name: "有界执行任务"
about: "一个独立可验证结果的 Issue 驱动派工"
title: "[Execution] "
labels: ""
assignees: ""
---

依据：[Issue 驱动派工规则 V1](https://github.com/qq550723504/task-processor/blob/main/docs/engineering/issue-driven-development.md)。TDD、Legacy 与 Architecture First 准入沿用 [AGENTS.md](https://github.com/qq550723504/task-processor/blob/main/AGENTS.md)。所有正式开发任务必须填写 Design Basis；只有符合规则的简单任务才可写 N/A。

## 目标

<!-- 一个可观察、独立交付的结果。 -->

## 范围

- 领域 owner：
- 主要修改区域：
- 明确不做：

## 依据与依赖

- 当前批准文档 / 前置 Issue 或 PR：
- 开工条件（Ready 必须已满足 Design Basis；Ready 不等于自动启动）：
- 接线 / 上线条件（与开工分别说明）：
- 已有接单会话 / 分支 / PR（无则写无）：

## Design Basis / Architecture Admission

- 任务分类：Independent Architecture / Reuse Existing Architecture / N/A
- 产品 / Figma Authority：
- Architecture / Contract：
- Domain / Fact Owner：
- 主要调用路径（contract → implementation → injection → consumer）：
- 新增或修改的高风险边界：
- 明确不改变的状态机 / 权限 / 事实 owner / 持久化 / 恢复协议：
- Legacy decision：EXTRACT / RETIRE / N/A
- Architecture Review：链接 / N/A
- Admission Status：NOT_READY / IMPLEMENTATION_READY / N/A
- 若为 N/A，理由：

> Independent Architecture 必须在 Writer 修改生产代码前达到 IMPLEMENTATION_READY；Reuse Existing Architecture 必须引用当前已批准设计并说明不变量；N/A 只适用于不改变业务运行语义的简单任务。

## 验收

<!-- 将以下项目具体化；无关项写 N/A 及原因，不发明新产品/安全要求。 -->
- [ ] 目标行为与可观察结果：
- [ ] 风险匹配的失败 / 权限 / 幂等验证：
- [ ] 领域 / Legacy 边界：
- [ ] 最终 HEAD 适用 CI 与独立评审证据：
- [ ] 本任务不包含的生产验收已注明：

## 权限

默认允许任务独立工作区内实现、隔离测试、提交、推送和创建 / 维护关联 PR；禁止自行合并、关闭 Issue、部署、操作真实数据或变更保护规则。标签和普通评论不扩大权限。

- 特殊授权引用（无则写无）：

## 交付与责任

- 实现负责人 / 会话（同一时间一个写入负责人）：
- 独立 Reviewer（可在进入评审前指定，不向实现分支写代码）：
- 关联 PR（部分交付使用 Refs / Relates to）：
- 当前状态及唯一状态来源（已有看板字段 / 标签；无则本字段）：Backlog
  - 未满足 Design Basis / Architecture Admission 时必须保持 Backlog 或 Blocked，不得进入 In Progress。
- 当前阻塞（无则写无；否则写缺失条件、owner、解除条件及影响开工 / 切片 / 合并 / 上线哪一层）：

进度在接单、阻塞、进入评审、完成 / 取消时写 Issue；当前要求变更由协调方更新正文。完整验收映射、最终 HEAD、CI / 评审、NOT_RUN、剩余阻塞和批准写 PR。

启动示例：执行 Issue #<编号>，读取完整正文、引用依据与 AGENTS.md；先核对 Design Basis / Architecture Admission，Independent Architecture 必须为 IMPLEMENTATION_READY；再检查接单和关联 PR并使用独立工作区。未满足准入时只做获准的只读调查 / Spike，不修改正式生产业务路径；证据写 GitHub，不自行合并或操作真实环境 / 数据。
