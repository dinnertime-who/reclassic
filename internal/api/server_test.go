package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
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
			srv := NewServer(fakePinger{err: tt.pingErr}, fakeReader{}, "test", discardLogger())

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
	srv := NewServer(fakePinger{}, fakeReader{view: view}, "test", discardLogger())

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
	srv := NewServer(fakePinger{}, fakeReader{err: book.ErrNotFound}, "test", discardLogger())

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/books/100/chapters/0", nil)
	srv.Router().ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}
