package api

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/dinnertime/reclassic/internal/api/gen"
	"github.com/dinnertime/reclassic/internal/auth"
	gendb "github.com/dinnertime/reclassic/internal/db/gen"
)

// SessionStore는 요청에서 사용자를 찾고 세션을 발급·폐기한다.
type SessionStore interface {
	User(ctx context.Context, r *http.Request) (*gendb.User, error)
	Issue(ctx context.Context, w http.ResponseWriter, userID int64, userAgent string) error
	Revoke(ctx context.Context, w http.ResponseWriter, r *http.Request) error
}

// GoogleLogin은 OAuth 흐름이다.
type GoogleLogin interface {
	Start(w http.ResponseWriter) (string, error)
	Callback(ctx context.Context, w http.ResponseWriter, r *http.Request, code, state string) (*gendb.User, error)
	SuccessRedirect() string
}

// 인증이 필요한 오퍼레이션. openapi.yaml의 security 선언과 같아야 한다 —
// 여기 넣는 것을 잊으면 무인증으로 열린다.
var authedOperations = map[string]bool{
	"CreateProposal":          true,
	"ReviewProposal":          true,
	"GetCurrentUser":          true,
	"Logout":                  true,
	"RequestBook":             true,
	"CreateProject":           true,
	"ListNeedsReviewBooks":    true,
	"ListOrphanedSuccessions": true,
	"ListUsers":               true,
	"SetUserRole":             true,
	"SetProjectStatus":        true,
	"ListAdminProjects":       true,
}

// 그중 admin만 할 수 있는 것.
var adminOperations = map[string]bool{
	"RequestBook":             true,
	"CreateProject":           true,
	"ListNeedsReviewBooks":    true,
	"ListOrphanedSuccessions": true,
	"ListUsers":               true,
	"SetUserRole":             true,
	"SetProjectStatus":        true,
	"ListAdminProjects":       true,
}

type userKey struct{}

// withSession은 세션이 있으면 사용자를 컨텍스트에 넣는다. 거부하지는 않는다 —
// 읽기 화면은 비로그인도 볼 수 있어야 하므로 거부는 authGuard가 오퍼레이션별로 한다.
func (s *Server) withSession(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, err := s.sessions.User(r.Context(), r)
		if err == nil {
			r = r.WithContext(context.WithValue(r.Context(), userKey{}, user))
		} else if !errors.Is(err, auth.ErrNoSession) {
			// DB 장애를 로그인 안 함으로 조용히 넘기지 않는다.
			s.log.ErrorContext(r.Context(), "세션 조회 실패", slog.String("err", err.Error()))
		}
		next.ServeHTTP(w, r)
	})
}

func userFrom(ctx context.Context) (*gendb.User, bool) {
	u, ok := ctx.Value(userKey{}).(*gendb.User)
	return u, ok
}

// authGuard는 오퍼레이션별로 로그인과 권한을 확인한다.
func (s *Server) authGuard(next gen.StrictHandlerFunc, operationID string) gen.StrictHandlerFunc {
	return func(ctx context.Context, w http.ResponseWriter, r *http.Request, request any) (any, error) {
		if !authedOperations[operationID] {
			return next(ctx, w, r, request)
		}

		user, ok := userFrom(ctx)
		if !ok {
			return unauthorized(operationID), nil
		}
		if adminOperations[operationID] && !auth.IsAdmin(user.Role) {
			return forbidden(operationID), nil
		}
		return next(ctx, w, r, request)
	}
}

// 생성 코드가 오퍼레이션마다 다른 응답 타입을 만들어서 갈라 준다.
func unauthorized(operationID string) any {
	const msg = "로그인이 필요하다"
	switch operationID {
	case "RequestBook":
		return gen.RequestBook401JSONResponse{Message: msg}
	case "CreateProject":
		return gen.CreateProject401JSONResponse{Message: msg}
	case "CreateProposal":
		return gen.CreateProposal401JSONResponse{Message: msg}
	case "ReviewProposal":
		return gen.ReviewProposal401JSONResponse{Message: msg}
	case "ListNeedsReviewBooks":
		return gen.ListNeedsReviewBooks401JSONResponse{Message: msg}
	case "ListOrphanedSuccessions":
		return gen.ListOrphanedSuccessions401JSONResponse{Message: msg}
	case "ListUsers":
		return gen.ListUsers401JSONResponse{Message: msg}
	case "SetUserRole":
		return gen.SetUserRole401JSONResponse{Message: msg}
	case "SetProjectStatus":
		return gen.SetProjectStatus401JSONResponse{Message: msg}
	case "ListAdminProjects":
		return gen.ListAdminProjects401JSONResponse{Message: msg}
	default:
		return gen.GetCurrentUser401JSONResponse{Message: msg}
	}
}

