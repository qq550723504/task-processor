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

	if crawlerTask.SourceAccountID < 0 {
		return newAccountProfileError(AccountProfileUnavailable, "1688 account is unavailable")
	}

	var product *model.Product1688
	if crawlerTask.SourceAccountID > 0 {
		profile, err := p.service.resolveAccountProfile(ctx, crawlerTask.TenantID, crawlerTask.SourceAccountID)
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

func (s *Service) resolveAccountProfile(ctx context.Context, tenantID, accountID int64) (AccountProfile, error) {
	if s == nil || tenantID <= 0 || accountID <= 0 || s.accountProfileResolver == nil {
		return AccountProfile{}, newAccountProfileError(AccountProfileUnavailable, "1688 account is unavailable")
	}

	profile, err := s.accountProfileResolver.ResolveAlibaba1688Account(ctx, tenantID, accountID)
	if err != nil {
		if AccountProfileErrorCode(err) == AccountProfileDisabled {
			return AccountProfile{}, newAccountProfileError(AccountProfileDisabled, "1688 account is disabled")
		}
		return AccountProfile{}, newAccountProfileError(AccountProfileUnavailable, "1688 account is unavailable")
	}
	if profile.ID != accountID || profile.TenantID != tenantID || strings.TrimSpace(profile.ProfileDir) == "" {
		return AccountProfile{}, newAccountProfileError(AccountProfileUnavailable, "1688 account is unavailable")
	}
	return profile, nil
}
