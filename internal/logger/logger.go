package logger

import (
	"encoding/json"
	"log"
	"os"
	"sync"
	"time"
)

// Logger 是轻量 JSON 日志器，满足 PRD 第 25 节要求。
// 禁止记录密码、Session Token、Cookie 完整值。
type Logger struct {
	mu     sync.Mutex
	out    *log.Logger
	level  Level
	format string
}

type Level int

const (
	LevelDebug Level = iota
	LevelInfo
	LevelWarn
	LevelError
)

func (l Level) String() string {
	switch l {
	case LevelDebug:
		return "debug"
	case LevelInfo:
		return "info"
	case LevelWarn:
		return "warn"
	case LevelError:
		return "error"
	default:
		return "info"
	}
}

var defaultLogger = New("info", "json")

// New 创建日志器。
func New(level, format string) *Logger {
	lv := LevelInfo
	switch level {
	case "debug":
		lv = LevelDebug
	case "warn", "warning":
		lv = LevelWarn
	case "error":
		lv = LevelError
	}
	f := format
	if f != "json" {
		f = "text"
	}
	return &Logger{
		out:    log.New(os.Stdout, "", 0),
		level:  lv,
		format: f,
	}
}

// SetDefault 替换全局默认日志器。
func SetDefault(l *Logger) { defaultLogger = l }

func (l *Logger) enabled(level Level) bool { return level >= l.level }

func (l *Logger) write(level Level, msg string, fields map[string]interface{}) {
	if !l.enabled(level) {
		return
	}
	entry := map[string]interface{}{
		"ts":    time.Now().Format(time.RFC3339),
		"level": level.String(),
		"msg":   msg,
	}
	for k, v := range fields {
		entry[k] = v
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.format == "json" {
		if b, err := json.Marshal(entry); err == nil {
			l.out.Println(string(b))
			return
		}
	}
	// text 格式
	ts := entry["ts"].(string)
	l.out.Printf("%s [%s] %s %v\n", ts, level.String(), msg, fields)
}

func (l *Logger) Debug(msg string, fields ...map[string]interface{}) {
	l.write(LevelDebug, msg, merge(fields...))
}
func (l *Logger) Info(msg string, fields ...map[string]interface{}) {
	l.write(LevelInfo, msg, merge(fields...))
}
func (l *Logger) Warn(msg string, fields ...map[string]interface{}) {
	l.write(LevelWarn, msg, merge(fields...))
}
func (l *Logger) Error(msg string, fields ...map[string]interface{}) {
	l.write(LevelError, msg, merge(fields...))
}

func merge(fields ...map[string]interface{}) map[string]interface{} {
	out := map[string]interface{}{}
	for _, f := range fields {
		for k, v := range f {
			out[k] = v
		}
	}
	return out
}

// 包级便捷方法
func Debug(msg string, fields ...map[string]interface{}) { defaultLogger.Debug(msg, fields...) }
func Info(msg string, fields ...map[string]interface{})  { defaultLogger.Info(msg, fields...) }
func Warn(msg string, fields ...map[string]interface{})  { defaultLogger.Warn(msg, fields...) }
func Error(msg string, fields ...map[string]interface{}) { defaultLogger.Error(msg, fields...) }
