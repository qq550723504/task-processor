# test-sds

用于本地验证 SDS 接口连通性。

示例：

```bash
cd hack/debug
go run ./test-sds -mode option-groups -token <access-token> -merchant-id <merchant-id>
go run ./test-sds -mode list -token <access-token> -page 1 -size 20
go run ./test-sds -mode detail -product-id 239998
go run ./test-sds -mode cycle -product-id 239998
```

说明：

- 如果本地已经有 `data/sds/auth_state.json`
  - 可以直接省略 `-token` 和 `-merchant-id`
- 该工具只验证 SDS 登录和商品目录读取。
- SDS 设计同步只能通过正式用例读取 `ApprovedAssetInventory`，不提供裸 URL、本地文件或旧 ProductImage 调试入口。
