package book

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	gen "github.com/dinnertime/reclassic/internal/db/gen"
)

// ErrNotFound는 활성 revision이 없거나 그 안에 해당 챕터가 없다는 뜻이다.
// 둘을 구분하지 않는다 — 읽는 쪽에서 보면 둘 다 "그 주소에 읽을 것이 없다"이고,
// 어느 쪽인지 알려주면 needs_review인 책의 존재가 새어 나간다.
var ErrNotFound = errors.New("활성 revision 또는 챕터가 없다")

// ChapterView는 읽기 화면 한 페이지에 필요한 전부다.
// 챕터 단위로 자르는 이유는 책 하나에 문단이 5,000개를 넘길 수 있기 때문이다.
type ChapterView struct {
	Idx           int
	Title         string
	TotalChapters int
	Paragraphs    []ParagraphView
}

// ParagraphView에 번역 필드는 없다. 번역 테이블이 아직 없으므로
// 항상 null인 필드를 넣으면 계약이 거짓말을 하게 된다.
type ParagraphView struct {
	StableID   string
	SourceText string
}

type Reader struct {
	pool *pgxpool.Pool
}

func NewReader(pool *pgxpool.Pool) *Reader {
	return &Reader{pool: pool}
}

// Chapter는 활성 revision의 챕터 하나를 읽는다.
// 활성 revision이 없으면(수집 실패, 게이트에 걸림) ErrNotFound다.
func (r *Reader) Chapter(ctx context.Context, gutenbergID, idx int) (*ChapterView, error) {
	q := gen.New(r.pool)

	rev, err := q.GetActiveRevision(ctx, int32(gutenbergID))
	if err != nil {
		if isNoRows(err) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get active revision %d: %w", gutenbergID, err)
	}

	ch, err := q.GetChapter(ctx, gen.GetChapterParams{RevisionID: rev.ID, Idx: int32(idx)})
	if err != nil {
		if isNoRows(err) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get chapter %d/%d: %w", gutenbergID, idx, err)
	}

	total, err := q.CountChapters(ctx, rev.ID)
	if err != nil {
		return nil, fmt.Errorf("count chapters %d: %w", gutenbergID, err)
	}

	rows, err := q.ListParagraphsByChapter(ctx, ch.ID)
	if err != nil {
		return nil, fmt.Errorf("list paragraphs %d/%d: %w", gutenbergID, idx, err)
	}

	view := &ChapterView{
		Idx:           int(ch.Idx),
		Title:         ch.Title,
		TotalChapters: int(total),
		Paragraphs:    make([]ParagraphView, 0, len(rows)),
	}
	for _, row := range rows {
		view.Paragraphs = append(view.Paragraphs, ParagraphView{
			StableID:   row.StableID,
			SourceText: row.Text,
		})
	}
	return view, nil
}
