package repo

import (
	"database/sql"
	"time"

	"private-chat/internal/model"
	"private-chat/internal/util"
)

// UserRepo 负责 users 表访问。
type UserRepo struct {
	db *sql.DB
}

func NewUserRepo(db *sql.DB) *UserRepo { return &UserRepo{db: db} }

func (r *UserRepo) Create(u *model.User) error {
	if u.ID == "" {
		u.ID = util.GenID()
	}
	now := time.Now()
	u.CreatedAt = now
	u.UpdatedAt = now
	_, err := r.db.Exec(
		`INSERT INTO users (id, username, password_hash, nickname, role, enabled, created_at, updated_at, last_login_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		u.ID, u.Username, u.PasswordHash, u.Nickname, u.Role, boolToInt(u.Enabled), now, now, nil,
	)
	return err
}

func (r *UserRepo) GetByUsername(username string) (*model.User, error) {
	return r.queryOne(`SELECT id, username, password_hash, nickname, role, enabled, created_at, updated_at, last_login_at FROM users WHERE username = ?`, username)
}

func (r *UserRepo) GetByID(id string) (*model.User, error) {
	return r.queryOne(`SELECT id, username, password_hash, nickname, role, enabled, created_at, updated_at, last_login_at FROM users WHERE id = ?`, id)
}

func (r *UserRepo) List() ([]*model.User, error) {
	rows, err := r.db.Query(`SELECT id, username, password_hash, nickname, role, enabled, created_at, updated_at, last_login_at FROM users ORDER BY created_at ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*model.User
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

// ListEnabledActive 用于在线用户统计等（实际在线由 WS Hub 维护）。
func (r *UserRepo) Count() (int, error) {
	var n int
	err := r.db.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&n)
	return n, err
}

func (r *UserRepo) Update(u *model.User) error {
	u.UpdatedAt = time.Now()
	_, err := r.db.Exec(
		`UPDATE users SET nickname = ?, enabled = ?, password_hash = ?, updated_at = ? WHERE id = ?`,
		u.Nickname, boolToInt(u.Enabled), u.PasswordHash, u.UpdatedAt, u.ID,
	)
	return err
}

// UpdateEnabled 单独更新启用状态。
func (r *UserRepo) UpdateEnabled(id string, enabled bool) error {
	_, err := r.db.Exec(`UPDATE users SET enabled = ?, updated_at = ? WHERE id = ?`, boolToInt(enabled), time.Now(), id)
	return err
}

// UpdatePassword 更新密码哈希。
func (r *UserRepo) UpdatePassword(id, passwordHash string) error {
	_, err := r.db.Exec(`UPDATE users SET password_hash = ?, updated_at = ? WHERE id = ?`, passwordHash, time.Now(), id)
	return err
}

func (r *UserRepo) UpdateLastLogin(id string, t time.Time) error {
	_, err := r.db.Exec(`UPDATE users SET last_login_at = ?, updated_at = ? WHERE id = ?`, t, t, id)
	return err
}

func (r *UserRepo) Delete(id string) error {
	_, err := r.db.Exec(`DELETE FROM users WHERE id = ?`, id)
	return err
}

func (r *UserRepo) ExistsByUsername(username string) (bool, error) {
	var n int
	err := r.db.QueryRow(`SELECT COUNT(*) FROM users WHERE username = ?`, username).Scan(&n)
	return n > 0, err
}

func (r *UserRepo) queryOne(query string, args ...interface{}) (*model.User, error) {
	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	if !rows.Next() {
		return nil, sql.ErrNoRows
	}
	return scanUser(rows)
}

func scanUser(rows *sql.Rows) (*model.User, error) {
	var (
		u                 model.User
		enabled           int
		lastLogin         sql.NullTime
		passwordHash      string
	)
	if err := rows.Scan(&u.ID, &u.Username, &passwordHash, &u.Nickname, &u.Role, &enabled, &u.CreatedAt, &u.UpdatedAt, &lastLogin); err != nil {
		return nil, err
	}
	u.PasswordHash = passwordHash
	u.Enabled = enabled != 0
	if lastLogin.Valid {
		t := lastLogin.Time
		u.LastLoginAt = &t
	}
	return &u, nil
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
