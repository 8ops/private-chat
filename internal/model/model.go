package model

import "time"

// 角色常量
const (
	RoleAdmin = "admin"
	RoleUser  = "user"
)

// 消息类型常量
const (
	MsgTypeText    = "text"
	MsgTypeEmoji   = "emoji"
	MsgTypeSticker = "sticker"
	MsgTypeImage   = "image"
	MsgTypeFile    = "file"
)

// 默认聊天室
const DefaultRoom = "general"

// User 对应 users 表。
type User struct {
	ID           string     `json:"id"`
	Username     string     `json:"username"`
	PasswordHash string     `json:"-"`
	Nickname     string     `json:"nickname"`
	Role         string     `json:"role"`
	Enabled      bool       `json:"enabled"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
	LastLoginAt  *time.Time `json:"last_login_at,omitempty"`
}

// PublicUser 是对外安全的用户视图，不含密码哈希。
type PublicUser struct {
	ID          string     `json:"id"`
	Username    string     `json:"username"`
	Nickname    string     `json:"nickname"`
	Role        string     `json:"role"`
	Enabled     bool       `json:"enabled"`
	CreatedAt   time.Time  `json:"created_at"`
	LastLoginAt *time.Time `json:"last_login_at,omitempty"`
}

// ToPublic 转换为安全视图。
func (u *User) ToPublic() PublicUser {
	return PublicUser{
		ID:          u.ID,
		Username:    u.Username,
		Nickname:    u.Nickname,
		Role:        u.Role,
		Enabled:     u.Enabled,
		CreatedAt:   u.CreatedAt,
		LastLoginAt: u.LastLoginAt,
	}
}

// Session 对应 sessions 表。
type Session struct {
	ID         string    `json:"id"`
	UserID     string    `json:"user_id"`
	ExpiresAt  time.Time `json:"expires_at"`
	CreatedAt  time.Time `json:"created_at"`
	LastSeenAt time.Time `json:"last_seen_at"`
}

// Message 对应 messages 表。
type Message struct {
	ID          string    `json:"id"`
	RoomID      string    `json:"room_id"`
	SenderID    string    `json:"sender_id"`
	SenderName  string    `json:"sender_name"`
	MessageType string    `json:"message_type"`
	Content     string    `json:"content"`
	FileID      string    `json:"file_id,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

// File 对应 files 表。
type File struct {
	ID           string     `json:"id"`
	OriginalName string     `json:"original_name"`
	StoredName   string     `json:"-"`
	Path         string     `json:"-"`
	MimeType     string     `json:"mime_type"`
	Size         int64      `json:"size"`
	UploaderID   string     `json:"uploader_id"`
	CreatedAt    time.Time  `json:"created_at"`
	DeletedAt    *time.Time `json:"-"`
}

// FileView 是对外安全的文件视图。
type FileView struct {
	ID           string    `json:"id"`
	OriginalName string    `json:"original_name"`
	MimeType     string    `json:"mime_type"`
	Size         int64     `json:"size"`
	UploaderID   string    `json:"uploader_id"`
	CreatedAt    time.Time `json:"created_at"`
	URL          string    `json:"url"`
	DownloadURL  string    `json:"download_url"`
}

// ToView 转换为对外视图。
func (f *File) ToView() FileView {
	return FileView{
		ID:           f.ID,
		OriginalName: f.OriginalName,
		MimeType:     f.MimeType,
		Size:         f.Size,
		UploaderID:   f.UploaderID,
		CreatedAt:    f.CreatedAt,
		URL:          "/api/files/" + f.ID,
		DownloadURL:  "/api/files/" + f.ID + "/download",
	}
}

// CleanupTask 对应 cleanup_tasks 表，用于文件删除失败重试。
type CleanupTask struct {
	ID         string     `json:"id"`
	FileID     string     `json:"file_id"`
	Status     string     `json:"status"`
	RetryCount int        `json:"retry_count"`
	LastError  string     `json:"last_error"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}
