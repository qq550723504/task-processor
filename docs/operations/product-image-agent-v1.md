# 完整商品图片智能体 v1 运行与使用

对应 #612 / PR #616，Design Basis 为 [完整图片架构](../architecture/product-image-agent-v1.md)。这是实现交接；真实 provider、平台发布和用户验收分别保留 NOT_RUN，不能由 CI 或合成响应代替。

## 使用入口

1. 正常登录并选择当前企业，在 `/workbench/agents/mine/product.image.agent` 显式启用图片智能体，创建并选择精确版本的图片模板。标题智能体保留自己的配置和审核路径。
2. 在已保存的采集商品 `/workbench/supply/acquisition/operation/<operationId>`，或“我的供应链 → 商品资料与平台适配”打开完整图片面板。也可在图片智能体的最近任务中读取原任务。
3. 选择主图/轮播与详情内容、真实原始素材。共用只表示共用原图，两组仍逐图调用和计费。八项详情是可选内容库；缺少规格、说明或配件依据的任务须补充商品资料或取消。
4. 通用素材可直接准备。平台素材从已经保存的实际店铺/站点/叶子类目/商品类型/属性/变体/应用模式读取当前官方图片要求，再为各项选择合法位置。首版开放 SHEIN 美国站；不能把 8+8 当作平台统一图片数量。当前输出为 1024×1024，界面标出不适用的类型；不调用额外放大模型。
5. 准备只冻结配方、原图字节哈希、配置和报价，不调用生成模型。确认真实总点数上限后才启动原 Temporal 任务。每张分别执行与结算，进度来自实际 Slot；部分成功和已产生费用保留。
6. 对照原图与结果，明确勾选最终采用的图片，安排主图/详情组内顺序并预览完整集合，再人工批准保存。可保留当前正式素材、采用已批准的通用素材到当前平台，或使用已有文件上传能力人工替换。文件替换由服务端读取真实不可变文件；浏览器不提供批准 URL、尺寸或计费证明。
7. 保存到唯一 Product Asset owner 后，当前供应链重新读取正式库存。正式发布仍逐项检查该平台所有必须位置；批准部分素材本身不意味着能够发布。

## 配置与启动

沿用 [当前供应链运行配置](my-supply-chain-shein-v1.md) 与 [企业智能体配置](organization-agent-configuration-v1.md)。当前默认 Account Compose 不自动启用供应链、完整图片 worker、付费 provider 或正式应用。已有单图试验 overlay 的 OpenAI 合成协议不是本实现的 GRSAI 接线；不要同时运行其旧 worker 来消费完整集合任务。

维护者在已经获准的全新业务环境准备私有 current-application JSON 和现有 worker YAML。服务启动验证已有 schema/grant，不安装或迁移业务表，也不自动启用企业。

| 私有配置 | 必要事实 |
| --- | --- |
| `productAcquisitionDatabase` / `productCollections` | 当前 Product owner 和现有集合/采集/Review schema，真实原始发布与 Apply 读取 |
| `productAgent.database` | 已安装 `agent_configuration` 的当前配置 owner；仅图片配置可用 `enabled:false`，无需启用标题执行 |
| `supplyChain` | 实际 Product/Store/Asset ports、当前官方规则及同一 namespace 的 Supply Temporal |
| `supplyChain.assetDatabase` | 与 `imageAgent.database` 同一物理 Image/Asset 数据库，使用独立 `supply_asset_runtime` pool，最多8连接 |
| `imageAgent.database` | `image_agent_runtime` 独立 HTTP pool，最多8连接；当前 Organization Image schema |
| `imageAgent.workerConfigFile` | 受保护的绝对路径，指向现有 worker YAML，例如 `C:\private\image-set-worker.yaml`；提供相同物理 Image DB 的 `image_agent_worker_runtime` pool，1–8连接 |
| 两份配置的 generation | 同一个真实 `priceVersion` 与已确认正整数 `pointsPerImage`；无默认价格，不能用美元 costMicros 代替点数 |
| 两份配置的 admission | enabled、相同企业 allowlist；实际企业仍须显式启用配置，原 actor/member 每次派发重新授权 |
| 两份配置的 artifactStore | 相同公开 base 与 bucket；worker YAML 含既有 S3 私有凭据、region/endpoint 与不可覆盖模式。API manifest 不含 worker 的存储凭据 |
| `commercialOwnerDatabase` | 当前 orgresource owner 的窄权限 pool，实际点数与成员月限；无需另建账本 |
| 企业模型 credential owner | 当前企业的 `image_gpt_image_2` route；HTTPS submit URL、`api_style=grsai`、`model=gpt-image-2.5`、有效私有 credential，超时最多5分钟 |
| 可选 `sourceMedia` | 已有商品 JPEG/PNG 文件上传与不可变读回。缺少该 port 时只关闭人工上传替换 |

