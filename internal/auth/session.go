// Package auth는 Google 로그인과 세션을 담당한다.
//
// 비밀번호를 다루지 않는다. 계정은 Google에 위임하고,
// 이 패키지는 "누구인지"를 세션 쿠키로 유지하는 일만 한다 (ADR-027).
package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	gen "github.com/dinnertime/reclassic/internal/db/gen"
)

// SessionCookie는 세션 식별자를 담는 쿠키 이름이다.
const SessionCookie = "reclassic_session"

// SessionTTL은 세션 수명이다. 갱신하지 않는다 — 필요해지면 그때 넣는다.
const SessionTTL = 30 * 24 * time.Hour

var ErrNoSession = errors.New("세션이 없거나 만료됐다")

// CookieConfig는 배포 환경에 따라 달라지는 쿠키 속성이다.
type CookieConfig struct {
	// Secure는 로컬 http에서는 꺼야 쿠키가 붙는다.
	// 프로덕션에서 켜는 것을 잊지 말 것.
	Secure bool
	// Domain은 웹과 API가 다른 서브도메인일 때 상위 도메인으로 공유하기 위한 것이다.
	// 로컬에서는 비운다.
	Domain string
}

type Sessions struct {
	pool   *pgxpool.Pool
	cookie CookieConfig
}

func NewSessions(pool *pgxpool.Pool, cookie CookieConfig) *Sessions {
	return &Sessions{pool: pool, cookie: cookie}
}

// newToken은 세션 토큰을 만든다. 이 값은 쿠키에만 들어가고 DB에는 해시가 들어간다.
func newToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate session token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

// hashToken은 DB에 저장할 값이다.
// 토큰을 평문으로 저장하면 DB 유출이 곧 전 사용자 세션 탈취가 된다.
func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// Issue는 세션을 만들고 쿠키를 굽는다.
func (s *Sessions) Issue(ctx context.Context, w http.ResponseWriter, userID int64, userAgent string) error {
	token, err := newToken()
	if err != nil {
		return err
	}

	expires := time.Now().Add(SessionTTL)
	if _, err := gen.New(s.pool).CreateSession(ctx, gen.CreateSessionParams{
		ID:        hashToken(token),
		UserID:    userID,
		ExpiresAt: pgtype.Timestamptz{Time: expires, Valid: true},
		UserAgent: truncate(userAgent, 500),
	}); err != nil {
		return fmt.Errorf("create session: %w", err)
	}

	http.SetCookie(w, &http.Cookie{
		Name:  SessionCookie,
		Value: token,
		Path:  "/",
		// JS가 읽을 수 있으면 XSS 하나로 세션이 털린다.
		HttpOnly: true,
		Secure:   s.cookie.Secure,
		Domain:   s.cookie.Domain,
		// OAuth 콜백이 top-level GET이라 Lax로 충분하다.
		SameSite: http.SameSiteLaxMode,
		Expires:  expires,
	})
	return nil
}

// User는 요청의 세션 쿠키로 사용자를 찾는다.
// 만료된 세션은 쿼리에서 걸러진다 — 정리 잡을 기다리지 않는다.
func (s *Sessions) User(ctx context.Context, r *http.Request) (*gen.User, error) {
	c, err := r.Cookie(SessionCookie)
	if err != nil || c.Value == "" {
		return nil, ErrNoSession
	}

	row, err := gen.New(s.pool).GetSessionUser(ctx, hashToken(c.Value))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNoSession
		}
		return nil, fmt.Errorf("get session user: %w", err)
	}
	return &row.User, nil
}

// Revoke는 세션을 즉시 무효화한다.
// 서명 쿠키 대신 테이블을 고른 이유가 이것이다 (ADR-027).
func (s *Sessions) Revoke(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
	if c, err := r.Cookie(SessionCookie); err == nil && c.Value != "" {
		if err := gen.New(s.pool).DeleteSession(ctx, hashToken(c.Value)); err != nil {
			return fmt.Errorf("delete session: %w", err)
		}
	}
	s.clearCookie(w)
	return nil
}

func (s *Sessions) clearCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookie,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   s.cookie.Secure,
		Domain:   s.cookie.Domain,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
