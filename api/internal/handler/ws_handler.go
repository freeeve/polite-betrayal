package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/efreeman/polite-betrayal/api/internal/auth"
	"github.com/gorilla/websocket"
)

const (
	writeWait   = 10 * time.Second
	pongWait    = 60 * time.Second
	pingPeriod  = 54 * time.Second // Must be less than pongWait
	maxMsgSize  = 4096
	sendBufSize = 256
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // CORS handled by middleware; tighten in production
	},
}

// WSHandler handles WebSocket connections.
type WSHandler struct {
	hub    *Hub
	jwtMgr *auth.JWTManager
}

// NewWSHandler creates a WSHandler.
func NewWSHandler(hub *Hub, jwtMgr *auth.JWTManager) *WSHandler {
	return &WSHandler{hub: hub, jwtMgr: jwtMgr}
}

// ServeWS handles GET /api/v1/ws — upgrades to WebSocket.
// Auth via ?token= query parameter (WebSocket can't send headers).
func (h *WSHandler) ServeWS(w http.ResponseWriter, r *http.Request) {
	tokenStr := r.URL.Query().Get("token")
	if tokenStr == "" {
		http.Error(w, `{"error":"missing token parameter"}`, http.StatusUnauthorized)
		return
	}

	claims, err := h.jwtMgr.ValidateToken(tokenStr)
	if err != nil {
		http.Error(w, `{"error":"invalid or expired token"}`, http.StatusUnauthorized)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Error().Err(err).Msg("WebSocket upgrade failed")
		return
	}

	client := &WSConn{
		conn:   conn,
		userID: claims.UserID,
		send:   make(chan []byte, sendBufSize),
	}
	h.hub.Register(client)

	// Send a welcome message so the client can confirm the connection is live.
	welcome, _ := json.Marshal(map[string]any{
		"type":    "connected",
		"game_id": "",
		"data":    map[string]any{},
	})
	client.send <- welcome

	go h.writePump(client)
	go h.readPump(client)

	log.Info().Str("userId", claims.UserID).Int("total", h.hub.ConnectionCount()).Msg("WebSocket client connected")
}

// readPump reads messages from the WebSocket connection.
func (h *WSHandler) readPump(c *WSConn) {
	defer func() {
		h.hub.Unregister(c)
		c.conn.Close()
		log.Info().Str("userId", c.userID).Msg("WebSocket client disconnected")
	}()

	c.conn.SetReadLimit(maxMsgSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				log.Warn().Err(err).Str("userId", c.userID).Msg("WebSocket unexpected close")
			}
			break
		}

		var msg ClientMessage
		if err := json.Unmarshal(message, &msg); err != nil {
			continue
		}

		switch msg.Action {
		case "subscribe":
			if msg.GameID != "" {
				h.hub.Subscribe(c, msg.GameID)
			}
		case "unsubscribe":
			if msg.GameID != "" {
				h.hub.Unsubscribe(c, msg.GameID)
			}
		}
	}
}

// writePump writes messages to the WebSocket connection.
func (h *WSHandler) writePump(c *WSConn) {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)

			// Drain queued messages into the same write
			n := len(c.send)
			for i := 0; i < n; i++ {
				w.Write([]byte("\n"))
				w.Write(<-c.send)
			}

			if err := w.Close(); err != nil {
				return
			}
		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
