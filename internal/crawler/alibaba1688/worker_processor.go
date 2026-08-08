// Package alibaba1688 提供1688爬虫处理器
package alibaba1688

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"task-processor/internal/crawler/alibaba1688/model"
	"task-processor/internal/crawler/shared"
	"task-processor/internal/infra/worker"
)

type globallyUniqueAccountProfileResolver interface {
	AccountProfileResolver
	ResolveAlibaba1688AccountByUniqueID(context.Context, int64) (AccountProfile, error)
}

// Crawler1688Processor 实现 worker.Processor 接口
type Crawler1688Processor struct {
	service *Service
}

// Start 启动处理器
func (p *Crawler1688Processor) Start(_ context.Context) error { return nil }

// Close 关闭处理器
func (p *Crawler1688Processor) Close(_ context.Context) {}

// ProcessTask 处理任务
func (p *Crawler1688Processor) ProcessTask(ctx context.Context, job worker.WorkerJob) error {
	var crawlerTask shared.CrawlerTask
	if err := json.Unmarshal([]byte(job.TaskData), &crawlerTask); err != nil {
		return fmt.Errorf("解析任务数据失败: %w", err)
	}

	var product *model.Product1688
	if crawlerTask.SourceAccountID > 0 {
		profile, err := p.service.resolveAccountProfile(ctx, crawlerTask.SourceAccountID)
		if err != nil {
			return err
		}
		product, err = p.service.processor1688.ProcessWithAccountProfile(crawlerTask.URL, profile)
		if err != nil {
			return err
		}
	} else {
		resolvedProduct, err := p.service.processor1688.Process(crawlerTask.URL)
		if err != nil {
			return err
		}
		product = resolvedProduct
	}

	p.service.UpdateResult(crawlerTask.TaskID, func(result *shared.CrawlerResult) {
		result.ProductData = shared.ProductToMap(product)
	})

	return nil
}

func (s *Service) resolveAccountProfile(ctx context.Context, accountID int64) (AccountProfile, error) {
	if s == nil || accountID <= 0 || s.accountProfileResolver == nil {
		return AccountProfile{}, newAccountProfileError(AccountProfileUnavailable, "1688 account is unavailable")
	}

	resolver, ok := s.accountProfileResolver.(globallyUniqueAccountProfileResolver)
	if !ok {
		// The worker task has no trusted tenant context. A tenant-scoped resolver
		// must never be invoked with caller-controlled or invented tenant data.
		return AccountProfile{}, newAccountProfileError(AccountProfileUnavailable, "1688 account is unavailable")
	}

	profile, err := resolver.ResolveAlibaba1688AccountByUniqueID(ctx, accountID)
	if err != nil || profile.ID != accountID || profile.TenantID <= 0 || strings.TrimSpace(profile.ProfileDir) == "" {
		return AccountProfile{}, newAccountProfileError(AccountProfileUnavailable, "1688 account is unavailable")
	}
	return profile, nil
}
