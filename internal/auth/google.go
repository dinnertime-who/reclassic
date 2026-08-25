package auth

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"

	gen "github.com/dinnertime/reclassic/internal/db/gen"
)

// stateCookie는 로그인 CSRF를 막는 짧은 수명 쿠키다.
const stateCookie = "reclassic_oauth_state"

const stateTTL = 10 * time.Minute

// userInfoURL은 액세스 토큰으로 프로필을 읽는 곳이다.
//
// ID 토큰(JWT)을 직접 검증하지 않는다. 코드 교환이 서버 대 서버 TLS이므로
// 받은 토큰으로 Google에 한 번 더 물으면 된다.
// JWKS 캐싱과 서명 검증을 들이지 않는 만큼 틀릴 여지가 준다.
const userInfoURL = "https://www.googleapis.com/oauth2/v3/userinfo"

var (
	ErrBadState    = errors.New("state가 없거나 일치하지 않는다")
	ErrNotVerified = errors.New("Google이 이메일을 검증하지 않았다")
)

// GoogleProfile은 userinfo 응답 중 쓰는 것만 담는다.
type GoogleProfile struct {
	Sub           string `json:"sub"`
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
	Name          string `json:"name"`
}

// ProfileFetcher는 액세스 토큰으로 프로필을 읽는다.
// 테스트에서 Google을 실제로 부르지 않기 위해 인터페이스로 둔다.
type ProfileFetcher interface {
	Profile(ctx context.Context, client *http.Client) (*GoogleProfile, error)
}

type googleFetcher struct{}

func (googleFetcher) Profile(ctx context.Context, client *http.Client) (*GoogleProfile, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, userInfoURL, nil)
	if err != nil {
		return nil, fmt.Errorf("build userinfo request: %w", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch userinfo: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("userinfo → %d", resp.StatusCode)
	}
	var p GoogleProfile
	if err := json.NewDecoder(resp.Body).Decode(&p); err != nil {
		return nil, fmt.Errorf("decode userinfo: %w", err)
	}
	return &p, nil
}

// GoogleConfig는 로그인에 필요한 설정이다.
type GoogleConfig struct {
	ClientID     string
	ClientSecret string
	// RedirectURL은 API가 콜백을 받는 주소다. 클라이언트 시크릿을 쓰므로 웹이 아니라 API다.
	RedirectURL string
	// AdminEmail과 일치하면 admin을 준다. 마스터 계정이다 (ADR-027).
	AdminEmail string
	// SuccessRedirect는 로그인 후 브라우저를 보낼 웹 주소다.
	SuccessRedirect string
}

type Google struct {
	pool    *pgxpool.Pool
	oauth   *oauth2.Config
	fetcher ProfileFetcher
	cfg     GoogleConfig
	cookie  CookieConfig
}

func NewGoogle(pool *pgxpool.Pool, cfg GoogleConfig, cookie CookieConfig) *Google {
	return &Google{
		pool: pool,
		oauth: &oauth2.Config{
			ClientID:     cfg.ClientID,
			ClientSecret: cfg.ClientSecret,
			RedirectURL:  cfg.RedirectURL,
			Endpoint:     google.Endpoint,
			// 필요한 최소만 요청한다. 더 받아봐야 보관 책임만 는다.
			Scopes: []string{"openid", "email", "profile"},
		},
		fetcher: googleFetcher{},
		cfg:     cfg,
		cookie:  cookie,
	}
}

// SetProfileFetcher는 테스트에서 Google 호출을 대역으로 바꾼다.
func (g *Google) SetProfileFetcher(f ProfileFetcher) { g.fetcher = f }

// Start는 state를 굽고 Google 동의 화면으로 보낼 주소를 만든다.
func (g *Google) Start(w http.ResponseWriter) (string, error) {
	buf := make([]byte, 24)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate state: %w", err)
	}
	state := base64.RawURLEncoding.EncodeToString(buf)

	http.SetCookie(w, &http.Cookie{
		Name:     stateCookie,
		Value:    state,
		Path:     "/",
		HttpOnly: true,
		Secure:   g.cookie.Secure,
		Domain:   g.cookie.Domain,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(stateTTL.Seconds()),
	})

	return g.oauth.AuthCodeURL(state), nil
}

// Callback은 state를 대조하고 코드를 교환해 사용자를 확정한다.
func (g *Google) Callback(ctx context.Context, w http.ResponseWriter, r *http.Request, code, state string) (*gen.User, error) {
	if err := g.checkState(r, state); err != nil {
		return nil, err
	}
	g.clearState(w)

	token, err := g.oauth.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("exchange code: %w", err)
	}

	profile, err := g.fetcher.Profile(ctx, g.oauth.Client(ctx, token))
	if err != nil {
		return nil, err
	}
	if profile.Sub == "" {
		return nil, errors.New("Google이 sub를 주지 않았다")
	}

	return g.upsertUser(ctx, profile)
}

