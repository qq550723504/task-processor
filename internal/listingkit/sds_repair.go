package listingkit

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

var (
	ErrSDSRepairInvalidRequest   = errors.New("invalid SDS repair request")
	ErrSDSRepairNotEligible      = errors.New("task is not eligible for SDS repair")
	ErrSDSRepairUnavailable      = errors.New("SDS repair is unavailable")
	ErrSDSRepairLayerUnavailable = errors.New("selected SDS layer is unavailable for this variant")
)

// TaskSDSRepairService exposes the task-scoped repair flow for stale SDS layer mappings.
type TaskSDSRepairService interface {
	GetTaskSDSRepair(ctx context.Context, taskID string) (*TaskSDSRepairSession, error)
	RepairAndRetryTaskSDS(ctx context.Context, taskID string, req *ApplyTaskSDSRepairRequest) (*TaskResult, error)
}

type TaskSDSRepairSession struct {
	TaskID   string             `json:"task_id"`
	Variants []SDSRepairVariant `json:"variants"`
}

type SDSRepairVariant struct {
	VariantID  int64            `json:"variant_id"`
	VariantSKU string           `json:"variant_sku,omitempty"`
	Color      string           `json:"color,omitempty"`
	Size       string           `json:"size,omitempty"`
	OldLayerID string           `json:"old_layer_id"`
	Layers     []SDSRepairLayer `json:"layers"`
}

type SDSRepairLayer struct {
	ID   string `json:"id"`
	Name string `json:"name,omitempty"`
}

type ApplyTaskSDSRepairRequest struct {
	Variants []SDSRepairVariantSelection `json:"variants"`
}

type SDSRepairVariantSelection struct {
	VariantID int64  `json:"variant_id"`
	LayerID   string `json:"layer_id"`
}

func (s *service) GetTaskSDSRepair(ctx context.Context, taskID string) (*TaskSDSRepairSession, error) {
	if s == nil || s.repo == nil || strings.TrimSpace(taskID) == "" {
		return nil, ErrSDSRepairInvalidRequest
	}
	task, err := s.repo.GetTask(ctx, strings.TrimSpace(taskID))
	if err != nil {
		return nil, err
	}
	if !TaskEligibleForSDSRepair(task) {
		return nil, ErrSDSRepairNotEligible
	}
	remote := resolveSDSBaselineRemoteProvider(s)
	if remote == nil {
		return nil, ErrSDSRepairUnavailable
	}
	options := task.Request.Options.SDS
	session := &TaskSDSRepairSession{TaskID: task.ID, Variants: make([]SDSRepairVariant, 0, len(options.Variants))}
	for _, variant := range options.Variants {
		variantOptions := *options
		variantOptions.VariantID = variant.VariantID
		variantOptions.PrototypeGroupID = variant.PrototypeGroupID
		variantOptions.LayerID = variant.LayerID
		page, err := getSDSBaselineDesignProduct(ctx, remote, &variantOptions)
		if err != nil {
			return nil, err
		}
		if page == nil {
			return nil, ErrSDSRepairUnavailable
		}
		layers := make([]SDSRepairLayer, 0, len(page.Layers))
		for _, layer := range page.Layers {
			id := strings.TrimSpace(string(layer.ID))
			if id == "" {
				continue
			}
			layers = append(layers, SDSRepairLayer{ID: id, Name: strings.TrimSpace(layer.Name)})
		}
		session.Variants = append(session.Variants, SDSRepairVariant{
			VariantID: variant.VariantID, VariantSKU: variant.VariantSKU, Color: variant.Color,
			Size: variant.Size, OldLayerID: strings.TrimSpace(variant.LayerID), Layers: layers,
		})
	}
	if len(session.Variants) == 0 {
		return nil, ErrSDSRepairNotEligible
	}
	return session, nil
}

func TaskEligibleForSDSRepair(task *Task) bool {
	if task == nil || task.Request == nil || task.Request.Options == nil || task.Request.Options.SDS == nil {
		return false
	}
	if task.Status == TaskStatusPending || task.Status == TaskStatusProcessing || task.Result == nil {
		return false
	}
	return childTaskHasFailed(task.Result, "sds_design_sync")
}

