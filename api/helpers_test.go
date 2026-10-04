package api

import (
	"errors"
	"fmt"
	"testing"

	"github.com/xraph/sentinel"
)

// An import the caller can fix answers 400, not 500. The engine wraps
// these sentinels with a message naming the row or the format.
func TestMapStoreErrorRefusesBadImportsAsBadRequest(t *testing.T) {
	for _, base := range []error{sentinel.ErrInvalidInput, sentinel.ErrUnsupportedFormat} {
		err := mapStoreError(fmt.Errorf("%w: row 2 has no input", base))
		var coded interface{ StatusCode() int }
		if !errors.As(err, &coded) || coded.StatusCode() != 400 {
			t.Errorf("%v mapped to %v, want a 400", base, err)
		}
	}
}
