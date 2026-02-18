//go:build integration

package redis

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	goredis "github.com/redis/go-redis/v9"

	"github.com/freeeve/polite-betrayal/api/internal/testutil"
)

var testRDB *goredis.Client

func setup(t *testing.T) *Client {
	t.Helper()
	if testRDB == nil {
		testRDB = testutil.SetupRedis(t)
	}
	testutil.CleanupRedis(t, testRDB)
	return &Client{rdb: testRDB}
}

func TestGameStateRoundTrip(t *testing.T) {
	c := setup(t)
	ctx := context.Background()
	gameID := "test-game-1"

	state := json.RawMessage(`{"year":1901,"season":"spring","units":[{"type":"army","location":"par"}]}`)

	if err := c.SetGameState(ctx, gameID, state); err != nil {
		t.Fatalf("set game state: %v", err)
	}

	got, err := c.GetGameState(ctx, gameID)
	if err != nil {
		t.Fatalf("get game state: %v", err)
	}
	if got == nil {
		t.Fatal("expected non-nil state")
	}

	var original, fetched map[string]any
	json.Unmarshal(state, &original)
	json.Unmarshal(got, &fetched)
	if fetched["year"].(float64) != 1901 {
		t.Fatalf("state round-trip failed: %s", string(got))
	}
}

func TestGameStateNotFound(t *testing.T) {
	c := setup(t)
	ctx := context.Background()

	got, err := c.GetGameState(ctx, "nonexistent")
	if err != nil {
		t.Fatalf("get missing state: %v", err)
	}
	if got != nil {
		t.Fatal("expected nil for missing game state")
	}
}

func TestOrdersSetAndGet(t *testing.T) {
	c := setup(t)
	ctx := context.Background()
	gameID := "test-game-2"

	franceOrders := json.RawMessage(`[{"type":"hold","location":"par"}]`)
	germanyOrders := json.RawMessage(`[{"type":"move","location":"mun","target":"bur"}]`)

	c.SetOrders(ctx, gameID, "france", franceOrders)
	c.SetOrders(ctx, gameID, "germany", germanyOrders)

	got, err := c.GetOrders(ctx, gameID, "france")
	if err != nil {
		t.Fatalf("get orders: %v", err)
	}
	if string(got) != string(franceOrders) {
		t.Fatalf("expected %s, got %s", franceOrders, got)
	}

	// Missing power returns nil
	missing, err := c.GetOrders(ctx, gameID, "england")
	if err != nil {
		t.Fatalf("get missing orders: %v", err)
	}
	if missing != nil {
		t.Fatal("expected nil for power with no orders")
	}
}

