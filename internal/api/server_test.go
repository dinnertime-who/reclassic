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
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/dinnertime/reclassic/internal/api/gen"
	"github.com/dinnertime/reclassic/internal/auth"
	"github.com/dinnertime/reclassic/internal/book"
	gendb "github.com/dinnertime/reclassic/internal/db/gen"
	"github.com/dinnertime/reclassic/internal/translate"
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
	if d.Catalog == nil {
		d.Catalog = fakeCatalog{}
	}
	if d.Users == nil {
		d.Users = &fakeUsers{byID: map[int64]*auth.UserItem{
			2: {ID: 2, Handle: "alice", DisplayName: "Alice", Role: auth.RoleMember},
		}}
	}
	if d.Translate == nil {
		d.Translate = &fakeTranslator{}
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
				Translate: &fakeTranslator{},
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
	if got.Handle != "tester" || got.Role != gen.CurrentUserRoleReviewer {
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

// fakeTranslator는 검수·목록·상태 전이를 DB 없이 보기 위한 대역이다.
type fakeTranslator struct {
	Translator
	projects    []translate.ProjectItem
	stored      *gendb.TranslationProject
	contents    *translate.ContentsView
	contentsErr error
}

func (*fakeTranslator) Approve(context.Context, int64, int64, string) (*gendb.ParagraphTranslation, error) {
	return &gendb.ParagraphTranslation{}, nil
}

func (f *fakeTranslator) Contents(context.Context, int64) (*translate.ContentsView, error) {
	if f.contentsErr != nil {
		return nil, f.contentsErr
	}
	if f.contents == nil {
		return &translate.ContentsView{Title: "Pride and Prejudice", TargetLang: "ko"}, nil
	}
	return f.contents, nil
}

func (*fakeTranslator) CreateProject(_ context.Context, _ int, targetLang string) (*gendb.TranslationProject, error) {
	return &gendb.TranslationProject{ID: 1, BookID: 1, TargetLang: targetLang, Status: "open"}, nil
}

func (f *fakeTranslator) ListProjects(_ context.Context, status string) ([]translate.ProjectItem, error) {
	out := make([]translate.ProjectItem, 0, len(f.projects))
	for _, p := range f.projects {
		if status == "" || p.Status == status {
			out = append(out, p)
		}
	}
	return out, nil
}

func (f *fakeTranslator) SetProjectStatus(_ context.Context, id int64, status string) (*gendb.TranslationProject, error) {
	if f.stored == nil {
		f.stored = &gendb.TranslationProject{ID: id}
	}
	f.stored.ID = id
	f.stored.Status = status
	if status == "published" && !f.stored.PublishedAt.Valid {
		f.stored.PublishedAt = pgtype.Timestamptz{Time: time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC), Valid: true}
	}
	out := *f.stored
	return &out, nil
}

type fakeCatalog struct {
	published   []book.CatalogItem
	needsReview []book.NeedsReviewItem
	orphans     []book.OrphanItem
}

func (f fakeCatalog) ListPublished(context.Context) ([]book.CatalogItem, error) {
	if f.published == nil {
		return []book.CatalogItem{}, nil
	}
	return f.published, nil
}

func (f fakeCatalog) ListNeedsReview(context.Context) ([]book.NeedsReviewItem, error) {
	if f.needsReview == nil {
		return []book.NeedsReviewItem{}, nil
	}
	return f.needsReview, nil
}

func (f fakeCatalog) ListOrphans(context.Context) ([]book.OrphanItem, error) {
	if f.orphans == nil {
		return []book.OrphanItem{}, nil
	}
	return f.orphans, nil
}

type fakeUsers struct {
	items []auth.UserItem
	byID  map[int64]*auth.UserItem
}

func (f *fakeUsers) List(context.Context) ([]auth.UserItem, error) {
	if f.items == nil {
		return []auth.UserItem{}, nil
	}
	return f.items, nil
}

func (f *fakeUsers) SetRole(_ context.Context, id int64, role string) (*auth.UserItem, error) {
	if role != auth.RoleMember && role != auth.RoleReviewer {
		return nil, auth.ErrInvalidRole
	}
	u, ok := f.byID[id]
	if !ok {
		return nil, auth.ErrUserNotFound
	}
	u.Role = role
	return u, nil
}

func postJSON(srv *Server, path, body string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	srv.Router().ServeHTTP(rec, req)
	return rec
}

func getPath(srv *Server, path string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	srv.Router().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	return rec
}

// 새 관리자 오퍼레이션마다 비로그인 401 · 비관리자 403 · 관리자 2xx.
// auth.go의 맵·switch에서 하나라도 빠지면 여기가 잡는다 — 테스트·빌드·배포가
// 성공한 채로 관리자 API가 열리는 것을 막기 위한 그물이다.
func TestAdminOperationsAuthz(t *testing.T) {
	ops := []struct {
		name, method, path, body string
		admin                    int
	}{
		{"RequestBook", http.MethodPost, "/admin/books", `{"gutenbergId":1342,"title":"Pride and Prejudice"}`, http.StatusAccepted},
		{"CreateProject", http.MethodPost, "/admin/projects", `{"gutenbergId":1342,"targetLang":"ko"}`, http.StatusCreated},
		{"ListNeedsReviewBooks", http.MethodGet, "/admin/books/needs-review", "", http.StatusOK},
		{"ListOrphanedSuccessions", http.MethodGet, "/admin/successions/orphans", "", http.StatusOK},
		{"ListUsers", http.MethodGet, "/admin/users", "", http.StatusOK},
		{"SetUserRole", http.MethodPost, "/admin/users/2/role", `{"role":"reviewer"}`, http.StatusOK},
		{"SetProjectStatus", http.MethodPost, "/admin/projects/1/status", `{"status":"published"}`, http.StatusOK},
		{"ListAdminProjects", http.MethodGet, "/admin/projects", "", http.StatusOK},
	}
	roles := []struct {
		name string
		user *gendb.User
		want func(admin int) int
	}{
		{"비로그인", nil, func(int) int { return http.StatusUnauthorized }},
		{"member", userWithRole(auth.RoleMember), func(int) int { return http.StatusForbidden }},
		{"reviewer", userWithRole(auth.RoleReviewer), func(int) int { return http.StatusForbidden }},
		{"admin", userWithRole(auth.RoleAdmin), func(admin int) int { return admin }},
	}

	for _, op := range ops {
		for _, role := range roles {
			t.Run(op.name+"/"+role.name, func(t *testing.T) {
				srv := newTestServer(Deps{Sessions: &fakeSessions{user: role.user}})
				var rec *httptest.ResponseRecorder
				if op.method == http.MethodGet {
					rec = getPath(srv, op.path)
				} else {
					rec = postJSON(srv, op.path, op.body)
				}
				if rec.Code != role.want(op.admin) {
					t.Errorf("status = %d, want %d (body: %s)", rec.Code, role.want(op.admin), rec.Body.String())
				}
			})
		}
	}
}

func TestReadListEndpointsNeedNoLogin(t *testing.T) {
	srv := newTestServer(Deps{})
	for _, path := range []string{"/books", "/projects"} {
		if rec := getPath(srv, path); rec.Code != http.StatusOK {
			t.Errorf("%s status = %d, want 200", path, rec.Code)
		}
	}
}

// 목차는 읽기 경로다. 어떤 역할로도 200이고 비로그인도 200이다 —
// auth.go의 authedOperations에 ListProjectChapters가 실수로 들어가면 여기가 잡는다.
// 관리자 표(TestAdminOperationsAuthz)의 반대편 그물이다.
func TestListProjectChaptersAuthz(t *testing.T) {
	roles := []struct {
		name string
		user *gendb.User
	}{
		{"비로그인", nil},
		{"member", userWithRole(auth.RoleMember)},
		{"reviewer", userWithRole(auth.RoleReviewer)},
		{"admin", userWithRole(auth.RoleAdmin)},
	}
	for _, role := range roles {
		t.Run(role.name, func(t *testing.T) {
			srv := newTestServer(Deps{Sessions: &fakeSessions{user: role.user}})
			if rec := getPath(srv, "/projects/1/chapters"); rec.Code != http.StatusOK {
				t.Errorf("status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
			}
		})
	}
}

// 목차는 장마다 진행도를 담고, 진행도 0%인 장도 뺴지 않는다 —
// 원문은 있으므로 읽을 수 있고, 빠지면 그 장으로 갈 길이 없어진다.
func TestListProjectChaptersCarriesProgress(t *testing.T) {
	srv := newTestServer(Deps{Translate: &fakeTranslator{contents: &translate.ContentsView{
		Title:      "Pride and Prejudice",
		Author:     "Jane Austen",
		TargetLang: "ko",
		Progress:   translate.Coverage{Total: 30, Approved: 12},
		Chapters: []translate.ChapterProgress{
			{Idx: 0, Title: "CHAPTER I.", Coverage: translate.Coverage{Total: 20, Approved: 12}},
			{Idx: 1, Title: "", Coverage: translate.Coverage{Total: 10, Approved: 0}},
		},
	}}})

	rec := getPath(srv, "/projects/7/chapters")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}

	var got gen.ProjectChapterList
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("디코드: %v", err)
	}
	if got.Book.Title != "Pride and Prejudice" || got.Book.Author == nil || *got.Book.Author != "Jane Austen" {
		t.Errorf("book = %+v", got.Book)
	}
	if got.Progress.Approved != 12 || got.Progress.Total != 30 || got.Progress.Ratio != 0.4 {
		t.Errorf("progress = %+v, want 12/30 (0.4)", got.Progress)
	}
	if len(got.Items) != 2 {
		t.Fatalf("items = %+v, want 2개 (0%%인 장도 남는다)", got.Items)
	}
	if got.Items[0].Idx != 0 || got.Items[0].Title != "CHAPTER I." || got.Items[0].Coverage.Ratio != 0.6 {
		t.Errorf("items[0] = %+v", got.Items[0])
	}
	if got.Items[1].Coverage.Approved != 0 || got.Items[1].Coverage.Ratio != 0 {
		t.Errorf("items[1] = %+v, want 진행도 0", got.Items[1])
	}
}

