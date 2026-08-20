package auth

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// 세션 토큰을 평문으로 저장하면 DB 유출이 곧 전 사용자 세션 탈취가 된다.
func TestHashTokenIsNotTheToken(t *testing.T) {
	token, err := newToken()
	if err != nil {
		t.Fatalf("newToken: %v", err)
	}
	h := hashToken(token)

	if h == token {
		t.Fatal("해시가 토큰과 같다")
	}
	if strings.Contains(h, token) {
		t.Fatal("해시에 토큰이 들어 있다")
	}
	if hashToken(token) != h {
		t.Error("같은 토큰인데 해시가 다르다")
	}

	other, _ := newToken()
	if token == other {
		t.Error("토큰이 반복된다 — 난수가 아니다")
	}
}

// 쿠키 속성이 하나라도 빠지면 세션이 새어 나간다.
func TestSessionCookieAttributes(t *testing.T) {
	for _, secure := range []bool{false, true} {
		s := NewSessions(nil, CookieConfig{Secure: secure, Domain: ".example.com"})
		rec := httptest.NewRecorder()
		s.clearCookie(rec)

		c := rec.Result().Cookies()[0]
		if c.Name != SessionCookie {
			t.Errorf("이름 = %q", c.Name)
		}
		if !c.HttpOnly {
			t.Error("HttpOnly가 꺼져 있다 — XSS 하나로 세션이 털린다")
		}
		if c.Secure != secure {
			t.Errorf("Secure = %v, want %v", c.Secure, secure)
		}
		if c.SameSite != http.SameSiteLaxMode {
			t.Errorf("SameSite = %v, want Lax", c.SameSite)
		}
		// RFC 6265에서 선행 점은 무시되므로 Go가 되읽을 때 떼어낸다.
		if c.Domain != "example.com" || c.Path != "/" {
			t.Errorf("Domain=%q Path=%q", c.Domain, c.Path)
		}
	}
}

// state가 없거나 다르면 로그인 CSRF다. 통과시키면 안 된다.
func TestCheckState(t *testing.T) {
	g := &Google{}

	tests := []struct {
		name          string
		cookie, query string
		wantErr       bool
	}{
		{"일치", "abc123", "abc123", false},
		{"불일치", "abc123", "different", true},
		{"쿠키 없음", "", "abc123", true},
		{"쿼리 없음", "abc123", "", true},
		{"둘 다 없음", "", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/auth/google/callback", nil)
			if tt.cookie != "" {
				r.AddCookie(&http.Cookie{Name: stateCookie, Value: tt.cookie})
			}
			err := g.checkState(r, tt.query)
			if (err != nil) != tt.wantErr {
				t.Errorf("err = %v, wantErr = %v", err, tt.wantErr)
			}
		})
	}
}
