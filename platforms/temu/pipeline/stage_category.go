package pipeline

import (
	commonPipeline "task-processor/common/pipeline"
	"task-processor/platforms/temu/handlers"
)

// addCategoryHandlers 添加分类和SKU处理阶段处理器（11-17）
func (b *Builder) addCategoryHandlers(p *commonPipeline.Pipeline) {
	p.AddHandler(handlers.NewCategoryRecommendHandler()). // 11. 分类推荐
								AddHandler(handlers.NewCategoryDisclaimHandler()). // 12. 成本模板
								AddHandler(handlers.NewCommitCreateHandler()).     // 13. 提交创建
								AddHandler(handlers.NewCommitDetailHandler()).     // 14. 提交详情查询
								AddHandler(handlers.NewCostTemplateHandler()).     // 15. 成本模板
								AddHandler(handlers.NewOutGoodsSnCheckHandler()).  // 16. SKU编码重复检查
								AddHandler(handlers.NewCategoryHandler())          // 17. 分类处理
}
