package service

// Broadcaster sends real-time events to connected clients.
// Implemented by the WebSocket hub.
type Broadcaster interface {
	BroadcastGameEvent(gameID string, eventType string, data any)
}

// NoopBroadcaster is a no-op implementation for testing or when WS is disabled.
type NoopBroadcaster struct{}

func (NoopBroadcaster) BroadcastGameEvent(string, string, any) {}
