# API 文档目录

## 用途

存放项目的 API 接口文档，包括 REST API、内部接口等。

## 目录结构

```
api/
├── README.md           # 本文件
├── rest-api.md         # REST API 文档
├── openapi.yaml        # OpenAPI 规范文件
├── postman/            # Postman 集合
│   └── collection.json
└── examples/           # API 调用示例
    ├── curl.md
    └── code-examples.md
```

## 应该放置的文件

### 1. REST API 文档（rest-api.md）

记录所有 REST API 端点的详细信息。

**内容包括：**
- API 概述
- 认证方式
- 请求格式
- 响应格式
- 错误码说明
- 每个端点的详细文档

**模板：**
```markdown
# REST API 文档

## 概述

本文档描述了 task-processor 的 REST API 接口。

## 基础信息

- Base URL: `http://localhost:8080/api/v1`
- 认证方式: Bearer Token
- 内容类型: `application/json`

## 认证

所有 API 请求需要在 Header 中包含认证令牌：

```
Authorization: Bearer <access_token>
```

## 端点列表

### 1. 获取任务列表

**端点：** `GET /tasks`

**描述：** 获取所有任务列表

**请求参数：**
| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| page | int | 否 | 页码，默认 1 |
| size | int | 否 | 每页数量，默认 20 |
| status | string | 否 | 任务状态过滤 |

**响应示例：**
```json
{
  "code": 0,
  "message": "success",
  "data": {
    "total": 100,
    "items": [
      {
        "id": 1,
        "platform": "temu",
        "status": "processing",
        "created_at": "2024-01-01T00:00:00Z"
      }
    ]
  }
}
```

**错误响应：**
```json
{
  "code": 401,
  "message": "Unauthorized",
  "data": null
}
```
```

### 2. OpenAPI 规范（openapi.yaml）

使用 OpenAPI 3.0 规范定义 API。

**示例：**
```yaml
openapi: 3.0.0
info:
  title: Task Processor API
  version: 1.0.0
  description: 任务处理器 API 文档

servers:
  - url: http://localhost:8080/api/v1
    description: 开发环境

paths:
  /tasks:
    get:
      summary: 获取任务列表
      tags:
        - Tasks
      parameters:
        - name: page
          in: query
          schema:
            type: integer
            default: 1
        - name: size
          in: query
          schema:
            type: integer
            default: 20
      responses:
        '200':
          description: 成功
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/TaskListResponse'

components:
  schemas:
    TaskListResponse:
      type: object
      properties:
        code:
          type: integer
        message:
          type: string
        data:
          type: object
```

### 3. Postman 集合

导出的 Postman 集合文件，方便 API 测试。

**位置：** `postman/collection.json`

**使用方法：**
1. 打开 Postman
2. 导入 `collection.json`
3. 配置环境变量
4. 开始测试

### 4. API 调用示例

提供各种语言的 API 调用示例代码。

**curl 示例（curl.md）：**
```markdown
# cURL 示例

## 获取任务列表

```bash
curl -X GET "http://localhost:8080/api/v1/tasks?page=1&size=20" \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -H "Content-Type: application/json"
```

## 创建任务

```bash
curl -X POST "http://localhost:8080/api/v1/tasks" \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "platform": "temu",
    "product_id": "12345",
    "action": "sync"
  }'
```
```

**代码示例（code-examples.md）：**
```markdown
# 代码示例

## Go

```go
package main

import (
    "bytes"
    "encoding/json"
    "net/http"
)

func createTask() error {
    url := "http://localhost:8080/api/v1/tasks"
    
    payload := map[string]interface{}{
        "platform": "temu",
        "product_id": "12345",
        "action": "sync",
    }
    
    body, _ := json.Marshal(payload)
    req, _ := http.NewRequest("POST", url, bytes.NewBuffer(body))
    req.Header.Set("Authorization", "Bearer YOUR_TOKEN")
    req.Header.Set("Content-Type", "application/json")
    
    client := &http.Client{}
    resp, err := client.Do(req)
    // 处理响应...
    return err
}
```

## Python

```python
import requests

def create_task():
    url = "http://localhost:8080/api/v1/tasks"
    headers = {
        "Authorization": "Bearer YOUR_TOKEN",
        "Content-Type": "application/json"
    }
    payload = {
        "platform": "temu",
        "product_id": "12345",
        "action": "sync"
    }
    
    response = requests.post(url, json=payload, headers=headers)
    return response.json()
```
```

## 文档编写规范

### 1. 端点命名

- 使用 RESTful 风格
- 使用复数名词：`/tasks` 而不是 `/task`
- 使用小写字母和连字符

### 2. 参数说明

必须包含：
- 参数名称
- 数据类型
- 是否必填
- 默认值（如果有）
- 参数说明

### 3. 响应格式

统一的响应格式：
```json
{
  "code": 0,        // 状态码，0 表示成功
  "message": "",    // 消息
  "data": {}        // 数据
}
```

### 4. 错误码

| 错误码 | 说明 |
|--------|------|
| 0 | 成功 |
| 400 | 请求参数错误 |
| 401 | 未授权 |
| 403 | 禁止访问 |
| 404 | 资源不存在 |
| 500 | 服务器内部错误 |

## 工具推荐

### 1. Swagger UI

用于可视化 OpenAPI 文档：

```bash
# 使用 Docker 运行
docker run -p 8081:8080 \
  -e SWAGGER_JSON=/api/openapi.yaml \
  -v $(pwd)/docs/api:/api \
  swaggerapi/swagger-ui
```

访问：http://localhost:8081

### 2. Postman

用于 API 测试和文档生成。

### 3. Redoc

另一个 OpenAPI 文档渲染工具：

```bash
npx redoc-cli serve docs/api/openapi.yaml
```

## 维护指南

### 1. 更新流程

1. 修改代码时同步更新 API 文档
2. 更新 OpenAPI 规范文件
3. 更新 Postman 集合
4. 添加新的示例代码

### 2. 版本管理

- API 版本在 URL 中体现：`/api/v1/`
- 重大变更时增加版本号
- 保持向后兼容性

### 3. 审查检查清单

- [ ] 所有端点都有文档
- [ ] 参数说明完整
- [ ] 响应示例准确
- [ ] 错误情况已说明
- [ ] 示例代码可运行
- [ ] OpenAPI 规范有效

## 注意事项

1. 保持文档与代码同步
2. 提供完整的示例
3. 说明认证和授权方式
4. 记录所有可能的错误情况
5. 使用标准的 HTTP 状态码
6. 提供多语言示例
