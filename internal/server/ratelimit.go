package server

import (
	"sync"
	"time"
)

// RateLimiter 简单的滑动窗口限流，防止消息刷屏（PRD 第 8 节）。
type RateLimiter struct {
	mu      sync.Mutex
	counts  map[string][]time.Time
	limit   int
	window  time.Duration
}

// NewRateLimiter 创建限流器的默认实例：单用户 10 秒内最多 30 条消息。
func NewRateLimiter() *RateLimiter {
	return &RateLimiter{
		counts: make(map[string][]time.Time),
		limit:  30,
		window: 10 * time.Second,
	}
}

// Allow 判断是否放行；超限返回 false。
func (r *RateLimiter) Allow(userID string) bool {
	now := time.Now()
	r.mu.Lock()
	defer r.mu.Unlock()
	times := r.counts[userID]
	kept := times[:0]
	for _, t := range times {
		if now.Sub(t) < r.window {
			kept = append(kept, t)
		}
	}
	if len(kept) >= r.limit {
		r.counts[userID] = kept
		return false
	}
	r.counts[userID] = append(kept, now)
	return true
}
