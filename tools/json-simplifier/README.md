# JSON 简化工具

将复杂的 JSON 数据简化，保留结构但缩短值，方便 AI 理解数据结构。

## 功能

- 保留 JSON 完整结构
- 缩短字符串值（默认最大10字符）
- 数组只保留第一个元素作为示例
- 智能识别 URL、邮箱、ID 等特殊格式

## 使用方法

### 编译

```bash
cd tools/json-simplifier
go build -o json-simplifier
```

### 从文件读取

```bash
./json-simplifier -i input.json -o output.json
```

### 从标准输入读取

```bash
echo '{"name":"123456789","email":"test@example.com"}' | ./json-simplifier
```

### 自定义最大长度

```bash
./json-simplifier -i input.json -max 5
```

## 示例

输入：
```json
{
  "name": "这是一个很长的名字1234567890",
  "email": "user@example.com",
  "url": "https://www.example.com/very/long/path",
  "items": [
    {"id": "item1", "value": 100},
    {"id": "item2", "value": 200},
    {"id": "item3", "value": 300}
  ]
}
```

输出：
```json
{
  "name": "这是一个很长的名...",
  "email": "email@example.com",
  "url": "https://...",
  "items": [
    {
      "id": "item1",
      "value": 100
    }
  ]
}
```
