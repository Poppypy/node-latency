package util

import (
	"encoding/base64"
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"
)

func GetString(m map[string]interface{}, key string) string {
	v, ok := m[key]
	if !ok || v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return t
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	case int:
		return strconv.Itoa(t)
	default:
		return fmt.Sprintf("%v", t)
	}
}

func GetStringFromMap(m map[string]interface{}, key string) string {
	if m == nil {
		return ""
	}
	v, ok := m[key]
	if !ok || v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return t
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	case int:
		return strconv.Itoa(t)
	case fmt.Stringer:
		return t.String()
	default:
		return fmt.Sprint(t)
	}
}

func GetIntFromMap(m map[string]interface{}, key string) (int, error) {
	if m == nil {
		return 0, errors.New("缺少字段")
	}
	v, ok := m[key]
	if !ok || v == nil {
		return 0, errors.New("缺少端口")
	}
	switch t := v.(type) {
	case int:
		return t, nil
	case int64:
		return int(t), nil
	case float64:
		return int(t), nil
	case string:
		return ParsePort(t)
	default:
		return 0, errors.New("端口格式错误")
	}
}

func GetBoolFromMap(m map[string]interface{}, key string) (bool, bool) {
	if m == nil {
		return false, false
	}
	v, ok := m[key]
	if !ok || v == nil {
		return false, false
	}
	switch t := v.(type) {
	case bool:
		return t, true
	case string:
		return ParseBoolStr(t)
	default:
		return false, false
	}
}

func GetIntFromMapDefault(m map[string]interface{}, key string, def int) int {
	if m == nil {
		return def
	}
	v, ok := m[key]
	if !ok || v == nil {
		return def
	}
	switch t := v.(type) {
	case int:
		return t
	case int64:
		return int(t)
	case float64:
		return int(t)
	case string:
		if i, err := strconv.Atoi(t); err == nil {
			return i
		}
		return def
	default:
		return def
	}
}

func GetStringMap(m map[string]interface{}, key string) (map[string]string, bool) {
	if m == nil {
		return nil, false
	}
	v, ok := m[key]
	if !ok || v == nil {
		return nil, false
	}
	switch t := v.(type) {
	case map[string]string:
		return t, true
	case map[string]interface{}:
		out := make(map[string]string, len(t))
		for k, val := range t {
			if s, ok := val.(string); ok {
				out[k] = s
			}
		}
		return out, true
	case map[interface{}]interface{}:
		out := make(map[string]string, len(t))
		for k, val := range t {
			if s, ok := val.(string); ok {
				out[fmt.Sprint(k)] = s
			}
		}
		return out, true
	default:
		return nil, false
	}
}

func ToSlice(v interface{}) ([]interface{}, bool) {
	if v == nil {
		return nil, false
	}
	switch t := v.(type) {
	case []interface{}:
		return t, true
	default:
		return nil, false
	}
}

func ToStringMap(v interface{}) (map[string]interface{}, bool) {
	switch m := v.(type) {
	case map[string]interface{}:
		return m, true
	case map[interface{}]interface{}:
		out := make(map[string]interface{}, len(m))
		for k, val := range m {
			out[fmt.Sprint(k)] = val
		}
		return out, true
	default:
		return nil, false
	}
}

