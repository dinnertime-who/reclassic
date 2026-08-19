package jobs

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/riverqueue/river/rivermigrate"
)

// Migrate는 River 자체 테이블을 만든다.
// goose 마이그레이션이 먼저 돌아야 한다 (ADR-022).
func Migrate(ctx context.Context, pool *pgxpool.Pool) error {
	migrator, err := rivermigrate.New(riverpgxv5.New(pool), nil)
	if err != nil {
		return fmt.Errorf("new river migrator: %w", err)
	}
	if _, err := migrator.Migrate(ctx, rivermigrate.DirectionUp, nil); err != nil {
		return fmt.Errorf("river migrate: %w", err)
	}
	return nil
}

// NewInsertOnlyClient는 잡을 넣기만 하는 클라이언트다. api가 쓴다.
func NewInsertOnlyClient(pool *pgxpool.Pool) (*river.Client[pgx.Tx], error) {
	return river.NewClient(riverpgxv5.New(pool), &river.Config{})
}

// Workers는 워커 등록을 모아 둔다.
type Workers struct {
	Fetch *FetchSourceWorker
	Parse *ParseBookWorker
}

// NewWorkerClient는 실제로 잡을 소비하는 클라이언트다. cmd/worker가 쓴다.
//
// 큐를 나누는 이유는 동시성이 다르기 때문이다.
// fetch는 반드시 1이다 — Gutenberg에 병렬 요청을 보내면 IP가 차단된다.
func NewWorkerClient(pool *pgxpool.Pool, w Workers, parseConcurrency int, log *slog.Logger) (*river.Client[pgx.Tx], error) {
	workers := river.NewWorkers()
	if err := river.AddWorkerSafely(workers, w.Fetch); err != nil {
		return nil, fmt.Errorf("register fetch worker: %w", err)
	}
	if err := river.AddWorkerSafely(workers, w.Parse); err != nil {
		return nil, fmt.Errorf("register parse worker: %w", err)
	}

	client, err := river.NewClient(riverpgxv5.New(pool), &river.Config{
		Logger:  log,
		Workers: workers,
		Queues: map[string]river.QueueConfig{
			QueueFetch: {MaxWorkers: 1},
			QueueParse: {MaxWorkers: parseConcurrency},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("new river client: %w", err)
	}
	return client, nil
}

// Enqueuer는 book.FetchQueue를 구현한다.
// 도메인이 River 타입을 몰라도 되도록 잡 인자 조립을 여기서 한다.
type Enqueuer struct {
	client *river.Client[pgx.Tx]
}

func NewEnqueuer(client *river.Client[pgx.Tx]) *Enqueuer {
	return &Enqueuer{client: client}
}

// EnqueueFetch는 진행 중인 트랜잭션에 잡을 넣는다.
// tx를 받는 것이 요점이다 — 커밋이 실패하면 잡도 없다 (ADR-003).
func (e *Enqueuer) EnqueueFetch(ctx context.Context, tx pgx.Tx, bookID int64, gutenbergID int, title string) error {
	_, err := e.client.InsertTx(ctx, tx, FetchSourceArgs{
		BookID:      bookID,
		GutenbergID: gutenbergID,
		Title:       title,
	}, &river.InsertOpts{Queue: QueueFetch})
	if err != nil {
		return fmt.Errorf("insert fetch job %d: %w", gutenbergID, err)
	}
	return nil
}
