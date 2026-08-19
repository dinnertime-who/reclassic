package book

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	gen "github.com/dinnertime/reclassic/internal/db/gen"
)

// ErrAlreadyRequested는 이미 수집을 지시한 책이라는 뜻이다.
var ErrAlreadyRequested = errors.New("이미 수집이 지시된 책이다")

// FetchQueue는 진행 중인 트랜잭션에 수집 잡을 넣는다.
//
// tx를 인자로 받는 것이 이 인터페이스의 전부이자 ADR-003의 전부다 —
// Redis 기반 큐로는 표현할 수 없는 시그니처다.
// 도메인은 잡 타입을 알 필요가 없다. 구현은 internal/jobs에 있다.
type FetchQueue interface {
	EnqueueFetch(ctx context.Context, tx pgx.Tx, bookID int64, gutenbergID int, title string) error
}

// Requester는 관리자의 수집 지시를 받는다.
type Requester struct {
	pool  *pgxpool.Pool
	queue FetchQueue
}

func NewRequester(pool *pgxpool.Pool, queue FetchQueue) *Requester {
	return &Requester{pool: pool, queue: queue}
}

// Request는 books 행 생성과 FetchSource 잡 등록을 한 트랜잭션 안에서 한다.
//
// 이것이 ADR-003이 Redis 대신 River를 고른 유일한 근거다.
// "책 레코드는 생성됐는데 잡 등록이 실패"하는 틈이 생기지 않는다.
// 잡 등록을 트랜잭션 밖으로 빼면 River를 쓸 이유가 사라진다.
func (r *Requester) Request(ctx context.Context, gutenbergID int, title, language string) (int64, error) {
	if language == "" {
		language = "en"
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	q := gen.New(tx)

	// 이미 수집이 끝났거나 진행 중이면 다시 지시하지 않는다.
	if existing, err := q.GetBookByGutenbergID(ctx, int32(gutenbergID)); err == nil {
		if existing.Status != StatusFailed {
			return existing.ID, ErrAlreadyRequested
		}
	} else if !isNoRows(err) {
		return 0, fmt.Errorf("lookup book %d: %w", gutenbergID, err)
	}

	book, err := q.UpsertBook(ctx, gen.UpsertBookParams{
		GutenbergID: int32(gutenbergID),
		Title:       title,
		Author:      pgtype.Text{},
		Language:    language,
	})
	if err != nil {
		return 0, fmt.Errorf("upsert book %d: %w", gutenbergID, err)
	}
	if err := q.SetBookStatus(ctx, gen.SetBookStatusParams{ID: book.ID, Status: StatusPending}); err != nil {
		return 0, fmt.Errorf("set status %d: %w", gutenbergID, err)
	}

	if err := r.queue.EnqueueFetch(ctx, tx, book.ID, gutenbergID, title); err != nil {
		return 0, fmt.Errorf("enqueue fetch %d: %w", gutenbergID, err)
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("commit %d: %w", gutenbergID, err)
	}
	return book.ID, nil
}
