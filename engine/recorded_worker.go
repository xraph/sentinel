package engine

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/xraph/sentinel"
	"github.com/xraph/sentinel/recorded"
)

// RecordedWorkerStatus describes local worker and backend availability.
type RecordedWorkerStatus struct {
	Durable       bool `json:"durable"`
	WorkerRunning bool `json:"worker_running"`
	Ready         bool `json:"ready"`
}

type recordedPersistence interface {
	RecordedPersistent(context.Context) (bool, error)
}

func recordedWorkerKey(ctx context.Context) string {
	return strconv.Quote(sentinel.AppFromContext(ctx)) + "," + strconv.Quote(sentinel.TenantFromContext(ctx))
}

// RecordedStatus is conservative: readiness requires a persistent store and a
// running worker in this process for the caller's exact app and tenant.
func (e *Engine) RecordedStatus(ctx context.Context) (RecordedWorkerStatus, error) {
	var status RecordedWorkerStatus
	if sentinel.AppFromContext(ctx) == "" {
		return status, fmt.Errorf("%w: recorded worker requires app scope", sentinel.ErrInvalidInput)
	}
	if e.store == nil {
		return status, sentinel.ErrNoStore
	}
	if persistence, ok := e.store.(recordedPersistence); ok {
		durable, err := persistence.RecordedPersistent(ctx)
		if err != nil {
			return status, err
		}
		status.Durable = durable
	}
	e.recordedMu.Lock()
	status.WorkerRunning = e.recordedWorkers[recordedWorkerKey(ctx)] > 0
	e.recordedMu.Unlock()
	status.Ready = status.Durable && status.WorkerRunning
	return status, nil
}

// WorkRecorded runs explicitly scoped durable work until cancellation or Stop.
// Hosts call it for each authenticated app/tenant binding. It never scans across
// scopes and never invokes a target. Memory backends support tests only.
func (e *Engine) WorkRecorded(ctx context.Context) error {
	if sentinel.AppFromContext(ctx) == "" {
		return fmt.Errorf("%w: recorded worker requires app scope", sentinel.ErrInvalidInput)
	}
	if e.store == nil {
		return sentinel.ErrNoStore
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	stop := context.AfterFunc(e.baseCtx, cancel)
	defer stop()
	e.runs.Add(1)
	defer e.runs.Done()
	key := recordedWorkerKey(ctx)
	e.recordedMu.Lock()
	if e.recordedWorkers == nil {
		e.recordedWorkers = make(map[string]int)
	}
	e.recordedWorkers[key]++
	e.recordedMu.Unlock()
	defer func() { e.recordedMu.Lock(); e.recordedWorkers[key]--; e.recordedMu.Unlock() }()
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		after := ""
		for {
			jobs, err := e.store.ListRecordedJobs(ctx, recorded.Query{Active: true, After: after, Limit: 100})
			if err != nil {
				return err
			}
			for _, j := range jobs {
				if j.State == recorded.JobRunning && j.LeaseUntil.After(time.Now()) {
					continue
				}
				result, evalErr := e.evaluateRecordedJob(ctx, j, false)
				if ctx.Err() != nil {
					return ctx.Err()
				}
				if evalErr != nil && !errors.Is(evalErr, recorded.ErrConflict) {
					if result != nil && result.Recorded.Terminal() {
						continue
					}
					// Results and bounded attempts remain durable. Returning exposes a store
					// failure to the host instead of silently reporting a healthy worker.
					return evalErr
				}
			}
			if len(jobs) < 100 {
				break
			}
			after = jobs[len(jobs)-1].ID.String()
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}
