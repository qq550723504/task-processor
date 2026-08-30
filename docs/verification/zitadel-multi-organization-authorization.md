# ZITADEL 多 Organization 授权验证

```yaml
real_environment_status: pending
observation_time: pending
issuer_host: pending
http_status: pending
identifier_suffixes:
  subject: pending
  home_organization: pending
  project: pending
  organization_a: pending
  organization_b: pending
organizations:
  - name: ListingKit Acceptance Organization A
    role_keys: [listingkit_admin]
  - name: ListingKit Acceptance Organization B
    role_keys: [listingkit_viewer]
revocation_propagation_status: pending
```

本轮只完成了 loopback-only、显式确认、幂等 provision 的代码和合成测试；没有运行
`provision-multi-org-acceptance`，也没有创建、修改、删除或恢复真实 ZITADEL 资源。
因此不能声称真实双 Organization 验收通过。

获得修改本地可重置 ZITADEL 测试数据的明确批准后，按本地验收文档运行 opt-in
命令。随后通过现有 Auth.js 流程登录，服务端使用其持有的浏览器 bearer token 调用：

```text
POST /zitadel.authorization.v2.AuthorizationService/ListAuthorizations
Connect-Protocol-Version: 1
```

只记录脱敏后的 issuer host、标识符后缀、Organization 名称、role key、HTTP 状态和
观察时间。验收必须观察到两个 active 项，并逐项确认：用户 ID 等于 introspection
得到的 subject；两个项目 ID 都等于 runtime 中的 ListingKit Project；用户的 Home
Organization 保持相同；授权 Organization ID 在 A、B 之间不同；A 仅有
`listingkit_admin`，B 仅有 `listingkit_viewer`。

角色撤销传播是另一项外部变更。只有再次获得明确批准后才能撤销和恢复授权；在此之前
保持 `revocation_propagation_status: pending`。
