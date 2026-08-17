package repo

import (
	"database/sql"
	"time"

	"private-chat/internal/model"
	"private-chat/internal/util"
)

// FileRepo 负责 files 表访问。
type FileRepo struct {
	db *sql.DB
}

func NewFileRepo(db *sql.DB) *FileRepo { return &FileRepo{db: db} }

func (r *FileRepo) Create(f *model.File) error {
	if f.ID == "" {
		f.ID = util.GenID()
	}
	if f.CreatedAt.IsZero() {
		f.CreatedAt = time.Now()
	}
	_, err := r.db.Exec(
		`INSERT INTO files (id, original_name, stored_name, path, mime_type, size, uploader_id, created_at, deleted_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		f.ID, f.OriginalName, f.StoredName, f.Path, f.MimeType, f.Size, f.UploaderID, f.CreatedAt, nil,
	)
	return err
}

func (r *FileRepo) GetByID(id string) (*model.File, error) {
	var f model.File
	var deletedAt sql.NullTime
	err := r.db.QueryRow(
		`SELECT id, original_name, stored_name, path, mime_type, size, uploader_id, created_at, deleted_at
		 FROM files WHERE id = ?`, id,
	).Scan(&f.ID, &f.OriginalName, &f.StoredName, &f.Path, &f.MimeType, &f.Size, &f.UploaderID, &f.CreatedAt, &deletedAt)
	if err != nil {
		return nil, err
	}
	if deletedAt.Valid {
		t := deletedAt.Time
		f.DeletedAt = &t
	}
	return &f, nil
}

func (r *FileRepo) MarkDeleted(id string) error {
	now := time.Now()
	_, err := r.db.Exec(`UPDATE files SET deleted_at = ? WHERE id = ?`, now, id)
	return err
}

// ListExpired 返回已标记删除且超过保留期的文件（清理重试用）。
func (r *FileRepo) ListExpired(before time.Time) ([]*model.File, error) {
	rows, err := r.db.Query(
		`SELECT id, original_name, stored_name, path, mime_type, size, uploader_id, created_at, deleted_at
		 FROM files WHERE deleted_at IS NOT NULL AND deleted_at < ?`, before,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*model.File
	for rows.Next() {
		var f model.File
		var deletedAt sql.NullTime
		if err := rows.Scan(&f.ID, &f.OriginalName, &f.StoredName, &f.Path, &f.MimeType, &f.Size, &f.UploaderID, &f.CreatedAt, &deletedAt); err != nil {
			return nil, err
		}
		if deletedAt.Valid {
			t := deletedAt.Time
			f.DeletedAt = &t
		}
		out = append(out, &f)
	}
	return out, rows.Err()
}
