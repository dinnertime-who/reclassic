package translate

import (
	"context"
	"fmt"

	gen "github.com/dinnertime/reclassic/internal/db/gen"
)

// ChapterProgress는 프로젝트 안 챕터 하나의 진행률이다.
// 사이트맵 잡이 색인 대상을 고르는 데 쓰고, 목차 화면이 같은 숫자를 그린다.
// 두 곳이 다른 쿼리를 쓰면 "색인은 됐는데 목차는 79%"처럼 갈라진다 (ADR-007 + ADR-023).
type ChapterProgress struct {
	Idx      int
	Title    string
	Coverage Coverage
}

// ChapterProgress는 프로젝트의 챕터별 진행률을 한 번에 읽는다.
func (s *Service) ChapterProgress(ctx context.Context, projectID int64) ([]ChapterProgress, error) {
	rows, err := gen.New(s.pool).ListProjectChapterCoverage(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("list chapter coverage %d: %w", projectID, err)
	}
	out := make([]ChapterProgress, 0, len(rows))
	for _, r := range rows {
		out = append(out, ChapterProgress{
			Idx:      int(r.Idx),
			Title:    r.Title,
			Coverage: Coverage{Total: int(r.Total), Approved: int(r.Approved)},
		})
	}
	return out, nil
}
