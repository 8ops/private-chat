package repo

import (
	"database/sql"
	"time"

	"private-chat/internal/model"
	"private-chat/internal/util"
)

// SessionRepo 负责 sessions 表访问。
type SessionRepo struct {
	db *sql.DB
}

func NewSessionRepo(db *sql.DB) *SessionRepo { return &SessionRepo{db: db} }

// Create 创建会话，返回生成的会话 ID。
func (r *SessionRepo) Create(userID string, ttl time.Duration) (string, error) {
	id := util.GenToken()
	now := time.Now()
	_, err := r.db.Exec(
		`INSERT INTO sessions (id, user_id, expires_at, created_at, last_seen_at) VALUES (?, ?, ?, ?, ?)`,
		id, userID, now.Add(ttl), now, now,
	)
	return id, err
}

// Get 获取会话并附带用户信息（用于校验）。
func (r *SessionRepo) Get(id string) (*model.Session, error) {
	var s model.Session
	err := r.db.QueryRow(
		`SELECT id, user_id, expires_at, created_at, last_seen_at FROM sessions WHERE id = ?`, id,
	).Scan(&s.ID, &s.UserID, &s.ExpiresAt, &s.CreatedAt, &s.LastSeenAt)
	if err != nil {
		return nil, err
	}
	return &s, nil
}

// Touch 更新 last_seen_at，并用当前时间刷新过期时间。
func (r *SessionRepo) Touch(id string, ttl time.Duration) error {
	now := time.Now()
	_, err := r.db.Exec(`UPDATE sessions SET last_seen_at = ?, expires_at = ? WHERE id = ?`, now, now.Add(ttl), id)
	return err
}

// Delete 删除单个会话。
func (r *SessionRepo) Delete(id string) error {
	_, err := r.db.Exec(`DELETE FROM sessions WHERE id = ?`, id)
	return err
}

// DeleteByUser 删除某用户所有会话（强制下线）。
func (r *SessionRepo) DeleteByUser(userID string) error {
	_, err := r.db.Exec(`DELETE FROM sessions WHERE user_id = ?`, userID)
	return err
}

// DeleteExpired 删除所有过期的会话，返回删除数量。
func (r *SessionRepo) DeleteExpired(now time.Time) (int64, error) {
	res, err := r.db.Exec(`DELETE FROM sessions WHERE expires_at < ?`, now)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	return n, nil
}
