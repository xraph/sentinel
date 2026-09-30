package memory_test

import (
	"testing"

	"github.com/xraph/sentinel/store"
	"github.com/xraph/sentinel/store/memory"
	"github.com/xraph/sentinel/store/storetest"
)

func TestConformance(t *testing.T) {
	storetest.Run(t, func(*testing.T) store.Store { return memory.New() })
}
