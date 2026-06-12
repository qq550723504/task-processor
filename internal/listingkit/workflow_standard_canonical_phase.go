package listingkit

import (
	"context"
	"fmt"

	"github.com/sirupsen/logrus"

	"task-processor/internal/catalog/canonical"
	"task-processor/internal/productenrich"
)

type standardWorkflowCanonicalPhase struct {
	service *service
}

func buildStandardWorkflowCanonicalPhase(s *service) *standardWorkflowCanonicalPhase {
	return &standardWorkflowCanonicalPhase{service: s}
}

func (p *standardWorkflowCanonicalPhase) run(
	ctx context.Context,
	task *Task,
	result *ListingKitResult,
	recorder *workflowRecorder,
	log *logrus.Entry,
) (*canonical.Product, error) {
	if baseline, ok, baselineErr := p.service.sdsBaselineOrDefault().GetCachedBaseline(ctx, task); baselineErr != nil {
		log.WithError(baselineErr).Warn("sds baseline lookup failed; continuing")
	} else if ok {
		logCanonicalProductReuse(log, baseline, "reused SDS baseline canonical product for listing kit workflow")
		return baseline, nil
	}

	if shouldUseStudioCatalogCanonical(task) {
		stage := recorder.Start("sds_catalog_product", "")
		canonicalProduct := buildStudioFallbackCanonicalProduct(task)
		if canonicalProduct == nil {
			stage.Fail("sds_catalog_product_failed", "Failed to build SDS studio product", "")
			recorder.FinalizeSummary()
			return nil, fmt.Errorf("failed to build SDS studio product")
		}
		markChildTask(result, "sds_catalog_product", "", string(TaskStatusCompleted), "")
		stage.Complete()
		return canonicalProduct, nil
	}

	if cached, ok, cacheErr := p.service.getCachedCanonicalProduct(ctx, task); cacheErr != nil {
		log.WithError(cacheErr).Warn("canonical product cache lookup failed; running product enrich")
	} else if ok {
		stage := recorder.Start("product_enrich", "")
		markChildTask(result, "product_enrich", "", string(productenrich.TaskStatusCompleted), "")
		stage.Complete()
		logCanonicalProductReuse(log, cached, "reused cached canonical product for listing kit workflow")
		return cached, nil
	}

	stage := recorder.Start("product_enrich", "")
	productSvc := resolveWorkflowProductService(p.service)
	productTask, err := productSvc.CreateGenerateTask(productenrich.WithInlineTaskExecution(ctx), toProductGenerateRequest(task))
	if err != nil {
		markChildTask(result, "product_enrich", "", string(TaskStatusFailed), err.Error())
		stage.Fail("product_task_creation_failed", "Product enrichment task creation failed", err.Error())
		recorder.FinalizeSummary()
		return nil, fmt.Errorf("failed to create product task: %w", err)
	}
	stage.SetTaskID(productTask.ID)
	markChildTask(result, "product_enrich", productTask.ID, string(productenrich.TaskStatusPending), "")

	productJSON, err := productSvc.ProcessProduct(ctx, productTask)
	if err != nil {
		markChildTask(result, "product_enrich", productTask.ID, string(TaskStatusFailed), err.Error())
		if !shouldUseStudioProductFallback(task) {
			stage.Fail("product_enrich_failed", "Product enrichment failed", err.Error())
			recorder.FinalizeSummary()
			return nil, fmt.Errorf("product enrichment failed: %w", err)
		}
		canonicalProduct := buildStudioFallbackCanonicalProduct(task)
		if canonicalProduct == nil {
			stage.Fail("product_enrich_failed", "Product enrichment failed", err.Error())
			recorder.FinalizeSummary()
			return nil, fmt.Errorf("product enrichment failed: %w", err)
		}
		appendWarning(result, "product enrichment failed, studio fallback canonical product used: "+err.Error())
		stage.Degrade("product_enrich_studio_fallback", "Product enrichment failed; studio fallback canonical product used", err.Error())
		return canonicalProduct, nil
	}

	markChildTask(result, "product_enrich", productTask.ID, string(productenrich.TaskStatusCompleted), "")
	stage.Complete()
	canonicalProduct := productenrich.BuildCanonicalProduct(productTask.Request, productJSON)
	if cacheErr := p.service.saveCanonicalProductCache(ctx, task, canonicalProduct); cacheErr != nil {
		log.WithError(cacheErr).Warn("canonical product cache save failed")
	}
	log.WithFields(logrus.Fields{
		"child_task_id": productTask.ID,
		"title": func() string {
			if canonicalProduct == nil {
				return ""
			}
			return canonicalProduct.Title
		}(),
	}).Info("product enrichment completed for listing kit workflow")
	return canonicalProduct, nil
}

func logCanonicalProductReuse(log *logrus.Entry, product *canonical.Product, message string) {
	log.WithFields(logrus.Fields{
		"title": func() string {
			if product == nil {
				return ""
			}
			return product.Title
		}(),
	}).Info(message)
}