func forbidden(operationID string) any {
	const msg = "관리자 권한이 없다"
	switch operationID {
	case "CreateProject":
		return gen.CreateProject403JSONResponse{Message: msg}
	case "ListNeedsReviewBooks":
		return gen.ListNeedsReviewBooks403JSONResponse{Message: msg}
	case "ListOrphanedSuccessions":
		return gen.ListOrphanedSuccessions403JSONResponse{Message: msg}
	case "ListUsers":
		return gen.ListUsers403JSONResponse{Message: msg}
	case "SetUserRole":
		return gen.SetUserRole403JSONResponse{Message: msg}
	case "SetProjectStatus":
		return gen.SetProjectStatus403JSONResponse{Message: msg}
	case "ListAdminProjects":
		return gen.ListAdminProjects403JSONResponse{Message: msg}
	default:
		return gen.RequestBook403JSONResponse{Message: msg}
	}
}

// GetCurrentUser는 로그인 상태를 돌려준다. 웹이 헤더를 그리는 데 쓴다.
func (s *Server) GetCurrentUser(ctx context.Context, _ gen.GetCurrentUserRequestObject) (gen.GetCurrentUserResponseObject, error) {
	user, ok := userFrom(ctx)
	if !ok {
		return gen.GetCurrentUser401JSONResponse{Message: "로그인이 필요하다"}, nil
	}
	return gen.GetCurrentUser200JSONResponse{
		Handle:      user.Handle,
		DisplayName: user.DisplayName,
		Role:        gen.CurrentUserRole(user.Role),
	}, nil
}

// Logout은 세션을 즉시 무효화한다.
// 서명 쿠키가 아니라 테이블을 고른 이유가 이것이다 (ADR-027).
func (s *Server) Logout(ctx context.Context, _ gen.LogoutRequestObject) (gen.LogoutResponseObject, error) {
	w, r, ok := httpFrom(ctx)
	if !ok {
		return nil, errors.New("요청 컨텍스트가 없다")
	}
	if err := s.sessions.Revoke(ctx, w, r); err != nil {
		return nil, err
	}
	return gen.Logout204Response{}, nil
}

// OAuth 리다이렉트 엔드포인트는 openapi.yaml에 넣지 않는다.
// JSON을 주고받지 않고 브라우저가 직접 이동하는 302라서, 계약에 넣어도
// 생성기가 쓸모없는 클라이언트를 만들 뿐이다. 라우터에 직접 단다.
func (s *Server) mountOAuth(r chi.Router) {
	r.Get("/auth/google/start", func(w http.ResponseWriter, r *http.Request) {
		url, err := s.google.Start(w)
		if err != nil {
			s.log.ErrorContext(r.Context(), "로그인 시작 실패", slog.String("err", err.Error()))
			http.Error(w, "로그인을 시작할 수 없다", http.StatusInternalServerError)
			return
		}
		http.Redirect(w, r, url, http.StatusFound)
	})

	r.Get("/auth/google/callback", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if errParam := q.Get("error"); errParam != "" {
			// 사용자가 동의를 거부한 경우다. 실패로 시끄럽게 굴 필요는 없다.
			http.Redirect(w, r, s.google.SuccessRedirect(), http.StatusFound)
			return
		}

		user, err := s.google.Callback(r.Context(), w, r, q.Get("code"), q.Get("state"))
		if err != nil {
			if errors.Is(err, auth.ErrBadState) {
				// state 불일치는 로그인 CSRF 시도일 수 있다. 조용히 넘기지 않는다.
				s.log.WarnContext(r.Context(), "OAuth state 불일치")
				http.Error(w, "잘못된 로그인 요청", http.StatusBadRequest)
				return
			}
			s.log.ErrorContext(r.Context(), "로그인 콜백 실패", slog.String("err", err.Error()))
			http.Error(w, "로그인에 실패했다", http.StatusBadGateway)
			return
		}

		if err := s.sessions.Issue(r.Context(), w, user.ID, r.UserAgent()); err != nil {
			s.log.ErrorContext(r.Context(), "세션 발급 실패", slog.String("err", err.Error()))
			http.Error(w, "로그인에 실패했다", http.StatusInternalServerError)
			return
		}

		s.log.InfoContext(r.Context(), "로그인",
			slog.String("handle", user.Handle), slog.String("role", user.Role))
		http.Redirect(w, r, s.google.SuccessRedirect(), http.StatusFound)
	})
}

// Logout이 쿠키를 지우려면 ResponseWriter가 필요한데 strict 핸들러는 받지 못한다.
// 미들웨어에서 컨텍스트에 실어 준다.
type httpKey struct{}

type httpPair struct {
	w http.ResponseWriter
	r *http.Request
}

func withHTTP(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), httpKey{}, httpPair{w: w, r: r})))
	})
}

func httpFrom(ctx context.Context) (http.ResponseWriter, *http.Request, bool) {
	p, ok := ctx.Value(httpKey{}).(httpPair)
	if !ok {
		return nil, nil, false
	}
	return p.w, p.r, true
}
