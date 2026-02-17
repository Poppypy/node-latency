package util

import (
	"bytes"
	"strings"
	"unicode"
)

func IsUnicodeNoncharacter(r rune) bool {
	if r >= 0xFDD0 && r <= 0xFDEF {
		return true
	}
	if r&0xFFFF == 0xFFFE || r&0xFFFF == 0xFFFF {
		return true
	}
	return false
}

func SanitizeString(s string) string {
	if s == "" {
		return s
	}
	s = strings.ToValidUTF8(s, " ")
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		switch {
		case r == '\u00a0':
			b.WriteRune(' ')
		case r == '\u200b' || r == '\u200c' || r == '\u200d' || r == '\ufeff':
		case r >= 0xD800 && r <= 0xDFFF:
			b.WriteRune(' ')
		case IsUnicodeNoncharacter(r):
			b.WriteRune(' ')
		case unicode.IsControl(r):
			b.WriteRune(' ')
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

func SanitizeTextForYAML(text string) string {
	if text == "" {
		return text
	}
	text = strings.ToValidUTF8(text, " ")
	var b strings.Builder
	b.Grow(len(text))

	skipNextLF := false
	for _, r := range text {
		if skipNextLF {
			skipNextLF = false
			if r == '\n' {
				continue
			}
		}
		switch {
		case r == '\u00a0':
			b.WriteRune(' ')
		case r == '\u200b' || r == '\u200c' || r == '\u200d' || r == '\ufeff':
		case r == '\r':
			b.WriteRune('\n')
			skipNextLF = true
		case r == '\n':
			b.WriteRune('\n')
		case r == '\t':
			b.WriteRune(' ')
		case r >= 0xD800 && r <= 0xDFFF:
			b.WriteRune(' ')
		case IsUnicodeNoncharacter(r):
			b.WriteRune(' ')
		case unicode.IsControl(r):
			b.WriteRune(' ')
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

func SanitizeYAMLOutput(data []byte) []byte {
	if len(data) == 0 {
		return data
	}
	clean := bytes.ToValidUTF8(data, []byte(" "))
	return []byte(SanitizeTextForYAML(string(clean)))
}

func SanitizeLineText(text string) string {
	if text == "" {
		return text
	}
	text = strings.ToValidUTF8(text, " ")
	var b strings.Builder
	b.Grow(len(text))
	for _, r := range text {
		switch {
		case r == '\u00a0':
			b.WriteRune(' ')
		case r == '\u200b' || r == '\u200c' || r == '\u200d' || r == '\ufeff':
		case r >= 0xD800 && r <= 0xDFFF:
			b.WriteRune(' ')
		case IsUnicodeNoncharacter(r):
			b.WriteRune(' ')
		case unicode.IsControl(r):
			b.WriteRune(' ')
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

func NormalizeTokenValue(raw string) string {
	v := strings.ToLower(strings.TrimSpace(SanitizeString(raw)))
	if v == "" {
		return ""
	}
	for i, r := range v {
		if !(unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' || r == '_') {
			return strings.TrimSpace(v[:i])
		}
	}
	return v
}

func CleanToken(s string) string {
	s = SanitizeString(s)
	s = strings.TrimSpace(s)
	s = strings.Trim(s, "\"'")
	s = strings.TrimSpace(s)
	return s
}
