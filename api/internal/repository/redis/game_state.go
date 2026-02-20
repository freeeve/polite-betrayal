package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// Key patterns for Redis game state.
func stateKey(gameID string) string         { return "game:" + gameID + ":state" }
func ordersKey(gameID, power string) string { return "game:" + gameID + ":orders:" + power }
func readyKey(gameID string) string         { return "game:" + gameID + ":ready" }
func timerKey(gameID string) string         { return "game:" + gameID + ":timer" }
func drawVoteKey(gameID string) string      { return "game:" + gameID + ":draw_votes" }

// SetGameState stores the live game state JSON.
func (c *Client) SetGameState(ctx context.Context, gameID string, state json.RawMessage) error {
	return c.rdb.Set(ctx, stateKey(gameID), []byte(state), 0).Err()
}

// GetGameState retrieves the live game state JSON.
func (c *Client) GetGameState(ctx context.Context, gameID string) (json.RawMessage, error) {
	data, err := c.rdb.Get(ctx, stateKey(gameID)).Bytes()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get game state: %w", err)
	}
	return json.RawMessage(data), nil
}

// SetOrders stores a power's orders for the current phase.
func (c *Client) SetOrders(ctx context.Context, gameID, power string, orders json.RawMessage) error {
	return c.rdb.Set(ctx, ordersKey(gameID, power), []byte(orders), 0).Err()
}

// GetOrders retrieves a power's submitted orders.
func (c *Client) GetOrders(ctx context.Context, gameID, power string) (json.RawMessage, error) {
	data, err := c.rdb.Get(ctx, ordersKey(gameID, power)).Bytes()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get orders: %w", err)
	}
	return json.RawMessage(data), nil
}

// GetAllOrders retrieves orders from all powers that have submitted.
func (c *Client) GetAllOrders(ctx context.Context, gameID string, powers []string) (map[string]json.RawMessage, error) {
	result := make(map[string]json.RawMessage)
	for _, power := range powers {
		data, err := c.GetOrders(ctx, gameID, power)
		if err != nil {
			return nil, err
		}
		if data != nil {
			result[power] = data
		}
	}
	return result, nil
}

// MarkReady adds a power to the ready set for the game.
func (c *Client) MarkReady(ctx context.Context, gameID, power string) error {
	return c.rdb.SAdd(ctx, readyKey(gameID), power).Err()
}

// UnmarkReady removes a power from the ready set.
func (c *Client) UnmarkReady(ctx context.Context, gameID, power string) error {
	return c.rdb.SRem(ctx, readyKey(gameID), power).Err()
}

// ReadyCount returns how many powers have marked ready.
func (c *Client) ReadyCount(ctx context.Context, gameID string) (int64, error) {
	return c.rdb.SCard(ctx, readyKey(gameID)).Result()
}

// ReadyPowers returns the set of powers that have marked ready.
func (c *Client) ReadyPowers(ctx context.Context, gameID string) ([]string, error) {
	return c.rdb.SMembers(ctx, readyKey(gameID)).Result()
}

// phaseGracePeriod is the extra time after the displayed deadline before
// phase resolution triggers, giving players a few seconds of leeway.
const phaseGracePeriod = 5 * time.Second

// SetTimer creates a timer key with a TTL. When the key expires,
// Redis keyspace notifications trigger phase resolution.
// The TTL includes a grace period so the key expires slightly after the displayed deadline.
func (c *Client) SetTimer(ctx context.Context, gameID string, deadline time.Time) error {
	ttl := time.Until(deadline) + phaseGracePeriod
	if ttl <= 0 {
		ttl = time.Second
	}
	return c.rdb.Set(ctx, timerKey(gameID), deadline.Unix(), ttl).Err()
}

// ClearTimer removes the timer for a game.
func (c *Client) ClearTimer(ctx context.Context, gameID string) error {
	return c.rdb.Del(ctx, timerKey(gameID)).Err()
}

// AddDrawVote adds a power to the draw vote set.
func (c *Client) AddDrawVote(ctx context.Context, gameID, power string) error {
	return c.rdb.SAdd(ctx, drawVoteKey(gameID), power).Err()
}

// RemoveDrawVote removes a power from the draw vote set.
func (c *Client) RemoveDrawVote(ctx context.Context, gameID, power string) error {
	return c.rdb.SRem(ctx, drawVoteKey(gameID), power).Err()
}

// DrawVoteCount returns how many powers have voted for a draw.
func (c *Client) DrawVoteCount(ctx context.Context, gameID string) (int64, error) {
	return c.rdb.SCard(ctx, drawVoteKey(gameID)).Result()
}

// DrawVotePowers returns the set of powers that have voted for a draw.
func (c *Client) DrawVotePowers(ctx context.Context, gameID string) ([]string, error) {
	return c.rdb.SMembers(ctx, drawVoteKey(gameID)).Result()
}

// ClearPhaseData removes all orders, ready status, and timer for a game.
// Called after phase resolution to prepare for the next phase.
func (c *Client) ClearPhaseData(ctx context.Context, gameID string, powers []string) error {
	keys := []string{readyKey(gameID), timerKey(gameID), drawVoteKey(gameID)}
	for _, power := range powers {
		keys = append(keys, ordersKey(gameID, power))
	}
	return c.rdb.Del(ctx, keys...).Err()
}

// DeleteGameData removes all Redis data for a game (on game end).
func (c *Client) DeleteGameData(ctx context.Context, gameID string, powers []string) error {
	keys := []string{stateKey(gameID), readyKey(gameID), timerKey(gameID), drawVoteKey(gameID)}
	for _, power := range powers {
		keys = append(keys, ordersKey(gameID, power))
	}
	return c.rdb.Del(ctx, keys...).Err()
}
