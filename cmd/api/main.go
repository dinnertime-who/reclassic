// JSON API 서버의 진입점. 얇게 유지한다 — 라우터 조립과 핸들러는 internal/api에 있다.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/dinnertime/reclassic/internal/api"
	"github.com/dinnertime/reclassic/internal/book"
	"github.com/dinnertime/reclassic/internal/config"
	"github.com/dinnertime/reclassic/internal/db"
	"github.com/dinnertime/reclassic/internal/jobs"
	"github.com/dinnertime/reclassic/internal/translate"
)

// version은 빌드 시 -ldflags로 주입한다. 개발 중에는 dev.
var version = "dev"

const shutdownTimeout = 10 * time.Second

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	if err := run(log); err != nil {
		log.Error("api 기동 실패", slog.String("err", err.Error()))
		os.Exit(1)
	}
}

func run(log *slog.Logger) error {
	config.LoadDotEnv(".env")

	databaseURL, err := config.Require("DATABASE_URL")
	if err != nil {
		return err
	}
	port, err := config.Require("PORT")
	if err != nil {
		return err
	}
	// 관리자 엔드포인트를 막는 임시 가드. 비어 있으면 기동하지 않는다 —
	// 무인증으로 열린 채 배포되는 것이 최악이다.
	adminToken, err := config.Require("ADMIN_TOKEN")
	if err != nil {
		return err
	}

	// SIGINT/SIGTERM에서 취소되는 컨텍스트. Railway는 배포 교체 시 SIGTERM을 보낸다.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := db.Connect(ctx, databaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	// api는 잡을 넣기만 한다. 소비는 worker가 한다 (ADR-001).
	riverClient, err := jobs.NewInsertOnlyClient(pool)
	if err != nil {
		return err
	}

	srv := &http.Server{
		// Railway 프라이빗 네트워크는 IPv6 전용이다 (불변식 4).
		// 0.0.0.0에 바인딩하면 서비스 간 내부 호출을 받지 못한다.
		Addr: "[::]:" + port,
		Handler: api.NewServer(api.Deps{
			DB:         pool,
			Reader:     book.NewReader(pool),
			Requester:  book.NewRequester(pool, jobs.NewEnqueuer(riverClient)),
			Translate:  translate.NewService(pool),
			AdminToken: adminToken,
			Version:    version,
			Log:        log,
		}).Router(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		log.Info("api 기동", slog.String("addr", srv.Addr), slog.String("version", version))
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- fmt.Errorf("listen and serve: %w", err)
		}
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		log.Info("종료 신호 수신. graceful shutdown 시작")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown: %w", err)
	}

	log.Info("api 종료 완료")
	return nil
}