// 활성 revision이 없는 책은 빈 목차가 아니라 404다.
// 빈 목록으로 내리면 화면이 "장이 없는 책"이라고 거짓말을 한다.
func TestListProjectChaptersNotFound(t *testing.T) {
	srv := newTestServer(Deps{Translate: &fakeTranslator{contentsErr: translate.ErrNotFound}})
	if rec := getPath(srv, "/projects/999/chapters"); rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}

func TestListBooksOnlyPublished(t *testing.T) {
	srv := newTestServer(Deps{Catalog: fakeCatalog{published: []book.CatalogItem{
		{GutenbergID: 1342, Title: "Pride and Prejudice", Author: "Jane Austen", ProjectID: 7, TargetLang: "ko"},
	}}})
	rec := getPath(srv, "/books")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}
	var got gen.BookList
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("디코드: %v", err)
	}
	if len(got.Items) != 1 || got.Items[0].ProjectId != 7 {
		t.Errorf("items = %+v", got.Items)
	}
}

func TestListProjectsPublicOnlyPublished(t *testing.T) {
	srv := newTestServer(Deps{Translate: &fakeTranslator{projects: []translate.ProjectItem{
		{ID: 1, Title: "열린 책", Status: "open", GutenbergID: 84, TargetLang: "ko"},
		{ID: 2, Title: "공개된 책", Status: "published", GutenbergID: 1342, TargetLang: "ko"},
	}}})
	rec := getPath(srv, "/projects")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d (body: %s)", rec.Code, rec.Body.String())
	}
	var got gen.ProjectList
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("디코드: %v", err)
	}
	if len(got.Items) != 1 || got.Items[0].Id != 2 {
		t.Errorf("공개 목록이 published만 있어야 한다: %+v", got.Items)
	}
}

