package ws

import (
	"log/slog"
	"sync"

	"github.com/google/uuid"
	"golang.org/x/net/websocket"
)

type Hub struct {
	mu      sync.RWMutex
	clients map[uuid.UUID]map[*websocket.Conn]bool
}

func NewHub() *Hub {
	return &Hub{clients: make(map[uuid.UUID]map[*websocket.Conn]bool)}
}

func (h *Hub) Register(memberID uuid.UUID, conn *websocket.Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.clients[memberID] == nil {
		h.clients[memberID] = make(map[*websocket.Conn]bool)
	}
	h.clients[memberID][conn] = true
	slog.Info("ws client connected", "member_id", memberID)
}

func (h *Hub) Unregister(memberID uuid.UUID, conn *websocket.Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if conns, ok := h.clients[memberID]; ok {
		delete(conns, conn)
		if len(conns) == 0 {
			delete(h.clients, memberID)
		}
	}
}

func (h *Hub) SendToMember(memberID uuid.UUID, msg []byte) {
	h.mu.RLock()
	conns := make([]*websocket.Conn, 0, len(h.clients[memberID]))
	for conn := range h.clients[memberID] {
		conns = append(conns, conn)
	}
	h.mu.RUnlock()

	var failed []*websocket.Conn
	for _, conn := range conns {
		if _, err := conn.Write(msg); err != nil {
			slog.Warn("ws write failed", "member_id", memberID)
			failed = append(failed, conn)
		}
	}

	for _, conn := range failed {
		h.Unregister(memberID, conn)
		conn.Close()
	}
}

func (h *Hub) ActiveConnections() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	count := 0
	for _, conns := range h.clients {
		count += len(conns)
	}
	return count
}
