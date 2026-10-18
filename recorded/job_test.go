package recorded_test

import (
	"context"
	"testing"
	"time"

	"github.com/xraph/sentinel"
	"github.com/xraph/sentinel/recorded"
)

func TestRecordedLeaseFencesExpiredWorkersAndBoundsRetries(t *testing.T) {
	ctx := sentinel.WithApp(context.Background(), "app")
	job, err := recorded.NewJob(ctx, validRequest())
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	first, err := job.Claim("one", time.Second, now)
	if err != nil {
		t.Fatal(err)
	}
	if err = recorded.ValidateChange(ctx, job, first, now); err != nil {
		t.Fatal(err)
	}
	late := *first
	late.Revision++
	late.State = recorded.JobFailed
	if err = recorded.ValidateChange(ctx, first, &late, now.Add(2*time.Second)); err == nil {
		t.Fatal("expired worker committed")
	}
	second, err := first.Claim("two", time.Second, now.Add(2*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if second.Generation <= first.Generation {
		t.Fatal("new lease reused old fence")
	}
	if err = recorded.ValidateChange(ctx, first, second, now.Add(2*time.Second)); err != nil {
		t.Fatal(err)
	}
	third, err := second.Claim("three", time.Second, now.Add(4*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	exhausted, err := third.Claim("four", time.Second, now.Add(6*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if exhausted.State != recorded.JobFailed || exhausted.Attempts != 3 {
		t.Fatalf("retry limit bypassed: %+v", exhausted)
	}
	if err = recorded.ValidateChange(ctx, third, exhausted, now.Add(6*time.Second)); err != nil {
		t.Fatal(err)
	}
}

func TestRecordedJobCannotChangeCapturedInputs(t *testing.T) {
	ctx := sentinel.WithApp(context.Background(), "app")
	job, err := recorded.NewJob(ctx, validRequest())
	if err != nil {
		t.Fatal(err)
	}
	next, err := job.Claim("worker", time.Minute, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	next.Request.Cases[0].Output = "different answer"
	next.RequestDigest, err = next.Request.Digest()
	if err != nil {
		t.Fatal(err)
	}
	if err = recorded.ValidateChange(ctx, job, next, time.Now()); err == nil {
		t.Fatal("captured input replacement accepted")
	}
	if job.Request.Cases[0].Output != "answer" {
		t.Fatal("claim shared request memory")
	}
}
