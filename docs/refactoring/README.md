# ListingKit 重构文档

本目录包含 ListingKit 项目的重构计划和执行记录。

## 当前进行中的重构

- [architecture-improvement-plan.md](./architecture-improvement-plan.md) - 架构质量改进计划

## 重构原则

1. **向后兼容**: 所有改动必须保持 API 和行为兼容
2. **小步快跑**: 每个任务独立可测试,频繁提交
3. **测试先行**: 修改前先确保测试覆盖,重构后验证测试通过
4. **渐进式**: 避免大规模一次性重写,采用逐步替换策略

## 风险管控

- 每次重构前备份关键配置
- 保留回滚方案
- 重要改动需要 code review
