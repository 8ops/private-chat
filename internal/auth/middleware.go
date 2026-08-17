package auth

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"private-chat/internal/model"
)

const (
	ctxUserKey    = "auth_user"
	cookieName    = "session_id"
)

// CookieName 返回会话 Cookie 名称。
func CookieName() string { return cookieName }

// AuthMiddleware 校验会话 Cookie，将用户写入上下文。
func (s *Service) AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		sid, err := c.Cookie(cookieName)
		if err != nil || sid == "" {
			abortUnauthorized(c)
			return
		}
		user, err := s.Validate(sid)
		if err != nil {
			// 清除无效 Cookie。
			c.SetCookie(cookieName, "", -1, "/", "", false, true)
			abortUnauthorized(c)
			return
		}
		c.Set(ctxUserKey, user)
		c.Next()
	}
}

// AdminMiddleware 要求当前用户为管理员。
func (s *Service) AdminMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		user, ok := c.Get(ctxUserKey)
		if !ok {
			abortUnauthorized(c)
			return
		}
		u, ok := user.(*model.User)
		if !ok || u.Role != model.RoleAdmin {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"code":    10004,
				"message": "forbidden",
				"data":    nil,
			})
			return
		}
		c.Next()
	}
}

// GetUser 从上下文取当前用户。
func GetUser(c *gin.Context) *model.User {
	if v, ok := c.Get(ctxUserKey); ok {
		if u, ok := v.(*model.User); ok {
			return u
		}
	}
	return nil
}

func abortUnauthorized(c *gin.Context) {
	c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
		"code":    10001,
		"message": "unauthorized",
		"data":    nil,
	})
}