func TestGetAllOrders(t *testing.T) {
	c := setup(t)
	ctx := context.Background()
	gameID := "test-game-3"

	c.SetOrders(ctx, gameID, "france", json.RawMessage(`[{"type":"hold"}]`))
	c.SetOrders(ctx, gameID, "germany", json.RawMessage(`[{"type":"move"}]`))

	powers := []string{"france", "germany", "england"}
	all, err := c.GetAllOrders(ctx, gameID, powers)
	if err != nil {
		t.Fatalf("get all orders: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("expected 2 powers with orders, got %d", len(all))
	}
	if _, ok := all["france"]; !ok {
		t.Fatal("expected france in results")
	}
	if _, ok := all["germany"]; !ok {
		t.Fatal("expected germany in results")
	}
	if _, ok := all["england"]; ok {
		t.Fatal("did not expect england in results")
	}
}

func TestReadySetOperations(t *testing.T) {
	c := setup(t)
	ctx := context.Background()
	gameID := "test-game-4"

	// Initially empty
	count, _ := c.ReadyCount(ctx, gameID)
	if count != 0 {
		t.Fatalf("expected 0 ready, got %d", count)
	}

	c.MarkReady(ctx, gameID, "france")
	c.MarkReady(ctx, gameID, "germany")

	count, _ = c.ReadyCount(ctx, gameID)
	if count != 2 {
		t.Fatalf("expected 2 ready, got %d", count)
	}

	powers, _ := c.ReadyPowers(ctx, gameID)
	if len(powers) != 2 {
		t.Fatalf("expected 2 ready powers, got %d", len(powers))
	}

	// Mark same power again - idempotent
	c.MarkReady(ctx, gameID, "france")
	count, _ = c.ReadyCount(ctx, gameID)
	if count != 2 {
		t.Fatalf("expected 2 ready after duplicate, got %d", count)
	}

	c.UnmarkReady(ctx, gameID, "france")
	count, _ = c.ReadyCount(ctx, gameID)
	if count != 1 {
		t.Fatalf("expected 1 ready after unmark, got %d", count)
	}
}

func TestTimerWithTTL(t *testing.T) {
	c := setup(t)
	ctx := context.Background()
	gameID := "test-game-5"

	deadline := time.Now().Add(10 * time.Second)
	if err := c.SetTimer(ctx, gameID, deadline); err != nil {
		t.Fatalf("set timer: %v", err)
	}

	// Verify key exists with a TTL
	ttl := testRDB.TTL(ctx, timerKey(gameID)).Val()
	if ttl <= 0 || ttl > 11*time.Second {
		t.Fatalf("expected TTL ~10s, got %v", ttl)
	}

	c.ClearTimer(ctx, gameID)
	exists := testRDB.Exists(ctx, timerKey(gameID)).Val()
	if exists != 0 {
		t.Fatal("expected timer key to be deleted")
	}
}

func TestTimerPastDeadline(t *testing.T) {
	c := setup(t)
	ctx := context.Background()
	gameID := "test-game-5b"

	// Past deadline should set minimum 1s TTL
	deadline := time.Now().Add(-5 * time.Second)
	if err := c.SetTimer(ctx, gameID, deadline); err != nil {
		t.Fatalf("set timer past deadline: %v", err)
	}

	ttl := testRDB.TTL(ctx, timerKey(gameID)).Val()
	if ttl <= 0 || ttl > 2*time.Second {
		t.Fatalf("expected TTL ~1s for past deadline, got %v", ttl)
	}
}

func TestClearPhaseData(t *testing.T) {
	c := setup(t)
	ctx := context.Background()
	gameID := "test-game-6"
	powers := []string{"france", "germany"}

	// Set up state, orders, ready, timer
	c.SetGameState(ctx, gameID, json.RawMessage(`{"year":1901}`))
	c.SetOrders(ctx, gameID, "france", json.RawMessage(`[]`))
	c.SetOrders(ctx, gameID, "germany", json.RawMessage(`[]`))
	c.MarkReady(ctx, gameID, "france")
	c.SetTimer(ctx, gameID, time.Now().Add(10*time.Second))

	if err := c.ClearPhaseData(ctx, gameID, powers); err != nil {
		t.Fatalf("clear phase data: %v", err)
	}

	// Orders, ready, timer should be gone
	fr, _ := c.GetOrders(ctx, gameID, "france")
	if fr != nil {
		t.Fatal("expected france orders cleared")
	}
	count, _ := c.ReadyCount(ctx, gameID)
	if count != 0 {
		t.Fatal("expected ready cleared")
	}
	exists := testRDB.Exists(ctx, timerKey(gameID)).Val()
	if exists != 0 {
		t.Fatal("expected timer cleared")
	}

	// State should still exist
	state, _ := c.GetGameState(ctx, gameID)
	if state == nil {
		t.Fatal("expected game state to survive ClearPhaseData")
	}
}

func TestDeleteGameData(t *testing.T) {
	c := setup(t)
	ctx := context.Background()
	gameID := "test-game-7"
	powers := []string{"france", "germany"}

	c.SetGameState(ctx, gameID, json.RawMessage(`{"year":1901}`))
	c.SetOrders(ctx, gameID, "france", json.RawMessage(`[]`))
	c.MarkReady(ctx, gameID, "france")
	c.SetTimer(ctx, gameID, time.Now().Add(10*time.Second))

	if err := c.DeleteGameData(ctx, gameID, powers); err != nil {
		t.Fatalf("delete game data: %v", err)
	}

	// Everything should be gone including state
	state, _ := c.GetGameState(ctx, gameID)
	if state != nil {
		t.Fatal("expected game state deleted")
	}
	fr, _ := c.GetOrders(ctx, gameID, "france")
	if fr != nil {
		t.Fatal("expected orders deleted")
	}
	count, _ := c.ReadyCount(ctx, gameID)
	if count != 0 {
		t.Fatal("expected ready deleted")
	}
}
