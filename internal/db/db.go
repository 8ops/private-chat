package db

import (
	"database/sql"
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// Open 打开 SQLite 数据库，启用 WAL 与连接池，并执行迁移。
// 使用 modernc.org/sqlite（纯 Go，无 CGO），便于 Docker 跨平台构建。
func Open(dbPath string) (*sql.DB, error) {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		return nil, fmt.Errorf("create db dir: %w", err)
	}

	dsn := fmt.Sprintf("file:%s?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(ON)&_pragma=synchronous(NORMAL)", dbPath)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	// 连接池（SQLite 单写者，读连接可多开）。
	db.SetMaxOpenConns(4)
	db.SetMaxIdleConns(2)

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}

	if err := migrate(db); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return db, nil
}

func migrate(db *sql.DB) error {
	entries, err := migrationsFS.ReadDir("migrations")
	if err != nil {
		return err
	}
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".sql") {
			continue
		}
		data, err := migrationsFS.ReadFile("migrations/" + e.Name())
		if err != nil {
			return err
		}
		// 逐条执行，忽略 "IF NOT EXISTS" 重复。
		if _, err := db.Exec(string(data)); err != nil {
			return fmt.Errorf("exec %s: %w", e.Name(), err)
		}
	}
	return nil
}
