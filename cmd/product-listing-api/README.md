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

实际注册路由以服务启动日志和各模块的 `httpapi` 路由定义为准。

## 启动

```bash
go run ./cmd/product-listing-api \
  -config config/config-dev.yaml \
  -port 8085 \
  -log-level info
```

配置缺少模块所需的生产数据库、Catalog/Asset Repository 或外部工作流依赖时，模块不会通过内存实现静默降级。
