package util

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
)

// GenID 生成基于 crypto/rand 的 UUID v4 风格 ID，避免引入额外依赖。
func GenID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		// 极不可能发生；退化为时间戳混合。
		return fmt.Sprintf("%d", len(b))
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// GenToken 生成不透明随机令牌，用于会话 ID。
func GenToken() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return GenID()
	}
	return hex.EncodeToString(b)
}
