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
	"github.com/dinnertime/reclassic/internal/auth"
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
	// 브라우저가 이 API를 직접 부를 수 있는 출처. 쉼표로 구분한다 (ADR-026).
	// 기본값을 조용히 채우지 않는다 — 비어 있으면 교차 출처 호출이 전부 막힌다.
	origins, err := config.RequireList("CORS_ALLOWED_ORIGINS")
	if err != nil {
		return err
	}
	googleCfg, cookieCfg, err := authConfig()
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

	reader := book.NewReader(pool)
	srv := &http.Server{
		// Railway 프라이빗 네트워크는 IPv6 전용이다 (불변식 4).
		// 0.0.0.0에 바인딩하면 서비스 간 내부 호출을 받지 못한다.
		Addr: "[::]:" + port,
		Handler: api.NewServer(api.Deps{
			DB:             pool,
			Reader:         reader,
			Catalog:        reader,
			Users:          auth.NewUsers(pool),
			Requester:      book.NewRequester(pool, jobs.NewEnqueuer(riverClient)),
			Translate:      translate.NewService(pool),
			Sessions:       auth.NewSessions(pool, cookieCfg),
			Google:         auth.NewGoogle(pool, googleCfg, cookieCfg),
			AllowedOrigins: origins,
			Version:        version,
			Log:            log,
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

// authConfig는 Google 로그인과 쿠키 설정을 읽는다.
// 하나라도 비면 기동하지 않는다 — 로그인할 수 없는 배포는 사고다.
func authConfig() (auth.GoogleConfig, auth.CookieConfig, error) {
	var g auth.GoogleConfig
	var c auth.CookieConfig

	for _, f := range []struct {
		key string
		dst *string
	}{
		{"GOOGLE_CLIENT_ID", &g.ClientID},
		{"GOOGLE_CLIENT_SECRET", &g.ClientSecret},
		{"GOOGLE_REDIRECT_URL", &g.RedirectURL},
		// 마스터 관리자. 관리자가 아무도 없는 배포를 막는다 (ADR-027).
		{"ADMIN_EMAIL", &g.AdminEmail},
		{"LOGIN_SUCCESS_REDIRECT", &g.SuccessRedirect},
	} {
		v, err := config.Require(f.key)
		if err != nil {
			return g, c, err
		}
		*f.dst = v
	}

	// 로컬 http에서는 Secure를 꺼야 쿠키가 붙는다.
	// 프로덕션에서 켜는 것을 잊지 말 것.
	c.Secure = os.Getenv("COOKIE_SECURE") == "true"
	c.Domain = os.Getenv("COOKIE_DOMAIN")
	return g, c, nil
}
