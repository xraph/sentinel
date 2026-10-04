package storetest

import (
	"testing"

	"github.com/xraph/sentinel/evalrun"
	"github.com/xraph/sentinel/store"
	"github.com/xraph/sentinel/suite"
)

// A page past the end is empty. The memory store returned the whole list
// for an offset at or past its length, so a dashboard paging forward
// showed the first page again with hasMore still true.
func testOffsetPastEndIsEmpty(t *testing.T, s store.Store) {
	su := mustSuite(t, s)
	mustRun(t, s, su.ID)
	mustRun(t, s, su.ID)

	runs, err := s.ListRuns(bg(), &evalrun.ListFilter{SuiteID: su.ID, Limit: 10, Offset: 2})
	if err != nil {
		t.Fatalf("list runs: %v", err)
	}
	if len(runs) != 0 {
		t.Fatalf("offset 2 of 2 runs returned %d", len(runs))
	}
	suites, err := s.ListSuites(bg(), &suite.ListFilter{AppID: fixtureAppID, Limit: 10, Offset: 1})
	if err != nil {
		t.Fatalf("list suites: %v", err)
	}
	if len(suites) != 0 {
		t.Fatalf("offset 1 of 1 suite returned %d", len(suites))
	}
}
