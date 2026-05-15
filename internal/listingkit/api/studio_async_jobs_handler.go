package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"task-processor/internal/listingkit"
)

const (
	studioAsyncJobTTL    = time.Hour
	studioAsyncJobMaxLen = 256
)

type studioAsyncJobStatus string

const (
	studioAsyncJobRunning   studioAsyncJobStatus = "running"
	studioAsyncJobSucceeded studioAsyncJobStatus = "succeeded"
	studioAsyncJobFailed    studioAsyncJobStatus = "failed"
)

type startStudioAsyncJobRequest struct {
	Path string          `json:"path"`
	Body json.RawMessage `json:"body"`
}

type studioAsyncJob struct {
	ID             string               `json:"job_id"`
	Path           string               `json:"path"`
	Status         studioAsyncJobStatus `json:"status"`
	Result         any                  `json:"result,omitempty"`
	Error          string               `json:"error,omitempty"`
	UpstreamStatus int                  `json:"upstream_status,omitempty"`
	CreatedAt      time.Time            `json:"created_at"`
	UpdatedAt      time.Time            `json:"updated_at"`
	FinishedAt     *time.Time           `json:"finished_at,omitempty"`
}

type studioAsyncJobStore struct {
	mu       sync.Mutex
	jobs     map[string]*studioAsyncJob
	ttl      time.Duration
	max      int
	filePath string
}

func newStudioAsyncJobStore() *studioAsyncJobStore {
	return &studioAsyncJobStore{
		jobs: make(map[string]*studioAsyncJob),
		ttl:  studioAsyncJobTTL,
		max:  studioAsyncJobMaxLen,
	}
}

func newDefaultStudioAsyncJobStore() (*studioAsyncJobStore, error) {
	if path := strings.TrimSpace(os.Getenv("LISTINGKIT_STUDIO_ASYNC_JOB_STORE_PATH")); path != "" {
		store, err := newStudioAsyncJobFileStore(path, studioAsyncJobTTL, studioAsyncJobMaxLen)
		if err != nil {
			return nil, err
		}
		return store, nil
	}
	return newStudioAsyncJobStore(), nil
}

func newStudioAsyncJobFileStore(path string, ttl time.Duration, max int) (*studioAsyncJobStore, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	store := &studioAsyncJobStore{
		jobs:     make(map[string]*studioAsyncJob),
		ttl:      ttl,
		max:      max,
		filePath: path,
	}
	if err := store.loadFromFile(); err != nil {
		return nil, err
	}
	return store, nil
}

