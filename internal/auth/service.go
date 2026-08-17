package auth

import (
	"errors"
	"time"

	"private-chat/internal/config"
	"private-chat/internal/model"
	"private-chat/internal/repo"
)

// Service 处理登录、登出与会话校验。
type Service struct {
	cfg     *config.Config
	users   *repo.UserRepo
	session *repo.SessionRepo
}

func NewService(cfg *config.Config, users *repo.UserRepo, session *repo.SessionRepo) *Service {
	return &Service{cfg: cfg, users: users, session: session}
}

var (
	ErrUserDisabled   = errors.New("user disabled")
	ErrInvalidCred    = errors.New("invalid credentials")
	ErrSessionExpired = errors.New("session expired")
)

// Login 校验账号密码，成功返回会话 ID 与用户。
func (s *Service) Login(username, password string) (sessionID string, user *model.User, err error) {
	u, err := s.users.GetByUsername(username)
	if err != nil {
		// 统一返回凭证错误，避免用户枚举。
		return "", nil, ErrInvalidCred
	}
	if !u.Enabled {
		return "", nil, ErrUserDisabled
	}
	if !VerifyPassword(password, u.PasswordHash) {
		return "", nil, ErrInvalidCred
	}
	id, err := s.session.Create(u.ID, s.cfg.SessionTTL())
	if err != nil {
		return "", nil, err
	}
	_ = s.users.UpdateLastLogin(u.ID, time.Now())
	return id, u, nil
}

// Logout 删除指定会话。
func (s *Service) Logout(sessionID string) error {
	return s.session.Delete(sessionID)
}

// Validate 校验会话有效性，自动续期并返回用户。
func (s *Service) Validate(sessionID string) (*model.User, error) {
	if sessionID == "" {
		return nil, ErrSessionExpired
	}
	sess, err := s.session.Get(sessionID)
	if err != nil {
		return nil, ErrSessionExpired
	}
	if time.Now().After(sess.ExpiresAt) {
		_ = s.session.Delete(sessionID)
		return nil, ErrSessionExpired
	}
	u, err := s.users.GetByID(sess.UserID)
	if err != nil {
		_ = s.session.Delete(sessionID)
		return nil, ErrSessionExpired
	}
	if !u.Enabled {
		_ = s.session.Delete(sessionID)
		return nil, ErrUserDisabled
	}
	// 续期。
	_ = s.session.Touch(sessionID, s.cfg.SessionTTL())
	return u, nil
}

// ForceLogout 强制用户退出所有会话（管理员操作）。
func (s *Service) ForceLogout(userID string) error {
	return s.session.DeleteByUser(userID)
}
