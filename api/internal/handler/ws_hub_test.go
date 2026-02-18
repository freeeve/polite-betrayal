package handler

import (
	"encoding/json"
	"sync"
	"testing"
	"time"
)

func newTestConn(userID string) *WSConn {
	return &WSConn{
		conn:   nil, // no real connection for hub tests
		userID: userID,
		send:   make(chan []byte, 256),
	}
}

func TestHubRegisterUnregister(t *testing.T) {
	hub := NewHub()
	c := newTestConn("user-1")

	hub.Register(c)
	if hub.ConnectionCount() != 1 {
		t.Errorf("expected 1 connection, got %d", hub.ConnectionCount())
	}

	hub.Unregister(c)
	if hub.ConnectionCount() != 0 {
		t.Errorf("expected 0 connections, got %d", hub.ConnectionCount())
	}
}

func TestHubSubscribeUnsubscribe(t *testing.T) {
	hub := NewHub()
	c := newTestConn("user-1")
	hub.Register(c)
	defer hub.Unregister(c)

	hub.Subscribe(c, "game-1")
	if hub.GameSubscriberCount("game-1") != 1 {
		t.Errorf("expected 1 subscriber, got %d", hub.GameSubscriberCount("game-1"))
	}

	hub.Unsubscribe(c, "game-1")
	if hub.GameSubscriberCount("game-1") != 0 {
		t.Errorf("expected 0 subscribers, got %d", hub.GameSubscriberCount("game-1"))
	}
}

func TestHubBroadcastToGame(t *testing.T) {
	hub := NewHub()
	c1 := newTestConn("user-1")
	c2 := newTestConn("user-2")
	c3 := newTestConn("user-3") // not subscribed

	hub.Register(c1)
	hub.Register(c2)
	hub.Register(c3)
	defer hub.Unregister(c1)
	defer hub.Unregister(c2)
	defer hub.Unregister(c3)

	hub.Subscribe(c1, "game-1")
	hub.Subscribe(c2, "game-1")

	hub.BroadcastToGame("game-1", WSEvent{
		Type:   EventPhaseChanged,
		GameID: "game-1",
		Data:   map[string]string{"season": "spring"},
	})

	// c1 and c2 should receive, c3 should not
	select {
	case msg := <-c1.send:
		var event WSEvent
		json.Unmarshal(msg, &event)
		if event.Type != EventPhaseChanged {
			t.Errorf("expected phase_changed, got %s", event.Type)
		}
	case <-time.After(time.Second):
		t.Error("c1 did not receive broadcast")
	}

	select {
	case <-c2.send:
		// ok
	case <-time.After(time.Second):
		t.Error("c2 did not receive broadcast")
	}

	select {
	case <-c3.send:
		t.Error("c3 should not have received broadcast")
	default:
		// ok
	}
}

func TestHubBroadcastToUser(t *testing.T) {
	hub := NewHub()
	c1 := newTestConn("user-1")
	c2 := newTestConn("user-1") // same user, two connections
	c3 := newTestConn("user-2")

	hub.Register(c1)
	hub.Register(c2)
	hub.Register(c3)
	defer hub.Unregister(c1)
	defer hub.Unregister(c2)
	defer hub.Unregister(c3)

	hub.BroadcastToUser("user-1", WSEvent{
		Type:   EventMessage,
		GameID: "game-1",
		Data:   map[string]string{"content": "hello"},
	})

	// Both c1 and c2 should receive (same user), c3 should not
	for _, c := range []*WSConn{c1, c2} {
		select {
		case <-c.send:
			// ok
		case <-time.After(time.Second):
			t.Errorf("connection for user-1 did not receive broadcast")
		}
	}

	select {
	case <-c3.send:
		t.Error("user-2 should not have received user-1's message")
	default:
		// ok
	}
}

func TestHubUnregisterCleansUpSubscriptions(t *testing.T) {
	hub := NewHub()
	c := newTestConn("user-1")
	hub.Register(c)
	hub.Subscribe(c, "game-1")
	hub.Subscribe(c, "game-2")

	hub.Unregister(c)

	if hub.GameSubscriberCount("game-1") != 0 {
		t.Errorf("expected 0 subscribers for game-1 after unregister")
	}
	if hub.GameSubscriberCount("game-2") != 0 {
		t.Errorf("expected 0 subscribers for game-2 after unregister")
	}
}

func TestHubConcurrentAccess(t *testing.T) {
	hub := NewHub()
	var wg sync.WaitGroup

	// Concurrently register, subscribe, broadcast, unregister
	for i := range 50 {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			c := newTestConn("user")
			hub.Register(c)
			hub.Subscribe(c, "game-1")
			hub.BroadcastToGame("game-1", WSEvent{Type: "test", GameID: "game-1"})
			hub.Unsubscribe(c, "game-1")
			hub.Unregister(c)
		}(i)
	}

	wg.Wait()
	if hub.ConnectionCount() != 0 {
		t.Errorf("expected 0 connections after concurrent test, got %d", hub.ConnectionCount())
	}
}

func TestHubBroadcastGameEvent(t *testing.T) {
	hub := NewHub()
	c := newTestConn("user-1")
	hub.Register(c)
	defer hub.Unregister(c)
	hub.Subscribe(c, "game-1")

	hub.BroadcastGameEvent("game-1", "phase_resolved", map[string]string{"year": "1901"})

	select {
	case msg := <-c.send:
		var event WSEvent
		json.Unmarshal(msg, &event)
		if event.Type != "phase_resolved" {
			t.Errorf("expected phase_resolved, got %s", event.Type)
		}
		if event.GameID != "game-1" {
			t.Errorf("expected game-1, got %s", event.GameID)
		}
	case <-time.After(time.Second):
		t.Error("did not receive broadcast")
	}
}

func TestWSEventSerialization(t *testing.T) {
	event := WSEvent{
		Type:   EventGameStarted,
		GameID: "game-42",
		Data:   map[string]any{"year": 1901, "season": "spring"},
	}

	data, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var parsed WSEvent
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if parsed.Type != EventGameStarted {
		t.Errorf("expected game_started, got %s", parsed.Type)
	}
	if parsed.GameID != "game-42" {
		t.Errorf("expected game-42, got %s", parsed.GameID)
	}
}

func TestClientMessageSerialization(t *testing.T) {
	msg := ClientMessage{Action: "subscribe", GameID: "game-1"}
	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var parsed ClientMessage
	json.Unmarshal(data, &parsed)
	if parsed.Action != "subscribe" {
		t.Errorf("expected subscribe, got %s", parsed.Action)
	}
	if parsed.GameID != "game-1" {
		t.Errorf("expected game-1, got %s", parsed.GameID)
	}
}
