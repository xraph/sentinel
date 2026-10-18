//go:build integration

package postgres_test

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/url"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/xraph/grove"
	"github.com/xraph/grove/drivers/pgdriver"
	_ "github.com/xraph/grove/drivers/pgdriver/pgmigrate"

	"github.com/xraph/sentinel/store"
	sentinelpg "github.com/xraph/sentinel/store/postgres"
	"github.com/xraph/sentinel/store/storetest"
)

var (
	pgOnce      sync.Once
	pgDSN       string
	errPostgres error
)

// adminDSN is SENTINEL_TEST_DSN when set, otherwise a shared
// postgres:16-alpine container started on first use.
func adminDSN(t *testing.T) string {
	t.Helper()
	if dsn := os.Getenv("SENTINEL_TEST_DSN"); dsn != "" {
		return dsn
	}
	pgOnce.Do(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
		defer cancel()
		c, err := tcpostgres.Run(ctx, "postgres:16-alpine",
			tcpostgres.WithDatabase("sentinel_admin"),
			tcpostgres.WithUsername("sentinel"),
			tcpostgres.WithPassword("sentinel"),
			tcpostgres.BasicWaitStrategies(),
		)
		if err != nil {
			errPostgres = fmt.Errorf("start postgres container: %w", err)
			return
		}
		pgDSN, errPostgres = c.ConnectionString(ctx, "sslmode=disable")
	})
	if errPostgres != nil {
		t.Skipf("postgres unavailable: %v", errPostgres)
	}
	return pgDSN
}

func TestConformance(t *testing.T) {
	storetest.Run(t, func(t *testing.T) store.Store {
		t.Helper()
		admin := adminDSN(t)
		var rnd [4]byte
		_, _ = rand.Read(rnd[:])
		name := "sentinel_test_" + hex.EncodeToString(rnd[:])
		ctx := context.Background()

		conn, err := pgx.Connect(ctx, admin)
		if err != nil {
			t.Fatalf("admin connect: %v", err)
		}
		if _, createErr := conn.Exec(ctx, "CREATE DATABASE "+name); createErr != nil {
			t.Fatalf("create database: %v", createErr)
		}
		_ = conn.Close(ctx)

		u, err := url.Parse(admin)
		if err != nil {
			t.Fatalf("parse dsn: %v", err)
		}
		u.Path = "/" + name
		drv := pgdriver.New()
		if openErr := drv.Open(ctx, u.String()); openErr != nil {
			t.Fatalf("open postgres: %v", openErr)
		}
		db, err := grove.Open(drv)
		if err != nil {
			t.Fatalf("grove open: %v", err)
		}
		t.Cleanup(func() {
			_ = drv.Close()
			c, err := pgx.Connect(context.Background(), admin)
			if err == nil {
				_, _ = c.Exec(context.Background(), "DROP DATABASE IF EXISTS "+name+" WITH (FORCE)")
				_ = c.Close(context.Background())
			}
		})
		s := sentinelpg.New(db)
		if err := s.Migrate(ctx); err != nil {
			t.Fatalf("migrate: %v", err)
		}
		return s
	})
}