func TestListAdminProjectsIncludesOpen(t *testing.T) {
	srv := newTestServer(Deps{
		Sessions: &fakeSessions{user: userWithRole(auth.RoleAdmin)},
		Translate: &fakeTranslator{projects: []translate.ProjectItem{
			{ID: 1, Title: "열린 책", Status: "open", GutenbergID: 84, TargetLang: "ko"},
			{ID: 2, Title: "공개된 책", Status: "published", GutenbergID: 1342, TargetLang: "ko"},
		}},
	})
	rec := getPath(srv, "/admin/projects")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d (body: %s)", rec.Code, rec.Body.String())
	}
	var got gen.ProjectList
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("디코드: %v", err)
	}
	if len(got.Items) != 2 {
		t.Errorf("관리자 목록은 open도 있어야 한다: %+v", got.Items)
	}
}

func TestListNeedsReviewIncludesCounts(t *testing.T) {
	srv := newTestServer(Deps{
		Sessions: &fakeSessions{user: userWithRole(auth.RoleAdmin)},
		Catalog: fakeCatalog{needsReview: []book.NeedsReviewItem{
			{GutenbergID: 100, Title: "The Complete Works of William Shakespeare", ChapterCount: 1684, ParagraphCount: 39354},
		}},
	})
	rec := getPath(srv, "/admin/books/needs-review")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d (body: %s)", rec.Code, rec.Body.String())
	}
	var got gen.NeedsReviewBookList
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("디코드: %v", err)
	}
	if len(got.Items) != 1 || got.Items[0].ChapterCount != 1684 || got.Items[0].ParagraphCount != 39354 {
		t.Errorf("items = %+v", got.Items)
	}
}

