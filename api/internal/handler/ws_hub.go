package handler

import (
	"encoding/json"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/rs/zerolog/log"
)

// Event types sent over WebSocket.
const (
	EventPhaseChanged  = "phase_changed"
	EventPhaseResolved = "phase_resolved"
	EventPlayerReady   = "player_ready"
	EventMessage       = "message"
	EventGameStarted   = "game_started"
	EventGameEnded     = "game_ended"
	EventPowerChanged  = "power_changed"
)

// WSEvent is the envelope for all WebSocket messages.
type WSEvent struct {
	Type   string `json:"type"`
	GameID string `json:"game_id"`
	Data   any    `json:"data"`
}

// ClientMessage is the envelope for messages sent from the client.
type ClientMessage struct {
	Action string `json:"action"` // "subscribe" or "unsubscribe"
	GameID string `json:"game_id"`
}

// WSConn wraps a WebSocket connection with its user and subscriptions.
type WSConn struct {
	conn   *websocket.Conn
	userID string
	send   chan []byte
}

// Hub manages WebSocket connections and game-channel subscriptions.
type Hub struct {
	mu          sync.RWMutex
	connections map[*WSConn]bool
	games       map[string]map[*WSConn]bool // gameID -> set of connections
}

// NewHub creates a new Hub.
func NewHub() *Hub {
	return &Hub{
		connections: make(map[*WSConn]bool),
		games:       make(map[string]map[*WSConn]bool),
	}
}

// Register adds a connection to the hub.
func (h *Hub) Register(c *WSConn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.connections[c] = true
}

// Unregister removes a connection from the hub and all its subscriptions.
func (h *Hub) Unregister(c *WSConn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.connections, c)
	for gameID, conns := range h.games {
		delete(conns, c)
		if len(conns) == 0 {
			delete(h.games, gameID)
		}
	}
	close(c.send)
}

// Subscribe adds a connection to a game channel.
func (h *Hub) Subscribe(c *WSConn, gameID string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.games[gameID] == nil {
		h.games[gameID] = make(map[*WSConn]bool)
	}
	h.games[gameID][c] = true
}

// Unsubscribe removes a connection from a game channel.
func (h *Hub) Unsubscribe(c *WSConn, gameID string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if conns, ok := h.games[gameID]; ok {
		delete(conns, c)
		if len(conns) == 0 {
			delete(h.games, gameID)
		}
	}
}

// BroadcastToGame sends an event to all connections subscribed to a game.
func (h *Hub) BroadcastToGame(gameID string, event WSEvent) {
	data, err := json.Marshal(event)
	if err != nil {
		log.Error().Err(err).Str("gameId", gameID).Msg("Failed to marshal WebSocket event")
		return
	}

	h.mu.RLock()
	defer h.mu.RUnlock()

	for c := range h.games[gameID] {
		select {
		case c.send <- data:
		default:
			log.Warn().Str("userId", c.userID).Str("gameId", gameID).Msg("Dropping WebSocket message, buffer full")
		}
	}
}

// BroadcastToUser sends an event to a specific user across all their connections.
func (h *Hub) BroadcastToUser(userID string, event WSEvent) {
	data, err := json.Marshal(event)
	if err != nil {
		log.Error().Err(err).Str("userId", userID).Msg("Failed to marshal WebSocket event")
		return
	}

	h.mu.RLock()
	defer h.mu.RUnlock()

	for c := range h.connections {
		if c.userID == userID {
			select {
			case c.send <- data:
			default:
			}
		}
	}
}

// ConnectionCount returns the total number of active connections.
func (h *Hub) ConnectionCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.connections)
}

// GameSubscriberCount returns the number of connections subscribed to a game.
func (h *Hub) GameSubscriberCount(gameID string) int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.games[gameID])
}
