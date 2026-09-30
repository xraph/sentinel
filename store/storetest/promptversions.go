package storetest

import (
	"errors"
	"testing"
	"time"

	"github.com/xraph/sentinel"
	"github.com/xraph/sentinel/id"
	"github.com/xraph/sentinel/promptversion"
	"github.com/xraph/sentinel/store"
)

func mustPV(t *testing.T, s store.Store, suiteID id.SuiteID, version int) *promptversion.PromptVersion {
	t.Helper()
	pv := &promptversion.PromptVersion{ID: id.NewPromptVersionID(), SuiteID: suiteID, Version: version, SystemPrompt: "prompt", CreatedAt: time.Now().UTC()}
	if err := s.CreatePromptVersion(bg(), pv); err != nil {
		t.Fatalf("create prompt version %d: %v", version, err)
	}
	return pv
}

func testSetCurrentPromptVersionIsScoped(t *testing.T, s store.Store) {
	a := mustSuite(t, s, "app_a")
	b := mustSuite(t, s, "app_a")
	a1, a2 := mustPV(t, s, a.ID, 1), mustPV(t, s, a.ID, 2)
	b1 := mustPV(t, s, b.ID, 1)

	if err := s.SetCurrentPromptVersion(bg(), a.ID, a2.ID); err != nil {
		t.Fatalf("set current: %v", err)
	}
	cur, err := s.GetCurrentPromptVersion(bg(), a.ID)
	if err != nil || cur.ID.String() != a2.ID.String() {
		t.Fatalf("current of a should be a2: %v %v", cur, err)
	}
	if got, _ := s.GetPromptVersion(bg(), a1.ID); got.IsCurrent {
		t.Fatal("a1 must no longer be current")
	}

	// b1 belongs to suite b. Naming it for suite a must refuse and change
	// nothing in either suite.
	err = s.SetCurrentPromptVersion(bg(), a.ID, b1.ID)
	if !errors.Is(err, sentinel.ErrPromptVersionNotFound) {
		t.Fatalf("foreign version: want ErrPromptVersionNotFound, got %v", err)
	}
	if cur, _ := s.GetCurrentPromptVersion(bg(), a.ID); cur == nil || cur.ID.String() != a2.ID.String() {
		t.Fatal("a refusal must leave a2 current")
	}
	if got, _ := s.GetPromptVersion(bg(), b1.ID); got.IsCurrent {
		t.Fatal("a refusal must not make b1 current")
	}
}

func testDuplicatePromptVersionIsRefused(t *testing.T, s store.Store) {
	a := mustSuite(t, s, "app_a")
	mustPV(t, s, a.ID, 1)
	dup := &promptversion.PromptVersion{ID: id.NewPromptVersionID(), SuiteID: a.ID, Version: 1, SystemPrompt: "again", CreatedAt: time.Now().UTC()}
	if err := s.CreatePromptVersion(bg(), dup); err == nil {
		t.Fatal("a second version 1 for the same suite must be refused")
	}
}
