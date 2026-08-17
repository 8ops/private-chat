package server

import (
	"crypto/rand"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"private-chat/internal/auth"
	"private-chat/internal/logger"
	"private-chat/internal/model"
	"private-chat/internal/util"
)

// ensureAdmin 首次启动时创建管理员账号。
// 优先级：环境变量 ADMIN_USERNAME/ADMIN_PASSWORD > 自动生成默认管理员（仅当完全无管理员时）。
func (app *App) ensureAdmin() error {
	users, err := app.users.List()
	if err != nil {
		return err
	}
	hasAdmin := false
	for _, u := range users {
		if u.Role == model.RoleAdmin {
			hasAdmin = true
			break
		}
	}
	if hasAdmin {
		return nil
	}

	username := app.cfg.AdminUsername
	password := app.cfg.AdminPassword
	if username != "" && password != "" {
		if err := app.createUser(username, password, "Administrator", model.RoleAdmin, true); err != nil {
			return err
		}
		logger.Info("admin created from env", map[string]interface{}{"username": username})
		return nil
	}

	// 兜底：无管理员且无环境变量时，生成默认管理员并打印初始密码。
	username = "admin"
	password = randomPassword(12)
	if err := app.createUser(username, password, "Administrator", model.RoleAdmin, true); err != nil {
		return err
	}
	logger.Warn("default admin created", map[string]interface{}{
		"username": username,
		"password": password,
		"notice":   "CHANGE THIS PASSWORD IMMEDIATELY via admin panel or ADMIN_USERNAME/ADMIN_PASSWORD env",
	})
	return nil
}

func (app *App) createUser(username, password, nickname, role string, enabled bool) error {
	hash, err := auth.HashPassword(password)
	if err != nil {
		return err
	}
	u := &model.User{
		Username:     username,
		PasswordHash: hash,
		Nickname:     nickname,
		Role:         role,
		Enabled:      enabled,
	}
	return app.users.Create(u)
}

// handleAdminListUsers 返回全部用户（含状态信息）。
func (app *App) handleAdminListUsers(c *gin.Context) {
	users, err := app.users.List()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 10014, "message": "internal error", "data": nil})
		return
	}
	out := make([]gin.H, 0, len(users))
	for _, u := range users {
		out = append(out, gin.H{
			"id":            u.ID,
			"username":      u.Username,
			"nickname":      u.Nickname,
			"role":          u.Role,
			"enabled":       u.Enabled,
			"created_at":    u.CreatedAt,
			"last_login_at": u.LastLoginAt,
		})
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "data": out})
}

// handleAdminCreateUser 创建普通用户。
func (app *App) handleAdminCreateUser(c *gin.Context) {
	var body struct {
		Username string `json:"username"`
		Password string `json:"password"`
		Nickname string `json:"nickname"`
		Enabled  *bool  `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 10007, "message": "bad request", "data": nil})
		return
	}
	if body.Username == "" || body.Password == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 10007, "message": "username and password required", "data": nil})
		return
	}
	if len(body.Password) < 6 {
		c.JSON(http.StatusBadRequest, gin.H{"code": 10007, "message": "password too short (>=6)", "data": nil})
		return
	}
	exists, err := app.users.ExistsByUsername(body.Username)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 10014, "message": "internal error", "data": nil})
		return
	}
	if exists {
		c.JSON(http.StatusConflict, gin.H{"code": 10007, "message": "username already exists", "data": nil})
		return
	}
	enabled := true
	if body.Enabled != nil {
		enabled = *body.Enabled
	}
	nickname := body.Nickname
	if nickname == "" {
		nickname = body.Username
	}
	if err := app.createUser(body.Username, body.Password, nickname, model.RoleUser, enabled); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 10014, "message": "internal error", "data": nil})
		return
	}
	logger.Info("admin created user", map[string]interface{}{"username": body.Username, "by": auth.GetUser(c).Username})
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "data": nil})
}

// handleAdminUpdateUser 更新用户（昵称/启用/密码）。
func (app *App) handleAdminUpdateUser(c *gin.Context) {
	id := c.Param("id")
	var body struct {
		Nickname *string `json:"nickname"`
		Enabled  *bool   `json:"enabled"`
		Password string  `json:"password"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 10007, "message": "bad request", "data": nil})
		return
	}
	u, err := app.users.GetByID(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 10005, "message": "user not found", "data": nil})
		return
	}
	if body.Nickname != nil {
		u.Nickname = *body.Nickname
	}
	if body.Enabled != nil {
		u.Enabled = *body.Enabled
	}
	if body.Password != "" {
		if len(body.Password) < 6 {
			c.JSON(http.StatusBadRequest, gin.H{"code": 10007, "message": "password too short (>=6)", "data": nil})
			return
		}
		hash, err := auth.HashPassword(body.Password)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": 10014, "message": "internal error", "data": nil})
			return
		}
		u.PasswordHash = hash
	}
	if err := app.users.Update(u); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 10014, "message": "internal error", "data": nil})
		return
	}
	logger.Info("admin updated user", map[string]interface{}{"user_id": id, "by": auth.GetUser(c).Username})
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "data": nil})
}

// handleAdminDeleteUser 删除用户并清理其会话。
func (app *App) handleAdminDeleteUser(c *gin.Context) {
	id := c.Param("id")
	u, err := app.users.GetByID(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 10005, "message": "user not found", "data": nil})
		return
	}
	if u.Role == model.RoleAdmin {
		c.JSON(http.StatusForbidden, gin.H{"code": 10004, "message": "cannot delete admin", "data": nil})
		return
	}
	if err := app.users.Delete(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 10014, "message": "internal error", "data": nil})
		return
	}
	_ = app.session.DeleteByUser(id)
	app.hub.RemoveUserClients(id)
	logger.Info("admin deleted user", map[string]interface{}{"user_id": id, "by": auth.GetUser(c).Username})
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "data": nil})
}

// handleAdminResetSession 强制指定用户退出所有会话。
func (app *App) handleAdminResetSession(c *gin.Context) {
	id := c.Param("id")
	if _, err := app.users.GetByID(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 10005, "message": "user not found", "data": nil})
		return
	}
	if err := app.authSvc.ForceLogout(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 10014, "message": "internal error", "data": nil})
		return
	}
	app.hub.RemoveUserClients(id)
	logger.Info("admin reset session", map[string]interface{}{"user_id": id, "by": auth.GetUser(c).Username})
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "data": nil})
}

// handleAdminStats 返回系统运行状态。
func (app *App) handleAdminStats(c *gin.Context) {
	total, _ := app.users.Count()
	data := gin.H{
		"users_total":              total,
		"online_users":             app.hub.OnlineCount(),
		"online_connections":       app.hub.ClientCount(),
		"message_retention_hours":  app.cfg.Chat.MessageRetentionHours,
		"session_expiration_hours": app.cfg.Session.ExpirationHours,
		"db_path":                  app.cfg.Database.Path,
		"upload_dir":               app.cfg.Storage.UploadDir,
		"max_image_size":           app.cfg.Storage.MaxImageSize,
		"max_file_size":            app.cfg.Storage.MaxFileSize,
		"server_time":              time.Now(),
		"version":                  "1.0.0",
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "data": data})
}

func randomPassword(n int) string {
	const chars = "abcdefghijkmnpqrstuvwxyzABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return util.GenToken()[:n]
	}
	for i := range b {
		b[i] = chars[int(b[i])%len(chars)]
	}
	return fmt.Sprintf("Pc%s!", string(b))
}
