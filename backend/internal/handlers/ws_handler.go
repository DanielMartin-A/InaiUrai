package handlers

import (
	"context"
	"log/slog"
	"net/http"

	"golang.org/x/net/websocket"

	"github.com/DanielMartin-A/InaiUrai/backend/internal/repository"
	"github.com/DanielMartin-A/InaiUrai/backend/internal/services"
	wsHub "github.com/DanielMartin-A/InaiUrai/backend/internal/ws"
)

type WSHandler struct {
	hub        *wsHub.Hub
	memberRepo *repository.MemberRepo
	taskMgr    *services.TaskManager
	orgRepo    *repository.OrgRepo
}

func NewWSHandler(hub *wsHub.Hub, mr *repository.MemberRepo, tm *services.TaskManager, or *repository.OrgRepo) *WSHandler {
	return &WSHandler{hub: hub, memberRepo: mr, taskMgr: tm, orgRepo: or}
}

func (h *WSHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	websocket.Handler(h.handleConn).ServeHTTP(w, r)
}

func (h *WSHandler) handleConn(conn *websocket.Conn) {
	defer conn.Close()
	ctx := context.Background()

	var authMsg struct {
		Type  string `json:"type"`
		Token string `json:"token"`
	}
	if err := websocket.JSON.Receive(conn, &authMsg); err != nil || authMsg.Type != "auth" {
		websocket.JSON.Send(conn, map[string]string{"type": "error", "content": "First message must be auth"})
		return
	}

	member, err := h.memberRepo.GetByAPIToken(ctx, authMsg.Token)
	if err != nil || member == nil {
		websocket.JSON.Send(conn, map[string]string{"type": "error", "content": "Invalid token"})
		return
	}

	h.hub.Register(member.ID, conn)
	defer h.hub.Unregister(member.ID, conn)

	websocket.JSON.Send(conn, map[string]string{"type": "auth_ok"})
	slog.Info("ws authenticated", "member_id", member.ID)

	for {
		var msg struct {
			Type    string `json:"type"`
			Content string `json:"content"`
		}
		if err := websocket.JSON.Receive(conn, &msg); err != nil {
			break
		}

		switch msg.Type {
		case "message":
			if msg.Content == "" {
				continue
			}
			org, _ := h.orgRepo.GetByID(ctx, member.OrgID)
			if org == nil {
				continue
			}

			websocket.JSON.Send(conn, map[string]string{"type": "ack", "content": "Working on it..."})

			result, _ := h.taskMgr.ExecuteSoloTask(ctx, org, member, msg.Content)
			if result != "" {
				websocket.JSON.Send(conn, map[string]string{"type": "done", "content": result})
			}

		case "ping":
			websocket.JSON.Send(conn, map[string]string{"type": "pong"})
		}
	}
}
