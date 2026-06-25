# ListingKit SHEIN 活动报名来源 SDS 排查记录

这份记录用于排查 `SHEIN 活动报名 -> 成本价维护` 中，来源 POD/SDS 商品信息显示不完整的问题。

本次典型症状是：页面能显示来源 POD/SDS 编码，例如 `XB0610007001`、`XB0608018002`，但标题、价格、发货地等来源商品基础信息为空。

## 根因摘要

这次问题不是单纯的前端展示问题，而是多层问题叠加：

1. 前端已经发起了来源元数据请求，但接口返回 `200 {"items":[]}`。
2. 后端鉴权链路里，ZITADEL/Auth.js session 的业务用户 ID 与 Go 后端 bearer introspection 得到的 subject 可能不一致。
3. 当前用户具备 `listingkit_admin`，会被识别为 ListingKit 平台管理员。
4. 来源 SDS 历史任务使用的 tenant 可能是旧 tenant ID，和当前请求解析出的 tenant scope 不一致。
5. 原实现对平台管理员跳过了跨 tenant fallback，导致明明数据库里有 SDS 任务，接口仍返回空数组。
6. 打开 fallback 后，第一次实现又把店铺下所有 SDS 任务取出再在内存中过滤，数据量大时会触发 Next.js 代理 15 秒超时，表现为 `504 listingkit_upstream_unavailable`。

最终修复点是：平台管理员也允许走来源 SDS fallback，并且把 source code 过滤条件下推到 SQL，避免加载整店 SDS 任务后再过滤。

## 排查顺序

遇到“来源 SDS 标题/价格/发货地不显示”时，按下面顺序查，不要先反复改 UI：

1. 在浏览器 Network 里确认是否请求了：

   ```text
   /api/listing-kits/shein-sync/stores/{storeId}/source-sds-metadata?source_codes=...
   ```

2. 如果没有请求，优先查前端 query enable 条件、query key、source code 提取逻辑。
3. 如果返回 `401` 或 `403`，优先查 ZITADEL session、bearer token introspection、`X-User-ID`/`X-Tenant-ID` 注入。
4. 如果返回 `200 {"items":[]}`，不要继续调 CSS 或 React 展示逻辑，改查后端 repository 是否按正确用户、tenant、source code 查到了 SDS 任务。
5. 如果返回 `504 listingkit_upstream_unavailable`，重点查 repository 是否在拉全量 SDS 任务后内存过滤；应把 source code 条件下推到数据库。
6. 如果接口返回了 title/price/shipment area，但页面仍不显示，再回到前端组件确认字段映射和渲染条件。

## 关键代码入口

- 后端 HTTP handler：
  `internal/listingkit/api/shein_sync_handler_products.go`
- 后端路由：
  `internal/listingkit/httpapi/routes_shein_sync.go`
  `internal/listingkit/httpapi/routes_descriptor_shein_sync.go`
- 来源 SDS 元数据 repository：
  `internal/listingkit/store/task_repo_shein_source_metadata.go`
- ZITADEL 鉴权身份注入：
  `internal/listingkit/httpapi/zitadel_auth_middleware.go`
- ListingKit 角色到权限映射：
  `internal/authz/listingkit.go`
- 前端 API：
  `web/listingkit-ui/src/lib/api/shein-enrollment.ts`
- 前端成本价维护表格：
  `web/listingkit-ui/src/components/listingkit/shein-enrollment/shein-cost-price-table.tsx`

## 本次踩坑细节

### 1. `200 items: []` 不代表前端没拿字段

这次页面一直没有标题，最初容易误判为字段名没对上。但 Network 实际证明：

```json
{"items":[]}
```

也就是说，页面没有标题是后端没有返回来源 SDS 元数据，不是 React 没有渲染 title。

### 2. session 里的用户 ID 和后端上下文用户 ID 可能不是一回事

前端 `/api/zitadel-auth/session` 能看到 Auth.js session 中的 `identity.userId`，但 Go 后端还会对 bearer token 做 introspection。

如果 introspection payload 里没有业务 `user_id`，中间件可能会退回使用 subject。排查时要同时看：

- 浏览器 session 里的业务用户 ID
- Go 请求上下文里的 `X-User-ID`
- `X-Tenant-ID`
- 当前角色是否包含 `listingkit_admin`

这次已调整过优先级：有业务 `identity.UserID` 时优先使用它，不优先用 subject 覆盖。

### 3. 平台管理员也需要跨 tenant fallback

`listingkit_admin` 会映射为 ListingKit platform admin。原先 fallback 对平台管理员跳过，导致：

- 当前请求 tenant scope 查不到历史 SDS 任务
- 因为是 admin 又不走 fallback
- 最终接口返回空数组

后续不要默认认为“管理员权限更大，所以可以跳过 fallback”。这里管理员反而更需要通过 fallback 兼容历史 tenant 数据。

### 4. fallback 不能全量加载整店 SDS 任务

打开 fallback 后，如果直接按店铺加载所有 SDS 任务再在 Go 内存里过滤 source code，真实数据量下容易超时。

正确做法是把来源编码过滤下推到 SQL，至少覆盖这些字段：

- `request.options.sds.variant_sku`
- `request.options.sds.product_sku`
- `request.options.sds.variants[].variant_sku`

对应实现入口是：

```text
applySheinSourceSDSMetadataTargetScope
```

## 推荐验证方式

后端改动后，优先跑：

```powershell
go test ./internal/listingkit/store -run TestTaskRepositoryListSheinSourceSDSMetadata -count=1
go test ./internal/app/httpapi ./internal/listingkit ./internal/listingkit/api ./internal/listingkit/httpapi ./internal/listingkit/store
```

前端改动后，优先跑：

```powershell
cd web/listingkit-ui
npm test -- shein-cost-price-table.test.tsx
npm run lint -- --file src/components/listingkit/shein-enrollment/shein-cost-price-table.tsx --file src/lib/api/shein-enrollment.ts
```

如果要本地连真实数据复验，先按固定端口启动本地 API：

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File scripts/start-listingkit-local-api.ps1 -Port 8085 -ConfigPath config/config-dev.yaml -LogLevel info
```

然后用浏览器打开：

```text
http://localhost:3000/listing-kits/shein-enrollment/{storeId}?tab=costs&activityType=PROMOTION
```

本次修复后，`source-sds-metadata` 接口应能返回类似：

```json
{
  "items": [
    {
      "source_code": "XB0608018002",
      "title": "三折钱包",
      "variant_sku": "XB0608018002",
      "price": 22.5,
      "variant_label": "black / One size"
    },
    {
      "source_code": "XB0610007001",
      "title": "方形双层腰包 -（单图多拼可选）",
      "variant_sku": "XB0610007001",
      "price": 34.5,
      "variant_label": "white / 16x23cm"
    }
  ]
}
```

页面上应能看到来源 SDS 标题、POD/SDS 编码、变体、POD 价格和发货地。

## 判断口诀

- 没请求：查前端 query 条件。
- `401/403`：查 ZITADEL 与上下文身份。
- `200 items: []`：查 tenant/user scope 与 repository 查询。
- `504`：查是否全量扫描后内存过滤。
- 接口有数据但页面空：再查字段映射和组件渲染。
