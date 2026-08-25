package auth

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	gen "github.com/dinnertime/reclassic/internal/db/gen"
)

var (
	ErrUserNotFound = errors.New("사용자를 찾을 수 없다")
	ErrInvalidRole  = errors.New("부여할 수 없는 역할이다")
)

// Users는 목록과 역할 부여다. admin은 여기서 만들지 않는다 (ADR-027).
type Users struct {
	pool *pgxpool.Pool
}

func NewUsers(pool *pgxpool.Pool) *Users {
	return &Users{pool: pool}
}

// UserItem은 목록에 내리는 사용자다. 이메일은 없다.
type UserItem struct {
	ID          int64
	Handle      string
	DisplayName string
	Role        string
}

func (u *Users) List(ctx context.Context) ([]UserItem, error) {
	rows, err := gen.New(u.pool).ListUsers(ctx)
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}
	out := make([]UserItem, 0, len(rows))
	for _, row := range rows {
		out = append(out, UserItem{
			ID:          row.ID,
			Handle:      row.Handle,
			DisplayName: row.DisplayName,
			Role:        row.Role,
		})
	}
	return out, nil
}

// SetRole은 member ↔ reviewer만 받는다. admin 부여 경로가 아니다.
func (u *Users) SetRole(ctx context.Context, id int64, role string) (*UserItem, error) {
	if role != RoleMember && role != RoleReviewer {
		return nil, ErrInvalidRole
	}
	user, err := gen.New(u.pool).SetUserRole(ctx, gen.SetUserRoleParams{ID: id, Role: role})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("set user %d role %s: %w", id, role, err)
	}
	return &UserItem{
		ID:          user.ID,
		Handle:      user.Handle,
		DisplayName: user.DisplayName,
		Role:        user.Role,
	}, nil
}
