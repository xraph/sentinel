// Package memory provides an in-memory implementation of the Sentinel composite
// store. Suitable for testing and development.
package memory

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/xraph/sentinel"
	"github.com/xraph/sentinel/baseline"
	"github.com/xraph/sentinel/evalrun"
	"github.com/xraph/sentinel/id"
	"github.com/xraph/sentinel/promptversion"
	"github.com/xraph/sentinel/store"
	"github.com/xraph/sentinel/suite"
	"github.com/xraph/sentinel/testcase"
)

// Compile-time interface check.
var _ store.Store = (*Store)(nil)

// Store is an in-memory implementation of the composite Sentinel store.
type Store struct {
	mu             sync.RWMutex
	suites         map[string]*suite.Suite
	cases          map[string]*testcase.Case
	runs           map[string]*evalrun.Run
	results        map[string]*evalrun.Result
	baselines      map[string]*baseline.Baseline
	promptVersions map[string]*promptversion.PromptVersion
}

// New creates a new in-memory store.
func New() *Store {
	return &Store{
		suites:         make(map[string]*suite.Suite),
		cases:          make(map[string]*testcase.Case),
		runs:           make(map[string]*evalrun.Run),
		results:        make(map[string]*evalrun.Result),
		baselines:      make(map[string]*baseline.Baseline),
		promptVersions: make(map[string]*promptversion.PromptVersion),
	}
}

// ──────────────────────────────────────────────────
// Lifecycle
// ──────────────────────────────────────────────────

func (s *Store) Migrate(_ context.Context) error { return nil }
func (s *Store) Ping(_ context.Context) error    { return nil }
func (s *Store) Close() error                    { return nil }

// ──────────────────────────────────────────────────
// Suite operations
// ──────────────────────────────────────────────────

func (s *Store) CreateSuite(_ context.Context, su *suite.Suite) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := su.ID.String()
	if _, exists := s.suites[key]; exists {
		return sentinel.ErrSuiteAlreadyExists
	}
	for _, existing := range s.suites {
		if existing.AppID == su.AppID && existing.Name == su.Name {
			return sentinel.ErrSuiteAlreadyExists
		}
	}
	now := time.Now().UTC()
	su.CreatedAt = now
	su.UpdatedAt = now
	s.suites[key] = su
	return nil
}

func (s *Store) GetSuite(_ context.Context, suiteID id.SuiteID) (*suite.Suite, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	su, ok := s.suites[suiteID.String()]
	if !ok {
		return nil, sentinel.ErrSuiteNotFound
	}
	return su, nil
}

func (s *Store) GetSuiteByName(_ context.Context, appID, name string) (*suite.Suite, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, su := range s.suites {
		if su.AppID == appID && su.Name == name {
			return su, nil
		}
	}
	return nil, sentinel.ErrSuiteNotFound
}

func (s *Store) UpdateSuite(_ context.Context, su *suite.Suite) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := su.ID.String()
	if _, exists := s.suites[key]; !exists {
		return sentinel.ErrSuiteNotFound
	}
	su.UpdatedAt = time.Now().UTC()
	s.suites[key] = su
	return nil
}

func (s *Store) DeleteSuite(_ context.Context, suiteID id.SuiteID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := suiteID.String()
	if _, exists := s.suites[key]; !exists {
		return sentinel.ErrSuiteNotFound
	}
	delete(s.suites, key)
	return nil
}

func (s *Store) ListSuites(_ context.Context, filter *suite.ListFilter) ([]*suite.Suite, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*suite.Suite
	for _, su := range s.suites {
		if filter != nil && filter.AppID != "" && su.AppID != filter.AppID {
			continue
		}
		result = append(result, su)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt.Before(result[j].CreatedAt)
	})
	if filter != nil && filter.Offset > 0 && filter.Offset < len(result) {
		result = result[filter.Offset:]
	}
	if filter != nil && filter.Limit > 0 && filter.Limit < len(result) {
		result = result[:filter.Limit]
	}
	return result, nil
}

// ──────────────────────────────────────────────────
// Case operations
// ──────────────────────────────────────────────────

func (s *Store) CreateCase(_ context.Context, tc *testcase.Case) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UTC()
	tc.CreatedAt = now
	tc.UpdatedAt = now
	s.cases[tc.ID.String()] = tc
	return nil
}

func (s *Store) CreateCaseBatch(_ context.Context, cases []*testcase.Case) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UTC()
	for _, tc := range cases {
		tc.CreatedAt = now
		tc.UpdatedAt = now
		s.cases[tc.ID.String()] = tc
	}
	return nil
}

func (s *Store) GetCase(_ context.Context, caseID id.CaseID) (*testcase.Case, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	tc, ok := s.cases[caseID.String()]
	if !ok {
		return nil, sentinel.ErrCaseNotFound
	}
	return tc, nil
}

