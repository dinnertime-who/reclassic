package translate

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	gen "github.com/dinnertime/reclassic/internal/db/gen"
)

// ProjectItem은 번역 프로젝트 목록의 한 행이다.
type ProjectItem struct {
	ID          int64
	BookID      int64
	GutenbergID int
	Title       string
	Author      string
	TargetLang  string
	Status      string
	PublishedAt *time.Time
}

// ListProjects는 번역 프로젝트 목록이다. status가 비면 전부, 있으면 그 상태만 (D5).
func (s *Service) ListProjects(ctx context.Context, status string) ([]ProjectItem, error) {
	rows, err := gen.New(s.pool).ListProjects(ctx, optionalText(status))
	if err != nil {
		return nil, fmt.Errorf("list projects: %w", err)
	}
	out := make([]ProjectItem, 0, len(rows))
	for _, row := range rows {
		out = append(out, ProjectItem{
			ID:          row.ID,
			BookID:      row.BookID,
			GutenbergID: int(row.GutenbergID),
			Title:       row.Title,
			Author:      textOrEmpty(row.Author),
			TargetLang:  row.TargetLang,
			Status:      row.Status,
			PublishedAt: timestamptzPtr(row.PublishedAt),
		})
	}
	return out, nil
}

// SetProjectStatus는 open ↔ published 전이다 (D4 · ADR-036).
// published_at은 처음 published가 된 시각만 찍힌다. 내리는 쪽에서 비우지 않는다.
func (s *Service) SetProjectStatus(ctx context.Context, id int64, status string) (*gen.TranslationProject, error) {
	p, err := gen.New(s.pool).SetProjectStatus(ctx, gen.SetProjectStatusParams{
		ID:     id,
		Status: status,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("set project %d status %s: %w", id, status, err)
	}
	return &p, nil
}

func optionalText(s string) pgtype.Text {
	if s == "" {
		return pgtype.Text{}
	}
	return pgtype.Text{String: s, Valid: true}
}

func textOrEmpty(t pgtype.Text) string {
	if !t.Valid {
		return ""
	}
	return t.String
}

func timestamptzPtr(t pgtype.Timestamptz) *time.Time {
	if !t.Valid {
		return nil
	}
	v := t.Time
	return &v
}
