package parse

import (
	"regexp"
	"strings"
)

var (
	startMarker = regexp.MustCompile(`(?i)\*{2,}\s*START OF (?:THE|THIS) PROJECT GUTENBERG EBOOK[^*]*\*{2,}`)
	endMarker   = regexp.MustCompile(`(?i)\*{2,}\s*END OF (?:THE|THIS) PROJECT GUTENBERG EBOOK[^*]*\*{2,}`)
)

// StripBoilerplate는 Gutenberg 라이선스 마커 사이의 본문만 남긴다.
// 마커가 없으면 원문을 그대로 반환하고 found=false.
func StripBoilerplate(raw string) (body string, foundStart, foundEnd bool) {
	start := startMarker.FindStringIndex(raw)
	end := endMarker.FindStringIndex(raw)

	from := 0
	to := len(raw)
	if start != nil {
		foundStart = true
		from = start[1]
	}
	if end != nil {
		foundEnd = true
		to = end[0]
	}
	if to < from {
		return raw, foundStart, foundEnd
	}
	return strings.TrimSpace(raw[from:to]), foundStart, foundEnd
}
