# 运营驾驶舱首版交接

执行 Issue #627、主要 PR #629；设计依据为 [operations-cockpit-v1](../architecture/operations-cockpit-v1.md)，`IMPLEMENTATION_READY / FROZEN`。用户已确认人工录入收入、退款与成本，系统计算利润；创建人或当前目标管理者维护目标；建议使用规则依据并由人处理。用户已将驾驶舱共享接线指派本线程，并授权通知 #625 接线线程协调写入；本批保持一个 Writer、一个主要 PR。

## 正常启动

本功能要求当前 Store Center 及其专用 PostgreSQL 数据库、既有商业资源 owner、正常 ZITADEL 身份与企业上下文。按 [Store Center 架构](../architecture/store-center-current-application-v1.md) 和 [RUN-1](../operations/current-application-run1.md) 准备私密 owner/serving manifest；在 serving manifest 的现有 `storeCenter` 内增加 `"operationsCockpit": true`，并保留 `"enabled": true` 与原专用数据库配置。默认关闭；关闭时也禁止运行角色拥有本功能额外数据库权限。

仅在获准的新空 Store 实例上，通过既有 schema-owner 入口安装。以下绝对配置路径为示意，须替换为已准备的私密配置文件；serving manifest 使用 `store_center_runtime`，不能使用 owner 凭据。

```powershell
go run ./cmd/store-center-schema-init -config C:\private\store-owner.json -operations-cockpit
go run ./cmd/current-application -config C:\private\current-application.json
```

如新实例同时安装已有平台观察，则初始化命令同时增加 `-observations`，并按 [观察功能交接](store-observations-integration-handoff.md) 准备其配置与 worker。人工经营功能无需观察 worker；未装配观察时，订单观察明确显示未取得能力。启动只核验 schema 与角色权限，不安装或修复数据库。不得将上述初始化当成已有业务实例升级、迁移或权限修复操作；本批没有改动保留试用实例。

Console 复用正常启动与 Auth.js 配置，`LISTINGKIT_SERVICE_API_BASE` 指向上述 current backend 的 `/api/v1`；无需另加前端启用变量。从 `web/listingkit-ui` 执行正常 `pnpm.cmd dev`，或者 `pnpm.cmd build` 后 `pnpm.cmd start`。沿 RUN-1 的正常登录入口选择企业。

## 入口与操作

- `/workbench/overview/goals`：当前企业目标、评判规则、所用记录版本、缺口、历史恢复；`/workbench/overview/goals/settings` 设置明确店铺范围与自然日/周/月目标。
- `/workbench/overview/stores`：近7/30完整日或自选期间的同周期店铺矩阵，2–5店比较，查看/录入收入和成本、日期金额修正及不可变版本历史。
- `/workbench/overview/alerts`、`/workbench/overview/advice`：当前亏损、数据缺口及目标进度/利润率规则，查看原期间依据并跳转已有页面处理。

通过现有企业权限管理授予相应四模块。目标模块授予读取/创建，店铺矩阵授予读取/录入，预警与建议各自只读；目标管理权限复用当前 `listingkit_admin`，创建人也可在保有当前目标读取及新范围 Store 授权时维护。仍需 Store read 与每家店铺的现有成员授权；未授权店铺不进入矩阵或利润合计。

先在店铺中心正常创建店铺并授予访问，在矩阵点“查看 / 录入”。人民币元输入最多两位小数，七项必须填写；未发生的项目明确填0。只允许 UTC+8 已完成且不超过366天的闭区间，同店当前记录不得重叠；跨查询边界记录保留原期间并排除，不按天摊销。缺口不算健康或目标失败。利润 = 退款前收入 − 退款 − 售出采购成本 − 物流 − 平台 − 广告 − 其他成本。

再设置目标，选择1–50家店铺及日/周一开始的一周/自然月，填写正利润目标与可选利润率底线。目标是企业当前单一目标；修改与恢复均生成新版本，创建人不变。日期/金额修正使用稳定记录 ID 与当前版本，冲突后需重新读取。目标旧范围失效时，合法维护者仅取得版本标识，以完整新范围重配；旧数据及历史保持当前授权检查。

保存结果未知时，原请求及编号保存在浏览器当前用户/企业的会话存储，点击“重试原操作”确认同一结果；不要另起重复录入。企业切换后清除当前页面草稿，待确认操作仍按原身份企业保存。确认后可继续新录入。目标与经营事实分别保留待确认操作；原目标因旧店铺撤权无法回放时，合法维护者重新读取当前版本，可明确点击“保留原操作并维护当前版本”，以独立的新范围 CAS 操作维护，原请求/编号/重试入口仍保留，新操作未知也保留其原请求。会话存储不是业务事实源。

## 保存与当前限制

经营事实、目标、不可变版本与提交回执保存在 Store 专用数据库的 `operations_cockpit` schema；head/version/receipt 在同一个事务提交，使用原生 Store 同事务锁读。利润与目标依据来自同一个 SQL 快照。正常 stop/restart 保留数据库/命名卷，不使用删除卷命令。

规则无“处理中/已恢复”持久化生命周期。已有 SHEIN 异常订单按原保存观察范围与时间单独显示，不计入人工利润；不调用平台获取新数据。库存总量预警、平台财务导入、AI方案、预计收益、自动创建任务与外部平台处理尚未开放。

本批开发自检覆盖原生角色/成员授权、真实隔离 PostgreSQL Store/成员锁、窄角色启用/关闭 preflight、回放/CAS/重叠及事务回滚、同快照判断、严格 HTTP/BFF、企业切换和未知结果重试。测试 fixture 不是产品验收。保留实例部署、真实浏览器画面对比、真实平台与用户试用仍为 `NOT_RUN`；实现完成后由用户或指定独立验证者确认实际使用效果。最终候选、CI与独立交付检查证据写入主要 PR。
