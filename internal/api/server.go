// Package api는 HTTP 라우터 조립과 핸들러 구현을 담는다.
// 생성 코드(internal/api/gen)는 손대지 않는다 — openapi.yaml을 고치고 make generate를 돌린다.
package api

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/dinnertime/reclassic/internal/api/gen"
)

// Pinger는 헬스체크가 DB에 대해 알아야 하는 전부다.
// 외부 I/O는 인터페이스 뒤에 둔다 (CONVENTIONS) — 테스트에서 갈아끼운다.
type Pinger interface {
	Ping(ctx context.Context) error
}

// Server는 생성된 StrictServerInterface를 구현한다.
type Server struct {
	db      Pinger
	version string
	log     *slog.Logger
}

func NewServer(database Pinger, version string, log *slog.Logger) *Server {
	return &Server{db: database, version: version, log: log}
}

// GetHealthz는 실제 Ping 결과를 돌려준다.
// 상수를 돌려주면 DB 배선이 검증되지 않는다 (SLICE_SKELETON §4.4).
func (s *Server) GetHealthz(ctx context.Context, _ gen.GetHealthzRequestObject) (gen.GetHealthzResponseObject, error) {
	dbState := gen.HealthDbOk
	if err := s.db.Ping(ctx); err != nil {
		// DB가 죽어도 서비스는 200으로 상태를 보고한다. 그래야 관측이 된다.
		s.log.WarnContext(ctx, "db ping 실패", slog.String("err", err.Error()))
		dbState = gen.HealthDbDown
	}

	return gen.GetHealthz200JSONResponse{
		Status:  gen.HealthStatusOk,
		Db:      dbState,
		Version: s.version,
	}, nil
}

// Router는 미들웨어를 붙이고 생성된 라우팅을 얹는다.
func (s *Server) Router() http.Handler {
	r := chi.NewRouter()

	// 미들웨어 3종. 전부 표준 http.Handler 래퍼다 (ADR-018).
	r.Use(middleware.Recoverer)
	r.Use(middleware.RequestID)
	r.Use(requestLogger(s.log))

	// 인증 미들웨어 자리. 세션·권한 분리는 이번 슬라이스 범위가 아니다.

	return gen.HandlerFromMux(gen.NewStrictHandler(s, nil), r)
}

// requestLogger는 요청 한 건을 구조화 필드로 남긴다.
// 문자열 포매팅으로 값을 섞지 않는다 (CONVENTIONS).
func requestLogger(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)

			next.ServeHTTP(ww, r)

			log.InfoContext(r.Context(), "http request",
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.Int("status", ww.Status()),
				slog.Int("bytes", ww.BytesWritten()),
				slog.Duration("took", time.Since(start)),
				slog.String("request_id", middleware.GetReqID(r.Context())),
			)
		})
	}
}
