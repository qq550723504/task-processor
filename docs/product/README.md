# 产品文档

这个目录放 `task-processor` 和 ListingKit 的产品视角文档。这里不按代码包解释系统，而是说明产品解决什么问题、用户如何完成任务、关键能力如何映射到后端与前端模块。

## 建议阅读顺序

1. [ListingKit 产品总览](./listingkit-product-overview.md)
2. [ListingKit 操作指南](./listingkit-operating-guide.md)
3. [ListingKit 产品路线图](./listingkit-product-roadmap.md)
4. [ListingKit 错误恢复手册](./listingkit-error-recovery.md)

## 面向读者

- 产品和运营负责人：理解 ListingKit 的业务闭环和用户路径。
- 前后端工程师：在改实现前对齐产品词汇、页面责任和状态语义。
- QA 和交付负责人：按真实用户流程做验证，而不是只验证接口是否返回。

## 文档边界

本目录只记录产品级语义。除非字段会直接影响用户操作，否则不展开底层字段清单。

实现和接口细节继续看：

- [product-listing-api README](../../cmd/product-listing-api/README.md)
- [ListingKit UI README](../../web/listingkit-ui/README.md)
- [ListingKit 重构边界](../architecture/listingkit-refactor-status.md)
- [ListingKit 真实接口联调清单](../../web/listingkit-ui/REAL_API_VALIDATION_CHECKLIST.md)
