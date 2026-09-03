// Package api는 HTTP 라우터 조립과 핸들러 구현을 담는다.
// 생성 코드(internal/api/gen)는 손대지 않는다 — openapi.yaml을 고치고 make generate를 돌린다.
package api

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"

	"github.com/dinnertime/reclassic/internal/api/gen"
	"github.com/dinnertime/reclassic/internal/auth"
	"github.com/dinnertime/reclassic/internal/book"
	"github.com/dinnertime/reclassic/internal/translate"
)

// Pinger는 헬스체크가 DB에 대해 알아야 하는 전부다.
// 외부 I/O는 인터페이스 뒤에 둔다 (CONVENTIONS) — 테스트에서 갈아끼운다.
type Pinger interface {
	Ping(ctx context.Context) error
}

// ChapterReader는 읽기 핸들러가 도메인에 대해 알아야 하는 전부다.
type ChapterReader interface {
	Chapter(ctx context.Context, gutenbergID, idx int) (*book.ChapterView, error)
}

// BookCatalog는 도서·고아 목록이다. Reader와 같은 객체일 수 있다.
type BookCatalog interface {
	ListPublished(ctx context.Context) ([]book.CatalogItem, error)
	ListNeedsReview(ctx context.Context) ([]book.NeedsReviewItem, error)
	ListOrphans(ctx context.Context) ([]book.OrphanItem, error)
}

// UserAdmin은 사용자 목록과 역할 부여다. admin은 여기서 만들지 않는다.
type UserAdmin interface {
	List(ctx context.Context) ([]auth.UserItem, error)
	SetRole(ctx context.Context, id int64, role string) (*auth.UserItem, error)
}

// BookRequester는 관리자 수집 지시를 받는다.
type BookRequester interface {
	Request(ctx context.Context, gutenbergID int, title, language string) (int64, error)
}

// Server는 생성된 StrictServerInterface를 구현한다.
type Server struct {
	db             Pinger
	reader         ChapterReader
	catalog        BookCatalog
	users          UserAdmin
	requester      BookRequester
	translate      Translator
	sessions       SessionStore
	google         GoogleLogin
	allowedOrigins []string
	indexThreshold float64
	version        string
	log            *slog.Logger
}

type Deps struct {
	DB        Pinger
	Reader    ChapterReader
	Catalog   BookCatalog
	Users     UserAdmin
	Requester BookRequester
	Translate Translator
	Sessions  SessionStore
	Google    GoogleLogin
	// AllowedOrigins는 브라우저가 이 API를 직접 부를 수 있는 출처다 (ADR-026).
	// 웹과 API가 서로 다른 도메인에 있으므로 CORS가 필요하다.
	AllowedOrigins []string
	// IndexThreshold는 챕터 색인 기준이다 (ADR-023). 0이면 기본값 0.80.
	IndexThreshold float64
	Version        string
	Log            *slog.Logger
}

func NewServer(d Deps) *Server {
	threshold := d.IndexThreshold
	if threshold == 0 {
		threshold = translate.DefaultIndexThreshold
	}
	return &Server{
		db:             d.DB,
		reader:         d.Reader,
		catalog:        d.Catalog,
		users:          d.Users,
		requester:      d.Requester,
		translate:      d.Translate,
		sessions:       d.Sessions,
		google:         d.Google,
		allowedOrigins: d.AllowedOrigins,
		indexThreshold: threshold,
		version:        d.Version,
		log:            d.Log,
	}
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

// GetBookChapter는 활성 revision의 챕터 하나를 돌려준다.
// 활성 revision이 없으면 404다 — 게이트에 걸린 책은 읽기에 노출되지 않는다.
func (s *Server) GetBookChapter(ctx context.Context, req gen.GetBookChapterRequestObject) (gen.GetBookChapterResponseObject, error) {
	view, err := s.reader.Chapter(ctx, req.GutenbergId, req.Idx)
	if err != nil {
		if errors.Is(err, book.ErrNotFound) {
			return gen.GetBookChapter404JSONResponse{Message: "그 챕터를 찾을 수 없다"}, nil
		}
		return nil, err
	}

	paragraphs := make([]gen.Paragraph, 0, len(view.Paragraphs))
	for _, p := range view.Paragraphs {
		paragraphs = append(paragraphs, gen.Paragraph{
			StableId:   p.StableID,
			SourceText: p.SourceText,
		})
	}

	return gen.GetBookChapter200JSONResponse{
		Chapter:       gen.Chapter{Idx: view.Idx, Title: view.Title},
		Paragraphs:    paragraphs,
		TotalChapters: view.TotalChapters,
	}, nil
}

// RequestBook은 수집을 지시한다. 실제 수집은 워커가 한다.
func (s *Server) RequestBook(ctx context.Context, req gen.RequestBookRequestObject) (gen.RequestBookResponseObject, error) {
	if req.Body == nil {
		return gen.RequestBook409JSONResponse{Message: "본문이 없다"}, nil
	}

	language := "en"
	if req.Body.Language != nil && *req.Body.Language != "" {
		language = *req.Body.Language
	}

	bookID, err := s.requester.Request(ctx, req.Body.GutenbergId, req.Body.Title, language)
	if err != nil {
		if errors.Is(err, book.ErrAlreadyRequested) {
			return gen.RequestBook409JSONResponse{Message: "이미 수집이 지시된 책이다"}, nil
		}
		return nil, err
	}

	return gen.RequestBook202JSONResponse{
		BookId: bookID,
		Status: gen.BookRequestAcceptedStatusPending,
	}, nil
}

// Router는 미들웨어를 붙이고 생성된 라우팅을 얹는다.
func (s *Server) Router() http.Handler {
	r := chi.NewRouter()

	// 미들웨어. 전부 표준 http.Handler 래퍼다 (ADR-018).
	r.Use(middleware.Recoverer)
	r.Use(middleware.RequestID)
	r.Use(requestLogger(s.log))
	r.Use(s.corsMiddleware())

	// 세션이 있으면 사용자를 컨텍스트에 싣는다. 거부는 오퍼레이션별로 authGuard가 한다.
	r.Use(withHTTP)
	r.Use(s.withSession)

	// OAuth 리다이렉트는 JSON 계약이 아니라 라우터에 직접 단다.
	s.mountOAuth(r)

	return gen.HandlerFromMux(gen.NewStrictHandler(s, []gen.StrictMiddlewareFunc{s.authGuard}), r)
}

// corsMiddleware는 브라우저가 다른 출처에서 이 API를 부를 수 있게 한다 (ADR-026).
//
// 프로덕션에서도 필요하다 — 웹은 공개 도메인, API는 별도 도메인이고
// 브라우저는 SSR이 아니라 자기가 직접 API를 부른다 (ARCHITECTURE "핵심 불변식 4").
//
// 와일드카드를 쓰지 않는다. credentials를 함께 보내므로 규격상 허용되지 않고,
// 허용해서도 안 된다 — 아무 사이트나 사용자 세션으로 이 API를 부르게 된다.
func (s *Server) corsMiddleware() func(http.Handler) http.Handler {
	return cors.Handler(cors.Options{
		AllowedOrigins:   s.allowedOrigins,
		AllowedMethods:   []string{http.MethodGet, http.MethodPost, http.MethodOptions},
		AllowedHeaders:   []string{"Content-Type"},
		AllowCredentials: true,
		MaxAge:           300,
	})
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
