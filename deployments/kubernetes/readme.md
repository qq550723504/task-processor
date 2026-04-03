# Kubernetes 部署清单

当前仓库内提供这些 K8s 清单：

- `amazon-crawler-api`
- `amazon-crawler-external-lb`
- `shein-listing`
- `temu-listing`
- `amazon-listing`

建议部署顺序：

1. `yudao-cloud`
2. RabbitMQ / Redis
3. `amazon-crawler-api`（如果 Amazon 上架依赖远程爬虫）
4. `shein-listing`
5. `temu-listing`
6. `amazon-listing`

示例：

```bash
kubectl apply -k deployments/kubernetes/shein-listing/overlays/staging
kubectl apply -k deployments/kubernetes/temu-listing/overlays/staging
kubectl apply -k deployments/kubernetes/amazon-listing/overlays/staging
```

注意：

- `base/secret.example.yaml` 只是示例，不要直接用于生产
- 生产前请替换镜像名、RabbitMQ 地址、管理端地址、OpenAI/SP-API 凭证
- 三个平台默认都走平台共享队列；只有显式启用 `useStoreQueues` 才走店铺队列

# 查看token
kubectl -n kubernetes-dashboard create token admin-user