func (s *studioAsyncJobStore) create(path string) studioAsyncJob {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.cleanupLocked(time.Now())
	job := &studioAsyncJob{
		ID:        newStudioAsyncJobID(),
		Path:      path,
		Status:    studioAsyncJobRunning,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	s.jobs[job.ID] = job
	s.persistLocked()
	return cloneStudioAsyncJob(job)
}

func (s *studioAsyncJobStore) get(id string) (studioAsyncJob, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.cleanupLocked(time.Now())
	job, ok := s.jobs[id]
	if !ok {
		return studioAsyncJob{}, false
	}
	return cloneStudioAsyncJob(job), true
}

func (s *studioAsyncJobStore) succeed(id string, result any) {
	s.update(id, studioAsyncJobSucceeded, result, "", http.StatusOK)
}

func (s *studioAsyncJobStore) fail(id string, err error, status int) {
	message := "async job failed"
	if err != nil {
		message = err.Error()
	}
	s.update(id, studioAsyncJobFailed, nil, message, status)
}

func (s *studioAsyncJobStore) update(id string, status studioAsyncJobStatus, result any, message string, upstreamStatus int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	job, ok := s.jobs[id]
	if !ok {
		return
	}
	now := time.Now().UTC()
	job.Status = status
	job.Result = result
	job.Error = message
	job.UpstreamStatus = upstreamStatus
	job.UpdatedAt = now
	job.FinishedAt = &now
	s.persistLocked()
}

func (s *studioAsyncJobStore) cleanupLocked(now time.Time) {
	for id, job := range s.jobs {
		if now.Sub(job.UpdatedAt) > s.ttl {
			delete(s.jobs, id)
		}
	}
	if len(s.jobs) <= s.max {
		return
	}
	for id := range s.jobs {
		delete(s.jobs, id)
		if len(s.jobs) <= s.max {
			return
		}
	}
}

func (s *studioAsyncJobStore) loadFromFile() error {
	if strings.TrimSpace(s.filePath) == "" {
		return nil
	}
	data, err := os.ReadFile(s.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	var payload struct {
		Jobs []*studioAsyncJob `json:"jobs"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return err
	}
	now := time.Now()
	for _, job := range payload.Jobs {
		if job == nil || job.ID == "" || now.Sub(job.UpdatedAt) > s.ttl {
			continue
		}
		cloned := cloneStudioAsyncJob(job)
		s.jobs[cloned.ID] = &cloned
	}
	return nil
}

func (s *studioAsyncJobStore) persistLocked() {
	if strings.TrimSpace(s.filePath) == "" {
		return
	}
	payload := struct {
		Jobs []*studioAsyncJob `json:"jobs"`
	}{
		Jobs: make([]*studioAsyncJob, 0, len(s.jobs)),
	}
	for _, job := range s.jobs {
		cloned := cloneStudioAsyncJob(job)
		payload.Jobs = append(payload.Jobs, &cloned)
	}
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return
	}
	if err := os.MkdirAll(filepath.Dir(s.filePath), 0o755); err != nil {
		return
	}
	_ = os.WriteFile(s.filePath, data, 0o644)
}

func cloneStudioAsyncJob(job *studioAsyncJob) studioAsyncJob {
	if job == nil {
		return studioAsyncJob{}
	}
	cloned := *job
	if job.FinishedAt != nil {
		finished := *job.FinishedAt
		cloned.FinishedAt = &finished
	}
	return cloned
}

func newStudioAsyncJobID() string {
	var data [12]byte
	if _, err := rand.Read(data[:]); err != nil {
		return strings.ReplaceAll(time.Now().UTC().Format(time.RFC3339Nano), ":", "")
	}
	return hex.EncodeToString(data[:])
}

func (h *handler) StartStudioAsyncJob(c *gin.Context) {
	var req startStudioAsyncJobRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": err.Error()})
		return
	}
	req.Path = strings.TrimSpace(req.Path)
	if req.Path != "/studio/designs" && req.Path != "/studio/product-images" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": "unsupported async job path"})
		return
	}
	if len(req.Body) == 0 {
		req.Body = json.RawMessage(`{}`)
	}

	job := h.studioAsyncJobs.create(req.Path)
	ctx := detachedRequestContext(c)
	baseURL := requestBaseURL(c)
	go h.runStudioAsyncJob(ctx, job.ID, req.Path, req.Body, baseURL)

	c.JSON(http.StatusAccepted, job)
}

func (h *handler) GetStudioAsyncJob(c *gin.Context) {
	job, ok := h.studioAsyncJobs.get(c.Param("job_id"))
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "not_found", "message": "studio async job not found"})
		return
	}
	c.JSON(http.StatusOK, job)
}

func (h *handler) runStudioAsyncJob(ctx context.Context, jobID string, path string, body json.RawMessage, baseURL string) {
	var result any
	var err error
	status := http.StatusInternalServerError

	switch path {
	case "/studio/designs":
		var req listingkit.StudioDesignRequest
		if decodeErr := json.Unmarshal(body, &req); decodeErr != nil {
			err = decodeErr
			status = http.StatusBadRequest
			break
		}
		response, callErr := h.service.GenerateStudioDesigns(ctx, &req)
		if callErr != nil {
			err = callErr
			break
		}
		if response != nil {
			for idx := range response.Images {
				response.Images[idx].ImageURL = absolutizeUploadedImageURLsWithBase(baseURL, []string{response.Images[idx].ImageURL})[0]
			}
		}
		result = response
	case "/studio/product-images":
		var req listingkit.StudioProductImageRequest
		if decodeErr := json.Unmarshal(body, &req); decodeErr != nil {
			err = decodeErr
			status = http.StatusBadRequest
			break
		}
		response, callErr := h.service.GenerateStudioProductImages(ctx, &req)
		if callErr != nil {
			err = callErr
			break
		}
		if response != nil {
			for idx := range response.Images {
				response.Images[idx].ImageURL = absolutizeUploadedImageURLsWithBase(baseURL, []string{response.Images[idx].ImageURL})[0]
			}
		}
		result = response
	default:
		err = listingkit.ErrTaskNotFound
		status = http.StatusBadRequest
	}

	if err != nil {
		if strings.Contains(err.Error(), "invalid request") {
			status = http.StatusBadRequest
		}
		h.studioAsyncJobs.fail(jobID, err, status)
		return
	}
	h.studioAsyncJobs.succeed(jobID, result)
}

func requestBaseURL(c *gin.Context) string {
	scheme := "http"
	if c.Request.TLS != nil || strings.EqualFold(c.GetHeader("X-Forwarded-Proto"), "https") {
		scheme = "https"
	}
	return scheme + "://" + c.Request.Host
}
