package book

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	gen "github.com/dinnertime/reclassic/internal/db/gen"
	"github.com/dinnertime/reclassic/internal/parse"
)

// Source는 적재할 원문 한 건이다. 어디서 왔는지는 호출부가 정한다 —
// 지금은 로컬 캐시, 나중에는 R2다.
type Source struct {
	GutenbergID int
	Title       string
	Language    string
	HTML        []byte
	// S3Key는 R2 도입 전에는 비어 있다. FetchSource 잡이 채운다.
	S3Key string
}

// IngestResult는 적재 한 건의 결과다.
type IngestResult struct {
	GutenbergID int
	BookID      int64
	RevisionID  int64
	Status      string
	Strategy    string
	Confidence  float64
	Coverage    float64
	Chapters    int
	Paragraphs  int
	Gate        GateResult
	// Skipped는 같은 (book, source, parser_version) revision이 이미 있어
	// 아무것도 하지 않았다는 뜻이다. 적재는 멱등이어야 한다.
	Skipped bool
}

// Ingester는 파싱 결과를 Postgres에 적재한다.
type Ingester struct {
	pool *pgxpool.Pool
	log  *slog.Logger
}

func NewIngester(pool *pgxpool.Pool, log *slog.Logger) *Ingester {
	return &Ingester{pool: pool, log: log}
}

