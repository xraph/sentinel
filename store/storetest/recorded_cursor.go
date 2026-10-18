package storetest

import (
	"context"
	"errors"
	"testing"

	"github.com/xraph/sentinel"
	"github.com/xraph/sentinel/id"
	"github.com/xraph/sentinel/recorded"
	"github.com/xraph/sentinel/store"
)

func testRecordedCursor(t *testing.T, s store.Store) {
	t.Helper()
	ctx := sentinel.WithApp(sentinel.WithTenant(context.Background(), "tenant-a"), "cursor-app")
	req := recorded.Request{PlanID: id.New("eplan"), PlanDigest: recorded.InputDigest("plan"), IdempotencyKey: "source/1", SourceBindingID: "binding-v1", SourceCursor: "0001/run-1", Cases: []recorded.Case{{CaseID: id.NewCaseID(), InputDigest: recorded.InputDigest("input"), Outcome: recorded.Completed}}}
	job, err := recorded.NewJob(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	advance := recorded.CursorAdvance{BindingID: req.SourceBindingID, JobID: job.ID}
	if _, err = s.AdvanceRecordedCursor(ctx, advance); !errors.Is(err, recorded.ErrNotFound) {
		t.Fatalf("cursor advanced before enqueue: %v", err)
	}
	job, err = s.CreateRecordedJob(ctx, job)
	if err != nil {
		t.Fatal(err)
	}
	cursor, err := s.AdvanceRecordedCursor(ctx, advance)
	if err != nil {
		t.Fatal(err)
	}
	if cursor.Revision != 1 || cursor.Position != req.SourceCursor || cursor.JobID != job.ID {
		t.Fatalf("wrong acknowledged cursor: %+v", cursor)
	}
	retry, err := s.AdvanceRecordedCursor(ctx, advance)
	if err != nil || retry.Revision != 1 {
		t.Fatalf("duplicate acknowledgment changed cursor: %+v %v", retry, err)
	}
	req.IdempotencyKey = "source/2"
	req.SourceCursor = "0002/run-2"
	second, err := recorded.NewJob(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	second, err = s.CreateRecordedJob(ctx, second)
	if err != nil {
		t.Fatal(err)
	}
	next := recorded.CursorAdvance{BindingID: req.SourceBindingID, JobID: second.ID}
	if _, err = s.AdvanceRecordedCursor(ctx, next); !errors.Is(err, recorded.ErrConflict) {
		t.Fatalf("stale cursor version accepted: %v", err)
	}
	next.ExpectedRevision = 1
	cursor, err = s.AdvanceRecordedCursor(ctx, next)
	if err != nil {
		t.Fatal(err)
	}
	if cursor.Revision != 2 || cursor.Position != req.SourceCursor {
		t.Fatalf("cursor did not advance: %+v", cursor)
	}
	replay, err := s.AdvanceRecordedCursor(ctx, advance)
	if err != nil || replay.Revision != 2 || replay.Position != req.SourceCursor {
		t.Fatalf("overlapping replay moved cursor backwards: %+v %v", replay, err)
	}
	wrong := next
	wrong.BindingID = "other-binding"
	if _, err = s.AdvanceRecordedCursor(ctx, wrong); !errors.Is(err, recorded.ErrConflict) {
		t.Fatalf("wrong binding acknowledged: %v", err)
	}
	for _, other := range []context.Context{context.Background(), sentinel.WithTenant(ctx, "tenant-b"), sentinel.WithApp(ctx, "other-app")} {
		if _, err = s.GetRecordedCursor(other, req.SourceBindingID); !errors.Is(err, recorded.ErrNotFound) {
			t.Errorf("cursor scope leak: %v", err)
		}
	}
}
