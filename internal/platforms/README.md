# platforms 目录

## 用途

平台层，实现各个电商平台的业务逻辑，包括任务处理、数据转换、平台特定的规则等。

## 目录结构

```
platforms/
├── amazon/   # Amazon 平台业务逻辑
├── common/   # 平台通用组件
├── shein/    # Shein 平台业务逻辑
└── temu/     # Temu 平台业务逻辑
```

## 子目录说明

### amazon
- Amazon 平台特定的业务逻辑
- Amazon 数据模型转换

### common
- 平台通用的接口定义
- 平台通用的工具函数
- 平台基类

### shein
- Shein 平台任务处理
- Shein 产品数据处理
- Shein 平台规则

### temu
- Temu 平台任务处理
- Temu 产品数据处理
- Temu 平台规则

## 编码规范

1. 每个平台实现统一的接口
2. 平台特定的逻辑封装在对应目录
3. 使用工厂模式创建平台处理器
