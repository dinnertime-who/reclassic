package auth

// 역할. 마이그레이션의 CHECK 제약과 같아야 한다.
const (
	RoleMember   = "member"
	RoleReviewer = "reviewer"
	RoleAdmin    = "admin"
)

// CanReview는 검수 권한이다. 제안은 누구나 할 수 있다.
func CanReview(role string) bool {
	return role == RoleReviewer || role == RoleAdmin
}

// IsAdmin은 관리자 전용 조작 권한이다.
func IsAdmin(role string) bool {
	return role == RoleAdmin
}
