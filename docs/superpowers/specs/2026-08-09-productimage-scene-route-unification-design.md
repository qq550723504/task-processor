# ProductImage 场景治理路由统一设计

## 目标

让 ProductImage 场景治理的路由目录、凭据解析和实际图片客户端统一使用已有的 `image_gpt_image_2` 客户端配置，消除治理路由查找 `image`、而租户设置使用 `image_gpt_image_2` 导致的错配。

## 设计

### 统一凭据边界

- ProductImage 场景治理的 canonical credential reference 为 `image_gpt_image_2`。
- 路由目录通过同一凭据名解析租户/用户级配置，并仅接受空值、`openai` 或 `openai-compatible` API style，其他 provider fail-closed。
- `RouteDecision` 的 `CredentialReference`、`ModelID`、`ConfigurationVersion` 必须与解析结果保持一致。

### 实际客户端绑定

- 治理开启时，`buildModelProvider` 使用 OpenAI Manager 的 `image_gpt_image_2` 逻辑客户端构造场景 generator。
- 旧的静态 `image`/Nano Banana 客户端仅保留给 legacy Studio 路径，不参与 ProductImage 场景治理。
- OpenAI Manager 允许在存在 resolver 时创建未预置静态配置的逻辑 image client；真正请求仍要求 resolver 返回完整配置，因此缺 resolver、缺 API key、缺 endpoint 或缺 model 均 fail-closed。

### 兼容与安全

- 默认 `productImageSceneEnabled=false` 不变。
- 不迁移、不复制、不回显任何 API key；不修改线上开关或租户凭据。
- 治理关闭时现有 legacy 行为保持不变。

## 验证

- 路由目录测试：确认 `image_gpt_image_2`、模型、provider 和配置版本来自同一 resolver 结果；缺凭据和不支持的 API style 拒绝。
- Manager 测试：确认 resolver-backed 未预置逻辑 client 可创建，resolver 缺失时仍拒绝未知 client。
- ProductImage builder 测试：治理开启时绑定 `image_gpt_image_2`，静态 Nano Banana 不会绕过治理；治理关闭时 legacy 行为不变。
- 运行目标包测试、全量 Go 测试和 `git diff --check`。