func ToStringSlice(v interface{}) ([]string, bool) {
	if v == nil {
		return nil, false
	}
	switch t := v.(type) {
	case []string:
		return t, true
	case []interface{}:
		out := make([]string, 0, len(t))
		for _, item := range t {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		return out, true
	default:
		return nil, false
	}
}

func CloneStringMap(v map[string]interface{}) (map[string]interface{}, bool) {
	cloned := CloneValue(v)
	out, ok := cloned.(map[string]interface{})
	return out, ok
}

func CloneValue(v interface{}) interface{} {
	switch t := v.(type) {
	case map[string]interface{}:
		out := make(map[string]interface{}, len(t))
		for k, val := range t {
			out[k] = CloneValue(val)
		}
		return out
	case map[interface{}]interface{}:
		out := make(map[string]interface{}, len(t))
		for k, val := range t {
			out[fmt.Sprint(k)] = CloneValue(val)
		}
		return out
	case []interface{}:
		out := make([]interface{}, 0, len(t))
		for _, val := range t {
			out = append(out, CloneValue(val))
		}
		return out
	case []byte:
		copied := make([]byte, len(t))
		copy(copied, t)
		return copied
	default:
		return t
	}
}

func SanitizeYAMLMap(root map[string]interface{}) map[string]interface{} {
	clean := SanitizeYAMLValue(root)
	if m, ok := clean.(map[string]interface{}); ok {
		return m
	}
	return root
}

func SanitizeYAMLValue(v interface{}) interface{} {
	if v == nil {
		return nil
	}
	switch t := v.(type) {
	case map[string]interface{}:
		out := make(map[string]interface{}, len(t))
		for k, val := range t {
			out[SanitizeString(k)] = SanitizeYAMLValue(val)
		}
		return out
	case map[interface{}]interface{}:
		out := make(map[string]interface{}, len(t))
		for k, val := range t {
			out[SanitizeString(fmt.Sprint(k))] = SanitizeYAMLValue(val)
		}
		return out
	case map[string]string:
		out := make(map[string]interface{}, len(t))
		for k, val := range t {
			out[SanitizeString(k)] = SanitizeString(val)
		}
		return out
	case []interface{}:
		out := make([]interface{}, 0, len(t))
		for _, val := range t {
			out = append(out, SanitizeYAMLValue(val))
		}
		return out
	case []string:
		out := make([]interface{}, 0, len(t))
		for _, s := range t {
			out = append(out, SanitizeString(s))
		}
		return out
	case []byte:
		return SanitizeString(string(t))
	case string:
		return SanitizeString(t)
	default:
		rv := reflect.ValueOf(v)
		if !rv.IsValid() {
			return v
		}
		for rv.Kind() == reflect.Interface || rv.Kind() == reflect.Pointer {
			if rv.IsNil() {
				return v
			}
			rv = rv.Elem()
		}
		switch rv.Kind() {
		case reflect.Slice, reflect.Array:
			n := rv.Len()
			out := make([]interface{}, 0, n)
			for i := 0; i < n; i++ {
				out = append(out, SanitizeYAMLValue(rv.Index(i).Interface()))
			}
			return out
		case reflect.Map:
			out := make(map[string]interface{}, rv.Len())
			for _, k := range rv.MapKeys() {
				ks := SanitizeString(fmt.Sprint(k.Interface()))
				out[ks] = SanitizeYAMLValue(rv.MapIndex(k).Interface())
			}
			return out
		default:
			return v
		}
	}
}

func SplitCSV(v string) []string {
	parts := strings.FieldsFunc(v, func(r rune) bool {
		return r == ',' || r == ' ' || r == ';'
	})
	var out []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func ParseBoolStr(v string) (bool, bool) {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "yes", "y", "on":
		return true, true
	case "0", "false", "no", "n", "off":
		return false, true
	default:
		return false, false
	}
}

func FirstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func ParsePort(s string) (int, error) {
	if s == "" {
		return 0, errors.New("empty port")
	}
	if p, err := strconv.Atoi(s); err == nil {
		return p, nil
	}
	if f, err := strconv.ParseFloat(s, 64); err == nil {
		return int(f), nil
	}
	return 0, errors.New("invalid port")
}

func DecodeBase64(s string) ([]byte, error) {
	if b, err := base64.RawStdEncoding.DecodeString(s); err == nil {
		return b, nil
	}
	if b, err := base64.StdEncoding.DecodeString(s); err == nil {
		return b, nil
	}
	if b, err := base64.RawURLEncoding.DecodeString(s); err == nil {
		return b, nil
	}
	if b, err := base64.URLEncoding.DecodeString(s); err == nil {
		return b, nil
	}
	return nil, errors.New("invalid base64")
}
