# data 目录

## 用途

存放应用程序运行时需要的静态数据文件，如规则配置、敏感词库、禁止项列表等。

## 目录结构

```
data/
├── prohibited_items_temu.json    # Temu 平台禁止商品列表
├── sensitive_words_shein.json    # Shein 平台敏感词库
├── sensitive_words_temu.json     # Temu 平台敏感词库
└── sensitive_words.json          # 通用敏感词库
```

## 应该放置的文件

- 敏感词库（JSON/TXT 格式）
- 禁止商品列表
- 分类映射表
- 规则配置数据
- 静态参考数据
- 测试数据集

## 文件命名规范

1. 使用小写字母和下划线：`sensitive_words.json`
2. 平台特定数据添加平台后缀：`{数据类型}_{平台}.json`
3. 使用描述性名称，清晰表达文件内容

## 数据文件格式示例

```json
// sensitive_words.json
{
  "version": "1.0.0",
  "updated_at": "2024-01-01",
  "words": [
    "敏感词1",
    "敏感词2",
    "敏感词3"
  ],
  "patterns": [
    "正则表达式1",
    "正则表达式2"
  ]
}
```

```json
// prohibited_items_temu.json
{
  "version": "1.0.0",
  "categories": [
    {
      "id": "weapons",
      "name": "武器类",
      "keywords": ["刀具", "枪支"]
    },
    {
      "id": "drugs",
      "name": "药品类",
      "keywords": ["处方药", "违禁药"]
    }
  ]
}
```

## 注意事项

- 数据文件应该有版本号和更新时间
- 使用 JSON 格式便于程序解析
- 大型数据文件考虑压缩存储
- 定期更新和维护数据文件
- 提供数据文件的 schema 说明
- 敏感数据考虑加密存储
