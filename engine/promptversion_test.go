package engine_test

import (
	"errors"
	"sort"
	"sync"
	"testing"

	"github.com/xraph/sentinel"
	"github.com/xraph/sentinel/promptversion"
)

func TestPromptVersionsNumberThemselves(t *testing.T) {
	e := newEngine(t)
	s := seedSuite(t, e, "base")
	for i := 0; i < 3; i++ {
		if err := e.CreatePromptVersion(bg(), &promptversion.PromptVersion{SuiteID: s.ID, SystemPrompt: "p"}); err != nil {
			t.Fatalf("create %d: %v", i, err)
		}
	}
	list, _ := e.ListPromptVersions(bg(), s.ID)
	for i, pv := range list {
		if pv.Version != i+1 {
			t.Fatalf("version %d is %d, want %d", i, pv.Version, i+1)
		}
	}
}

func TestCreateAsCurrentLeavesOneCurrent(t *testing.T) {
	e := newEngine(t)
	s := seedSuite(t, e, "base")
	first := &promptversion.PromptVersion{SuiteID: s.ID, SystemPrompt: "one", IsCurrent: true}
	second := &promptversion.PromptVersion{SuiteID: s.ID, SystemPrompt: "two", IsCurrent: true}
	for _, pv := range []*promptversion.PromptVersion{first, second} {
		if err := e.CreatePromptVersion(bg(), pv); err != nil {
			t.Fatalf("create: %v", err)
		}
	}
	list, _ := e.ListPromptVersions(bg(), s.ID)
	var current []int
	for _, pv := range list {
		if pv.IsCurrent {
			current = append(current, pv.Version)
		}
	}
	if len(current) != 1 || current[0] != 2 {
		t.Fatalf("exactly version 2 should be current, got %v", current)
	}
}

// Review focus 3: two creates at once must both succeed with distinct numbers.
func TestConcurrentCreatesGetDistinctVersions(t *testing.T) {
	e := newEngine(t)
	s := seedSuite(t, e, "base")
	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i := range errs {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			errs[i] = e.CreatePromptVersion(bg(), &promptversion.PromptVersion{SuiteID: s.ID, SystemPrompt: "p"})
		}(i)
	}
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Fatalf("create %d: %v", i, err)
		}
	}
	list, _ := e.ListPromptVersions(bg(), s.ID)
	got := []int{list[0].Version, list[1].Version}
	sort.Ints(got)
	if got[0] != 1 || got[1] != 2 {
		t.Fatalf("versions %v, want [1 2]", got)
	}
}

func TestSetCurrentRefusesAnotherSuitesVersion(t *testing.T) {
	e := newEngine(t)
	a, b := seedSuite(t, e, "a"), seedSuite(t, e, "b")
	pvB := &promptversion.PromptVersion{SuiteID: b.ID, SystemPrompt: "b"}
	if err := e.CreatePromptVersion(bg(), pvB); err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := e.SetCurrentPromptVersion(bg(), a.ID, pvB.ID); !errors.Is(err, sentinel.ErrPromptVersionNotFound) {
		t.Fatalf("want ErrPromptVersionNotFound, got %v", err)
	}
}