JSON 的 `imageAgent` 除上述 database/workerConfigFile/generation 还须提供 `temporalAddress`（literal IPv4 loopback）、`temporalNamespace`、`allowedOrganizationIds`、`publicBase`、`bucket`。worker YAML 使用现有 `imageagent.generation`、`imageagent.admission`、`imageagent.artifactStore` 字段；两份配置不一致拒绝启动。模型密钥只由现有企业 credential owner 提供，模板不能改变模型、凭据、权限或价格。

新配置 schema 通过已有 `cmd/agent-configuration-schema-init` 安装；Image schema 与窄 HTTP/worker grant 通过已有 `cmd/product-listing-api-schema-migrate` 的 Organization Image 初始化选项安装；Image 空库初始化已同时安装当前 Asset schema；随后用已有 `cmd/supply-asset-init` 对同一物理 owner 补 `supply_asset_runtime` 授权，省略 `-install-empty-schema`。只能对获准的新环境执行维护命令。旧的单图配置/状态不迁移、不包装到新流程。

```powershell
# 后端：仓库根目录，私有 manifest 已包含上表实际依赖。
go run ./cmd/current-application -config C:\private\image-set-current-application.json
# 前端：沿用正常 Auth.js/ZITADEL 登录配置与真实 service API base。
Set-Location web/listingkit-ui
pnpm.cmd build
pnpm.cmd exec next start --hostname 127.0.0.1 --port 3000
```

不需要新增 worker 命令：current-application 装配完整 Set 专用 worker，监听成功后启动，停止服务时先停 worker，再关闭原 DB/Temporal 连接。同一 organization task queue 只能由本完整装配的 worker 消费；不要再启动旧单图独立 worker。Docker 部署需把 workerConfigFile 指向的私有文件只读挂载到 current-application；不把 storage credential 放入浏览器或仓库。

## 刷新、核实与保留

页面在发送前保存当前企业/身份/商品作用域下的原命令。刷新后只读同一 Run、原准备键或原不可变批准回执。准备/批准 ACK 未明时不会因为当前库存变化而推断成功。已准入但 Temporal Start ACK 丢失时，显式“继续原确认”使用相同确认动作与 workflow ID。未知 provider 结果只核实原 effect，不重新 POST。未找到不能视为未执行证明。

所有原 effect 已知收束后，可以选定部分内容创建新的 Run/新报价/新确认；原成功结果仍持有原 ID，不复制生成或重复扣点。停用阻止新准入；已准入任务、读取和人工审核保留，实际领域权限仍实时检查。

原图与集合/Apply 在 Product DB，模板与准入回执在 ProductAgentDB，Run/Plan/effect 与批准图片在各自同一物理 Image/Asset owner 的事实表，点数在 orgresource owner，图片字节在既有不可变 object storage，workflow 在原 Temporal namespace。停止或重启沿用原私有配置和数据；不重新初始化，不使用 `down -v`。

本批次开发自检覆盖真实领域/持久化测试、原 effect 收束、配置准入和原动作恢复，以及 Console/BFF 的完整交互。具体最终 HEAD、CI、独立检查和可运行实例情况记录在 PR #616；此说明不声称已经启动含真实付费模型和正式平台的实例。