func (s *Store) UpdateCase(_ context.Context, tc *testcase.Case) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := tc.ID.String()
	if _, exists := s.cases[key]; !exists {
		return sentinel.ErrCaseNotFound
	}
	tc.UpdatedAt = time.Now().UTC()
	s.cases[key] = tc
	return nil
}

func (s *Store) DeleteCase(_ context.Context, caseID id.CaseID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := caseID.String()
	if _, exists := s.cases[key]; !exists {
		return sentinel.ErrCaseNotFound
	}
	delete(s.cases, key)
	return nil
}

func (s *Store) ListCases(_ context.Context, suiteID id.SuiteID) ([]*testcase.Case, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	sid := suiteID.String()
	var result []*testcase.Case
	for _, tc := range s.cases {
		if tc.SuiteID.String() == sid {
			result = append(result, tc)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt.Before(result[j].CreatedAt)
	})
	return result, nil
}

func (s *Store) CountCases(_ context.Context, suiteID id.SuiteID) (int64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	sid := suiteID.String()
	var count int64
	for _, tc := range s.cases {
		if tc.SuiteID.String() == sid {
			count++
		}
	}
	return count, nil
}

func (s *Store) ImportCases(_ context.Context, _ id.SuiteID, _ string, _ []byte) (int64, error) {
	return 0, nil
}

// ──────────────────────────────────────────────────
// Run operations
// ──────────────────────────────────────────────────

func (s *Store) CreateRun(_ context.Context, run *evalrun.Run) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UTC()
	run.CreatedAt = now
	run.UpdatedAt = now
	s.runs[run.ID.String()] = run
	return nil
}

func (s *Store) GetRun(_ context.Context, runID id.EvalRunID) (*evalrun.Run, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	run, ok := s.runs[runID.String()]
	if !ok {
		return nil, sentinel.ErrRunNotFound
	}
	return run, nil
}

func (s *Store) UpdateRun(_ context.Context, run *evalrun.Run) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := run.ID.String()
	if _, exists := s.runs[key]; !exists {
		return sentinel.ErrRunNotFound
	}
	run.UpdatedAt = time.Now().UTC()
	s.runs[key] = run
	return nil
}

func (s *Store) ListRuns(_ context.Context, filter *evalrun.ListFilter) ([]*evalrun.Run, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var result []*evalrun.Run
	for _, run := range s.runs {
		if filter != nil {
			if filter.AppID != "" && run.AppID != filter.AppID {
				continue
			}
			if filter.State != "" && run.State != filter.State {
				continue
			}
			if filter.SuiteID.String() != "" && run.SuiteID.String() != filter.SuiteID.String() {
				continue
			}
		}
		result = append(result, run)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt.After(result[j].CreatedAt)
	})
	if filter != nil && filter.Offset > 0 && filter.Offset < len(result) {
		result = result[filter.Offset:]
	}
	if filter != nil && filter.Limit > 0 && filter.Limit < len(result) {
		result = result[:filter.Limit]
	}
	return result, nil
}

func (s *Store) ListRunsBySuite(_ context.Context, suiteID id.SuiteID) ([]*evalrun.Run, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	sid := suiteID.String()
	var result []*evalrun.Run
	for _, run := range s.runs {
		if run.SuiteID.String() == sid {
			result = append(result, run)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt.After(result[j].CreatedAt)
	})
	return result, nil
}

// ──────────────────────────────────────────────────
// Result operations
// ──────────────────────────────────────────────────

func (s *Store) CreateResult(_ context.Context, result *evalrun.Result) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UTC()
	result.CreatedAt = now
	result.UpdatedAt = now
	s.results[result.ID.String()] = result
	return nil
}

func (s *Store) CreateResultBatch(_ context.Context, results []*evalrun.Result) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UTC()
	for _, r := range results {
		r.CreatedAt = now
		r.UpdatedAt = now
		s.results[r.ID.String()] = r
	}
	return nil
}

func (s *Store) ListResults(_ context.Context, runID id.EvalRunID) ([]*evalrun.Result, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	rid := runID.String()
	var result []*evalrun.Result
	for _, r := range s.results {
		if r.RunID.String() == rid {
			result = append(result, r)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt.Before(result[j].CreatedAt)
	})
	return result, nil
}

