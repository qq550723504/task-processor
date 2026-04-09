# Alertmanager WeCom Adapter

这套清单用于把 Alertmanager webhook 告警转成企业微信机器人 markdown 消息。

## 包含内容

- 轻量 Python webhook 适配器
- `AlertmanagerConfig`
- `Service`

## Secret

需要先创建：

```bash
kubectl -n monitoring create secret generic alertmanager-wecom-secret \
  --from-literal=webhook-url='https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=YOUR_KEY'
```

## 应用

```bash
kubectl apply -k deployments/kubernetes/monitoring/alertmanager-wecom
```

## 当前路由范围

默认只接收：

- `service=amazon-crawler-api`

如果后续要扩展到其他服务，可以再加新的 `AlertmanagerConfig` 或放宽 matcher。
