package engine_test

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/xraph/grove"
	"github.com/xraph/grove/drivers/sqlitedriver"
	_ "github.com/xraph/grove/drivers/sqlitedriver/sqlitemigrate"

	"github.com/xraph/sentinel"
	"github.com/xraph/sentinel/engine"
	"github.com/xraph/sentinel/recorded"
	sentinelsqlite "github.com/xraph/sentinel/store/sqlite"
)

func openRecordedSQLite(t *testing.T, path string) *sentinelsqlite.Store {
	t.Helper()
	driver := sqlitedriver.New()
	if err := driver.Open(context.Background(), path); err != nil {
		t.Fatal(err)
	}
	db, err := grove.Open(driver)
	if err != nil {
		t.Fatal(err)
	}
	s := sentinelsqlite.New(db)
	if err = s.Migrate(context.Background()); err != nil {
		t.Fatal(err)
	}
	return s
}

func TestRecordedWorkerRestartUsesPersistedQueue(t *testing.T) {
	_, _, ctx, plan, req := recordedFixture(t)
	req.SourceBindingID = "binding/v1"
	req.SourceCursor = "0001/source-run"
	path := filepath.Join(t.TempDir(), "recorded.db")
	first := openRecordedSQLite(t, path)
	if err := first.CreateEvaluationPlan(ctx, plan); err != nil {
		t.Fatal(err)
	}
	e, err := engine.New(engine.WithStore(first))
	if err != nil {
		t.Fatal(err)
	}
	job, err := e.EnqueueRecorded(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	if _, cursorErr := first.AdvanceRecordedCursor(ctx, recorded.CursorAdvance{BindingID: req.SourceBindingID, JobID: job.ID}); cursorErr != nil {
		t.Fatal(cursorErr)
	}
	if err = first.Close(); err != nil {
		t.Fatal(err)
	}
	second := openRecordedSQLite(t, path)
	defer second.Close()
	cursor, cursorErr := second.GetRecordedCursor(ctx, req.SourceBindingID)
	if cursorErr != nil || cursor.Position != req.SourceCursor || cursor.JobID != job.ID {
		t.Fatalf("cursor lost after reopening: %+v %v", cursor, cursorErr)
	}
	restarted, err := engine.New(engine.WithStore(second))
	if err != nil {
		t.Fatal(err)
	}
	status, err := restarted.RecordedStatus(ctx)
	if err != nil || !status.Durable || status.Ready {
		t.Fatalf("false readiness before worker: %+v %v", status, err)
	}
	workerCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- restarted.WorkRecorded(workerCtx) }()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		stored, getErr := second.GetRecordedJob(ctx, job.ID)
		if getErr != nil {
			t.Fatal(getErr)
		}
		if stored.State == recorded.JobCompleted {
			status, err = restarted.RecordedStatus(ctx)
			if err != nil || !status.Ready {
				t.Fatalf("worker not ready: %+v %v", status, err)
			}
			cancel()
			select {
			case err = <-done:
				if !errors.Is(err, context.Canceled) {
					t.Fatal(err)
				}
			case <-time.After(2 * time.Second):
				t.Fatal("worker did not stop")
			}
			again, evalErr := restarted.EvaluateRecorded(ctx, req)
			if evalErr != nil || again.Run.ID != stored.RunID {
				t.Fatalf("durable retry changed result: %+v %v", again, evalErr)
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("persisted work was not processed")
}

func TestRecordedReadinessRejectsMemoryAndUnscopedWorker(t *testing.T) {
	e, _, ctx, _, _ := recordedFixture(t)
	status, err := e.RecordedStatus(ctx)
	if err != nil || status.Durable || status.Ready {
		t.Fatalf("memory advertised durable readiness: %+v %v", status, err)
	}
	if err = e.WorkRecorded(context.Background()); !errors.Is(err, sentinel.ErrInvalidInput) {
		t.Fatalf("unscoped worker accepted: %v", err)
	}
}
