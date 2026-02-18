package handler

// BroadcastGameEvent implements service.Broadcaster using the WebSocket hub.
func (h *Hub) BroadcastGameEvent(gameID string, eventType string, data any) {
	h.BroadcastToGame(gameID, WSEvent{
		Type:   eventType,
		GameID: gameID,
		Data:   data,
	})
}
