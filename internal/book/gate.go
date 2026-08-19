// Package book은 도서 도메인 — 파서 결과 적재와 읽기 조회를 담는다.
package book

import (
	"fmt"

	"github.com/dinnertime/reclassic/internal/parse"
)

// Status는 books.status의 값이다. 마이그레이션의 CHECK 제약과 같아야 한다.
const (
	StatusPending     = "pending"
	StatusReady       = "ready"
	StatusNeedsReview = "needs_review"
	StatusFailed      = "failed"
)

// 게이트 임계값.
//
// 크기 기준은 ADR-014다. 코퍼스 22권에서 깨끗하게 갈린다 —
// 셰익스피어 전집만 걸리고(842챕터 39,355문단) 2위는 145챕터 / 7,064문단이다.
// 하드 차단이 아니라 관리자 확인 큐로 보낸다. 나중에 작품 단위 분할을 붙일 때
// 차단해 뒀으면 되돌리기 번거롭다.
//
// 신뢰도 기준은 ARCHITECTURE "파서 전략 체인"의 0.85다.
const (
	MaxChapters   = 200
	MaxParagraphs = 15000
	MinConfidence = 0.85
)

// GateResult는 적재를 자동 확정할지 관리자 확인 큐로 보낼지의 판정이다.
type GateResult struct {
	Passed  bool
	Reasons []string
}

// Gate는 파싱 결과만 보고 판정한다. DB가 필요 없어 단위 테스트가 가능하다.
func Gate(chapters, paragraphs int, confidence float64) GateResult {
	var reasons []string

	if chapters > MaxChapters {
		reasons = append(reasons, fmt.Sprintf("챕터 %d개 — 합본 의심 (기준 %d 초과, ADR-014)", chapters, MaxChapters))
	}
	if paragraphs > MaxParagraphs {
		reasons = append(reasons, fmt.Sprintf("문단 %d개 — 합본 의심 (기준 %d 초과, ADR-014)", paragraphs, MaxParagraphs))
	}
	if confidence < MinConfidence {
		reasons = append(reasons, fmt.Sprintf("신뢰도 %.3f — 자동 확정 기준 %.2f 미만", confidence, MinConfidence))
	}

	return GateResult{Passed: len(reasons) == 0, Reasons: reasons}
}

// Counts는 추출 결과의 챕터·문단 수를 센다.
func Counts(res *parse.Result) (chapters, paragraphs int) {
	if res == nil {
		return 0, 0
	}
	chapters = len(res.Chapters)
	for _, ch := range res.Chapters {
		paragraphs += len(ch.Paragraphs)
	}
	return chapters, paragraphs
}
