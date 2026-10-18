package storetest

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/xraph/sentinel"
	"github.com/xraph/sentinel/evalrun"
	"github.com/xraph/sentinel/id"
	"github.com/xraph/sentinel/recorded"
	"github.com/xraph/sentinel/store"
)

func testRecordedJobs(t *testing.T, s store.Store) {
	t.Helper()
	ctx := sentinel.WithApp(sentinel.WithTenant(context.Background(), "tenant-a"), "app-a")
	req := recorded.Request{PlanID: id.New("eplan"), PlanDigest: recorded.InputDigest("plan"), IdempotencyKey: "source/v1", Cases: []recorded.Case{{CaseID: id.NewCaseID(), InputDigest: recorded.InputDigest("input"), Outcome: recorded.Completed, Output: "answer"}}}
	job, err := recorded.NewJob(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	stored, err := s.CreateRecordedJob(ctx, job)
	if err != nil {
		t.Fatal(err)
	}
	duplicate, err := recorded.NewJob(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	got, err := s.CreateRecordedJob(ctx, duplicate)
	if err != nil || got.ID != stored.ID {
		t.Fatalf("retry did not return original: %+v %v", got, err)
	}
	req.Cases[0].Output = "changed"
	conflict, err := recorded.NewJob(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.CreateRecordedJob(ctx, conflict); !errors.Is(err, recorded.ErrConflict) {
		t.Fatalf("conflicting request accepted: %v", err)
	}
	got.Request.Cases[0].Output = "caller mutation"
	got, err = s.GetRecordedJob(ctx, stored.ID)
	if err != nil || got.Request.Cases[0].Output != "answer" {
		t.Fatalf("mutable request: %+v %v", got, err)
	}
	for _, other := range []context.Context{context.Background(), sentinel.WithApp(ctx, "other"), sentinel.WithTenant(ctx, "other")} {
		if _, err = s.GetRecordedJob(other, stored.ID); !errors.Is(err, recorded.ErrNotFound) {
			t.Errorf("scope leak: %v", err)
		}
	}
	lease, err := stored.Claim("worker", time.Minute, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if err = s.UpdateRecordedJob(ctx, stored.Revision, lease); err != nil {
		t.Fatal(err)
	}
	stale, err := stored.Claim("stale", time.Minute, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if err = s.UpdateRecordedJob(ctx, stored.Revision, stale); !errors.Is(err, recorded.ErrConflict) {
		t.Fatalf("stale writer accepted: %v", err)
	}
	page, err := s.ListRecordedJobs(ctx, recorded.Query{Limit: 1})
	if err != nil || len(page) != 1 || page[0].ID != stored.ID {
		t.Fatalf("wrong page: %+v %v", page, err)
	}
	page, err = s.ListRecordedJobs(ctx, recorded.Query{After: stored.ID.String(), Limit: 1})
	if err != nil || len(page) != 0 {
		t.Fatalf("cursor repeated row: %+v %v", page, err)
	}
}

func testRecordedCheckpoint(t *testing.T, s store.Store) {
	t.Helper()
	ctx := sentinel.WithApp(context.Background(), "checkpoint-app")
	req := recorded.Request{PlanID: id.New("eplan"), PlanDigest: recorded.InputDigest("plan"), IdempotencyKey: "checkpoint/v1", Cases: []recorded.Case{{CaseID: id.NewCaseID(), InputDigest: recorded.InputDigest("input"), Outcome: recorded.Completed, Output: "answer"}}}
	j, err := recorded.NewJob(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	j, err = s.CreateRecordedJob(ctx, j)
	if err != nil {
		t.Fatal(err)
	}
	next, err := j.Claim("worker", time.Minute, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if err = s.UpdateRecordedJob(ctx, j.Revision, next); err != nil {
		t.Fatal(err)
	}
	revision := next.Revision
	next.Revision++
	next.Results = []recorded.CaseResult{{Key: req.Cases[0].Key(), Result: &evalrun.Result{ID: id.NewEvalResultID(), RunID: j.RunID, CaseID: req.Cases[0].CaseID, Status: evalrun.StatusPass, Output: "answer", Score: 1, ScorerResults: []evalrun.ScorerResult{{CriterionID: "quality", Required: true, Score: 1, Passed: true, Details: map[string]any{"nested": map[string]any{"value": "stable"}}}}}}}
	if err = s.UpdateRecordedJob(ctx, revision, next); err != nil {
		t.Fatal(err)
	}
	saved, err := s.GetRecordedJob(ctx, j.ID)
	if err != nil {
		t.Fatal(err)
	}
	saved.Revision++
	saved.Results[0].Result.Output = "changed"
	if err = s.UpdateRecordedJob(ctx, next.Revision, saved); !errors.Is(err, recorded.ErrConflict) {
		t.Fatalf("checkpoint overwritten: %v", err)
	}
	saved, err = s.GetRecordedJob(ctx, j.ID)
	if err != nil {
		t.Fatal(err)
	}
	saved.Revision++
	saved.State = recorded.JobCompleted
	if err = s.UpdateRecordedJob(ctx, next.Revision, saved); err != nil {
		t.Fatal(err)
	}
	if _, err = saved.Claim("retry", time.Minute, time.Now()); !errors.Is(err, recorded.ErrConflict) {
		t.Fatalf("completed evidence reclaimed: %v", err)
	}
	page, err := s.ListRecordedJobs(ctx, recorded.Query{Active: true})
	if err != nil || len(page) != 0 {
		t.Fatalf("completed job remains active: %+v %v", page, err)
	}
}

func testRecordedConcurrentClaims(t *testing.T, s store.Store) {
	t.Helper()
	ctx := sentinel.WithApp(context.Background(), "race-app")
	req := recorded.Request{PlanID: id.New("eplan"), PlanDigest: recorded.InputDigest("plan"), IdempotencyKey: "race/v1", Cases: []recorded.Case{{CaseID: id.NewCaseID(), InputDigest: recorded.InputDigest("input"), Outcome: recorded.Completed}}}
	const workers = 8
	jobs := make(chan *recorded.Job, workers)
	errs := make(chan error, workers)
	var wg sync.WaitGroup
	for range workers {
		wg.Go(func() {
			j, err := recorded.NewJob(ctx, req)
			if err == nil {
				j, err = s.CreateRecordedJob(ctx, j)
			}
			if err != nil {
				errs <- err
				return
			}
			jobs <- j
		})
	}
	wg.Wait()
	close(jobs)
	close(errs)
	for err := range errs {
		t.Error(err)
	}
	var first *recorded.Job
	for j := range jobs {
		if first == nil {
			first = j
		}
		if first.ID != j.ID {
			t.Fatalf("duplicate ingest: %s vs %s", first.ID, j.ID)
		}
	}
	if first == nil {
		t.Fatal("no stored job")
	}
	outcomes := make(chan error, workers)
	for range workers {
		wg.Go(func() {
			claim, err := first.Claim(id.New("worker").String(), time.Minute, time.Now().UTC())
			if err == nil {
				err = s.UpdateRecordedJob(ctx, first.Revision, claim)
			}
			outcomes <- err
		})
	}
	wg.Wait()
	close(outcomes)
	winners := 0
	for err := range outcomes {
		if err == nil {
			winners++
		} else if !errors.Is(err, recorded.ErrConflict) {
			t.Error(err)
		}
	}
	if winners != 1 {
		t.Fatalf("claim winners: %d", winners)
	}
}
