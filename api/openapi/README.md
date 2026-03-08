# OpenAPI/Swagger 规范

## 用途

存放 REST API 的 OpenAPI/Swagger 定义文件。

## 示例文件

创建 `api.yaml` 文件定义你的 API：

```yaml
openapi: 3.0.0
info:
  title: Task Processor API
  version: 1.0.0
  description: Task processing service API

servers:
  - url: http://localhost:8080/api/v1
    description: Development server

paths:
  /tasks:
    post:
      summary: Create a new task
      requestBody:
        required: true
        content:
          application/json:
            schema:
              $ref: '#/components/schemas/CreateTaskRequest'
      responses:
        '201':
          description: Task created successfully
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/Task'

components:
  schemas:
    CreateTaskRequest:
      type: object
      required:
        - platform
        - productId
      properties:
        platform:
          type: string
          enum: [amazon, temu, shein]
        productId:
          type: string
    
    Task:
      type: object
      properties:
        id:
          type: integer
        platform:
          type: string
        status:
          type: string
```

## 生成文档

使用 Swagger UI 查看文档：
```bash
# 使用 Docker 运行 Swagger UI
docker run -p 8081:8080 -e SWAGGER_JSON=/api/openapi/api.yaml -v $(pwd)/api:/api swaggerapi/swagger-ui
```

## 生成代码

使用 swagger-codegen 生成客户端代码：
```bash
swagger-codegen generate -i api/openapi/api.yaml -l go -o client/
```
