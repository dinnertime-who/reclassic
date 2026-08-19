// 잡 컨슈머의 진입점. 얇게 유지한다 — 잡 로직은 internal/jobs에 있다.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/dinnertime/reclassic/internal/book"
	"github.com/dinnertime/reclassic/internal/config"
	"github.com/dinnertime/reclassic/internal/db"
	"github.com/dinnertime/reclassic/internal/gutenberg"
	"github.com/dinnertime/reclassic/internal/jobs"
	"github.com/dinnertime/reclassic/internal/storage"
	"github.com/dinnertime/reclassic/internal/translate"
)

var version = "dev"

const shutdownTimeout = 30 * time.Second

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	if err := run(log); err != nil {
		log.Error("worker 기동 실패", slog.String("err", err.Error()))
		os.Exit(1)
	}
}

func run(log *slog.Logger) error {
	config.LoadDotEnv(".env")

	databaseURL, err := config.Require("DATABASE_URL")
	if err != nil {
		return err
	}
	// Gutenberg 수집 규칙. 없으면 기동을 거부한다 — 익명 크롤러가 되면 IP가 차단된다.
	userAgent, err := config.Require("GUTENBERG_USER_AGENT")
	if err != nil {
		return err
	}
	interval, err := fetchInterval()
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := db.Connect(ctx, databaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	store, err := objectStore(ctx)
	if err != nil {
		return err
	}
	// 로컬 MinIO는 버킷이 비어 있는 채로 뜬다. R2에서는 콘솔에서 미리 만들어 두므로
	// S3_ENDPOINT가 있을 때(=로컬)만 만든다.
	if os.Getenv("S3_ENDPOINT") != "" {
		if err := store.EnsureBucket(ctx); err != nil {
			return err
		}
	}

	// 사이트맵의 공개 URL. 로컬에서는 웹 개발 서버 주소다.
	publicBaseURL, err := config.Require("PUBLIC_BASE_URL")
	if err != nil {
		return err
	}

	client, err := jobs.NewWorkerClient(pool, jobs.Workers{
		Fetch: jobs.NewFetchSourceWorker(gutenberg.NewClient(userAgent, interval), store, log),
		Parse: jobs.NewParseBookWorker(store, book.NewIngester(pool, log), log),
		Sitemap: jobs.NewSitemapWorker(translate.NewService(pool), store,
			publicBaseURL, translate.DefaultIndexThreshold, log),
	}, parseConcurrency(), log)
	if err != nil {
		return err
	}

	if err := client.Start(ctx); err != nil {
		return fmt.Errorf("start river client: %w", err)
	}
	log.Info("worker 기동",
		slog.String("version", version),
		slog.Duration("gutenberg_interval", interval),
	)

	<-ctx.Done()
	log.Info("종료 신호 수신. 진행 중인 잡을 기다린다")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := client.Stop(shutdownCtx); err != nil && !errors.Is(err, context.Canceled) {
		return fmt.Errorf("stop river client: %w", err)
	}

	log.Info("worker 종료 완료")
	return nil
}

func objectStore(ctx context.Context) (*storage.S3Store, error) {
	accessKey, err := config.Require("R2_ACCESS_KEY_ID")
	if err != nil {
		return nil, err
	}
	secretKey, err := config.Require("R2_SECRET_ACCESS_KEY")
	if err != nil {
		return nil, err
	}
	bucket, err := config.Require("R2_BUCKET")
	if err != nil {
		return nil, err
	}
	return storage.NewS3Store(ctx, storage.Config{
		// S3_ENDPOINT는 로컬 MinIO용이다. R2에서는 비어 있고 AccountID로 조립한다.
		Endpoint:  os.Getenv("S3_ENDPOINT"),
		AccountID: os.Getenv("R2_ACCOUNT_ID"),
		AccessKey: accessKey,
		SecretKey: secretKey,
		Bucket:    bucket,
	})
}

// fetchInterval은 Gutenberg 요청 간 최소 간격이다.
// 1초 미만으로 낮추지 않는다 — 공격적으로 긁으면 IP가 차단되고 복구가 어렵다.
func fetchInterval() (time.Duration, error) {
	raw, err := config.Require("GUTENBERG_MIN_INTERVAL_MS")
	if err != nil {
		return 0, err
	}
	ms, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("GUTENBERG_MIN_INTERVAL_MS=%q: %w", raw, err)
	}
	if ms < 1000 {
		return 0, fmt.Errorf("GUTENBERG_MIN_INTERVAL_MS=%d — 1000 미만으로 낮추지 말 것", ms)
	}
	return time.Duration(ms) * time.Millisecond, nil
}

// parseConcurrency는 파싱 큐의 동시성이다. 수집 큐는 항상 1이다 (internal/jobs).
func parseConcurrency() int {
	if v, err := strconv.Atoi(os.Getenv("PARSE_CONCURRENCY")); err == nil && v > 0 {
		return v
	}
	return 2
}
