package ws

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"

	"private-chat/internal/auth"
	"private-chat/internal/logger"
	"private-chat/internal/model"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 4096,
	CheckOrigin:     func(r *http.Request) bool { return true }, // 内网部署，允许同源/跨域
}

// Client 表示一个 WebSocket 连接。
type Client struct {
	hub    *Hub
	conn   *websocket.Conn
	send   chan []byte
	user   *model.User
	roomID string
}

// ServeWS 处理 WebSocket 升级与连接生命周期。
func ServeWS(hub *Hub, authSvc *auth.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		sid, err := c.Cookie(auth.CookieName())
		if err != nil || sid == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"code": 10013, "message": "ws unauthorized", "data": nil})
			return
		}
		user, err := authSvc.Validate(sid)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"code": 10013, "message": "ws unauthorized", "data": nil})
			return
		}
		conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			logger.Error("ws upgrade failed", map[string]interface{}{"error": err.Error()})
			return
		}
		client := &Client{
			hub:    hub,
			conn:   conn,
			send:   make(chan []byte, 64),
			user:   user,
			roomID: model.DefaultRoom,
		}
		hub.register <- client
		go client.writePump()
		go client.readPump()
	}
}

func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()
	c.conn.SetReadLimit(maxMessageSize)
	_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		return c.conn.SetReadDeadline(time.Now().Add(pongWait))
	})

	for {
		_, data, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
				logger.Warn("ws read error", map[string]interface{}{"error": err.Error(), "user_id": c.user.ID})
			}
			break
		}
		var cm ClientMessage
		if err := json.Unmarshal(data, &cm); err != nil {
			c.sendError(10007, "bad request")
			continue
		}
		switch cm.Type {
		case "ping":
			// 应用层心跳：直接回 pong。
			c.sendEnvelope(Envelope{Type: "pong", Data: nil})
		case "message":
			c.handleMessage(cm.Data)
		default:
			c.sendError(10007, "unknown message type")
		}
	}
}

func (c *Client) handleMessage(in IncomingMessage) {
	msg, err := c.hub.handler.HandleIncoming(c.user, in)
	if err != nil {
		c.sendError(10007, err.Error())
		return
	}
	c.hub.BroadcastMessage(msg)
}

func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()
	for {
		select {
		case data, ok := <-c.send:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, data); err != nil {
				return
			}
		case <-ticker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (c *Client) sendEnvelope(env Envelope) {
	data, err := json.Marshal(env)
	if err != nil {
		return
	}
	select {
	case c.send <- data:
	default:
	}
}

func (c *Client) sendError(code int, message string) {
	c.sendEnvelope(Envelope{Type: "error", Data: gin.H{"code": code, "message": message}})
}

// Close 关闭连接（供 Hub 强制下线使用）。
func (c *Client) Close() {
	_ = c.conn.Close()
}

// SendJSON 向客户端发送任意信封（导出供 Hub/Kick 使用）。
func (c *Client) SendJSON(env Envelope) { c.sendEnvelope(env) }
