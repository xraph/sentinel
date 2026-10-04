// Package storetest is the conformance suite every Sentinel store backend
// runs. One set of assertions, four backends: a behaviour that differs
// between backends shows up here as a failure on the odd one out, not as a
// dashboard that works on sqlite and lies on mongo.
//
// Memory and sqlite run it under a plain `go test ./...`. Postgres and mongo
// run it under `go test -tags integration ./store/...`, against
// testcontainers, or against SENTINEL_TEST_DSN / SENTINEL_TEST_MONGO_URI
// when those are set.
package storetest

import (
	"testing"

	"github.com/xraph/sentinel/store"
)

// Factory returns a fresh, migrated, empty store for one subtest.
type Factory func(t *testing.T) store.Store

// Run executes every conformance check against the backend newStore builds.
func Run(t *testing.T, newStore Factory) {
	t.Run("JSONFieldsRoundTrip", func(t *testing.T) { testJSONFieldsRoundTrip(t, newStore(t)) })
	t.Run("ReturnedValuesAreIndependent", func(t *testing.T) { testReturnedValuesAreIndependent(t, newStore(t)) })
	t.Run("SetCurrentPromptVersionIsScoped", func(t *testing.T) { testSetCurrentPromptVersionIsScoped(t, newStore(t)) })
	t.Run("DuplicatePromptVersionIsRefused", func(t *testing.T) { testDuplicatePromptVersionIsRefused(t, newStore(t)) })
	t.Run("DeleteSuiteCascades", func(t *testing.T) { testDeleteSuiteCascades(t, newStore(t)) })
	t.Run("DeleteMissingSuite", func(t *testing.T) { testDeleteMissingSuite(t, newStore(t)) })
	t.Run("CancelRun", func(t *testing.T) { testCancelRun(t, newStore(t)) })
	t.Run("FinalizeRun", func(t *testing.T) { testFinalizeRun(t, newStore(t)) })
	t.Run("FinalizeKeepsCancel", func(t *testing.T) { testFinalizeKeepsCancel(t, newStore(t)) })
	t.Run("FinalizeRecordsFailure", func(t *testing.T) { testFinalizeRecordsFailure(t, newStore(t)) })
	t.Run("ConcurrentResultWrites", func(t *testing.T) { testConcurrentResultWrites(t, newStore(t)) })
	t.Run("OffsetPastEndIsEmpty", func(t *testing.T) { testOffsetPastEndIsEmpty(t, newStore(t)) })
}
