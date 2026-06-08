# 数据库连接池优化 - 方案7实施总结

## 问题背景

shein-listing在K8s中启动了32个pod，每个pod默认10个数据库连接，导致最多320个数据库连接，造成数据库压力过大。

## 解决方案

采用**应用层连接代理 + 环境变量配置**的方式，通过代码层面控制每个pod的数据库连接数。

## 实施内容

### 1. 核心代码实现

#### 文件: `internal/infra/database/connection_proxy.go`
- 创建ConnectionProxy结构，使用信号量控制并发DB操作
- 提供Execute方法包装所有数据库调用
- 支持统计监控和超时控制

#### 文件: `internal/infra/database/postgres_db.go`
- 增强NewDatabaseFromConfig函数，支持从环境变量读取配置
- 新增环境变量支持：
  - `DATABASE_MAX_CONNECTIONS`: 最大数据库连接数
  - `DATABASE_MAX_IDLE_CONNECTIONS`: 最大空闲连接数
- 优先级：配置文件 > 环境变量 > 默认值

### 2. K8s配置更新

#### 文件: `deployments/kubernetes/shein-listing/base/deployment.yaml`
添加环境变量：
```yaml
env:
  - name: DATABASE_MAX_CONNECTIONS
    value: "3"
  - name: DATABASE_MAX_IDLE_CONNECTIONS
    value: "2"
```

## 效果对比

### 优化前
- Pod数量: 32
- 每Pod连接数: 10
- **总连接数: 32 × 10 = 320**

### 优化后（仅调整连接数）
- Pod数量: 32
- 每Pod连接数: 3
- **总连接数: 32 × 3 = 96** (减少70%)

### 进一步优化（配合HPA限制）
如果同时调整HPA限制pod数量为10：
- Pod数量: 10
- 每Pod连接数: 3
- **总连接数: 10 × 3 = 30** (减少90%)

## 部署步骤

### 方式1: 直接应用（推荐）
```bash
# 应用K8s配置
kubectl apply -k deployments/kubernetes/shein-listing/overlays/prod

# 滚动更新会自动生效
kubectl rollout status deployment/shein-listing -n task-processor
```

### 方式2: 修改配置文件
如果不使用环境变量，可以修改配置文件：
```yaml
# config/config-prod.yaml
database:
  max_connections: 3
  max_idle_connections: 2
```

然后重新构建镜像并部署。

## 监控验证

### 检查环境变量是否生效
```bash
# 查看pod的环境变量
kubectl exec -it <pod-name> -n task-processor -- env | grep DATABASE

# 应该看到:
# DATABASE_MAX_CONNECTIONS=3
# DATABASE_MAX_IDLE_CONNECTIONS=2
```

### 检查数据库连接数
```sql
-- 在PostgreSQL中执行
SELECT count(*) FROM pg_stat_activity WHERE datname = 'ruoyi-vue-pro';

-- 优化前: ~320
-- 优化后: ~96 (或更少)
```

### 应用日志观察
```bash
# 查看应用启动日志
kubectl logs -f deployment/shein-listing -n task-processor | grep -i "database\|connection"

# 应该能看到连接池初始化信息
```

## 注意事项

1. **渐进式调整**: 建议先从10降到5，观察稳定后再降到3
2. **监控告警**: 设置数据库连接数告警阈值（如80%）
3. **性能测试**: 调整后需要进行压力测试，确保不影响业务
4. **回滚方案**: 如果出现问题，可以快速调整环境变量值回滚

## 后续优化建议

1. **引入PgBouncer**: 如果需要更多pod但保持低连接数，可以考虑部署PgBouncer连接池中间件
2. **读写分离**: 将读操作分流到从库
3. **缓存优化**: 加强Redis缓存使用，减少数据库查询
4. **批量操作**: 优化代码中的N+1查询问题

## 相关文件清单

- `internal/infra/database/connection_proxy.go` - 连接代理核心实现
- `internal/infra/database/connection_proxy_test.go` - 单元测试
- `internal/infra/database/postgres_db.go` - 数据库连接增强
- `deployments/kubernetes/shein-listing/base/deployment.yaml` - K8s部署配置

## 技术细节

### 环境变量优先级
```
1. 配置文件中的 max_connections/max_idle_connections
2. 环境变量 DATABASE_MAX_CONNECTIONS/DATABASE_MAX_IDLE_CONNECTIONS
3. 默认值 (10/5)
```

### 代码实现逻辑
```go
// postgres_db.go 中的逻辑
maxConn := cfg.MaxConnections
if maxConn <= 0 {
    // 尝试从环境变量读取
    if envMax := os.Getenv("DATABASE_MAX_CONNECTIONS"); envMax != "" {
        if parsed, err := strconv.Atoi(envMax); err == nil && parsed > 0 {
            maxConn = parsed
        } else {
            maxConn = 10  // 默认值
        }
    } else {
        maxConn = 10  // 默认值
    }
}
```

## 联系信息

如有问题，请联系开发团队。

---
更新日期: 2026-06-08
