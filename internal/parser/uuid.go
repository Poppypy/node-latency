package parser

import (
	"errors"
	"fmt"
	"net/url"
	"strings"

	"node-latency/internal/util"
)

func IsHexChar(r rune) bool {
	return (r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')
}

func IsHexString(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if !IsHexChar(r) {
			return false
		}
	}
	return true
}

// NormalizeUUID 统一 UUID：允许 32hex 或 36（带 -），统一输出 36 小写
func NormalizeUUID(raw string) (string, error) {
	s := util.CleanToken(raw)
	s = strings.TrimSpace(s)
	if s == "" {
		return "", errors.New("empty uuid")
	}
	s = strings.TrimPrefix(strings.ToLower(s), "urn:uuid:")
	s = strings.TrimPrefix(strings.ToLower(s), "uuid:")
	s = strings.Trim(s, "{}")
	s = strings.TrimSpace(s)

	// userinfo 里可能有 %xx（极少数订阅会这样）；尝试解码 2 次
	for i := 0; i < 2 && strings.Contains(s, "%"); i++ {
		if dec, err := url.PathUnescape(s); err == nil && dec != s {
			s = dec
		} else {
			break
		}
	}

	s = strings.ToLower(strings.TrimSpace(s))

	if len(s) == 32 && IsHexString(s) {
		return fmt.Sprintf("%s-%s-%s-%s-%s",
			s[0:8], s[8:12], s[12:16], s[16:20], s[20:32]), nil
	}
	if len(s) != 36 {
		return "", errors.New("invalid uuid length")
	}
	if s[8] != '-' || s[13] != '-' || s[18] != '-' || s[23] != '-' {
		return "", errors.New("invalid uuid hyphens")
	}
	for i, ch := range s {
		if i == 8 || i == 13 || i == 18 || i == 23 {
			continue
		}
		if !IsHexChar(ch) {
			return "", errors.New("invalid uuid char")
		}
	}
	return s, nil
}
