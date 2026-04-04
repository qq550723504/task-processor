# MySQL 延时退役任务

这套清单会创建一个一次性 `Job`：

- 创建后立刻启动
- 一直等待到 `2026-04-11 00:00:00 +08:00`
- 到点后自动清理 `yudao-cloud` 里的 MySQL 遗留资源

当前清理目标：

- `deployment/mysql`
- `service/mysql`
- `configmap/mysql-config`
- `configmap/mysql-daily-sql`
- `pvc/mysql-pvc`

## 部署命令

```bash
kubectl apply -k deployments/kubernetes/mysql-retirement-delayed-job
```

## 查看状态

```bash
kubectl -n yudao-cloud get job mysql-retirement-delayed-cleanup
kubectl -n yudao-cloud logs job/mysql-retirement-delayed-cleanup
```

## 注意

- 这是一次性延时任务，不会重复执行
- `ttlSecondsAfterFinished: 86400`，执行结束后 24 小时自动清理 Job 本身
- 如果你想取消，直接在到期前删除它：

```bash
kubectl -n yudao-cloud delete job mysql-retirement-delayed-cleanup
kubectl -n yudao-cloud delete rolebinding mysql-retirement-cleaner
kubectl -n yudao-cloud delete role mysql-retirement-cleaner
kubectl -n yudao-cloud delete serviceaccount mysql-retirement-cleaner
```
