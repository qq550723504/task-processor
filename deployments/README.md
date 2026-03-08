# 部署配置目录

## 用途

存放各种部署相关的配置文件，包括 Docker、Kubernetes、Terraform 等。

## 目录结构

```
deployments/
├── docker/              # Docker 相关
│   ├── Dockerfile
│   ├── docker-compose.yml
│   └── .dockerignore
├── kubernetes/          # Kubernetes 配置
│   ├── deployment.yaml
│   ├── service.yaml
│   ├── ingress.yaml
│   ├── configmap.yaml
│   └── secret.yaml
└── terraform/           # Terraform 基础设施代码
    ├── main.tf
    ├── variables.tf
    └── outputs.tf
```

## 使用场景

### Docker
```bash
# 构建镜像
docker build -f deployments/docker/Dockerfile -t myapp:latest .

# 使用 docker-compose 启动
docker-compose -f deployments/docker/docker-compose.yml up
```

### Kubernetes
```bash
# 应用配置
kubectl apply -f deployments/kubernetes/

# 查看状态
kubectl get pods
```

### Terraform
```bash
# 初始化
cd deployments/terraform
terraform init

# 规划
terraform plan

# 应用
terraform apply
```

## 最佳实践

1. 使用多阶段构建优化 Docker 镜像大小
2. 不要在镜像中包含敏感信息
3. 使用 ConfigMap 和 Secret 管理配置
4. 为不同环境创建不同的配置文件
5. 使用 Helm 管理复杂的 Kubernetes 应用
