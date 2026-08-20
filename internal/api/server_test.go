package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/dinnertime/reclassic/internal/api/gen"
	"github.com/dinnertime/reclassic/internal/auth"
	"github.com/dinnertime/reclassic/internal/book"
	gendb "github.com/dinnertime/reclassic/internal/db/gen"
)

// fakePinger는 DB 없이 헬스체크를 검증하기 위한 대역이다.
// DATABASE_URL 없이 make test가 통과해야 한다 (SLICE_SKELETON §5.6).
type fakePinger struct{ err error }

func (f fakePinger) Ping(context.Context) error { return f.err }

// fakeReader는 DB 없이 챕터 핸들러를 검증하기 위한 대역이다.
type fakeReader struct {
	view *book.ChapterView
	err  error
}

func (f fakeReader) Chapter(context.Context, int, int) (*book.ChapterView, error) {
	return f.view, f.err
}

// fakeRequester는 트랜잭션 없이 수집 지시 핸들러를 검증하기 위한 대역이다.
type fakeRequester struct {
	bookID int64
	err    error
}

func (f fakeRequester) Request(context.Context, int, string, string) (int64, error) {
	return f.bookID, f.err
}

// fakeSessions는 고정된 사용자를 돌려주는 대역이다.
// nil이면 비로그인이다.
type fakeSessions struct {
	user    *gendb.User
	revoked bool
}

func (f *fakeSessions) User(context.Context, *http.Request) (*gendb.User, error) {
	if f.user == nil {
		return nil, auth.ErrNoSession
	}
	return f.user, nil
}

func (f *fakeSessions) Issue(context.Context, http.ResponseWriter, int64, string) error { return nil }

func (f *fakeSessions) Revoke(context.Context, http.ResponseWriter, *http.Request) error {
	f.revoked = true
	return nil
}

type fakeGoogle struct{}

func (fakeGoogle) Start(http.ResponseWriter) (string, error) {
	return "https://accounts.google.example", nil
}
func (fakeGoogle) Callback(context.Context, http.ResponseWriter, *http.Request, string, string) (*gendb.User, error) {
	return nil, errors.New("사용하지 않음")
}
func (fakeGoogle) SuccessRedirect() string { return "http://localhost:3000/" }

func userWithRole(role string) *gendb.User {
	return &gendb.User{ID: 1, Handle: "tester", DisplayName: "Tester", Role: role}
}

// newTestServer는 빠진 의존성을 기본 대역으로 채운다. 기본은 비로그인이다.
func newTestServer(d Deps) *Server {
	if d.DB == nil {
		d.DB = fakePinger{}
	}
	if d.Reader == nil {
		d.Reader = fakeReader{}
	}
	if d.Requester == nil {
		d.Requester = fakeRequester{}
	}
	if d.Sessions == nil {
		d.Sessions = &fakeSessions{}
	}
	if d.Google == nil {
		d.Google = fakeGoogle{}
	}
	d.Version = "test"
	d.Log = discardLogger()
	return NewServer(d)
}

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestHealthzReflectsPing(t *testing.T) {
	tests := []struct {
		name    string
		pingErr error
		wantDB  gen.HealthDb
	}{
		{name: "DB 연결됨", pingErr: nil, wantDB: gen.HealthDbOk},
		{name: "DB 죽음", pingErr: errors.New("connection refused"), wantDB: gen.HealthDbDown},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := newTestServer(Deps{DB: fakePinger{err: tt.pingErr}})

			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
			srv.Router().ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
			}

			var got gen.Health
			if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
				t.Fatalf("응답 디코드: %v", err)
			}
			if got.Db != tt.wantDB {
				t.Errorf("db = %q, want %q", got.Db, tt.wantDB)
			}
			if got.Status != gen.HealthStatusOk {
				t.Errorf("status = %q, want %q", got.Status, gen.HealthStatusOk)
			}
			if got.Version != "test" {
				t.Errorf("version = %q, want %q", got.Version, "test")
			}
		})
	}
}

func TestGetBookChapter(t *testing.T) {
	view := &book.ChapterView{
		Idx:           2,
		Title:         "CHAPTER III.",
		TotalChapters: 61,
		Paragraphs: []book.ParagraphView{
			{StableID: "abc123", SourceText: "It is a truth universally acknowledged…"},
		},
	}
	srv := newTestServer(Deps{Reader: fakeReader{view: view}})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/books/1342/chapters/2", nil)
	srv.Router().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}

	var got gen.ChapterView
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("응답 디코드: %v", err)
	}
	if got.Chapter.Idx != 2 || got.TotalChapters != 61 {
		t.Errorf("chapter = %+v, totalChapters = %d", got.Chapter, got.TotalChapters)
	}
	if len(got.Paragraphs) != 1 || got.Paragraphs[0].StableId != "abc123" {
		t.Errorf("paragraphs = %+v", got.Paragraphs)
	}
}

// 활성 revision이 없으면 404다. 게이트에 걸린 책이 읽기에 노출되면 안 된다.
func TestGetBookChapterNotFound(t *testing.T) {
	srv := newTestServer(Deps{Reader: fakeReader{err: book.ErrNotFound}})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/books/100/chapters/0", nil)
	srv.Router().ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func postBook(srv *Server) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	body := strings.NewReader(`{"gutenbergId":1342,"title":"Pride and Prejudice"}`)
	req := httptest.NewRequest(http.MethodPost, "/admin/books", body)
	req.Header.Set("Content-Type", "application/json")
	srv.Router().ServeHTTP(rec, req)
	return rec
}

