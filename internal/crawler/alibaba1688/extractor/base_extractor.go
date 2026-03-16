// Package extractor 提供1688产品数据提取功能
package extractor

import (
	"task-processor/internal/crawler/alibaba1688/model"

	"github.com/playwright-community/playwright-go"
)

// BaseExtractor 基础提取器接口
type BaseExtractor interface {
	Extract(page playwright.Page, product *model.Product1688) error
}
