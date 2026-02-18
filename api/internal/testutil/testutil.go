//go:build integration

// Package testutil provides helpers for integration tests that run against
// real Postgres and Redis instances (via docker-compose.test.yml).
package testutil

import (
	"database/sql"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	_ "github.com/lib/pq"
	"github.com/redis/go-redis/v9"
)

const (
	defaultDatabaseURL = "postgres://postgres:postgres@localhost:5433/polite_betrayal_test?sslmode=disable"
	defaultRedisURL    = "redis://localhost:6380/0"
)

// SetupDB connects to the test Postgres, runs migrations, and registers cleanup.
func SetupDB(t *testing.T) *sql.DB {
	t.Helper()

	dbURL := os.Getenv("TEST_DATABASE_URL")
	if dbURL == "" {
		dbURL = defaultDatabaseURL
	}

	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	if err := db.Ping(); err != nil {
		t.Fatalf("ping test db: %v", err)
	}

	migrationSQL, err := os.ReadFile(migrationPath())
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}

	if _, err := db.Exec(string(migrationSQL)); err != nil {
		t.Fatalf("run migration: %v", err)
	}

	return db
}

// SetupRedis connects to the test Redis and registers cleanup.
func SetupRedis(t *testing.T) *redis.Client {
	t.Helper()

	redisURL := os.Getenv("TEST_REDIS_URL")
	if redisURL == "" {
		redisURL = defaultRedisURL
	}

	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		t.Fatalf("parse redis URL: %v", err)
	}
	rdb := redis.NewClient(opts)
	t.Cleanup(func() { rdb.Close() })

	if err := rdb.Ping(t.Context()).Err(); err != nil {
		t.Fatalf("ping test redis: %v", err)
	}

	return rdb
}

// CleanupDB truncates all tables between tests.
func CleanupDB(t *testing.T, db *sql.DB) {
	t.Helper()
	_, err := db.Exec("TRUNCATE users, games, game_players, phases, orders, messages CASCADE")
	if err != nil {
		t.Fatalf("truncate tables: %v", err)
	}
}

// CleanupRedis flushes the test Redis database between tests.
func CleanupRedis(t *testing.T, rdb *redis.Client) {
	t.Helper()
	if err := rdb.FlushDB(t.Context()).Err(); err != nil {
		t.Fatalf("flush redis: %v", err)
	}
}

// migrationPath resolves the path to the initial migration file relative to the project root.
func migrationPath() string {
	_, filename, _, _ := runtime.Caller(0)
	// testutil.go is at api/internal/testutil/testutil.go
	// migration is at api/migrations/001_initial.up.sql
	apiDir := filepath.Join(filepath.Dir(filename), "..", "..")
	return filepath.Join(apiDir, "migrations", "001_initial.up.sql")
}
