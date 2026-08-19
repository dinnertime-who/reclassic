package book

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	gen "github.com/dinnertime/reclassic/internal/db/gen"
	"github.com/dinnertime/reclassic/internal/parse"
)

// Succession은 저장된 활성 revision과 지금 파서의 결과를 stable_id로 대조한 결과다.
//
// ADR-004는 "revision 전환 시 해시 일치 → 자동 승계"라고 정했지만,
// 파서를 실제로 고쳤을 때 몇 퍼센트가 일치하는지는 측정된 적이 없다.
// 번역이 쌓인 뒤에 알면 늦다.
type Succession struct {
	GutenbergID int
	Title       string
	Stored      int // 저장된 문단 수
	Current     int // 지금 파서가 뽑은 문단 수
	Matched     int // 양쪽에 다 있는 stable_id
	Added       int // 지금 파서에만 있는 것 — 미번역으로 남을 문단
	Lost        int // 저장본에만 있는 것 — 번역이 붙어 있었다면 갈 곳을 잃는다
}

// Rate는 저장된 문단 중 승계되는 비율이다. 저장본이 비었으면 0이다.
func (s Succession) Rate() float64 {
	if s.Stored == 0 {
		return 0
	}
	return float64(s.Matched) / float64(s.Stored)
}

// MeasureSuccession은 DB에 쓰지 않는다. 읽기 전용이다.
//
// 파서를 안 고쳤으면 100%가 나와야 한다. 파서를 고친 뒤 이걸 돌려
// 승계율이 얼마나 떨어지는지 보는 것이 사용법이다.
// 낮게 나오는 것도 유효한 결과다 — 수치를 맞추려고 매칭 기준을 느슨하게 하지 말 것.
func MeasureSuccession(ctx context.Context, pool *pgxpool.Pool, gutenbergID int, title string, html []byte) (*Succession, error) {
	q := gen.New(pool)

	rev, err := q.GetActiveRevision(ctx, int32(gutenbergID))
	if err != nil {
		if isNoRows(err) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get active revision %d: %w", gutenbergID, err)
	}

	storedIDs, err := q.ListStableIDs(ctx, rev.ID)
	if err != nil {
		return nil, fmt.Errorf("list stable ids %d: %w", gutenbergID, err)
	}

	ev, err := parse.EvaluateHTML(html, nil)
	if err != nil {
		return nil, fmt.Errorf("reparse %d: %w", gutenbergID, err)
	}

	return diffStableIDs(gutenbergID, title, storedIDs, currentStableIDs(ev.Best.Result)), nil
}

func currentStableIDs(res *parse.Result) []string {
	var out []string
	if res == nil {
		return out
	}
	for _, ch := range res.Chapters {
		for _, p := range ch.Paragraphs {
			out = append(out, p.StableID)
		}
	}
	return out
}

// diffStableIDs는 DB 없이 테스트할 수 있도록 집합 연산만 한다.
func diffStableIDs(gutenbergID int, title string, stored, current []string) *Succession {
	storedSet := make(map[string]struct{}, len(stored))
	for _, id := range stored {
		storedSet[id] = struct{}{}
	}
	currentSet := make(map[string]struct{}, len(current))
	for _, id := range current {
		currentSet[id] = struct{}{}
	}

	s := &Succession{
		GutenbergID: gutenbergID,
		Title:       title,
		Stored:      len(stored),
		Current:     len(current),
	}
	for id := range storedSet {
		if _, ok := currentSet[id]; ok {
			s.Matched++
		} else {
			s.Lost++
		}
	}
	for id := range currentSet {
		if _, ok := storedSet[id]; !ok {
			s.Added++
		}
	}
	return s
}
