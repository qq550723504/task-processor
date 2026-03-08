# crawler 目录

## 用途

爬虫层，实现各个电商平台的网页爬取功能，包括页面解析、数据提取、反爬处理等。

## 目录结构

```
crawler/
├── alibaba1688/  # 1688 平台爬虫
├── amazon/       # Amazon 平台爬虫
└── shared/       # 共享的爬虫组件
```

## 子目录说明

### alibaba1688（1688 爬虫）
- 1688 平台页面爬取
- 产品信息提取
- 价格解析

**应该放置的文件：**
- `crawler.go` - 1688 爬虫主逻辑
- `parser.go` - 页面解析器
- `extractor.go` - 数据提取器

### amazon（Amazon 爬虫）
- Amazon 页面爬取
- 产品信息提取
- 价格和库存解析
- 评论爬取

**应该放置的文件：**
- `processor.go` - Amazon 处理器
- `parser.go` - 页面解析器
- `extractor.go` - 数据提取器

### shared（共享组件）
- 通用的爬虫工具
- 反爬处理
- 代理管理
- 浏览器池

**应该放置的文件：**
- `browser_pool.go` - 浏览器池
- `proxy_manager.go` - 代理管理
- `anti_crawler.go` - 反爬处理
- `utils.go` - 工具函数
