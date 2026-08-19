package jobs

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"

	"github.com/dinnertime/reclassic/internal/gutenberg"
	"github.com/dinnertime/reclassic/internal/storage"
)

// Fetcher는 원문을 받아오는 최소 인터페이스다. 테스트에서 갈아끼운다.
type Fetcher interface {
	GetRetry5xx(url string, maxAttempts int) ([]byte, int, error)
}

type FetchSourceWorker struct {
	river.WorkerDefaults[FetchSourceArgs]

	fetch Fetcher
	store storage.ObjectStore
	log   *slog.Logger
}

func NewFetchSourceWorker(fetch Fetcher, store storage.ObjectStore, log *slog.Logger) *FetchSourceWorker {
	return &FetchSourceWorker{fetch: fetch, store: store, log: log}
}

const fetchAttempts = 3

func (w *FetchSourceWorker) Work(ctx context.Context, job *river.Job[FetchSourceArgs]) error {
	args := job.Args

	body, lastStatus, err := w.download(args.GutenbergID)
	if err != nil {
		// 4xx는 재시도해도 같다. 취소하고 실패로 남긴다.
		if lastStatus >= 400 && lastStatus < 500 {
			return river.JobCancel(fmt.Errorf("gutenberg %d: %d — 재시도하지 않음: %w",
				args.GutenbergID, lastStatus, err))
		}
		return fmt.Errorf("fetch %d: %w", args.GutenbergID, err)
	}

	// 키에 내용 해시를 넣으므로 재시도해도 같은 자리에 덮어써진다.
	key := storage.SourceKey(args.GutenbergID, storage.HashContent(body))
	if err := w.store.Put(ctx, key, body, "text/html; charset=utf-8"); err != nil {
		return fmt.Errorf("store %d: %w", args.GutenbergID, err)
	}

	w.log.InfoContext(ctx, "원문 보관 완료",
		slog.Int("gutenberg_id", args.GutenbergID),
		slog.String("s3_key", key),
		slog.Int("bytes", len(body)),
	)

	// book_sources 기록은 ParseBook이 적재 트랜잭션 안에서 함께 한다.
	// 여기서 따로 쓰면 "원본은 기록됐는데 파싱 결과가 없는" 상태가 생긴다.
	// Safely 변형을 쓴다. 패닉을 제어 흐름으로 쓰지 않는다 (CONVENTIONS).
	client, err := river.ClientFromContextSafely[pgx.Tx](ctx)
	if err != nil {
		return fmt.Errorf("river client from context: %w", err)
	}
	if _, err := client.Insert(ctx, ParseBookArgs{
		BookID:      args.BookID,
		GutenbergID: args.GutenbergID,
		Title:       args.Title,
		S3Key:       key,
	}, &river.InsertOpts{Queue: QueueParse}); err != nil {
		return fmt.Errorf("enqueue parse %d: %w", args.GutenbergID, err)
	}
	return nil
}

// download는 SourceURLs를 순서대로 시도한다.
// 캐시를 보지 않는다 — 프로덕션에는 로컬 캐시가 없다.
func (w *FetchSourceWorker) download(gutenbergID int) ([]byte, int, error) {
	var lastStatus int
	var lastErr error
	for _, url := range gutenberg.SourceURLs(gutenbergID) {
		body, status, err := w.fetch.GetRetry5xx(url, fetchAttempts)
		if err == nil {
			return body, status, nil
		}
		lastStatus, lastErr = status, err
	}
	return nil, lastStatus, lastErr
}
