package productimage

import "task-processor/internal/productimage/domain"

var ErrTaskNotFound = domain.ErrTaskNotFound
var ErrTaskNotPending = domain.ErrTaskNotPending

type TaskStatus = domain.TaskStatus

const (
	TaskStatusPending     = domain.TaskStatusPending
	TaskStatusProcessing  = domain.TaskStatusProcessing
	TaskStatusCompleted   = domain.TaskStatusCompleted
	TaskStatusNeedsReview = domain.TaskStatusNeedsReview
	TaskStatusRejected    = domain.TaskStatusRejected
	TaskStatusFailed      = domain.TaskStatusFailed
)

type AssetType = domain.AssetType

const (
	AssetTypeMainImage     = domain.AssetTypeMainImage
	AssetTypeWhiteBgImage  = domain.AssetTypeWhiteBgImage
	AssetTypeSubjectCutout = domain.AssetTypeSubjectCutout
	AssetTypeGalleryImage  = domain.AssetTypeGalleryImage
	AssetTypeSourceImage   = domain.AssetTypeSourceImage
)

type ImageProcessRequest = domain.ImageProcessRequest
type ReviewTaskRequest = domain.ReviewTaskRequest
type Task = domain.Task
type SourceBundle = domain.SourceBundle
type ProductContext = domain.ProductContext
type ImageAudit = domain.ImageAudit
type ImageStageTrace = domain.ImageStageTrace
type ImageStageSummary = domain.ImageStageSummary
type ImageCandidateSet = domain.ImageCandidateSet
type ImageAsset = domain.ImageAsset
type ImageIssue = domain.ImageIssue
type ComplianceReport = domain.ComplianceReport
type QualityAssessment = domain.QualityAssessment
type ReviewDecision = domain.ReviewDecision
type IPRiskReport = domain.IPRiskReport
type ImageProcessResult = domain.ImageProcessResult
type TaskResult = domain.TaskResult
