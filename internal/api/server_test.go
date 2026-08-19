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
)

// fakePinger는 DB 없이 헬스체크를 검증하기 위한 대역이다.
// DATABASE_URL 없이 make test가 통과해야 한다 (SLICE_SKELETON §5.6).
type fakePinger struct{ err error }

func (f fakePinger) Ping(context.Context) error { return f.err }

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
			srv := NewServer(fakePinger{err: tt.pingErr}, "test", discardLogger())

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
