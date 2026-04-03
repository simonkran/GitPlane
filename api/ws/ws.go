// Package ws provides WebSocket handlers for real-time cluster status updates.
package ws

import (
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/labstack/echo/v4"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

// Client represents a connected WebSocket client watching a specific cluster.
type Client struct {
	ClusterID string
	Conn      *websocket.Conn
	Send      chan []byte
}

// Hub maintains the set of active clients and broadcasts messages to them.
type Hub struct {
	clients    map[*Client]bool
	mu         sync.RWMutex
	register   chan *Client
	unregister chan *Client
}

// NewHub creates a new Hub.
func NewHub() *Hub {
	return &Hub{
		clients:    make(map[*Client]bool),
		register:   make(chan *Client),
		unregister: make(chan *Client),
	}
}

// Run processes register/unregister events. Call in a goroutine.
func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			h.mu.Unlock()
		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.Send)
			}
			h.mu.Unlock()
		}
	}
}

// Broadcast sends a message to all clients watching the given cluster.
func (h *Hub) Broadcast(clusterID string, message []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	for client := range h.clients {
		if client.ClusterID == clusterID {
			select {
			case client.Send <- message:
			default:
				close(client.Send)
				delete(h.clients, client)
			}
		}
	}
}

// HandleWebSocket returns an Echo handler that upgrades to WebSocket.
func HandleWebSocket(hub *Hub) echo.HandlerFunc {
	return func(c echo.Context) error {
		clusterID := c.Param("id")

		conn, err := upgrader.Upgrade(c.Response(), c.Request(), nil)
		if err != nil {
			return err
		}

		client := &Client{
			ClusterID: clusterID,
			Conn:      conn,
			Send:      make(chan []byte, 256),
		}

		hub.register <- client

		// Writer goroutine.
		go func() {
			defer func() {
				hub.unregister <- client
				conn.Close()
			}()
			for message := range client.Send {
				if err := conn.WriteMessage(websocket.TextMessage, message); err != nil {
					return
				}
			}
		}()

		// Reader goroutine (just to detect disconnects).
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				hub.unregister <- client
				break
			}
		}

		return nil
	}
}
