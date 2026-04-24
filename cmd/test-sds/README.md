# test-sds

用于本地验证 SDS 接口连通性。

示例：

```bash
go run ./cmd/test-sds -mode option-groups -token <access-token> -merchant-id <merchant-id>
go run ./cmd/test-sds -mode list -token <access-token> -page 1 -size 20
go run ./cmd/test-sds -mode detail -product-id 239998
go run ./cmd/test-sds -mode cycle -product-id 239998
go run ./cmd/test-sds -mode sync-url -token <access-token> -merchant-id <merchant-id> -variant-id 89764 -image-url https://example.com/design.png
go run ./cmd/test-sds -mode sync-file -token <access-token> -merchant-id <merchant-id> -variant-id 89764 -image-file ./design.png
go run ./cmd/test-sds -mode sync-result-file -token <access-token> -merchant-id <merchant-id> -variant-id 89764 -result-file ./result.json
go run ./cmd/test-sds -mode process-and-sync -token <access-token> -merchant-id <merchant-id> -variant-id 89764 -image-url https://example.com/source.jpg
```

推荐优先使用：

```bash
go run ./cmd/test-sds -mode sync-file -variant-id 89764 -image-file ./.tmp/sds-live-test.png
```

说明：

- 如果本地已经有 `data/sds/auth_state.json`
  - 可以直接省略 `-token` 和 `-merchant-id`
- 当前已实测跑通的真实样例：
  - `variant-id = 89764`
  - `prototype-group-id = 14555`
  - `layer-id = 698744758333792256`
- `sync-file` 成功时，输出里应能看到：
  - `DesignResult.Request.product_id`
  - `DesignResult.Request.prototypes[0].layers[0].related_material_ids`
  - `DesignResult.Material.material.id`
