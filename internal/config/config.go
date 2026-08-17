package config

import (
	"os"
	"strconv"
	"time"

	"gopkg.in/yaml.v3"
)

// Config 是整个服务的配置聚合。
type Config struct {
	Server   ServerConfig   `yaml:"server"`
	Database DatabaseConfig `yaml:"database"`
	Storage  StorageConfig  `yaml:"storage"`
	Chat     ChatConfig     `yaml:"chat"`
	Session  SessionConfig  `yaml:"session"`
	Security SecurityConfig `yaml:"security"`
	Log      LogConfig      `yaml:"log"`

	// 以下两项仅来自环境变量，不写入配置文件。
	AdminUsername string `yaml:"-"`
	AdminPassword string `yaml:"-"`
}

type ServerConfig struct {
	Host string `yaml:"host"`
	Port int    `yaml:"port"`
}

type DatabaseConfig struct {
	Path string `yaml:"path"`
}

type StorageConfig struct {
	UploadDir     string `yaml:"upload_dir"`
	MaxImageSize  int64  `yaml:"max_image_size"`
	MaxFileSize   int64  `yaml:"max_file_size"`
}

type ChatConfig struct {
	MessageRetentionHours int `yaml:"message_retention_hours"`
}

type SessionConfig struct {
	ExpirationHours int `yaml:"expiration_hours"`
}

type SecurityConfig struct {
	CookieSecure bool `yaml:"cookie_secure"`
}

type LogConfig struct {
	Level  string `yaml:"level"`
	Format string `yaml:"format"` // json | text
}

// Default 返回内置默认值。
func Default() *Config {
	return &Config{
		Server:   ServerConfig{Host: "0.0.0.0", Port: 8080},
		Database: DatabaseConfig{Path: "./data/chat.db"},
		Storage: StorageConfig{
			UploadDir:    "./data/uploads",
			MaxImageSize: 10 * 1024 * 1024,
			MaxFileSize:  50 * 1024 * 1024,
		},
		Chat:    ChatConfig{MessageRetentionHours: 4},
		Session: SessionConfig{ExpirationHours: 24},
		Security: SecurityConfig{CookieSecure: false},
		Log:     LogConfig{Level: "info", Format: "json"},
	}
}

// Load 读取 YAML 配置（文件可不存在），再用环境变量覆盖。
// 生产环境通过环境变量覆盖关键配置（见 README）。
func Load(path string) (*Config, error) {
	cfg := Default()
	if path != "" {
		if data, err := os.ReadFile(path); err == nil {
			if err := yaml.Unmarshal(data, cfg); err != nil {
				return nil, err
			}
		}
	}
	applyEnvOverrides(cfg)
	return cfg, nil
}

func applyEnvOverrides(cfg *Config) {
	if v := os.Getenv("SERVER_HOST"); v != "" {
		cfg.Server.Host = v
	}
	if v := os.Getenv("SERVER_PORT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.Server.Port = n
		}
	}
	if v := os.Getenv("DATABASE_PATH"); v != "" {
		cfg.Database.Path = v
	}
	if v := os.Getenv("STORAGE_UPLOAD_DIR"); v != "" {
		cfg.Storage.UploadDir = v
	}
	if v := os.Getenv("STORAGE_MAX_IMAGE_SIZE"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			cfg.Storage.MaxImageSize = n
		}
	}
	if v := os.Getenv("STORAGE_MAX_FILE_SIZE"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			cfg.Storage.MaxFileSize = n
		}
	}
	if v := os.Getenv("CHAT_RETENTION_HOURS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.Chat.MessageRetentionHours = n
		}
	}
	if v := os.Getenv("SESSION_EXPIRATION_HOURS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.Session.ExpirationHours = n
		}
	}
	if v := os.Getenv("SECURITY_COOKIE_SECURE"); v != "" {
		cfg.Security.CookieSecure = v == "true" || v == "1"
	}
	cfg.AdminUsername = os.Getenv("ADMIN_USERNAME")
	cfg.AdminPassword = os.Getenv("ADMIN_PASSWORD")
}

// Addr 返回监听地址。
func (c *Config) Addr() string {
	return c.Server.Host + ":" + strconv.Itoa(c.Server.Port)
}

// SessionTTL 返回 Session 有效时长。
func (c *Config) SessionTTL() time.Duration {
	return time.Duration(c.Session.ExpirationHours) * time.Hour
}

// Retention 返回消息保留时长。
func (c *Config) Retention() time.Duration {
	return time.Duration(c.Chat.MessageRetentionHours) * time.Hour
}