// Ingest는 원문 하나를 파싱해 적재한다.
//
// 파싱은 트랜잭션 밖에서 한다. 셰익스피어 전집은 수 초가 걸리는데
// 그동안 트랜잭션을 열어 둘 이유가 없다. DB 쓰기만 한 트랜잭션으로 묶는다.
func (in *Ingester) Ingest(ctx context.Context, src Source) (*IngestResult, error) {
	out := &IngestResult{GutenbergID: src.GutenbergID}

	ev, parseErr := parse.EvaluateHTML(src.HTML, nil)
	if parseErr != nil {
		// 파싱 자체가 실패하면 revision을 만들지 않는다. 책 상태만 남긴다.
		if err := in.markFailed(ctx, src); err != nil {
			return nil, fmt.Errorf("mark failed %d: %w", src.GutenbergID, err)
		}
		out.Status = StatusFailed
		return out, fmt.Errorf("parse %d: %w", src.GutenbergID, parseErr)
	}

	res := ev.Best.Result
	out.Strategy = res.Strategy
	out.Confidence = float64(res.Confidence)
	out.Coverage = ev.Best.Signals.Coverage
	out.Chapters, out.Paragraphs = Counts(res)
	out.Gate = Gate(out.Chapters, out.Paragraphs, out.Confidence)

	tx, err := in.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	q := gen.New(tx)

	book, err := q.UpsertBook(ctx, gen.UpsertBookParams{
		GutenbergID: int32(src.GutenbergID),
		Title:       src.Title,
		// author는 Gutendex 카탈로그에서 온다. 수집 자동화 슬라이스 전까지는 비어 있다.
		Author:   pgtype.Text{},
		Language: src.Language,
	})
	if err != nil {
		return nil, fmt.Errorf("upsert book %d: %w", src.GutenbergID, err)
	}
	out.BookID = book.ID

	source, err := q.UpsertBookSource(ctx, gen.UpsertBookSourceParams{
		BookID:      book.ID,
		S3Key:       optionalText(src.S3Key),
		ContentHash: ev.SourceSHA256,
	})
	if err != nil {
		return nil, fmt.Errorf("upsert source %d: %w", src.GutenbergID, err)
	}

	// 멱등성. 같은 원문을 같은 파서 버전으로 다시 적재하면 아무것도 하지 않는다.
	existing, err := q.FindRevision(ctx, gen.FindRevisionParams{
		BookID:        book.ID,
		SourceID:      source.ID,
		ParserVersion: parse.Version,
	})
	if err == nil {
		out.RevisionID = existing.ID
		out.Skipped = true
		out.Status = book.Status
		if err := tx.Commit(ctx); err != nil {
			return nil, fmt.Errorf("commit: %w", err)
		}
		return out, nil
	}
	if !isNoRows(err) {
		return nil, fmt.Errorf("find revision %d: %w", src.GutenbergID, err)
	}

	warnings, err := json.Marshal(nonNilStrings(res.Warnings))
	if err != nil {
		return nil, fmt.Errorf("marshal warnings: %w", err)
	}

	rev, err := q.InsertRevision(ctx, gen.InsertRevisionParams{
		BookID:        book.ID,
		SourceID:      source.ID,
		ParserVersion: parse.Version,
		Strategy:      res.Strategy,
		Confidence:    float32(out.Confidence),
		Coverage:      float32(out.Coverage),
		Warnings:      warnings,
	})
	if err != nil {
		return nil, fmt.Errorf("insert revision %d: %w", src.GutenbergID, err)
	}
	out.RevisionID = rev.ID

	if err := insertContent(ctx, q, rev.ID, res); err != nil {
		return nil, fmt.Errorf("insert content %d: %w", src.GutenbergID, err)
	}

	// 게이트를 통과한 것만 활성으로 올린다.
	// needs_review여도 revision은 남긴다 — 관리자가 보고 판단해야 하므로 버리지 않는다.
	out.Status = StatusNeedsReview
	if out.Gate.Passed {
		if err := q.DeactivateRevisions(ctx, book.ID); err != nil {
			return nil, fmt.Errorf("deactivate revisions %d: %w", src.GutenbergID, err)
		}
		if err := q.ActivateRevision(ctx, rev.ID); err != nil {
			return nil, fmt.Errorf("activate revision %d: %w", src.GutenbergID, err)
		}
		out.Status = StatusReady
	}

	if err := q.SetBookStatus(ctx, gen.SetBookStatusParams{ID: book.ID, Status: out.Status}); err != nil {
		return nil, fmt.Errorf("set status %d: %w", src.GutenbergID, err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit %d: %w", src.GutenbergID, err)
	}

	if !out.Gate.Passed {
		in.log.WarnContext(ctx, "관리자 확인 큐로 보냄",
			slog.Int("gutenberg_id", src.GutenbergID),
			slog.Any("reasons", out.Gate.Reasons),
		)
	}
	return out, nil
}

// insertContent는 챕터와 문단을 넣는다.
// stable_id는 internal/parse가 만든 값을 그대로 저장한다 — 여기서 다시 계산하지 않는다.
// 두 곳에서 계산하면 언젠가 갈라진다.
func insertContent(ctx context.Context, q *gen.Queries, revisionID int64, res *parse.Result) error {
	for _, ch := range res.Chapters {
		chapterID, err := q.InsertChapter(ctx, gen.InsertChapterParams{
			RevisionID: revisionID,
			Idx:        int32(ch.Idx),
			Title:      ch.Title,
			Anchor:     ch.Anchor,
		})
		if err != nil {
			return fmt.Errorf("insert chapter %d: %w", ch.Idx, err)
		}

		if len(ch.Paragraphs) == 0 {
			continue
		}
		rows := make([]gen.InsertParagraphsParams, 0, len(ch.Paragraphs))
		for _, p := range ch.Paragraphs {
			rows = append(rows, gen.InsertParagraphsParams{
				RevisionID: revisionID,
				ChapterID:  chapterID,
				Idx:        int32(p.Idx),
				StableID:   p.StableID,
				Text:       p.Text,
				Html:       p.HTML,
			})
		}
		if _, err := q.InsertParagraphs(ctx, rows); err != nil {
			return fmt.Errorf("copy paragraphs of chapter %d: %w", ch.Idx, err)
		}
	}
	return nil
}

// markFailed는 파싱이 실패한 책의 상태만 기록한다. revision은 만들지 않는다.
func (in *Ingester) markFailed(ctx context.Context, src Source) error {
	tx, err := in.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	q := gen.New(tx)
	book, err := q.UpsertBook(ctx, gen.UpsertBookParams{
		GutenbergID: int32(src.GutenbergID),
		Title:       src.Title,
		Author:      pgtype.Text{},
		Language:    src.Language,
	})
	if err != nil {
		return fmt.Errorf("upsert book: %w", err)
	}
	if err := q.SetBookStatus(ctx, gen.SetBookStatusParams{ID: book.ID, Status: StatusFailed}); err != nil {
		return fmt.Errorf("set status: %w", err)
	}
	return tx.Commit(ctx)
}

func optionalText(s string) pgtype.Text {
	if s == "" {
		return pgtype.Text{}
	}
	return pgtype.Text{String: s, Valid: true}
}

// nonNilStrings는 nil 슬라이스를 JSON null이 아니라 []로 만든다.
// warnings 컬럼은 NOT NULL DEFAULT '[]'다.
func nonNilStrings(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

func isNoRows(err error) bool {
	return err == pgx.ErrNoRows || err != nil && err.Error() == pgx.ErrNoRows.Error()
}
