package repo

import (
	"database/sql"
	"time"

	"private-chat/internal/model"
	"private-chat/internal/util"
)

// MessageRepo 负责 messages 表访问。
type MessageRepo struct {
	db *sql.DB
}

func NewMessageRepo(db *sql.DB) *MessageRepo { return &MessageRepo{db: db} }

// Create 写入一条消息。
func (r *MessageRepo) Create(m *model.Message) error {
	if m.ID == "" {
		m.ID = util.GenID()
	}
	if m.CreatedAt.IsZero() {
		m.CreatedAt = time.Now()
	}
	if m.RoomID == "" {
		m.RoomID = model.DefaultRoom
	}
	_, err := r.db.Exec(
		`INSERT INTO messages (id, room_id, sender_id, message_type, content, file_id, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		m.ID, m.RoomID, m.SenderID, m.MessageType, m.Content, m.FileID, m.CreatedAt,
	)
	return err
}

func (r *MessageRepo) GetByID(id string) (*model.Message, error) {
	var m model.Message
	err := r.db.QueryRow(
		`SELECT id, room_id, sender_id, message_type, content, file_id, created_at FROM messages WHERE id = ?`, id,
	).Scan(&m.ID, &m.RoomID, &m.SenderID, &m.MessageType, &m.Content, &m.FileID, &m.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &m, nil
}

// GetRecent 获取指定聊天室最近消息。
// 安全约束：强制只返回 created_at >= since 的消息，禁止通过分页参数越界查询历史。
func (r *MessageRepo) GetRecent(roomID string, since time.Time, limit int) ([]*model.Message, error) {
	if roomID == "" {
		roomID = model.DefaultRoom
	}
	if limit <= 0 || limit > 500 {
		limit = 200
	}
	rows, err := r.db.Query(
		`SELECT id, room_id, sender_id, message_type, content, file_id, created_at
		 FROM messages WHERE room_id = ? AND created_at >= ? ORDER BY created_at ASC LIMIT ?`,
		roomID, since, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*model.Message
	for rows.Next() {
		var m model.Message
		if err := rows.Scan(&m.ID, &m.RoomID, &m.SenderID, &m.MessageType, &m.Content, &m.FileID, &m.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, &m)
	}
	return out, rows.Err()
}

// GetExpired 返回创建时间早于 before 的消息（清理用）。
func (r *MessageRepo) GetExpired(before time.Time, batch int) ([]*model.Message, error) {
	if batch <= 0 {
		batch = 500
	}
	rows, err := r.db.Query(
		`SELECT id, room_id, sender_id, message_type, content, file_id, created_at
		 FROM messages WHERE created_at < ? ORDER BY created_at ASC LIMIT ?`,
		before, batch,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*model.Message
	for rows.Next() {
		var m model.Message
		if err := rows.Scan(&m.ID, &m.RoomID, &m.SenderID, &m.MessageType, &m.Content, &m.FileID, &m.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, &m)
	}
	return out, rows.Err()
}

// Delete 删除单条消息。
func (r *MessageRepo) Delete(id string) error {
	_, err := r.db.Exec(`DELETE FROM messages WHERE id = ?`, id)
	return err
}

// CollectExpiredFileIDs 收集过期消息中引用且未被其他未过期消息引用的文件 ID。
// 仅当某 file_id 在所有未过期消息中均不再出现时才可删除，避免误删仍在展示的文件。
func (r *MessageRepo) CollectExpiredFileIDs(before time.Time) ([]string, error) {
	rows, err := r.db.Query(
		`SELECT DISTINCT m.file_id FROM messages m
		 WHERE m.created_at < ? AND m.file_id <> ''
		   AND m.file_id NOT IN (
		     SELECT file_id FROM messages WHERE created_at >= ? AND file_id <> ''
		   )`,
		before, before,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var fid string
		if err := rows.Scan(&fid); err != nil {
			return nil, err
		}
		out = append(out, fid)
	}
	return out, rows.Err()
}
