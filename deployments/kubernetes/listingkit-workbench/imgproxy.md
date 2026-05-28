# ListingKit imgproxy

这套 `imgproxy` 部署用于给 `listingkit-workbench` 提供缩略图、裁剪图和格式转换能力。

## 设计

- 原图继续保存在 `MinIO / S3`
- `imgproxy` 通过集群内 `minio.task-processor.svc.cluster.local:9000`
  直接读取 `listingkit-assets`
- 对外统一挂在 `pod.shuomiai.com/img`

## 依赖的配置

来自 `listingkit-workbench-config`:

- `IMGPROXY_BIND`
- `IMGPROXY_PATH_PREFIX`
- `IMGPROXY_USE_S3`
- `IMGPROXY_S3_BUCKET`
- `IMGPROXY_S3_REGION`
- `IMGPROXY_S3_ENDPOINT`
- `IMGPROXY_S3_ENDPOINT_USE_PATH_STYLE`
- `IMGPROXY_AUTO_WEBP`
- `IMGPROXY_AUTO_AVIF`
- `LISTINGKIT_IMGPROXY_BASE_URL`
- `NEXT_PUBLIC_LISTINGKIT_IMGPROXY_BASE_URL`

来自 `listingkit-workbench-secret`:

- `TASK_PROCESSOR_PRODUCTIMAGE_PUBLISHER_S3_ACCESSKEYID`
- `TASK_PROCESSOR_PRODUCTIMAGE_PUBLISHER_S3_SECRETACCESSKEY`
- 可选：`IMGPROXY_KEY`
- 可选：`IMGPROXY_SALT`

## 访问示例

未启用签名时，可以先用 `insecure` 路径验证：

```text
https://pod.shuomiai.com/img/insecure/rs:fit:320:320/plain/s3://listingkit-assets/20260529/demo.png@webp
```

说明：

- `rs:fit:320:320` 表示缩放到 320x320
- `plain/s3://listingkit-assets/...` 直接读取 MinIO/S3 对象
- `@webp` 让输出格式变成 WebP

如果后续前端正式接入，建议优先切换为签名 URL，再启用 `IMGPROXY_KEY` / `IMGPROXY_SALT`。
