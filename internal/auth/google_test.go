package auth

import "testing"

func TestSanitizeHandle(t *testing.T) {
	tests := []struct{ email, want string }{
		{"dinnertime.dev@gmail.com", "dinnertime-dev"},
		{"Alice_B@example.com", "alice-b"},
		{"a.b-c_d@x.io", "a-b-c-d"},
		{"한글@example.com", "user"},  // 남는 글자가 없으면 기본값
		{"...@example.com", "user"}, // 앞뒤 하이픈은 잘라낸다
		{"x1@example.com", "x1"},
	}
	for _, tt := range tests {
		if got := sanitizeHandle(tt.email); got != tt.want {
			t.Errorf("sanitizeHandle(%q) = %q, want %q", tt.email, got, tt.want)
		}
	}
}

func TestRoleForOnlyPromotesVerifiedMasterEmail(t *testing.T) {
	g := &Google{cfg: GoogleConfig{AdminEmail: "boss@example.com"}}

	tests := []struct {
		name    string
		profile GoogleProfile
		want    string
	}{
		{"마스터 계정", GoogleProfile{Email: "boss@example.com", EmailVerified: true}, RoleAdmin},
		{"대소문자 무시", GoogleProfile{Email: "BOSS@Example.com", EmailVerified: true}, RoleAdmin},
		{"검증 안 된 이메일은 승격하지 않는다", GoogleProfile{Email: "boss@example.com"}, RoleMember},
		{"다른 사람", GoogleProfile{Email: "someone@example.com", EmailVerified: true}, RoleMember},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := g.roleFor(&tt.profile); got != tt.want {
				t.Errorf("roleFor = %q, want %q", got, tt.want)
			}
		})
	}
}

// ADMIN_EMAIL이 비면 아무도 admin이 되지 않는다.
func TestRoleForWithoutAdminEmail(t *testing.T) {
	g := &Google{cfg: GoogleConfig{}}
	if got := g.roleFor(&GoogleProfile{Email: "any@example.com", EmailVerified: true}); got != RoleMember {
		t.Errorf("roleFor = %q, want %q", got, RoleMember)
	}
}

// 손으로 올려준 reviewer가 로그인할 때마다 member로 되돌아가면 안 된다.
func TestPromoteKeepsManualReviewer(t *testing.T) {
	tests := []struct{ existing, fromLogin, want string }{
		{RoleReviewer, RoleMember, RoleReviewer},
		{RoleMember, RoleMember, RoleMember},
		{RoleMember, RoleAdmin, RoleAdmin},
		{RoleReviewer, RoleAdmin, RoleAdmin},
		// admin은 ADMIN_EMAIL이 유일한 출처다. 목록에서 빠지면 내려와야 한다.
		{RoleAdmin, RoleMember, RoleMember},
	}
	for _, tt := range tests {
		if got := Promote(tt.existing, tt.fromLogin); got != tt.want {
			t.Errorf("Promote(%q, %q) = %q, want %q", tt.existing, tt.fromLogin, got, tt.want)
		}
	}
}

func TestCanReviewAndIsAdmin(t *testing.T) {
	if CanReview(RoleMember) {
		t.Error("member가 검수할 수 있다고 나온다")
	}
	for _, r := range []string{RoleReviewer, RoleAdmin} {
		if !CanReview(r) {
			t.Errorf("%s가 검수할 수 없다고 나온다", r)
		}
	}
	if IsAdmin(RoleReviewer) {
		t.Error("reviewer가 admin으로 나온다")
	}
	if !IsAdmin(RoleAdmin) {
		t.Error("admin이 admin이 아니라고 나온다")
	}
}
