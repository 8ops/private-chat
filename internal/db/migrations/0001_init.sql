-- Private Chat 初始化迁移
-- SQLite 数据库结构。V1 使用 SQLite，WAL 模式在代码中启用。

CREATE TABLE IF NOT EXISTS users (
    id            TEXT PRIMARY KEY,
    username      TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    nickname      TEXT NOT NULL DEFAULT '',
    role          TEXT NOT NULL DEFAULT 'user',
    enabled       INTEGER NOT NULL DEFAULT 1,
    created_at    DATETIME NOT NULL,
    updated_at    DATETIME NOT NULL,
    last_login_at DATETIME
);

CREATE TABLE IF NOT EXISTS sessions (
    id          TEXT PRIMARY KEY,
    user_id     TEXT NOT NULL,
    expires_at  DATETIME NOT NULL,
    created_at  DATETIME NOT NULL,
    last_seen_at DATETIME NOT NULL,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS messages (
    id           TEXT PRIMARY KEY,
    room_id      TEXT NOT NULL,
    sender_id    TEXT NOT NULL,
    message_type TEXT NOT NULL,
    content      TEXT NOT NULL DEFAULT '',
    file_id      TEXT NOT NULL DEFAULT '',
    created_at   DATETIME NOT NULL
);

CREATE TABLE IF NOT EXISTS files (
    id            TEXT PRIMARY KEY,
    original_name TEXT NOT NULL,
    stored_name   TEXT NOT NULL,
    path          TEXT NOT NULL,
    mime_type     TEXT NOT NULL,
    size          INTEGER NOT NULL,
    uploader_id   TEXT NOT NULL,
    created_at    DATETIME NOT NULL,
    deleted_at    DATETIME
);

CREATE TABLE IF NOT EXISTS cleanup_tasks (
    id          TEXT PRIMARY KEY,
    file_id     TEXT NOT NULL,
    status      TEXT NOT NULL DEFAULT 'pending',
    retry_count INTEGER NOT NULL DEFAULT 0,
    last_error  TEXT NOT NULL DEFAULT '',
    created_at  DATETIME NOT NULL,
    updated_at  DATETIME NOT NULL
);

-- 索引（PRD 第 19 节要求）
CREATE INDEX IF NOT EXISTS idx_messages_created_at ON messages(created_at);
CREATE INDEX IF NOT EXISTS idx_messages_room_created ON messages(room_id, created_at);
CREATE INDEX IF NOT EXISTS idx_sessions_expires_at ON sessions(expires_at);
CREATE INDEX IF NOT EXISTS idx_files_created_at ON files(created_at);
CREATE INDEX IF NOT EXISTS idx_cleanup_status ON cleanup_tasks(status);
