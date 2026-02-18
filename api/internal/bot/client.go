package bot

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/rs/zerolog/log"
)

// WSEvent mirrors handler.WSEvent for client-side deserialization.
type WSEvent struct {
	Type   string         `json:"type"`
	GameID string         `json:"game_id"`
	Data   map[string]any `json:"data"`
}

// Client is an HTTP+WebSocket client for a single bot player.
type Client struct {
	name     string
	baseURL  string
	token    string
	userID   string
	wsConn   *websocket.Conn
	events   chan WSEvent
	httpC    *http.Client
	mu       sync.Mutex
	closedWS bool
}

// NewClient creates a new bot client targeting the given server URL.
func NewClient(name, baseURL string) *Client {
	return &Client{
		name:    name,
		baseURL: strings.TrimRight(baseURL, "/"),
		events:  make(chan WSEvent, 64),
		httpC:   &http.Client{Timeout: 30 * time.Second},
	}
}

// Name returns the bot name.
func (c *Client) Name() string { return c.name }

// UserID returns the bot's user ID after login.
func (c *Client) UserID() string { return c.userID }

// Login authenticates via the dev login endpoint.
func (c *Client) Login() error {
	resp, err := c.httpC.Get(c.baseURL + "/auth/dev?name=" + url.QueryEscape(c.name))
	if err != nil {
		return fmt.Errorf("dev login request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("dev login status %d: %s", resp.StatusCode, body)
	}

	var tokens struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tokens); err != nil {
		return fmt.Errorf("decode tokens: %w", err)
	}
	c.token = tokens.AccessToken

	// Fetch user ID from /users/me
	user, err := c.getJSON("/api/v1/users/me")
	if err != nil {
		return fmt.Errorf("get user: %w", err)
	}
	if id, ok := user["id"].(string); ok {
		c.userID = id
	}
	log.Debug().Str("bot", c.name).Str("userId", c.userID).Msg("Bot logged in")
	return nil
}

// CreateGame creates a new game and returns its ID.
func (c *Client) CreateGame(name, turnDuration, retreatDuration, buildDuration string) (string, error) {
	body := map[string]string{
		"name":             name,
		"turn_duration":    turnDuration,
		"retreat_duration": retreatDuration,
		"build_duration":   buildDuration,
	}
	resp, err := c.postJSON("/api/v1/games", body)
	if err != nil {
		return "", err
	}
	id, _ := resp["id"].(string)
	return id, nil
}

// JoinGame joins an existing game.
func (c *Client) JoinGame(gameID string) error {
	_, err := c.postJSON("/api/v1/games/"+gameID+"/join", nil)
	return err
}

// StartGame starts a game (creator only).
func (c *Client) StartGame(gameID string) error {
	_, err := c.postJSON("/api/v1/games/"+gameID+"/start", nil)
	return err
}

// GetGame fetches game details.
func (c *Client) GetGame(gameID string) (map[string]any, error) {
	return c.getJSON("/api/v1/games/" + gameID)
}

// GetCurrentPhase fetches the current phase for a game.
func (c *Client) GetCurrentPhase(gameID string) (map[string]any, error) {
	return c.getJSON("/api/v1/games/" + gameID + "/phases/current")
}

// OrderInput matches service.OrderInput.
type OrderInput struct {
	UnitType    string `json:"unit_type"`
	Location    string `json:"location"`
	Coast       string `json:"coast,omitempty"`
	OrderType   string `json:"order_type"`
	Target      string `json:"target,omitempty"`
	TargetCoast string `json:"target_coast,omitempty"`
	AuxLoc      string `json:"aux_loc,omitempty"`
	AuxTarget   string `json:"aux_target,omitempty"`
	AuxUnitType string `json:"aux_unit_type,omitempty"`
}

// SubmitOrders submits orders for the current phase.
func (c *Client) SubmitOrders(gameID string, orders []OrderInput) error {
	payload := map[string]any{"orders": orders}
	return c.post("/api/v1/games/"+gameID+"/orders", payload)
}

// MarkReady marks this bot as ready.
func (c *Client) MarkReady(gameID string) error {
	return c.post("/api/v1/games/"+gameID+"/orders/ready", nil)
}

// ConnectWS opens a WebSocket connection and starts listening for events.
func (c *Client) ConnectWS() error {
	wsURL := strings.Replace(c.baseURL, "http", "ws", 1) + "/api/v1/ws?token=" + url.QueryEscape(c.token)
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		return fmt.Errorf("ws dial: %w", err)
	}
	c.wsConn = conn

	go c.readWSLoop()
	return nil
}

// SubscribeGame sends a subscribe message for the given game.
func (c *Client) SubscribeGame(gameID string) error {
	msg := map[string]string{"action": "subscribe", "game_id": gameID}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.wsConn.WriteJSON(msg)
}

// Events returns the channel of incoming WebSocket events.
func (c *Client) Events() <-chan WSEvent { return c.events }

// CloseWS closes the WebSocket connection.
func (c *Client) CloseWS() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.wsConn != nil && !c.closedWS {
		c.closedWS = true
		c.wsConn.WriteMessage(websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
		c.wsConn.Close()
	}
}

func (c *Client) readWSLoop() {
	defer close(c.events)
	for {
		_, msg, err := c.wsConn.ReadMessage()
		if err != nil {
			if !c.closedWS {
				log.Debug().Err(err).Str("bot", c.name).Msg("WS read error")
			}
			return
		}
		var event WSEvent
		if err := json.Unmarshal(msg, &event); err != nil {
			continue
		}
		c.events <- event
	}
}

func (c *Client) getJSON(path string) (map[string]any, error) {
	req, err := http.NewRequest(http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)

	resp, err := c.httpC.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("GET %s: status %d: %s", path, resp.StatusCode, body)
	}

	var result map[string]any
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return result, nil
}

// post sends a POST request and checks for errors without decoding the response body.
func (c *Client) post(path string, payload any) error {
	var bodyReader io.Reader
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		bodyReader = bytes.NewReader(data)
	} else {
		bodyReader = bytes.NewReader([]byte("{}"))
	}

	req, err := http.NewRequest(http.MethodPost, c.baseURL+path, bodyReader)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpC.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("POST %s: status %d: %s", path, resp.StatusCode, body)
	}
	return nil
}

func (c *Client) postJSON(path string, payload any) (map[string]any, error) {
	var bodyReader io.Reader
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return nil, err
		}
		bodyReader = bytes.NewReader(data)
	} else {
		bodyReader = bytes.NewReader([]byte("{}"))
	}

	req, err := http.NewRequest(http.MethodPost, c.baseURL+path, bodyReader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpC.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("POST %s: status %d: %s", path, resp.StatusCode, body)
	}

	var result map[string]any
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return result, nil
}
