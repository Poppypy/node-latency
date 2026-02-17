package parser

import (
	"net/url"
	"strings"
)

// ParseQueryKeepPlus 自己解析 query：使用 PathUnescape（不会把 + 当空格），避免 pbk 被破坏
func ParseQueryKeepPlus(rawQuery string) map[string]string {
	out := map[string]string{}
	rawQuery = strings.TrimSpace(rawQuery)
	if rawQuery == "" {
		return out
	}
	parts := strings.FieldsFunc(rawQuery, func(r rune) bool {
		return r == '&' || r == ';'
	})
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		kv := strings.SplitN(part, "=", 2)
		k := kv[0]
		v := ""
		if len(kv) == 2 {
			v = kv[1]
		}
		if uk, err := url.PathUnescape(k); err == nil {
			k = uk
		}
		if uv, err := url.PathUnescape(v); err == nil {
			v = uv
		}
		k = strings.ToLower(strings.TrimSpace(k))
		v = strings.TrimSpace(v)
		if k == "" {
			continue
		}
		out[k] = v
	}
	return out
}

func ParamGet(m map[string]string, keys ...string) string {
	if m == nil {
		return ""
	}
	for _, k := range keys {
		k = strings.ToLower(strings.TrimSpace(k))
		if k == "" {
			continue
		}
		if v, ok := m[k]; ok {
			if strings.TrimSpace(v) != "" {
				return v
			}
		}
	}
	return ""
}
