package dataset

import (
	"math/rand"

	"github.com/xraph/sentinel/testcase"
)

// Sample randomly selects n cases from the given slice. If n >= len(cases),
// all cases are returned (in shuffled order).
func Sample(cases []*testcase.Case, n int) []*testcase.Case {
	if n <= 0 || len(cases) == 0 {
		return nil
	}

	// Shuffle a copy.
	shuffled := make([]*testcase.Case, len(cases))
	copy(shuffled, cases)
	rand.Shuffle(len(shuffled), func(i, j int) {
		shuffled[i], shuffled[j] = shuffled[j], shuffled[i]
	})

	if n >= len(shuffled) {
		return shuffled
	}
	return shuffled[:n]
}
