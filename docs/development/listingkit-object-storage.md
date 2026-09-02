# ListingKit 对象存储开发说明

这份说明覆盖本地开发时如何把 ListingKit 图片上传切到 S3 兼容对象存储。ImageAgent durable artifact publication 使用独立的 `imageagent.artifactStore` 配置，不与 ListingKit 共享配置所有权。

当前实现约束：

- 配置只读 `listingkit.imageUpload.*`
- `provider=local` 时，ListingKit 上传仍走本地磁盘
- `provider=s3` 时，ListingKit 上传走 S3 兼容存储；client、uploader 或 store 构造失败会阻止模块启动，不会降级到 local

## 配置字段

```yaml
listingkit:
  imageUpload:
    provider: "s3"
    local:
      rootDir: "./.local/tmp/listingkit-uploads"
    s3:
      bucket: "listingkit-assets"
      region: "us-east-1"
      endpoint: "http://127.0.0.1:9100"
      accessKeyID: "minioadmin"
      secretAccessKey: "minioadmin"
      usePathStyle: true
      publicBase: "http://127.0.0.1:9100/listingkit-assets" # 可选
```

字段语义：

- `publicBase`
  - 优先级最高
  - 如果配置了，返回给前端的 `image_urls` 和发布 URL 都使用这个前缀
- `endpoint`
  - S3 兼容存储入口，例如 MinIO
- `usePathStyle`
  - MinIO 这类本地服务通常设为 `true`
- `bucket / region / accessKeyID / secretAccessKey`
  - 标准 S3 兼容配置

如果 `publicBase` 为空：

- 代码会按 `endpoint + bucket + usePathStyle` 推导对象公开 URL
- 不再默认硬编码为 `https://bucket.s3.amazonaws.com/...`

## 本地 MinIO 验证

### 1. 启动 MinIO

```powershell
docker run -d --name listingkit-minio-test `
  -p 9100:9000 `
  -p 9101:9001 `
  -e MINIO_ROOT_USER=minioadmin `
  -e MINIO_ROOT_PASSWORD=minioadmin `
  -v "D:\code\task-processor\tmp\listingkit-minio-data:/data" `
  quay.io/minio/minio server /data --console-address ":9001"
```

控制台：

- API: [http://127.0.0.1:9100](http://127.0.0.1:9100)
- Console: [http://127.0.0.1:9101](http://127.0.0.1:9101)

### 2. 建桶并开放匿名下载

```powershell
docker run --rm --entrypoint /bin/sh --network host minio/mc -c "mc alias set local http://127.0.0.1:9100 minioadmin minioadmin && mc mb --ignore-existing local/listingkit-assets && mc anonymous set download local/listingkit-assets"
```

### 3. 准备测试配置

推荐从 `config/config-test.yaml` 复制一份，只替换 `listingkit.imageUpload`：

```yaml
listingkit:
  imageUpload:
    provider: "s3"
    local:
      rootDir: "./.local/tmp/listingkit-uploads-test"
    s3:
      bucket: "listingkit-assets"
      region: "us-east-1"
      endpoint: "http://127.0.0.1:9100"
      accessKeyID: "minioadmin"
      secretAccessKey: "minioadmin"
      usePathStyle: true
      publicBase: "http://127.0.0.1:9100/listingkit-assets"
```

### 4. 启动后端

```powershell
go build -o .\tmp\product-listing-api-s3.exe .\cmd\product-listing-api
.\tmp\product-listing-api-s3.exe --config .\tmp\config-test-s3.yaml --port 8086
```

### 5. 上传 smoke test

```powershell
curl.exe -i -X POST -F "files=@D:/code/task-processor/.local/tmp/listingkit-smoke.jpg;type=image/jpeg" http://127.0.0.1:8086/api/v1/listing-kits/uploads/images
```

预期返回：

```json
{
  "image_urls": [
    "http://127.0.0.1:9100/listingkit-assets/20260419/xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx.jpg"
  ]
}
```

然后直接访问返回的 URL，应该得到 `200 OK`。

## 前端联调

ListingKit UI 仍然只消费 `image_urls`。

如果 UI 本地开发代理要指向这台后端：

```bash
LISTINGKIT_API_BASE=http://localhost:8086/api/v1/listing-kits
```

前端不需要为对象存储做额外适配。

## 已验证行为

在本地 MinIO 上已确认：

- `POST /api/v1/listing-kits/uploads/images` 会把对象写入桶
- 返回的 `image_urls` 是对象存储 URL，不是本地磁盘路由
- 直接访问返回 URL 可拿到图片内容

## 当前限制

- 没做 presigned upload，上传仍由后端接收文件再写对象存储
- `GET /api/v1/listing-kits/uploads/files/*key` 仍然保留，主要用于 `local` provider；S3 模式下前端默认直接使用对象 URL
