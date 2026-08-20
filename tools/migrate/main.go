// `make migrate`의 진입점. cmd/ 에는 상주 서비스(api·worker)와 스파이크 실행기(parsecheck)만
// 두고, 한 번 돌고 끝나는 도구는 tools/ 에 둔다.
//
// 상주하지는 않지만 이미지에는 들어간다 — Railway의 pre-deploy 명령이 `/migrate`를 부른다
// (ADR-030). 그래서 Dockerfile이 api·worker와 함께 이 바이너리도 빌드한다.
package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/dinnertime/reclassic/internal/config"
	"github.com/dinnertime/reclassic/internal/db"
	"github.com/dinnertime/reclassic/internal/jobs"
)

func main() {
	log := slog.New(slog.NewTextHandler(os.Stderr, nil))

	config.LoadDotEnv(".env")

	databaseURL, err := config.Require("DATABASE_URL")
	if err != nil {
		log.Error("설정 오류", "err", err.Error())
		os.Exit(1)
	}

	ctx := context.Background()

	// 순서가 중요하다: goose 먼저, River 나중 (ADR-022).
	if err := db.Migrate(ctx, databaseURL); err != nil {
		log.Error("goose 마이그레이션 실패", "err", err)
		os.Exit(1)
	}

	pool, err := db.Connect(ctx, databaseURL)
	if err != nil {
		log.Error("DB 연결 실패", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	if err := jobs.Migrate(ctx, pool); err != nil {
		log.Error("River 마이그레이션 실패", "err", err)
		os.Exit(1)
	}
	log.Info("River 마이그레이션 적용 완료")
}
