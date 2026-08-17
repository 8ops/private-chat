package ws

import (
	"encoding/json"
	"sync"
	"time"

	"private-chat/internal/logger"
	"private-chat/internal/model"
)

// IncomingMessage 是客户端发来的消息载荷。
type IncomingMessage struct {
	MessageType string `json:"message_type"`
	Content     string `json:"content"`
	FileID      string `json:"file_id"`
}

// ClientMessage 是客户端发来的 JSON 信封。
type ClientMessage struct {
	Type string          `json:"type"` // "message" | "ping"
	Data IncomingMessage `json:"data"`
}

// Envelope 是服务端发往客户端的 JSON 信封。
type Envelope struct {
	Type string      `json:"type"`
	Data interface{} `json:"data"`
}

// MessageHandler 由上层（server）实现，负责校验与持久化消息。
type MessageHandler interface {
	HandleIncoming(sender *model.User, msg IncomingMessage) (*model.Message, error)
}

// Hub 管理所有 WebSocket 连接与广播。
type Hub struct {
	mu          sync.RWMutex
	clients     map[*Client]struct{}
	userClients map[string]map[*Client]struct{} // userID -> clients
	register    chan *Client
	unregister  chan *Client
	broadcast   chan *model.Message
	handler     MessageHandler
}

// NewHub 创建 Hub。
func NewHub(handler MessageHandler) *Hub {
	return &Hub{
		clients:     make(map[*Client]struct{}),
		userClients: make(map[string]map[*Client]struct{}),
		register:    make(chan *Client, 16),
		unregister:  make(chan *Client, 16),
		broadcast:   make(chan *model.Message, 256),
		handler:     handler,
	}
}

// Run 启动事件循环。
func (h *Hub) Run() {
	for {
		select {
		case c := <-h.register:
			h.addClient(c)
		case c := <-h.unregister:
			h.removeClient(c)
		case m := <-h.broadcast:
			h.dispatch(m)
		}
	}
}

func (h *Hub) addClient(c *Client) {
	h.mu.Lock()
	h.clients[c] = struct{}{}
	if h.userClients[c.user.ID] == nil {
		h.userClients[c.user.ID] = make(map[*Client]struct{})
	}
	h.userClients[c.user.ID][c] = struct{}{}
	h.mu.Unlock()
	h.broadcastPresence()
	logger.Info("ws client connected", map[string]interface{}{"user_id": c.user.ID, "username": c.user.Username})
}

func (h *Hub) removeClient(c *Client) {
	h.mu.Lock()
	if _, ok := h.clients[c]; ok {
		delete(h.clients, c)
		if set, ok := h.userClients[c.user.ID]; ok {
			delete(set, c)
			if len(set) == 0 {
				delete(h.userClients, c.user.ID)
			}
		}
	}
	h.mu.Unlock()
	h.broadcastPresence()
	logger.Info("ws client disconnected", map[string]interface{}{"user_id": c.user.ID, "username": c.user.Username})
}

func (h *Hub) dispatch(m *model.Message) {
	data, err := json.Marshal(Envelope{Type: "message", Data: m})
	if err != nil {
		return
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	for c := range h.clients {
		select {
		case c.send <- data:
		default:
			// 发送队列满，丢弃（避免阻塞广播）。
			go c.Close()
		}
	}
}

// BroadcastMessage 推送一条消息给所有在线客户端。
func (h *Hub) BroadcastMessage(m *model.Message) {
	h.broadcast <- m
}

// Presence 描述在线用户快照。
type Presence struct {
	OnlineCount int                `json:"online_count"`
	Users       []model.PublicUser `json:"users"`
}

func (h *Hub) presence() Presence {
	h.mu.RLock()
	defer h.mu.RUnlock()
	users := make([]model.PublicUser, 0, len(h.userClients))
	for uid := range h.userClients {
		// 仅取第一个连接的用户对象构造公开视图。
		for c := range h.userClients[uid] {
			users = append(users, model.PublicUser{
				ID:       c.user.ID,
				Username: c.user.Username,
				Nickname: c.user.Nickname,
				Role:     c.user.Role,
				Enabled:  c.user.Enabled,
			})
			break
		}
	}
	return Presence{OnlineCount: len(h.userClients), Users: users}
}

func (h *Hub) broadcastPresence() {
	p := h.presence()
	data, err := json.Marshal(Envelope{Type: "presence", Data: p})
	if err != nil {
		return
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	for c := range h.clients {
		select {
		case c.send <- data:
		default:
		}
	}
}

// OnlineCount 返回当前在线用户数（去重）。
func (h *Hub) OnlineCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.userClients)
}

// ClientCount 返回当前连接数。
func (h *Hub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

// pushTo 向指定用户的所有连接推送消息（如强制下线通知）。
func (h *Hub) pushTo(userID string, env Envelope) {
	data, err := json.Marshal(env)
	if err != nil {
		return
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	if set, ok := h.userClients[userID]; ok {
		for c := range set {
			select {
			case c.send <- data:
			default:
			}
		}
	}
}

// KickUser 向指定用户广播强制下线通知（由 server 处理实际断连）。
func (h *Hub) KickUser(userID string) {
	h.pushTo(userID, Envelope{Type: "kicked", Data: map[string]string{"reason": "session_reset"}})
}

// RemoveUserClients 关闭指定用户的所有连接（管理员强制下线后调用）。
func (h *Hub) RemoveUserClients(userID string) {
	h.mu.Lock()
	set := h.userClients[userID]
	delete(h.userClients, userID)
	for c := range set {
		delete(h.clients, c)
		go c.Close()
	}
	h.mu.Unlock()
	h.broadcastPresence()
}

// 心跳间隔与超时。
const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = 30 * time.Second
	maxMessageSize = 4 * 1024 * 1024
)