// checkState는 쿠키의 state와 쿼리의 state를 상수 시간 비교한다.
// 이걸 빼면 로그인 CSRF가 열린다 — 공격자가 자기 계정으로 피해자를 로그인시킨다.
func (g *Google) checkState(r *http.Request, state string) error {
	c, err := r.Cookie(stateCookie)
	if err != nil || c.Value == "" || state == "" {
		return ErrBadState
	}
	if subtle.ConstantTimeCompare([]byte(c.Value), []byte(state)) != 1 {
		return ErrBadState
	}
	return nil
}

func (g *Google) clearState(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name: stateCookie, Value: "", Path: "/",
		HttpOnly: true, Secure: g.cookie.Secure, Domain: g.cookie.Domain,
		SameSite: http.SameSiteLaxMode, MaxAge: -1,
	})
}

// upsertUser는 google_sub으로 사용자를 찾거나 만든다.
// 이메일이 아니라 sub으로 찾는다 — 이메일은 바뀌고 sub은 안 바뀐다.
func (g *Google) upsertUser(ctx context.Context, p *GoogleProfile) (*gen.User, error) {
	q := gen.New(g.pool)
	role := g.roleFor(p)

	existing, err := q.GetUserByGoogleSub(ctx, text(p.Sub))
	switch {
	case err == nil:
		// 역할은 매 로그인마다 다시 판정한다.
		// ADMIN_EMAIL에서 빼면 다음 로그인에 권한이 사라져야 한다.
		updated, err := q.UpdateUserFromGoogle(ctx, gen.UpdateUserFromGoogleParams{
			ID:          existing.ID,
			DisplayName: displayName(p),
			Email:       text(p.Email),
			Role:        Promote(existing.Role, role),
		})
		if err != nil {
			return nil, fmt.Errorf("update user: %w", err)
		}
		return &updated, nil
	case errors.Is(err, pgx.ErrNoRows):
		handle, err := g.uniqueHandle(ctx, p.Email)
		if err != nil {
			return nil, err
		}
		created, err := q.CreateGoogleUser(ctx, gen.CreateGoogleUserParams{
			Handle:      handle,
			DisplayName: displayName(p),
			Role:        role,
			Email:       text(p.Email),
			GoogleSub:   text(p.Sub),
		})
		if err != nil {
			return nil, fmt.Errorf("create user: %w", err)
		}
		return &created, nil
	default:
		return nil, fmt.Errorf("lookup user by sub: %w", err)
	}
}

// roleFor는 마스터 계정만 admin으로 올린다.
// 검증되지 않은 이메일은 대조하지 않는다 — Google이 확인해 준 것만 믿는다.
func (g *Google) roleFor(p *GoogleProfile) string {
	if g.cfg.AdminEmail == "" || !p.EmailVerified {
		return RoleMember
	}
	if strings.EqualFold(strings.TrimSpace(p.Email), strings.TrimSpace(g.cfg.AdminEmail)) {
		return RoleAdmin
	}
	return RoleMember
}

// Promote는 로그인으로 부여할 역할과 기존 역할 중 높은 쪽을 남긴다.
// 손으로 올려준 reviewer를 로그인할 때마다 member로 되돌리지 않기 위함이다.
// 다만 admin은 ADMIN_EMAIL이 유일한 출처라 여기서 유지되지 않는다.
func Promote(existing, fromLogin string) string {
	if fromLogin == RoleAdmin {
		return RoleAdmin
	}
	if existing == RoleReviewer {
		return RoleReviewer
	}
	return fromLogin
}

// uniqueHandle은 이메일 로컬 파트에서 handle을 만들고 충돌하면 숫자를 붙인다.
func (g *Google) uniqueHandle(ctx context.Context, email string) (string, error) {
	base := sanitizeHandle(email)
	q := gen.New(g.pool)

	for i := 0; i < 50; i++ {
		candidate := base
		if i > 0 {
			candidate = base + strconv.Itoa(i+1)
		}
		taken, err := q.HandleExists(ctx, candidate)
		if err != nil {
			return "", fmt.Errorf("check handle: %w", err)
		}
		if !taken {
			return candidate, nil
		}
	}
	return "", errors.New("handle 후보를 찾지 못했다")
}

func sanitizeHandle(email string) string {
	local, _, _ := strings.Cut(email, "@")
	var b strings.Builder
	for _, r := range strings.ToLower(local) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '.' || r == '_' || r == '-':
			b.WriteRune('-')
		}
	}
	h := strings.Trim(b.String(), "-")
	if h == "" {
		return "user"
	}
	return h
}

func displayName(p *GoogleProfile) string {
	if p.Name != "" {
		return p.Name
	}
	return sanitizeHandle(p.Email)
}

func text(s string) pgtype.Text {
	if s == "" {
		return pgtype.Text{}
	}
	return pgtype.Text{String: s, Valid: true}
}

// SuccessRedirect는 로그인 후 브라우저를 보낼 웹 주소다.
func (g *Google) SuccessRedirect() string { return g.cfg.SuccessRedirect }
