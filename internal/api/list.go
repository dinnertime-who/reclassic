package api

import (
	"context"

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
