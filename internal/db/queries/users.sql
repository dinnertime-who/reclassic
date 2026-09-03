-- 사용자 목록과 역할 부여. 이메일은 고르지 않는다 — 개인정보고 화면이 쓰지 않는다.

-- name: ListUsers :many
SELECT id, handle, display_name, role, created_at
FROM users
ORDER BY handle;

-- name: SetUserRole :one
UPDATE users SET role = $2 WHERE id = $1
RETURNING *;