func TestListUsersOmitsEmail(t *testing.T) {
	srv := newTestServer(Deps{
		Sessions: &fakeSessions{user: userWithRole(auth.RoleAdmin)},
		Users: &fakeUsers{items: []auth.UserItem{
			{ID: 1, Handle: "boss", DisplayName: "Boss", Role: auth.RoleAdmin},
		}},
	})
	rec := getPath(srv, "/admin/users")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d (body: %s)", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "email") || strings.Contains(rec.Body.String(), "@") {
		t.Errorf("사용자 목록에 이메일이 있다: %s", rec.Body.String())
	}
}

func TestSetUserRoleRejectsSelf(t *testing.T) {
	srv := newTestServer(Deps{
		Sessions: &fakeSessions{user: userWithRole(auth.RoleAdmin)},
		Users:    &fakeUsers{byID: map[int64]*auth.UserItem{1: {ID: 1, Handle: "tester", Role: auth.RoleAdmin}}},
	})
	rec := postJSON(srv, "/admin/users/1/role", `{"role":"reviewer"}`)
	if rec.Code != http.StatusConflict {
		t.Errorf("자기 자신 강등 status = %d, want 409 (body: %s)", rec.Code, rec.Body.String())
	}
}

func TestSetUserRoleSurvivesRelogin(t *testing.T) {
	users := &fakeUsers{byID: map[int64]*auth.UserItem{
		2: {ID: 2, Handle: "alice", DisplayName: "Alice", Role: auth.RoleMember},
	}}
	srv := newTestServer(Deps{
		Sessions: &fakeSessions{user: userWithRole(auth.RoleAdmin)},
		Users:    users,
	})
	rec := postJSON(srv, "/admin/users/2/role", `{"role":"reviewer"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}
	if users.byID[2].Role != auth.RoleReviewer {
		t.Fatalf("부여 후 role = %q, want reviewer", users.byID[2].Role)
	}
	// 재로그인. ADMIN_EMAIL이 아니면 fromLogin은 member다.
	// Promote가 reviewer를 지키지 않으면 D3는 닫히지 않는다.
	if got := auth.Promote(users.byID[2].Role, auth.RoleMember); got != auth.RoleReviewer {
		t.Errorf("재로그인 후 role = %q, want reviewer", got)
	}
}

func TestSetProjectStatusKeepsPublishedAt(t *testing.T) {
	first := time.Date(2026, 3, 1, 9, 0, 0, 0, time.UTC)
	tr := &fakeTranslator{stored: &gendb.TranslationProject{
		ID:          1,
		BookID:      10,
		TargetLang:  "ko",
		Status:      "published",
		PublishedAt: pgtype.Timestamptz{Time: first, Valid: true},
	}}
	srv := newTestServer(Deps{
		Sessions:  &fakeSessions{user: userWithRole(auth.RoleAdmin)},
		Translate: tr,
	})
	rec := postJSON(srv, "/admin/projects/1/status", `{"status":"open"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}
	var got gen.Project
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("디코드: %v", err)
	}
	if got.Status != gen.ProjectStatusOpen {
		t.Errorf("status = %q, want open", got.Status)
	}
	if got.PublishedAt == nil || !got.PublishedAt.Equal(first) {
		t.Errorf("publishedAt = %v, want %v (ADR-036: 내려올 때 비우지 않는다)", got.PublishedAt, first)
	}
}
