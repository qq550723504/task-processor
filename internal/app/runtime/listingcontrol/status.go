package listingcontrol

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	controllib "task-processor/internal/listingcontrol"

	"github.com/sirupsen/logrus"
)

type ControlPlaneStatus struct {
	Status             string                     `json:"status"`
	Ready              bool                       `json:"ready"`
	StartedAt          time.Time                  `json:"startedAt"`
	Leader             LeaderSnapshot             `json:"leader"`
	LastCycleStartedAt *time.Time                 `json:"lastCycleStartedAt,omitempty"`
	LastCycleAt        *time.Time                 `json:"lastCycleAt,omitempty"`
	LastError          string                     `json:"lastError,omitempty"`
	ConsecutiveErrors  int                        `json:"consecutiveErrors"`
	Recovery           controllib.RecoverySummary `json:"recovery"`
	Dispatch           controllib.DispatchSummary `json:"dispatch"`
	SkippedByReason    map[string]int             `json:"skippedByReason,omitempty"`
	FailedByReason     map[string]int             `json:"failedByReason,omitempty"`
	Stores             []ControlPlaneStoreStatus  `json:"stores,omitempty"`
}

type LeaderSnapshot struct {
	Key        string     `json:"key,omitempty"`
	Owner      string     `json:"owner,omitempty"`
	IsLeader   bool       `json:"isLeader"`
	TTL        string     `json:"ttl,omitempty"`
	AcquiredAt *time.Time `json:"acquiredAt,omitempty"`
	RenewedAt  *time.Time `json:"renewedAt,omitempty"`
}

type ControlPlaneStoreStatus struct {
	TenantID  int64  `json:"tenantId"`
	StoreID   int64  `json:"storeId"`
	OwnerNode string `json:"ownerNode,omitempty"`
	Queue     string `json:"queue,omitempty"`
	Capacity  int    `json:"capacity"`
	Queued    int64  `json:"queued"`
	Action    string `json:"action"`
	Reason    string `json:"reason,omitempty"`
}

type StatusTracker struct {
	mu     sync.RWMutex
	status ControlPlaneStatus
}

func NewStatusTracker(now time.Time) *StatusTracker {
	if now.IsZero() {
		now = time.Now()
	}
	return &StatusTracker{
		status: ControlPlaneStatus{
			Status:    "starting",
			StartedAt: now,
		},
	}
}

