# ListingKit 执行 Backlog

> 这份文档用于把下一阶段计划拆成 GitHub Issues。真正建 issue 时，可以按下面标题和验收标准复制。

## Epic 1：SHEIN Real API Validation

目标：让真实任务可验收、可复盘。

### Issue 1.1：新增真实接口验收报告模板

- [ ] 新增 `docs/product/validation/listingkit-real-api-validation-report-template.md`。
- [ ] 覆盖 task_id、输入、状态流转、workspace payload、readiness、保存草稿、发布和失败恢复。
- [ ] 明确未知状态、未知 blocker、空错误响应必须记录。

验收标准：

- [ ] 团队可以用模板记录一轮完整真实任务。
- [ ] 模板能支持成功和失败路径。

### Issue 1.2：沉淀第一批真实任务验收报告

- [ ] 记录至少 1 条成功任务。
- [ ] 记录至少 1 条失败任务。
- [ ] 输出未知状态 / 未知 blocker 清单。

验收标准：

- [ ] 至少一条任务从创建到保存草稿或发布完整通过。
- [ ] 至少一条失败路径能被 UI 解释或进入恢复流程。

## Epic 2：Error Recovery SOP

目标：失败后运营知道下一步，工程拿到足够排查信息。

### Issue 2.1：新增错误恢复 SOP 初版

- [ ] 覆盖图片上传失败。
- [ ] 覆盖 SDS 同步失败。
- [ ] 覆盖类目解析失败。
- [ ] 覆盖属性缺失。
- [ ] 覆盖 workspace 缺数据。
- [ ] 覆盖保存草稿失败。
- [ ] 覆盖发布失败。
- [ ] 覆盖 AI 生成失败。
- [ ] 覆盖配置缺失或失效。

验收标准：

- [ ] 每类错误说明运营动作、工程边界和必填记录字段。
- [ ] 有可直接复制的工程升级消息模板。

### Issue 2.2：QA 失败样例补齐

- [ ] 设计图片上传失败样例。
- [ ] 设计属性缺失样例。
- [ ] 设计保存草稿失败样例。
- [ ] 设计发布失败样例。
- [ ] 设计 unknown blocker 样例。

验收标准：

- [ ] QA 能主动触发至少一个失败样例。
- [ ] 失败样例能进入 SOP 对应恢复路径。

## Epic 3：SHEIN Submission State Machine

目标：保存草稿 / 发布可见、可恢复、幂等。

### Issue 3.1：设计 SubmitAttempt / SubmitPhase 模型

- [ ] 定义 attempt_id、task_id、tenant_id、target_platform、action。
- [ ] 定义 status 和 phase。
- [ ] 定义 idempotency_key。
- [ ] 定义 remote id 和错误字段。

验收标准：

- [ ] 能表达 validate、prepare_product、upload_images、pre_validate、submit_remote、persist_result。
- [ ] 能表达成功、失败、恢复中状态。

### Issue 3.2：抽出通用提交模块边界

- [ ] 新增或迁移到 `internal/listing/submission`。
- [ ] 通用提交状态机不依赖 SHEIN 具体 payload。
- [ ] root `internal/listingkit` 只保留 facade / orchestration。

验收标准：

- [ ] 新的提交状态逻辑没有继续堆进 ListingKit 根包。
- [ ] 通用 submission 和 SHEIN publishing 边界清晰。

### Issue 3.3：实现 SHEIN 保存草稿 / 发布幂等保护

- [ ] 同 task + action + idempotency_key 不重复提交。
- [ ] 同任务同动作并发提交不会重复调用 SHEIN 远端接口。
- [ ] 已成功 attempt 再次请求返回已有结果。

验收标准：

- [ ] 重放同一 idempotency_key 不重复创建远端草稿或发布。
- [ ] 并发点击不会产生重复远端调用。

### Issue 3.4：提交阶段 UI 展示

- [ ] 工作台展示当前 phase。
- [ ] 失败时展示 phase、reason、recoverable、next action。
- [ ] UI 防重复点击。

验收标准：

- [ ] 运营能看懂提交卡在哪一步。
- [ ] 失败后知道是否可以重试。

## Epic 4：Readiness Blocker Taxonomy

目标：阻断项稳定映射到修复动作。

### Issue 4.1：定义后端 blocker taxonomy

- [ ] 定义 blocker_key。
- [ ] 定义 severity。
- [ ] 定义 domain。
- [ ] 定义 repair_target / repair_route。
- [ ] 定义 unknown blocker 兜底规则。

验收标准：

- [ ] 所有已知 blocker 都有稳定 key。
- [ ] 新 blocker 必须补 taxonomy。

### Issue 4.2：前端 blocker 修复跳转

- [ ] category blocker 跳到类目区。
- [ ] attribute blocker 跳到属性区。
- [ ] image blocker 跳到图片区。
- [ ] price blocker 跳到价格区。
- [ ] sku blocker 跳到 SKU 区。
- [ ] unknown blocker 显示兜底详情。

验收标准：

- [ ] 用户看到 blocker 后知道改哪里。
- [ ] unknown blocker 不会让用户无路可走。

## Epic 5：Operations Console

目标：任务列表和队列页能支撑日常运营。

### Issue 5.1：任务列表运营筛选和 next action

- [ ] 支持平台、来源、任务状态、readiness、提交状态、阻断类型筛选。
- [ ] 每行显示最合适的下一步动作。
- [ ] 失败任务和可提交任务可快速定位。

验收标准：

- [ ] 运营不用复制 task_id 到其他页面继续处理。
- [ ] 当天失败任务可快速筛出。

### Issue 5.2：队列页 Review / Retry / Inspect 语义

- [ ] 定义三种动作语义。
- [ ] 区分运营可处理和工程介入。
- [ ] 支持每日失败复盘字段。

验收标准：

- [ ] 运营负责人能用队列页判断任务卡住原因。
- [ ] 队列页信息能支持失败任务复盘。

## Epic 6：Configuration Health Check

目标：配置问题前置暴露。

### Issue 6.1：设置页健康检查接口和 UI

- [ ] AI client 检查。
- [ ] SHEIN token / 权限 / 类目接口检查。
- [ ] SDS 接口或登录态检查。
- [ ] 图片模型配置检查。
- [ ] 价格规则检查。
- [ ] 对象存储检查。

验收标准：

- [ ] 新建任务前可以确认关键配置是否可用。
- [ ] 配置错误在设置页暴露，不等任务失败。

### Issue 6.2：配置影响范围提示

- [ ] AI client 影响文案和属性生成。
- [ ] SHEIN token 影响保存草稿和发布。
- [ ] SDS 登录态影响 SDS 商品库和 Studio。
- [ ] 图片模型影响图片生成。
- [ ] 价格规则影响 readiness。

验收标准：

- [ ] 用户能理解配置错误会影响哪些后续任务。

## Epic 7：SHEIN Studio Batch Efficiency

目标：批量任务部分成功、部分失败时仍能高效处理。

### Issue 7.1：批量任务状态分组

- [ ] 可提交。
- [ ] 需修复。
- [ ] 处理中。
- [ ] 生成失败。
- [ ] 提交失败。
- [ ] 已保存草稿。
- [ ] 已发布。

验收标准：

- [ ] 一批任务中部分失败不会阻断其他可提交任务。

### Issue 7.2：失败项单独重试和结果回流

- [ ] 只重试失败项。
- [ ] 成功项不重复提交。
- [ ] 批量结果能回到任务列表继续处理。

验收标准：

- [ ] 批量保存草稿失败时可以只重试失败项。
- [ ] 用户能清楚知道哪些任务需要单独进入工作台。
