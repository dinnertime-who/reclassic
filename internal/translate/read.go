package translate

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	gen "github.com/dinnertime/reclassic/internal/db/gen"
)

// ChapterView는 번역 프로젝트의 챕터 한 페이지다.
type ChapterView struct {
	Idx           int
	Title         string
	TotalChapters int
	Coverage      Coverage
	Paragraphs    []ParagraphView
}

// ParagraphView는 문단 하나다.
// ApprovedTranslation이 비어 있으면 읽기 화면이 원문을 대신 보여준다 —
// 부분 공개를 허용한다. 100%를 기다리면 아무 책도 공개하지 못한다.
type ParagraphView struct {
	StableID            string
	SourceText          string
	ApprovedTranslation string
	ProposalCount       int
}

// Chapter는 프로젝트 기준으로 챕터 하나를 읽는다.
func (s *Service) Chapter(ctx context.Context, projectID int64, idx int) (*ChapterView, error) {
	q := gen.New(s.pool)

	row, err := q.GetProjectWithBook(ctx, projectID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get project %d: %w", projectID, err)
	}

	rev, err := q.GetActiveRevision(ctx, row.Book.GutenbergID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get active revision: %w", err)
	}

	chapter, err := q.GetChapter(ctx, gen.GetChapterParams{RevisionID: rev.ID, Idx: int32(idx)})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get chapter %d: %w", idx, err)
	}

	total, err := q.CountChapters(ctx, rev.ID)
	if err != nil {
		return nil, fmt.Errorf("count chapters: %w", err)
	}

	rows, err := q.ListChapterParagraphsWithTranslation(ctx,
		gen.ListChapterParagraphsWithTranslationParams{ChapterID: chapter.ID, ProjectID: projectID})
	if err != nil {
		return nil, fmt.Errorf("list paragraphs: %w", err)
	}

	view := &ChapterView{
		Idx:           int(chapter.Idx),
		Title:         chapter.Title,
		TotalChapters: int(total),
		Paragraphs:    make([]ParagraphView, 0, len(rows)),
	}
	for _, r := range rows {
		p := ParagraphView{
			StableID:      r.StableID,
			SourceText:    r.SourceText,
			ProposalCount: int(r.ProposalCount),
		}
		if r.ApprovedTranslation.Valid {
			p.ApprovedTranslation = r.ApprovedTranslation.String
			view.Coverage.Approved++
		}
		view.Coverage.Total++
		view.Paragraphs = append(view.Paragraphs, p)
	}
	return view, nil
}

// ProposalView는 제안 하나에 작성자 handle을 붙인 것이다.
type ProposalView struct {
	Proposal     gen.TranslationProposal
	AuthorHandle string
}

// Proposals는 문단 하나의 제안 목록이다. 작성자를 조인해서 가져온다 — N+1을 만들지 않는다.
func (s *Service) Proposals(ctx context.Context, projectID int64, stableID string) ([]ProposalView, error) {
	rows, err := gen.New(s.pool).ListProposals(ctx, gen.ListProposalsParams{
		ProjectID:         projectID,
		ParagraphStableID: stableID,
	})
	if err != nil {
		return nil, fmt.Errorf("list proposals: %w", err)
	}

	out := make([]ProposalView, 0, len(rows))
	for _, r := range rows {
		out = append(out, ProposalView{
			Proposal: gen.TranslationProposal{
				ID:                r.ID,
				ProjectID:         r.ProjectID,
				ParagraphStableID: r.ParagraphStableID,
				Text:              r.Text,
				AuthorID:          r.AuthorID,
				Status:            r.Status,
				ReviewedBy:        r.ReviewedBy,
				ReviewedAt:        r.ReviewedAt,
				ReviewNote:        r.ReviewNote,
				CreatedAt:         r.CreatedAt,
			},
			AuthorHandle: r.AuthorHandle,
		})
	}
	return out, nil
}

// UserByHandle은 임시 신원 헤더를 사용자로 바꾼다.
// 인증이 아니다 — 슬라이스 5에서 세션으로 대체한다.
func (s *Service) UserByHandle(ctx context.Context, handle string) (*gen.User, error) {
	u, err := gen.New(s.pool).GetUserByHandle(ctx, handle)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get user %q: %w", handle, err)
	}
	return &u, nil
}

// CreateProject는 번역 프로젝트를 만든다. 같은 책·언어면 기존 것을 돌려준다.
func (s *Service) CreateProject(ctx context.Context, gutenbergID int, targetLang string) (*gen.TranslationProject, error) {
	q := gen.New(s.pool)

	book, err := q.GetBookByGutenbergID(ctx, int32(gutenbergID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get book %d: %w", gutenbergID, err)
	}

	project, err := q.CreateProject(ctx, gen.CreateProjectParams{BookID: book.ID, TargetLang: targetLang})
	if err != nil {
		return nil, fmt.Errorf("create project: %w", err)
	}
	return &project, nil
}