func (t *StatusTracker) BeginCycle(now time.Time) {
	if t == nil {
		return
	}
	if now.IsZero() {
		now = time.Now()
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.status.LastCycleStartedAt = timePtr(now)
}

func (t *StatusTracker) RecordSuccess(recovery controllib.RecoverySummary, dispatch controllib.DispatchSummary, now time.Time) {
	if t == nil {
		return
	}
	if now.IsZero() {
		now = time.Now()
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.status.Status = "ok"
	t.status.Ready = true
	t.status.LastCycleAt = timePtr(now)
	t.status.LastError = ""
	t.status.ConsecutiveErrors = 0
	t.status.Recovery = recovery
	t.status.Dispatch = dispatch
	t.status.SkippedByReason = countReasons(dispatch.Decisions, controllib.DispatchActionSkipped, controllib.DispatchActionDryRun)
	t.status.FailedByReason = countReasons(dispatch.Decisions, controllib.DispatchActionFailed)
	t.status.Stores = storeStatuses(dispatch.Decisions)
}

func (t *StatusTracker) RecordLeader(snapshot LeaderSnapshot) {
	if t == nil {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.status.Leader = snapshot
}

func (t *StatusTracker) RecordStandby(snapshot LeaderSnapshot, now time.Time) {
	if t == nil {
		return
	}
	if now.IsZero() {
		now = time.Now()
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.status.Status = "standby"
	t.status.Ready = false
	t.status.LastCycleAt = timePtr(now)
	t.status.LastError = ""
	t.status.Leader = snapshot
}

func (t *StatusTracker) RecordError(err error, now time.Time) {
	if t == nil {
		return
	}
	if now.IsZero() {
		now = time.Now()
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.status.Status = "error"
	t.status.Ready = false
	t.status.LastCycleAt = timePtr(now)
	t.status.LastError = err.Error()
	t.status.ConsecutiveErrors++
}

func (t *StatusTracker) Snapshot() ControlPlaneStatus {
	if t == nil {
		return ControlPlaneStatus{Status: "unconfigured"}
	}
	t.mu.RLock()
	defer t.mu.RUnlock()
	out := t.status
	out.SkippedByReason = cloneStringIntMap(t.status.SkippedByReason)
	out.FailedByReason = cloneStringIntMap(t.status.FailedByReason)
	out.Stores = append([]ControlPlaneStoreStatus(nil), t.status.Stores...)
	out.Dispatch.Decisions = append([]controllib.DispatchDecision(nil), t.status.Dispatch.Decisions...)
	out.Recovery.ProcessingTaskIDs = append([]int64(nil), t.status.Recovery.ProcessingTaskIDs...)
	out.Recovery.StaleQueuedTaskIDs = append([]int64(nil), t.status.Recovery.StaleQueuedTaskIDs...)
	return out
}

func newStatusHandler(tracker *StatusTracker) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("/ready", func(w http.ResponseWriter, r *http.Request) {
		snapshot := tracker.Snapshot()
		if !snapshot.Ready {
			writeJSON(w, http.StatusServiceUnavailable, snapshot)
			return
		}
		writeJSON(w, http.StatusOK, snapshot)
	})
	mux.HandleFunc("/status", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, tracker.Snapshot())
	})
	return mux
}

func startStatusServer(ctx context.Context, port int, tracker *StatusTracker, logger *logrus.Logger) error {
	if port <= 0 {
		port = 8081
	}
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return fmt.Errorf("listen status server on port %d: %w", port, err)
	}
	server := &http.Server{
		Handler:           newStatusHandler(tracker),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil && logger != nil {
			logger.WithError(err).Warn("shutdown listing control-plane status server failed")
		}
	}()

	go func() {
		if logger != nil {
			logger.Infof("listing control-plane status server listening on :%d", port)
		}
		if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) && logger != nil {
			logger.WithError(err).Error("listing control-plane status server stopped")
		}
	}()
	return nil
}

func writeJSON(w http.ResponseWriter, statusCode int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(value)
}

func countReasons(decisions []controllib.DispatchDecision, actions ...string) map[string]int {
	actionSet := make(map[string]struct{}, len(actions))
	for _, action := range actions {
		actionSet[action] = struct{}{}
	}
	out := map[string]int{}
	for _, decision := range decisions {
		if _, ok := actionSet[decision.Action]; !ok {
			continue
		}
		reason := strings.TrimSpace(decision.Reason)
		if reason == "" {
			reason = "none"
		}
		out[reason]++
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func storeStatuses(decisions []controllib.DispatchDecision) []ControlPlaneStoreStatus {
	byStore := make(map[string]ControlPlaneStoreStatus)
	for _, decision := range decisions {
		if decision.StoreID == 0 {
			continue
		}
		key := fmt.Sprintf("%d:%d", decision.TenantID, decision.StoreID)
		byStore[key] = ControlPlaneStoreStatus{
			TenantID:  decision.TenantID,
			StoreID:   decision.StoreID,
			OwnerNode: decision.OwnerNode,
			Queue:     decision.Queue,
			Capacity:  decision.Capacity,
			Queued:    decision.Queued,
			Action:    decision.Action,
			Reason:    decision.Reason,
		}
	}
	out := make([]ControlPlaneStoreStatus, 0, len(byStore))
	for _, item := range byStore {
		out = append(out, item)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].TenantID != out[j].TenantID {
			return out[i].TenantID < out[j].TenantID
		}
		return out[i].StoreID < out[j].StoreID
	})
	return out
}

func cloneStringIntMap(in map[string]int) map[string]int {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]int, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func timePtr(value time.Time) *time.Time {
	return &value
}
