// 잡 컨슈머의 진입점.
// 이번 슬라이스에서는 잡을 소비하지 않는다 — 배포 단위가 둘이라는 것(ADR-001)만 확인한다.
// River 도입은 수집 자동화 슬라이스다.
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
)

var version = "dev"

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	log.Info("worker 기동", slog.String("version", version))

	<-ctx.Done()

	log.Info("종료 신호 수신. worker 종료 완료")
}