func TestRequestBookAccepted(t *testing.T) {
	srv := newTestServer(Deps{
		Requester: fakeRequester{bookID: 42},
		Sessions:  &fakeSessions{user: userWithRole(auth.RoleAdmin)},
	})

	rec := postBook(srv)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202 (body: %s)", rec.Code, rec.Body.String())
	}

	var got gen.BookRequestAccepted
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("응답 디코드: %v", err)
	}
	if got.BookId != 42 {
		t.Errorf("bookId = %d, want 42", got.BookId)
	}
}

// 관리자 엔드포인트가 무인증으로 열리면 안 된다.
func TestRequestBookRequiresLogin(t *testing.T) {
	srv := newTestServer(Deps{Requester: fakeRequester{bookID: 42}})
	if rec := postBook(srv); rec.Code != http.StatusUnauthorized {
		t.Errorf("비로그인 → status %d, want 401", rec.Code)
	}
}

// 로그인했어도 admin이 아니면 403이다.
func TestRequestBookRequiresAdminRole(t *testing.T) {
	for _, role := range []string{auth.RoleMember, auth.RoleReviewer} {
		srv := newTestServer(Deps{
			Requester: fakeRequester{bookID: 42},
			Sessions:  &fakeSessions{user: userWithRole(role)},
		})
		if rec := postBook(srv); rec.Code != http.StatusForbidden {
			t.Errorf("role %q → status %d, want 403", role, rec.Code)
		}
	}
}

func TestRequestBookConflict(t *testing.T) {
	srv := newTestServer(Deps{
		Requester: fakeRequester{err: book.ErrAlreadyRequested},
		Sessions:  &fakeSessions{user: userWithRole(auth.RoleAdmin)},
	})
	if rec := postBook(srv); rec.Code != http.StatusConflict {
		t.Errorf("status = %d, want 409", rec.Code)
	}
}

// 읽기 엔드포인트는 비로그인도 볼 수 있어야 한다.
func TestReadEndpointsNeedNoLogin(t *testing.T) {
	srv := newTestServer(Deps{Reader: fakeReader{view: &book.ChapterView{TotalChapters: 1}}})

	rec := httptest.NewRecorder()
	srv.Router().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/books/1342/chapters/0", nil))
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
}

// 검수는 로그인 + reviewer 이상이어야 한다.
func TestReviewProposalRequiresReviewerRole(t *testing.T) {
	post := func(srv *Server) int {
		rec := httptest.NewRecorder()
		body := strings.NewReader(`{"action":"approve"}`)
		req := httptest.NewRequest(http.MethodPost, "/proposals/1/review", body)
		req.Header.Set("Content-Type", "application/json")
		srv.Router().ServeHTTP(rec, req)
		return rec.Code
	}

	tests := []struct {
		name string
		user *gendb.User
		want int
	}{
		{"비로그인", nil, http.StatusUnauthorized},
		{"member", userWithRole(auth.RoleMember), http.StatusForbidden},
		{"reviewer", userWithRole(auth.RoleReviewer), http.StatusOK},
		{"admin", userWithRole(auth.RoleAdmin), http.StatusOK},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := newTestServer(Deps{
				Sessions:  &fakeSessions{user: tt.user},
				Translate: fakeTranslator{},
			})
			if got := post(srv); got != tt.want {
				t.Errorf("status = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestCurrentUser(t *testing.T) {
	srv := newTestServer(Deps{Sessions: &fakeSessions{user: userWithRole(auth.RoleReviewer)}})

	rec := httptest.NewRecorder()
	srv.Router().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/auth/me", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	var got gen.CurrentUser
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("디코드: %v", err)
	}
	if got.Handle != "tester" || got.Role != gen.Reviewer {
		t.Errorf("user = %+v", got)
	}
}

func TestCurrentUserUnauthenticated(t *testing.T) {
	srv := newTestServer(Deps{})
	rec := httptest.NewRecorder()
	srv.Router().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/auth/me", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}

// 로그아웃은 세션을 즉시 폐기해야 한다.
func TestLogoutRevokesSession(t *testing.T) {
	sessions := &fakeSessions{user: userWithRole(auth.RoleMember)}
	srv := newTestServer(Deps{Sessions: sessions})

	rec := httptest.NewRecorder()
	srv.Router().ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/auth/logout", nil))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rec.Code)
	}
	if !sessions.revoked {
		t.Error("세션이 폐기되지 않았다")
	}
}

// 로그인 시작은 Google로 리다이렉트한다.
func TestGoogleStartRedirects(t *testing.T) {
	srv := newTestServer(Deps{})
	rec := httptest.NewRecorder()
	srv.Router().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/auth/google/start", nil))

	if rec.Code != http.StatusFound {
		t.Fatalf("status = %d, want 302", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "https://accounts.google.example" {
		t.Errorf("Location = %q", loc)
	}
}

// fakeTranslator는 검수 권한 검사만 보기 위한 최소 대역이다.
type fakeTranslator struct{ Translator }

func (fakeTranslator) Approve(context.Context, int64, int64, string) (*gendb.ParagraphTranslation, error) {
	return &gendb.ParagraphTranslation{}, nil
}
