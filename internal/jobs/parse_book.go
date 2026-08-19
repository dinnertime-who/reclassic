package jobs

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/riverqueue/river"

	"github.com/dinnertime/reclassic/internal/book"
	"github.com/dinnertime/reclassic/internal/storage"
)

// Ingester는 적재 로직이다. internal/book이 이미 갖고 있고 이 잡은 호출만 한다 —
// CLI와 잡이 같은 코드를 쓴다.
type Ingester interface {
	Ingest(ctx context.Context, src book.Source) (*book.IngestResult, error)
}

type ParseBookWorker struct {
	river.WorkerDefaults[ParseBookArgs]

	store    storage.ObjectStore
	ingester Ingester
	log      *slog.Logger
}

func NewParseBookWorker(store storage.ObjectStore, ingester Ingester, log *slog.Logger) *ParseBookWorker {
	return &ParseBookWorker{store: store, ingester: ingester, log: log}
}

func (w *ParseBookWorker) Work(ctx context.Context, job *river.Job[ParseBookArgs]) error {
	args := job.Args

	body, err := w.store.Get(ctx, args.S3Key)
	if err != nil {
		return fmt.Errorf("read source %s: %w", args.S3Key, err)
	}

	res, err := w.ingester.Ingest(ctx, book.Source{
		GutenbergID: args.GutenbergID,
		Title:       args.Title,
		Language:    "en",
		HTML:        body,
		S3Key:       args.S3Key,
	})
	if err != nil {
		// 파싱 실패는 재시도해도 같은 결과다. Ingest가 이미 status='failed'로 남겼다.
		return river.JobCancel(fmt.Errorf("ingest %d: %w", args.GutenbergID, err))
	}

	w.log.InfoContext(ctx, "적재 완료",
		slog.Int("gutenberg_id", args.GutenbergID),
		slog.String("status", res.Status),
		slog.Int("chapters", res.Chapters),
		slog.Int("paragraphs", res.Paragraphs),
		slog.Bool("skipped", res.Skipped),
	)
	return nil
}
