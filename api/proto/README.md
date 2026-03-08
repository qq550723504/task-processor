# Protocol Buffers 定义

## 用途

存放 gRPC 服务的 Protocol Buffers 定义文件。

## 示例文件

创建 `service.proto` 文件：

```protobuf
syntax = "proto3";

package taskprocessor.v1;

option go_package = "github.com/yourorg/task-processor/api/proto/v1;v1";

service TaskService {
  rpc CreateTask(CreateTaskRequest) returns (Task);
  rpc GetTask(GetTaskRequest) returns (Task);
  rpc ListTasks(ListTasksRequest) returns (ListTasksResponse);
}

message CreateTaskRequest {
  string platform = 1;
  string product_id = 2;
  int64 tenant_id = 3;
  int64 store_id = 4;
}

message GetTaskRequest {
  int64 id = 1;
}

message ListTasksRequest {
  int32 page = 1;
  int32 page_size = 2;
}

message Task {
  int64 id = 1;
  string platform = 2;
  string product_id = 3;
  string status = 4;
  int64 created_at = 5;
}

message ListTasksResponse {
  repeated Task tasks = 1;
  int32 total = 2;
}
```

## 生成代码

```bash
# 安装 protoc 编译器和 Go 插件
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

# 生成代码
protoc --go_out=. --go_opt=paths=source_relative \
    --go-grpc_out=. --go-grpc_opt=paths=source_relative \
    api/proto/service.proto
```

## 最佳实践

1. 使用语义化版本（v1, v2）
2. 向后兼容的修改
3. 使用 buf 工具管理 proto 文件
4. 添加详细的注释
