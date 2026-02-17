package parser

import (
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"node-latency/internal/util"
)

// NormalizeRealityPublicKey Reality public-key：必须能 base64 解码成 32 字节（X25519 公钥）
func NormalizeRealityPublicKey(raw string) (string, error) {
	s := util.CleanToken(raw)
	if s == "" {
		return "", errors.New("empty public-key")
	}

	// 常见坑：某些解析把 + -> 空格；public-key 里不该有空白，直接去掉/修复
	if strings.ContainsAny(s, " \t\r\n") {
		s2 := strings.ReplaceAll(s, " ", "+")
		s2 = strings.ReplaceAll(s2, "\t", "")
		s2 = strings.ReplaceAll(s2, "\r", "")
		s2 = strings.ReplaceAll(s2, "\n", "")
		s = s2
	}

	b, err := util.DecodeBase64(s)
	if err != nil {
		return "", err
	}
	if len(b) != 32 {
		return "", fmt.Errorf("public-key bytes != 32 (got %d)", len(b))
	}

	// 归一化输出为 URL-safe Raw Base64（mihomo 文档示例风格）
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// NormalizeRealityShortID Reality short-id：hex 串，偶数长度，通常 <=16字节（这里放宽到 <=16字节=32 hex）
func NormalizeRealityShortID(raw string) (string, error) {
	s := util.CleanToken(raw)
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.TrimPrefix(s, "0x")
	if s == "" {
		return "", nil
	}
	if len(s)%2 == 1 {
		return "", errors.New("short-id length must be even")
	}
	if len(s) > 32 {
		return "", errors.New("short-id too long (>32 hex)")
	}
	for _, r := range s {
		if r < '0' || r > 'f' || (r > '9' && r < 'a') {
			return "", errors.New("short-id must be hex")
		}
	}
	return s, nil
}