func (s *service) RepairAndRetryTaskSDS(ctx context.Context, taskID string, req *ApplyTaskSDSRepairRequest) (*TaskResult, error) {
	if s == nil || s.repo == nil || strings.TrimSpace(taskID) == "" || req == nil {
		return nil, ErrSDSRepairInvalidRequest
	}
	task, err := s.repo.GetTask(ctx, strings.TrimSpace(taskID))
	if err != nil {
		return nil, err
	}
	if !TaskEligibleForSDSRepair(task) {
		return nil, ErrSDSRepairNotEligible
	}
	selected, err := normalizedSDSRepairSelections(req, task.Request.Options.SDS.Variants)
	if err != nil {
		return nil, err
	}
	remote := resolveSDSBaselineRemoteProvider(s)
	if remote == nil {
		return nil, ErrSDSRepairUnavailable
	}
	for _, variant := range task.Request.Options.SDS.Variants {
		variantOptions := *task.Request.Options.SDS
		variantOptions.VariantID = variant.VariantID
		variantOptions.PrototypeGroupID = variant.PrototypeGroupID
		page, err := getSDSBaselineDesignProduct(ctx, remote, &variantOptions)
		if err != nil {
			return nil, err
		}
		if !sdsBaselineLayerExists(page, selected[variant.VariantID]) {
			return nil, ErrSDSRepairLayerUnavailable
		}
	}
	options, err := cloneSDSSyncOptions(task.Request.Options.SDS)
	if err != nil {
		return nil, err
	}
	changes := make([]string, 0, len(options.Variants))
	for i := range options.Variants {
		oldLayerID := strings.TrimSpace(options.Variants[i].LayerID)
		newLayerID := selected[options.Variants[i].VariantID]
		options.Variants[i].LayerID = newLayerID
		changes = append(changes, fmt.Sprintf("variant %d: %s -> %s", options.Variants[i].VariantID, oldLayerID, newLayerID))
		if options.VariantID == options.Variants[i].VariantID {
			options.LayerID = newLayerID
		}
	}
	repairRepo, ok := s.repo.(TaskSDSRepairRepository)
	if !ok {
		return nil, ErrSDSRepairUnavailable
	}
	_, err = repairRepo.ReplaceTaskSDSOptionsForRetry(ctx, task.ID, options, PodExecutionAuditEvent{
		Kind: "sds_layer_repair", Code: "sds_layer_mapping_replaced",
		Message: "SDS layer mapping was replaced before retry.", Detail: strings.Join(changes, "; "),
		OccurredAt: time.Now().UTC(),
	})
	if err != nil {
		return nil, err
	}
	return s.RetryTaskChildTask(ctx, task.ID, &RetryChildTaskRequest{Kind: "sds_design_sync"})
}

func normalizedSDSRepairSelections(req *ApplyTaskSDSRepairRequest, variants []SDSSyncVariantOption) (map[int64]string, error) {
	if req == nil || len(req.Variants) != len(variants) || len(variants) == 0 {
		return nil, ErrSDSRepairInvalidRequest
	}
	selected := make(map[int64]string, len(req.Variants))
	for _, selection := range req.Variants {
		variantID := selection.VariantID
		layerID := strings.TrimSpace(selection.LayerID)
		if variantID <= 0 || layerID == "" {
			return nil, ErrSDSRepairInvalidRequest
		}
		if _, exists := selected[variantID]; exists {
			return nil, ErrSDSRepairInvalidRequest
		}
		selected[variantID] = layerID
	}
	for _, variant := range variants {
		if _, ok := selected[variant.VariantID]; !ok {
			return nil, ErrSDSRepairInvalidRequest
		}
	}
	return selected, nil
}

func cloneSDSSyncOptions(options *SDSSyncOptions) (*SDSSyncOptions, error) {
	if options == nil {
		return nil, ErrSDSRepairInvalidRequest
	}
	payload, err := json.Marshal(options)
	if err != nil {
		return nil, err
	}
	var copied SDSSyncOptions
	if err := json.Unmarshal(payload, &copied); err != nil {
		return nil, err
	}
	return &copied, nil
}
