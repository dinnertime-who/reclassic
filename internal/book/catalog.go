package book

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	gen "github.com/dinnertime/reclassic/internal/db/gen"
)

// CatalogItem은 공개 도서 목록의 한 행이다. published 프로젝트 하나와 그 원서.
type CatalogItem struct {
	GutenbergID int
	Title       string
	Author      string
	ProjectID   int64
	TargetLang  string
}

// NeedsReviewItem은 합본 게이트에 걸린 책이다. 챕터·문단 수가 있어야
// 화면이 ADR-014 임계값과 나란히 보여 줄 수 있다.
type NeedsReviewItem struct {
	GutenbergID    int
	Title          string
	Author         string
	ChapterCount   int
	ParagraphCount int
}

// OrphanItem은 확정 번역이 갈 곳을 잃은 승계 기록이다. 읽기만 한다.
type OrphanItem struct {
	GutenbergID int
	Title       string
	Orphaned    int
	CreatedAt   time.Time
}

// ListPublished는 도서 목록에 노출되는 행이다. published만 (ADR-036).
func (r *Reader) ListPublished(ctx context.Context) ([]CatalogItem, error) {
	rows, err := gen.New(r.pool).ListPublishedCatalog(ctx)
	if err != nil {
		return nil, fmt.Errorf("list published catalog: %w", err)
	}
	out := make([]CatalogItem, 0, len(rows))
	for _, row := range rows {
		out = append(out, CatalogItem{
			GutenbergID: int(row.GutenbergID),
			Title:       row.Title,
			Author:      textOrEmpty(row.Author),
			ProjectID:   row.ProjectID,
			TargetLang:  row.TargetLang,
		})
	}
	return out, nil
}

// ListNeedsReview는 books.status = needs_review 인 책이다 (D1).
func (r *Reader) ListNeedsReview(ctx context.Context) ([]NeedsReviewItem, error) {
	rows, err := gen.New(r.pool).ListBooks(ctx, pgtype.Text{String: StatusNeedsReview, Valid: true})
	if err != nil {
		return nil, fmt.Errorf("list needs_review books: %w", err)
	}
	out := make([]NeedsReviewItem, 0, len(rows))
	for _, row := range rows {
		out = append(out, NeedsReviewItem{
			GutenbergID:    int(row.GutenbergID),
			Title:          row.Title,
			Author:         textOrEmpty(row.Author),
			ChapterCount:   int(row.ChapterCount),
			ParagraphCount: int(row.ParagraphCount),
		})
	}
	return out, nil
}

// ListOrphans는 orphaned > 0 인 승계 기록이다 (D2). 고아를 되살리지 않는다.
func (r *Reader) ListOrphans(ctx context.Context) ([]OrphanItem, error) {
	rows, err := gen.New(r.pool).ListOrphanedSuccessions(ctx)
	if err != nil {
		return nil, fmt.Errorf("list orphaned successions: %w", err)
	}
	out := make([]OrphanItem, 0, len(rows))
	for _, row := range rows {
		out = append(out, OrphanItem{
			GutenbergID: int(row.GutenbergID),
			Title:       row.Title,
			Orphaned:    int(row.Orphaned),
			CreatedAt:   row.CreatedAt.Time,
		})
	}
	return out, nil
}

func textOrEmpty(t pgtype.Text) string {
	if !t.Valid {
		return ""
	}
	return t.String
}
