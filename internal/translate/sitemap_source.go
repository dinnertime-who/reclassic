package translate

import (
	"context"
	"fmt"

	gen "github.com/dinnertime/reclassic/internal/db/gen"
)

// ChapterProgress는 프로젝트 안 챕터 하나의 진행률이다.
// 사이트맵 잡이 색인 대상을 고르는 데 쓴다 (ADR-007 + ADR-023).
type ChapterProgress struct {
	Idx      int
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
			Coverage: Coverage{Total: int(r.Total), Approved: int(r.Approved)},
		})
	}
	return out, nil
}
