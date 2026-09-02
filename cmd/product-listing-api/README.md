# product-listing-api

`product-listing-api` 通过 `internal/app/httpapi` 装配产品目录读取方、已批准资产读取方和当前仍投产的 HTTP 模块。

产品丰富化和产品图片处理不再由该进程拥有 HTTP Task、Queue、Worker Pool 或 Task Repository。图片工作流由 ImageAgent 独占；ListingKit、SDS 和 AmazonListing 只读取规范 Product Snapshot 与已批准资产，不会回退到旧图片生成或丰富化任务。

## 运行时模块

- `GET /health`
- ImageAgent HTTP API（Temporal-backed）
- Product Sourcing 入口
- AmazonListing（仅在 Product Snapshot 与 Approved Asset reader 可用时注册）
- ListingKit（仅在 Product Snapshot 与 Approved Asset reader 可用时注册）
- SDS、Prompt、登录和任务状态支持模块

## 当前主要路由

AmazonListing 只读取指定 `product_key` 的 Product Snapshot 和 Approved Asset inventory：

- `POST /api/v1/amazon/listings/generate`
- `GET /api/v1/amazon/listings/tasks/:task_id`
- `GET /api/v1/amazon/listings/tasks/:task_id/workbench`
- `POST /api/v1/amazon/listings/tasks/:task_id/review`
- `POST /api/v1/amazon/listings/tasks/:task_id/submit`

完整定义见 [AmazonListing route descriptors](../../internal/amazonlisting/httpapi/routes.go)。

ListingKit 同样只读 Product Snapshot 和 Approved Asset inventory；生成、预览和导出只消费 inventory 中的已批准图片：

- `POST /api/v1/listing-kits/generate`
- `GET /api/v1/listing-kits/tasks/:task_id`
- `GET /api/v1/listing-kits/tasks/:task_id/preview`
- `GET /api/v1/listing-kits/tasks/:task_id/revision-history`
- `GET /api/v1/listing-kits/tasks/:task_id/revision-history/:revision_id`
- `GET /api/v1/listing-kits/tasks/:task_id/export`
- `POST /api/v1/listing-kits/tasks/:task_id/revision`
- `POST /api/v1/listing-kits/tasks/:task_id/revision/validate`

完整定义见 [ListingKit entrypoints](../../internal/listingkit/httpapi/routes_descriptor_entrypoints.go) 和 [ListingKit task route descriptors](../../internal/listingkit/httpapi/routes_descriptor_task.go)。

ImageAgent 当前入口以 `/api/v1/image-agent/runs` 为根，完整定义见 [ImageAgent route descriptors](../../internal/imageagent/httpapi/routes.go)。SDS 查询入口以 `/api/v1/sds` 为根，完整定义见 [SDS HTTP module](../../internal/sds/httpapi/http_module.go)。

## 启动

```bash
go run ./cmd/product-listing-api \
  -config config/config-dev.yaml \
  -port 8085 \
  -log-level info
```

AmazonListing 与 ListingKit 的产品事实只读自 Catalog Product Snapshot，图片只读自 Approved Asset inventory。退役的 ProductEnrich/ProductImage HTTP Task、Queue、Worker Pool 和 Task Repository 不再由该进程装配。

AmazonListing 与 ListingKit 的可变状态必须使用显式持久化仓储。生产组合层为完整 ListingKit repository set 只打开一次数据库连接，并独占其唯一 closer；模块 bootstrap 不读取数据库配置，也不会在配置缺失、连接失败或 repository 缺失时回退到内存实现。以上情况会在注册 route 或 worker pool 前使启动失败。内存仓储只允许由测试显式注入。SourceAccount 继续由自身 bootstrap 管理，私有账号存储不可用不会阻断无需账号的 1688 公共抓取。
