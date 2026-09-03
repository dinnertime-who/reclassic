package api

import (
	"context"
	"errors"

	"github.com/dinnertime/reclassic/internal/api/gen"
	"github.com/dinnertime/reclassic/internal/translate"
)

func (s *Server) ListBooks(ctx context.Context, _ gen.ListBooksRequestObject) (gen.ListBooksResponseObject, error) {
	rows, err := s.catalog.ListPublished(ctx)
	if err != nil {
		return nil, err
	}
	items := make([]gen.BookListItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, gen.BookListItem{
			GutenbergId: row.GutenbergID,
			Title:       row.Title,
			Author:      stringPtr(row.Author),
			ProjectId:   row.ProjectID,
			TargetLang:  row.TargetLang,
		})
	}
	return gen.ListBooks200JSONResponse{Items: items}, nil
}

func (s *Server) ListProjects(ctx context.Context, _ gen.ListProjectsRequestObject) (gen.ListProjectsResponseObject, error) {
	rows, err := s.translate.ListProjects(ctx, "published")
	if err != nil {
		return nil, err
	}
	return gen.ListProjects200JSONResponse{Items: toProjectListItems(rows)}, nil
}

// ListProjectChapters는 목차 화면이 쓰는 장 목록이다.
//
// 읽기 경로라 로그인을 요구하지 않는다 — auth.go의 authedOperations에 넣지 않았다.
// 진행도 0%인 장도 빼지 않는다. 원문은 있으므로 읽을 수 있고, 목차에서 빠지면
// 번역이 시작되지 않은 장으로 갈 길이 없어진다.
func (s *Server) ListProjectChapters(ctx context.Context, req gen.ListProjectChaptersRequestObject) (gen.ListProjectChaptersResponseObject, error) {
	view, err := s.translate.Contents(ctx, req.ProjectId)
	if err != nil {
		if errors.Is(err, translate.ErrNotFound) {
			return gen.ListProjectChapters404JSONResponse{Message: "그 번역 프로젝트를 찾을 수 없다"}, nil
		}
		return nil, err
	}

	items := make([]gen.ChapterListItem, 0, len(view.Chapters))
	for _, c := range view.Chapters {
		items = append(items, gen.ChapterListItem{
			Idx:      c.Idx,
			Title:    c.Title,
			Coverage: toCoverage(c.Coverage),
		})
	}

	return gen.ListProjectChapters200JSONResponse{
		Book: gen.ProjectBook{
			Title:      view.Title,
			Author:     stringPtr(view.Author),
			TargetLang: view.TargetLang,
		},
		Progress: toCoverage(view.Progress),
		Items:    items,
	}, nil
}

func toCoverage(c translate.Coverage) gen.Coverage {
	return gen.Coverage{Total: c.Total, Approved: c.Approved, Ratio: c.Ratio()}
}

func toProjectListItems(rows []translate.ProjectItem) []gen.ProjectListItem {
	items := make([]gen.ProjectListItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, gen.ProjectListItem{
			Id:          row.ID,
			BookId:      row.BookID,
			GutenbergId: row.GutenbergID,
			Title:       row.Title,
			Author:      stringPtr(row.Author),
			TargetLang:  row.TargetLang,
			Status:      gen.ProjectListItemStatus(row.Status),
			PublishedAt: row.PublishedAt,
		})
	}
	return items
}

func stringPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
