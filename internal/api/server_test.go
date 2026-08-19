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
	"github.com/dinnertime/reclassic/internal/book"
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

const testAdminToken = "test-admin-token"

// newTestServer는 빠진 의존성을 기본 대역으로 채운다.
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
	d.AdminToken = testAdminToken
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

func postBook(srv *Server, token string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	body := strings.NewReader(`{"gutenbergId":1342,"title":"Pride and Prejudice"}`)
	req := httptest.NewRequest(http.MethodPost, "/admin/books", body)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("X-Admin-Token", token)
	}
	srv.Router().ServeHTTP(rec, req)
	return rec
}

func TestRequestBookAccepted(t *testing.T) {
	srv := newTestServer(Deps{Requester: fakeRequester{bookID: 42}})

	rec := postBook(srv, testAdminToken)
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
func TestRequestBookRejectsBadToken(t *testing.T) {
	for _, token := range []string{"", "wrong-token"} {
		srv := newTestServer(Deps{Requester: fakeRequester{bookID: 42}})
		if rec := postBook(srv, token); rec.Code != http.StatusUnauthorized {
			t.Errorf("token %q → status %d, want 401", token, rec.Code)
		}
	}
}

func TestRequestBookConflict(t *testing.T) {
	srv := newTestServer(Deps{Requester: fakeRequester{err: book.ErrAlreadyRequested}})
	if rec := postBook(srv, testAdminToken); rec.Code != http.StatusConflict {
		t.Errorf("status = %d, want 409", rec.Code)
	}
}

// 읽기 엔드포인트는 관리자 토큰 없이 열려 있어야 한다.
func TestReadEndpointsNeedNoToken(t *testing.T) {
	srv := newTestServer(Deps{Reader: fakeReader{view: &book.ChapterView{TotalChapters: 1}}})

	rec := httptest.NewRecorder()
	srv.Router().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/books/1342/chapters/0", nil))
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
}
