package parse

import (
	"crypto/sha256"
	"encoding/hex"
	"html"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// Normalize는 문단 텍스트를 안정적인 해시 입력으로 만든다.
// 순서를 바꾸면 기존 stable_id가 전부 깨진다 (ARCHITECTURE 불변식 1).
func Normalize(s string) string {
	s = html.UnescapeString(s)
	s = norm.NFC.String(s)
	s = replaceQuotes(s)
	s = replaceDashes(s)
	s = collapseSpace(s)
	return strings.TrimSpace(s)
}

// StableID는 이미 정규화된 텍스트의 sha256 앞 16hex다.
func StableID(normalized string) string {
	sum := sha256.Sum256([]byte(normalized))
	return hex.EncodeToString(sum[:])[:16]
}

func replaceQuotes(s string) string {
	replacer := strings.NewReplacer(
		"\u2018", "'", // ‘
		"\u2019", "'", // ’
		"\u201A", "'", // ‚
		"\u201B", "'", // ‛
		"\u201C", "\"", // “
		"\u201D", "\"", // ”
		"\u201E", "\"", // „
		"\u201F", "\"", // ‟
	)
	return replacer.Replace(s)
}

func replaceDashes(s string) string {
	replacer := strings.NewReplacer(
		"\u2014", "-", // —
		"\u2013", "-", // –
		"\u2010", "-", // ‐
		"\u2011", "-", // non-breaking hyphen
		"\u2012", "-", // figure dash
		"\u2212", "-", // minus
	)
	return replacer.Replace(s)
}

func collapseSpace(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	prevSpace := false
	for _, r := range s {
		if unicode.IsSpace(r) {
			if !prevSpace {
				b.WriteByte(' ')
				prevSpace = true
			}
			continue
		}
		b.WriteRune(r)
		prevSpace = false
	}
	return b.String()
}
