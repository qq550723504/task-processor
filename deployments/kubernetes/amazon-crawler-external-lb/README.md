# Amazon Crawler External LB 部署说明

这套清单用于在 K8s 集群内暴露一组集群外部的 Amazon crawler API 节点，供 `amazon-listing` 通过集群内稳定地址访问。

对应 Service 名称：

- [service.yaml](D:/code/task-processor/deployments/kubernetes/amazon-crawler-external-lb/service.yaml)

对应后端节点清单：

- [endpoints.yaml](D:/code/task-processor/deployments/kubernetes/amazon-crawler-external-lb/endpoints.yaml)

## 作用

部署完成后，集群内可以通过这个地址访问外部 crawler：

`http://amazon-crawler-external-lb.task-processor.svc.cluster.local:8080`

`amazon-listing` 现在也已经默认指向这个地址。

## 部署前要改的只有一件事

编辑：

- [endpoints.yaml](D:/code/task-processor/deployments/kubernetes/amazon-crawler-external-lb/endpoints.yaml)

把里面的外部 crawler IP 改成你的真实节点列表。

注意：

- 这里只能写 IP，不能写域名
- 端口统一在 `ports.port` 配成 `8080`
- 新增 crawler 节点时，只要继续追加 `- ip: ...` 即可

## 部署命令

```bash
kubectl apply -k deployments/kubernetes/amazon-crawler-external-lb
```

## 部署后检查

```bash
kubectl -n task-processor get svc,endpoints amazon-crawler-external-lb
```

正常情况下应该看到：

- `Service` 已创建
- `Endpoints` 里有你配置的外部 IP

## 连通性验证

建议从集群内起一个临时 Pod 测试：

```bash
kubectl -n task-processor run curl-test --rm -it --image=curlimages/curl -- \
  curl -sS http://amazon-crawler-external-lb.task-processor.svc.cluster.local:8080/health
```

如果 crawler API 没有 `/health`，就改成你当前可用的探测接口。

## 和 amazon-listing 的关系

`amazon-listing` 现在默认配置的是：

- [configmap.yaml](D:/code/task-processor/deployments/kubernetes/amazon-listing/base/configmap.yaml)
- [secret.example.yaml](D:/code/task-processor/deployments/kubernetes/amazon-listing/base/secret.example.yaml)

对应地址已经改成：

`http://amazon-crawler-external-lb.task-processor.svc.cluster.local:8080`

所以推荐顺序就是：

1. 先部署 `amazon-crawler-external-lb`
2. 验证集群内能通外部 crawler
3. 再部署 `amazon-listing`