func (s *Store) GetResultStats(_ context.Context, runID id.EvalRunID) (*evalrun.ResultStats, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	rid := runID.String()

	stats := &evalrun.ResultStats{
		DimensionScores: make(map[string]float64),
	}
	dimCounts := make(map[string]int)
	var totalScore float64
	var totalLatency int64
	var totalTokens int
	var totalCost float64

	for _, r := range s.results {
		if r.RunID.String() != rid {
			continue
		}
		stats.TotalCases++
		totalScore += r.Score
		totalLatency += int64(r.LatencyMs)
		totalTokens += r.TokensUsed
		totalCost += r.Cost

		switch r.Status {
		case evalrun.StatusPass:
			stats.Passed++
		case evalrun.StatusFail:
			stats.Failed++
		case evalrun.StatusError:
			stats.Errored++
		}

		for dim, score := range r.DimensionScores {
			stats.DimensionScores[dim] += score
			dimCounts[dim]++
		}
	}

	if stats.TotalCases > 0 {
		stats.PassRate = float64(stats.Passed) / float64(stats.TotalCases)
		stats.AvgScore = totalScore / float64(stats.TotalCases)
		stats.AvgLatencyMs = int(totalLatency / int64(stats.TotalCases))
	}
	stats.TotalTokens = totalTokens
	stats.TotalCost = totalCost

	for dim, total := range stats.DimensionScores {
		if dimCounts[dim] > 0 {
			stats.DimensionScores[dim] = total / float64(dimCounts[dim])
		}
	}

	return stats, nil
}

// ──────────────────────────────────────────────────
// Baseline operations
// ──────────────────────────────────────────────────

func (s *Store) SaveBaseline(_ context.Context, b *baseline.Baseline) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if b.IsCurrent {
		sid := b.SuiteID.String()
		for _, existing := range s.baselines {
			if existing.SuiteID.String() == sid {
				existing.IsCurrent = false
			}
		}
	}
	b.CreatedAt = time.Now().UTC()
	s.baselines[b.ID.String()] = b
	return nil
}

func (s *Store) GetBaseline(_ context.Context, baselineID id.BaselineID) (*baseline.Baseline, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	b, ok := s.baselines[baselineID.String()]
	if !ok {
		return nil, sentinel.ErrBaselineNotFound
	}
	return b, nil
}

func (s *Store) GetLatestBaseline(_ context.Context, suiteID id.SuiteID) (*baseline.Baseline, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	sid := suiteID.String()
	for _, b := range s.baselines {
		if b.SuiteID.String() == sid && b.IsCurrent {
			return b, nil
		}
	}
	return nil, sentinel.ErrBaselineNotFound
}

func (s *Store) ListBaselines(_ context.Context, suiteID id.SuiteID) ([]*baseline.Baseline, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	sid := suiteID.String()
	var result []*baseline.Baseline
	for _, b := range s.baselines {
		if b.SuiteID.String() == sid {
			result = append(result, b)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt.After(result[j].CreatedAt)
	})
	return result, nil
}

func (s *Store) DeleteBaseline(_ context.Context, baselineID id.BaselineID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := baselineID.String()
	if _, exists := s.baselines[key]; !exists {
		return sentinel.ErrBaselineNotFound
	}
	delete(s.baselines, key)
	return nil
}

// ──────────────────────────────────────────────────
// Prompt version operations
// ──────────────────────────────────────────────────

func (s *Store) CreatePromptVersion(_ context.Context, pv *promptversion.PromptVersion) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	pv.CreatedAt = time.Now().UTC()
	s.promptVersions[pv.ID.String()] = pv
	return nil
}

func (s *Store) GetPromptVersion(_ context.Context, pvID id.PromptVersionID) (*promptversion.PromptVersion, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	pv, ok := s.promptVersions[pvID.String()]
	if !ok {
		return nil, sentinel.ErrPromptVersionNotFound
	}
	return pv, nil
}

func (s *Store) ListPromptVersions(_ context.Context, suiteID id.SuiteID) ([]*promptversion.PromptVersion, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	sid := suiteID.String()
	var result []*promptversion.PromptVersion
	for _, pv := range s.promptVersions {
		if pv.SuiteID.String() == sid {
			result = append(result, pv)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Version < result[j].Version
	})
	return result, nil
}

func (s *Store) GetCurrentPromptVersion(_ context.Context, suiteID id.SuiteID) (*promptversion.PromptVersion, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	sid := suiteID.String()
	for _, pv := range s.promptVersions {
		if pv.SuiteID.String() == sid && pv.IsCurrent {
			return pv, nil
		}
	}
	return nil, sentinel.ErrPromptVersionNotFound
}

func (s *Store) SetCurrentPromptVersion(_ context.Context, suiteID id.SuiteID, pvID id.PromptVersionID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	sid := suiteID.String()
	pvKey := pvID.String()

	found := false
	for _, pv := range s.promptVersions {
		if pv.SuiteID.String() == sid {
			pv.IsCurrent = pv.ID.String() == pvKey
			if pv.IsCurrent {
				found = true
			}
		}
	}
	if !found {
		return sentinel.ErrPromptVersionNotFound
	}
	return nil
}
