package translate

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	gen "github.com/dinnertime/reclassic/internal/db/gen"
)

// ContentsView는 번역 프로젝트의 목차 한 화면이다.
//
// 책 머리말(제목·저자)과 장 목록을 같이 담는다. 화면이 따로 부르면
// 목차 하나를 그리는 데 요청이 둘이 된다 — 읽기 화면이 챕터·문단·번역을
// 한 번에 조인해 받는 것과 같은 이유다 (ARCHITECTURE "API 계약").
type ContentsView struct {
	Title      string
	Author     string
	TargetLang string
	// Progress는 장별 진행도의 합이다. 화면이 다시 더하지 않는다 —
	// 두 곳에서 계산하면 머리말과 목록이 갈라진다.
	Progress Coverage
	Chapters []ChapterProgress
}

// Contents는 프로젝트의 장 목록을 진행도와 함께 읽는다.
//
// 활성 revision이 없으면 ErrNotFound다. 장 목록 쿼리만으로는 "장이 0개"와
// 구분되지 않는데, 게이트에 걸려 읽을 수 없는 책을 빈 목차로 보여 주면
// 화면이 "장이 없는 책"이라고 거짓말을 한다.
func (s *Service) Contents(ctx context.Context, projectID int64) (*ContentsView, error) {
	q := gen.New(s.pool)

	row, err := q.GetProjectWithBook(ctx, projectID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get project %d: %w", projectID, err)
	}

	if _, err := q.GetActiveRevision(ctx, row.Book.GutenbergID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get active revision: %w", err)
	}

	chapters, err := s.ChapterProgress(ctx, projectID)
	if err != nil {
		return nil, err
	}

	view := &ContentsView{
		Title:      row.Book.Title,
		Author:     textOrEmpty(row.Book.Author),
		TargetLang: row.TranslationProject.TargetLang,
		Chapters:   chapters,
	}
	for _, c := range chapters {
		view.Progress.Total += c.Coverage.Total
		view.Progress.Approved += c.Coverage.Approved
	}
	return view, nil
}
