package server

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"private-chat/internal/auth"
	"private-chat/internal/logger"
	"private-chat/internal/model"
)

// handleLogin 处理登录并下发 HttpOnly Session Cookie。
func (app *App) handleLogin(c *gin.Context) {
	var body struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 10007, "message": "bad request", "data": nil})
		return
	}
	if body.Username == "" || body.Password == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 10007, "message": "username and password required", "data": nil})
		return
	}

	// 登录失败限流（按客户端 IP）。
	if !app.limiter.Allow("login:" + c.ClientIP()) {
		c.JSON(http.StatusTooManyRequests, gin.H{"code": 10002, "message": "too many attempts", "data": nil})
		return
	}

	sid, user, err := app.authSvc.Login(body.Username, body.Password)
	if err != nil {
		switch {
		case errors.Is(err, auth.ErrUserDisabled):
			logger.Warn("login failed: disabled", map[string]interface{}{"username": body.Username})
			c.JSON(http.StatusForbidden, gin.H{"code": 10003, "message": "user disabled", "data": nil})
		case errors.Is(err, auth.ErrInvalidCred):
			logger.Warn("login failed: invalid credentials", map[string]interface{}{"username": body.Username})
			c.JSON(http.StatusUnauthorized, gin.H{"code": 10002, "message": "invalid credentials", "data": nil})
		default:
			logger.Error("login error", map[string]interface{}{"error": err.Error()})
			c.JSON(http.StatusInternalServerError, gin.H{"code": 10014, "message": "internal error", "data": nil})
		}
		return
	}

	app.setSessionCookie(c, sid)
	logger.Info("login success", map[string]interface{}{"user_id": user.ID, "username": user.Username})
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "data": user.ToPublic()})
}

// handleLogout 退出登录，清除会话与 Cookie。
func (app *App) handleLogout(c *gin.Context) {
	if sid, err := c.Cookie(auth.CookieName()); err == nil {
		_ = app.authSvc.Logout(sid)
	}
	app.clearSessionCookie(c)
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "data": nil})
}

// handleMe 返回当前登录用户信息。
func (app *App) handleMe(c *gin.Context) {
	user := auth.GetUser(c)
	if user == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 10001, "message": "unauthorized", "data": nil})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "data": user.ToPublic()})
}

func (app *App) setSessionCookie(c *gin.Context, sid string) {
	c.SetCookie(
		auth.CookieName(),
		sid,
		int(app.cfg.SessionTTL().Seconds()),
		"/",
		"",
		app.cfg.Security.CookieSecure,
		true, // HttpOnly
	)
}

func (app *App) clearSessionCookie(c *gin.Context) {
	c.SetCookie(auth.CookieName(), "", -1, "/", "", app.cfg.Security.CookieSecure, true)
}

// handleIndex 首页：已登录跳转聊天，否则登录页。
func (app *App) handleIndex(c *gin.Context) {
	if sid, err := c.Cookie(auth.CookieName()); err == nil {
		if _, err := app.authSvc.Validate(sid); err == nil {
			c.Redirect(http.StatusFound, "/chat")
			return
		}
	}
	c.HTML(http.StatusOK, "login.html", gin.H{
		"title":   "Private Chat",
		"version": "1.0.0",
	})
}

// handleChat 聊天页面（已通过 AuthMiddleware）。
func (app *App) handleChat(c *gin.Context) {
	user := auth.GetUser(c)
	c.HTML(http.StatusOK, "chat.html", gin.H{
		"title":    "Private Chat",
		"username": user.Username,
		"nickname": userNickname(user),
		"user_id":  user.ID,
		"role":     user.Role,
	})
}

// handleAdmin 管理后台页面（已通过 Auth + Admin 中间件）。
func (app *App) handleAdmin(c *gin.Context) {
	user := auth.GetUser(c)
	c.HTML(http.StatusOK, "admin.html", gin.H{
		"title":    "Private Chat - Admin",
		"username": user.Username,
		"nickname": userNickname(user),
	})
}

func userNickname(u *model.User) string {
	if u.Nickname != "" {
		return u.Nickname
	}
	return u.Username
}
