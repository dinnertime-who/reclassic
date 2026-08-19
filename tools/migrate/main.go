// `make migrate`의 진입점. 배포되지 않는 개발 도구라 cmd/ 가 아니라 tools/ 에 둔다.
// cmd/ 에는 배포 단위(api·worker)와 스파이크 실행기(parsecheck)만 있다.
package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/dinnertime/reclassic/internal/config"
	"github.com/dinnertime/reclassic/internal/db"
)

func main() {
	log := slog.New(slog.NewTextHandler(os.Stderr, nil))

	config.LoadDotEnv(".env")

	databaseURL, err := config.Require("DATABASE_URL")
	if err != nil {
		log.Error("설정 오류", "err", err.Error())
		os.Exit(1)
	}

	if err := db.Migrate(context.Background(), databaseURL); err != nil {
		log.Error("마이그레이션 실패", "err", err)
		os.Exit(1)
	}
}
