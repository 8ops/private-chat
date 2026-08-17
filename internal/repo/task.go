package repo

import (
	"database/sql"
	"time"

	"private-chat/internal/util"
)

// CleanupTaskRepo 负责 cleanup_tasks 表访问（文件删除失败重试）。
type CleanupTaskRepo struct {
	db *sql.DB
}

func NewCleanupTaskRepo(db *sql.DB) *CleanupTaskRepo { return &CleanupTaskRepo{db: db} }

// UpsertFailed 记录一次失败的文件删除任务；已存在则增加重试计数。
func (r *CleanupTaskRepo) UpsertFailed(fileID, lastErr string) error {
	now := time.Now()
	var existing string
	err := r.db.QueryRow(`SELECT id FROM cleanup_tasks WHERE file_id = ? AND status = 'pending'`, fileID).Scan(&existing)
	if err == sql.ErrNoRows {
		id := util.GenID()
		_, e := r.db.Exec(
			`INSERT INTO cleanup_tasks (id, file_id, status, retry_count, last_error, created_at, updated_at)
			 VALUES (?, ?, 'pending', 1, ?, ?, ?)`,
			id, fileID, lastErr, now, now,
		)
		return e
	}
	if err != nil {
		return err
	}
	_, e := r.db.Exec(
		`UPDATE cleanup_tasks SET retry_count = retry_count + 1, last_error = ?, updated_at = ? WHERE id = ?`,
		lastErr, now, existing,
	)
	return e
}

// ListPending 返回待重试且未超过最大次数的任务。
func (r *CleanupTaskRepo) ListPending(maxRetry int) ([]string, error) {
	rows, err := r.db.Query(
		`SELECT file_id FROM cleanup_tasks WHERE status = 'pending' AND retry_count < ?`, maxRetry,
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

// MarkDone 标记任务完成（删除任务记录）。
func (r *CleanupTaskRepo) MarkDone(fileID string) error {
	_, err := r.db.Exec(`DELETE FROM cleanup_tasks WHERE file_id = ?`, fileID)
	return err
}
