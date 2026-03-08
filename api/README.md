# API 定义目录

## 用途

存放 API 定义文件，包括 OpenAPI/Swagger 规范、Protocol Buffers 定义、GraphQL Schema 等。

## 目录结构

```
api/
├── openapi/          # OpenAPI/Swagger 规范
│   └── api.yaml
├── proto/            # Protocol Buffers 定义
│   └── service.proto
└── graphql/          # GraphQL Schema
    └── schema.graphql
```

## 使用场景

- **openapi/**：REST API 文档和规范
- **proto/**：gRPC 服务定义
- **graphql/**：GraphQL API Schema

## 最佳实践

1. API 定义应该是单一真实来源
2. 使用工具从定义生成代码（如 protoc、swagger-codegen）
3. 版本化 API 定义（如 v1/api.yaml, v2/api.yaml）
4. 保持定义与实现同步
