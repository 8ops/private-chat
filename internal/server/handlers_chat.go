package server

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"private-chat/internal/auth"
	"private-chat/internal/model"
	"private-chat/internal/ws"
)

// ChatMessage 是消息对外视图，附带文件信息。
type ChatMessage struct {
	model.Message
	File *model.FileView `json:"file,omitempty"`
}

// HandleIncoming 实现 ws.MessageHandler：校验并持久化一条消息。
func (app *App) HandleIncoming(sender *model.User, in ws.IncomingMessage) (*model.Message, error) {
	if !app.limiter.Allow(sender.ID) {
		return nil, errors.New("rate limited")
	}
	mt := in.MessageType
	switch mt {
	case model.MsgTypeText, model.MsgTypeEmoji:
		if len(in.Content) == 0 {
			return nil, errors.New("message content required")
		}
		if len(in.Content) > 2000 {
			return nil, errors.New("message too long")
		}
	case model.MsgTypeSticker:
		if len(in.Content) == 0 {
			return nil, errors.New("sticker id required")
		}
	case model.MsgTypeImage, model.MsgTypeFile:
		if in.FileID == "" {
			return nil, errors.New("file_id required")
		}
		if _, err := app.files.GetByID(in.FileID); err != nil {
			return nil, errors.New("file not found")
		}
	default:
		return nil, errors.New("unsupported message type")
	}

	msg := &model.Message{
		RoomID:      model.DefaultRoom,
		SenderID:    sender.ID,
		SenderName:  senderNickname(sender),
		MessageType: mt,
		Content:     in.Content,
		FileID:      in.FileID,
		CreatedAt:   time.Now(),
	}
	if err := app.messages.Create(msg); err != nil {
		return nil, err
	}
	return msg, nil
}

func senderNickname(u *model.User) string {
	if u.Nickname != "" {
		return u.Nickname
	}
	return u.Username
}

// handleGetMessages 返回最近 4 小时消息（强制时间下限）。
func (app *App) handleGetMessages(c *gin.Context) {
	limit := 200
	if v := c.Query("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			limit = n
		}
	}
	// 安全约束：无论前端如何传参，只返回保留期内的消息。
	since := time.Now().Add(-app.cfg.Retention())
	msgs, err := app.messages.GetRecent(model.DefaultRoom, since, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 10014, "message": "internal error", "data": nil})
		return
	}
	out := make([]ChatMessage, 0, len(msgs))
	for _, m := range msgs {
		out = append(out, app.enrich(m))
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "data": out})
}

// handlePostMessage 通过 HTTP 发送文本/表情/图片/文件消息并广播。
func (app *App) handlePostMessage(c *gin.Context) {
	user := auth.GetUser(c)
	var body struct {
		MessageType string `json:"message_type"`
		Content     string `json:"content"`
		FileID      string `json:"file_id"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 10007, "message": "bad request", "data": nil})
		return
	}
	msg, err := app.HandleIncoming(user, ws.IncomingMessage{
		MessageType: body.MessageType,
		Content:     body.Content,
		FileID:      body.FileID,
	})
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 10007, "message": err.Error(), "data": nil})
		return
	}
	app.hub.BroadcastMessage(msg)
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "data": app.enrich(msg)})
}

// enrich 为消息附加文件视图。
func (app *App) enrich(m *model.Message) ChatMessage {
	cm := ChatMessage{Message: *m}
	if m.FileID != "" {
		if f, err := app.files.GetByID(m.FileID); err == nil && f.DeletedAt == nil {
			v := f.ToView()
			cm.File = &v
		}
	}
	return cm
}

// handleListUsers 返回可展示的用户列表（公开信息）。
func (app *App) handleListUsers(c *gin.Context) {
	users, err := app.users.List()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 10014, "message": "internal error", "data": nil})
		return
	}
	out := make([]model.PublicUser, 0, len(users))
	for _, u := range users {
		out = append(out, u.ToPublic())
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "data": out})
}

// handleHealth 健康检查。
func (app *App) handleHealth(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "ok",
		"data":    gin.H{"status": "healthy"},
	})
}
