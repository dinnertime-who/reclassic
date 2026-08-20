package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/dinnertime/reclassic/internal/config"
	"github.com/dinnertime/reclassic/internal/db"
	gen "github.com/dinnertime/reclassic/internal/db/gen"
)

func testPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	config.LoadDotEnv("../../.env")
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skip("DATABASE_URL 없음 — 통합 테스트 건너뜀")
	}
	pool, err := db.Connect(context.Background(), url)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	if err := pool.Ping(context.Background()); err != nil {
		t.Skipf("Postgres에 연결할 수 없음 (%v) — `make dev`로 띄울 것", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func testUser(t *testing.T, pool *pgxpool.Pool) int64 {
	t.Helper()
	u, err := gen.New(pool).UpsertUser(context.Background(), gen.UpsertUserParams{
		Handle: "session-test", DisplayName: "Session Test", Role: RoleMember,
	})
	if err != nil {
		t.Fatalf("upsert user: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), "DELETE FROM users WHERE handle = 'session-test'")
	})
	return u.ID
}

// 세션을 발급하면 쿠키로 사용자를 찾을 수 있어야 한다.
func TestIssueAndResolve(t *testing.T) {
	pool := testPool(t)
	userID := testUser(t, pool)
	s := NewSessions(pool, CookieConfig{})
	ctx := context.Background()

	rec := httptest.NewRecorder()
	if err := s.Issue(ctx, rec, userID, "test-agent"); err != nil {
		t.Fatalf("Issue: %v", err)
	}

	cookies := rec.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("쿠키 %d개", len(cookies))
	}
	token := cookies[0].Value

	// DB에는 토큰이 아니라 해시가 있어야 한다.
	var stored int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM sessions WHERE id = $1", token).Scan(&stored); err != nil {
		t.Fatalf("count by token: %v", err)
	}
	if stored != 0 {
		t.Error("DB에 토큰이 평문으로 저장돼 있다")
	}
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM sessions WHERE id = $1", hashToken(token)).Scan(&stored); err != nil {
		t.Fatalf("count by hash: %v", err)
	}
	if stored != 1 {
		t.Fatalf("해시로 찾은 세션 %d개, want 1", stored)
	}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(cookies[0])
	user, err := s.User(ctx, req)
	if err != nil {
		t.Fatalf("User: %v", err)
	}
	if user.ID != userID {
		t.Errorf("user.ID = %d, want %d", user.ID, userID)
	}
}

// 로그아웃하면 그 세션이 즉시 무효가 되어야 한다.
// 서명 쿠키 대신 테이블을 고른 이유가 이것이다 (ADR-027).
func TestRevokeIsImmediate(t *testing.T) {
	pool := testPool(t)
	userID := testUser(t, pool)
	s := NewSessions(pool, CookieConfig{})
	ctx := context.Background()

	rec := httptest.NewRecorder()
	if err := s.Issue(ctx, rec, userID, ""); err != nil {
		t.Fatalf("Issue: %v", err)
	}
	cookie := rec.Result().Cookies()[0]

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(cookie)
	if _, err := s.User(ctx, req); err != nil {
		t.Fatalf("폐기 전 User: %v", err)
	}

	if err := s.Revoke(ctx, httptest.NewRecorder(), req); err != nil {
		t.Fatalf("Revoke: %v", err)
	}
	if _, err := s.User(ctx, req); err != ErrNoSession {
		t.Errorf("폐기 후 err = %v, want ErrNoSession", err)
	}
}

// 만료된 세션은 정리 잡을 기다리지 않고 조회 단계에서 걸러져야 한다.
func TestExpiredSessionIsRejected(t *testing.T) {
	pool := testPool(t)
	userID := testUser(t, pool)
	s := NewSessions(pool, CookieConfig{})
	ctx := context.Background()

	token, err := newToken()
	if err != nil {
		t.Fatalf("newToken: %v", err)
	}
	if _, err := gen.New(pool).CreateSession(ctx, gen.CreateSessionParams{
		ID:        hashToken(token),
		UserID:    userID,
		ExpiresAt: pgtype.Timestamptz{Time: time.Now().Add(-time.Minute), Valid: true},
	}); err != nil {
		t.Fatalf("create expired session: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: SessionCookie, Value: token})
	if _, err := s.User(ctx, req); err != ErrNoSession {
		t.Errorf("err = %v, want ErrNoSession", err)
	}
}

// 쿠키가 없거나 값이 이상하면 조용히 비로그인이다.
func TestUnknownTokenIsNoSession(t *testing.T) {
	pool := testPool(t)
	s := NewSessions(pool, CookieConfig{})
	ctx := context.Background()

	for _, c := range []*http.Cookie{nil, {Name: SessionCookie, Value: "does-not-exist"}} {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		if c != nil {
			req.AddCookie(c)
		}
		if _, err := s.User(ctx, req); err != ErrNoSession {
			t.Errorf("err = %v, want ErrNoSession", err)
		}
	}
}
