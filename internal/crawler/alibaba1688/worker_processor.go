// Package alibaba1688 提供1688爬虫处理器
package alibaba1688

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"

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

	updateResult := p.service.UpdateResult
	if crawlerTask.TenantID > 0 {
		updateResult = func(taskID string, fn func(*shared.CrawlerResult)) error {
			return p.service.UpdateResultForTenant(crawlerTask.TenantID, taskID, fn)
		}
	}

	product, accessMode, fallbackReason, err := p.fetchProduct(ctx, &crawlerTask)
	if err != nil {
		_ = updateResult(crawlerTask.TaskID, func(result *shared.CrawlerResult) {
			result.SourceAccessMode = string(accessMode)
			result.SourceFallbackReason = fallbackReason
		})
		return err
	}

	_ = updateResult(crawlerTask.TaskID, func(result *shared.CrawlerResult) {
		result.ProductData = shared.ProductToMap(product)
		result.SourceAccessMode = string(accessMode)
		result.SourceFallbackReason = fallbackReason
	})

	return nil
}

func (p *Crawler1688Processor) fetchProduct(ctx context.Context, task *shared.CrawlerTask) (*model.Product1688, sourceAccessMode, string, error) {
	if p == nil || p.service == nil || task == nil || task.SourceAccountID < 0 {
		return nil, sourceAccessModePublic, "", newAccountUnavailableError()
	}
	product, publicErr := p.service.processor1688.Process(ctx, task.URL)
	if publicErr == nil {
		p.service.recordSourceAccess("public")
		return product, sourceAccessModePublic, "", nil
	}
	if task.SourceAccountID == 0 || !IsAccountFallbackEligible(publicErr) {
		p.service.recordSourceAccess("source_public_unavailable")
		return nil, sourceAccessModePublic, "", newPublicUnavailableError()
	}
	profile, err := p.service.resolveAccountProfile(ctx, task.TenantID, task.SourceAccountID)
	if err != nil {
		if AccountProfileErrorCode(err) == AccountProfileDisabled {
			p.service.recordSourceAccess("source_account_disabled")
		} else {
			p.service.recordSourceAccess("source_account_unavailable")
		}
		return nil, sourceAccessModeAccountAssisted, sourceFallbackReason(publicErr), err
	}
	unlock := p.service.lockAccountProfile(profile)
	defer unlock()
	product, err = p.service.processor1688.ProcessWithAccountProfile(ctx, task.URL, profile)
	p.service.recordSourceAccess("account_assisted")
	if err != nil {
		return nil, sourceAccessModeAccountAssisted, sourceFallbackReason(publicErr), err
	}
	return product, sourceAccessModeAccountAssisted, sourceFallbackReason(publicErr), nil
}

type sourceAccessMode string

const (
	sourceAccessModePublic          sourceAccessMode = "public"
	sourceAccessModeAccountAssisted sourceAccessMode = "account_assisted"
)

func (s *Service) lockAccountProfile(profile AccountProfile) func() {
	key := strconv.FormatInt(profile.TenantID, 10) + ":" + strconv.FormatInt(profile.ID, 10)
	s.profileLocksMu.Lock()
	if s.profileLocks == nil {
		s.profileLocks = make(map[string]*sync.Mutex)
	}
	lock := s.profileLocks[key]
	if lock == nil {
		lock = &sync.Mutex{}
		s.profileLocks[key] = lock
	}
	s.profileLocksMu.Unlock()
	lock.Lock()
	return lock.Unlock
}

func (s *Service) resolveAccountProfile(ctx context.Context, tenantID, accountID int64) (AccountProfile, error) {
	if s == nil || tenantID <= 0 || accountID <= 0 || s.accountProfileResolver == nil {
		return AccountProfile{}, newAccountUnavailableError()
	}

	profile, err := s.accountProfileResolver.ResolveAlibaba1688Account(ctx, tenantID, accountID)
	if err != nil {
		if AccountProfileErrorCode(err) == AccountProfileDisabled {
			return AccountProfile{}, newAccountDisabledError()
		}
		return AccountProfile{}, newAccountUnavailableError()
	}
	if profile.ID != accountID || profile.TenantID != tenantID || strings.TrimSpace(profile.ProfileDir) == "" {
		return AccountProfile{}, newAccountUnavailableError()
	}
	return profile, nil
}
