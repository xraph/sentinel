//go:build integration

package mongo_test

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	tcmongo "github.com/testcontainers/testcontainers-go/modules/mongodb"

	"github.com/xraph/grove"
	"github.com/xraph/grove/drivers/mongodriver"

	"github.com/xraph/sentinel/store"
	sentinelmongo "github.com/xraph/sentinel/store/mongo"
	"github.com/xraph/sentinel/store/storetest"
)

var (
	mongoOnce sync.Once
	mongoURI  string
	mongoErr  error
)

func connURI(t *testing.T) string {
	t.Helper()
	if uri := os.Getenv("SENTINEL_TEST_MONGO_URI"); uri != "" {
		return uri
	}
	mongoOnce.Do(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
		defer cancel()
		c, err := tcmongo.Run(ctx, "mongo:7")
		if err != nil {
			mongoErr = fmt.Errorf("start mongo container: %w", err)
			return
		}
		mongoURI, mongoErr = c.ConnectionString(ctx)
	})
	if mongoErr != nil {
		t.Skipf("mongo unavailable: %v", mongoErr)
	}
	return mongoURI
}

func TestConformance(t *testing.T) {
	storetest.Run(t, func(t *testing.T) store.Store {
		t.Helper()
		var rnd [4]byte
		_, _ = rand.Read(rnd[:])
		ctx := context.Background()
		drv := mongodriver.New()
		if err := drv.Open(ctx, connURI(t), mongodriver.WithDatabase("sentinel_test_"+hex.EncodeToString(rnd[:]))); err != nil {
			t.Fatalf("open mongo: %v", err)
		}
		db, err := grove.Open(drv)
		if err != nil {
			t.Fatalf("grove open: %v", err)
		}
		t.Cleanup(func() {
			_ = drv.Database().Drop(context.Background())
			_ = drv.Close()
		})
		s := sentinelmongo.New(db)
		if err := s.Migrate(ctx); err != nil {
			t.Fatalf("migrate: %v", err)
		}
		return s
	})
}
