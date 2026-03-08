# 数据库迁移目录

## 用途

存放数据库迁移文件，用于版本化管理数据库 schema 变更。

## 命名规范

```
{version}_{description}.up.sql    # 升级脚本
{version}_{description}.down.sql  # 回滚脚本
```

## 示例

```
migrations/
├── 000001_create_users_table.up.sql
├── 000001_create_users_table.down.sql
├── 000002_create_orders_table.up.sql
├── 000002_create_orders_table.down.sql
├── 000003_add_email_to_users.up.sql
└── 000003_add_email_to_users.down.sql
```

## 推荐工具

- [golang-migrate](https://github.com/golang-migrate/migrate)
- [goose](https://github.com/pressly/goose)
- [sql-migrate](https://github.com/rubenv/sql-migrate)

## 使用方法

```bash
# 升级到最新版本
migrate -path migrations -database "postgres://localhost/mydb" up

# 回滚一个版本
migrate -path migrations -database "postgres://localhost/mydb" down 1

# 查看当前版本
migrate -path migrations -database "postgres://localhost/mydb" version
```

## 最佳实践

1. 每个迁移文件只做一件事
2. 总是提供 up 和 down 脚本
3. 在开发环境测试迁移
4. 迁移文件一旦提交就不要修改
5. 使用事务确保原子性
