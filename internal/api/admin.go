package api

import (
	"context"
	"errors"

	"github.com/dinnertime/reclassic/internal/api/gen"
	"github.com/dinnertime/reclassic/internal/auth"
	"github.com/dinnertime/reclassic/internal/translate"
)

func (s *Server) ListNeedsReviewBooks(ctx context.Context, _ gen.ListNeedsReviewBooksRequestObject) (gen.ListNeedsReviewBooksResponseObject, error) {
	rows, err := s.catalog.ListNeedsReview(ctx)
	if err != nil {
		return nil, err
	}
	items := make([]gen.NeedsReviewBook, 0, len(rows))
	for _, row := range rows {
		items = append(items, gen.NeedsReviewBook{
			GutenbergId:    row.GutenbergID,
			Title:          row.Title,
			Author:         stringPtr(row.Author),
			ChapterCount:   row.ChapterCount,
			ParagraphCount: row.ParagraphCount,
		})
	}
	return gen.ListNeedsReviewBooks200JSONResponse{Items: items}, nil
}

func (s *Server) ListOrphanedSuccessions(ctx context.Context, _ gen.ListOrphanedSuccessionsRequestObject) (gen.ListOrphanedSuccessionsResponseObject, error) {
	rows, err := s.catalog.ListOrphans(ctx)
	if err != nil {
		return nil, err
	}
	items := make([]gen.OrphanedSuccession, 0, len(rows))
	for _, row := range rows {
		items = append(items, gen.OrphanedSuccession{
			GutenbergId: row.GutenbergID,
			Title:       row.Title,
			Orphaned:    row.Orphaned,
			CreatedAt:   row.CreatedAt,
		})
	}
	return gen.ListOrphanedSuccessions200JSONResponse{Items: items}, nil
}

func (s *Server) ListAdminProjects(ctx context.Context, _ gen.ListAdminProjectsRequestObject) (gen.ListAdminProjectsResponseObject, error) {
	rows, err := s.translate.ListProjects(ctx, "")
	if err != nil {
		return nil, err
	}
	return gen.ListAdminProjects200JSONResponse{Items: toProjectListItems(rows)}, nil
}

func (s *Server) ListUsers(ctx context.Context, _ gen.ListUsersRequestObject) (gen.ListUsersResponseObject, error) {
	rows, err := s.users.List(ctx)
	if err != nil {
		return nil, err
	}
	items := make([]gen.UserListItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, toUserListItem(row))
	}
	return gen.ListUsers200JSONResponse{Items: items}, nil
}

func (s *Server) SetUserRole(ctx context.Context, req gen.SetUserRoleRequestObject) (gen.SetUserRoleResponseObject, error) {
	user, ok := userFrom(ctx)
	if !ok {
		return gen.SetUserRole401JSONResponse{Message: "로그인이 필요하다"}, nil
	}
	if req.Body == nil {
		return gen.SetUserRole404JSONResponse{Message: "본문이 없다"}, nil
	}
	if user.ID == req.UserId {
		return gen.SetUserRole409JSONResponse{Message: "자기 자신의 역할을 바꿀 수 없다"}, nil
	}

	role := string(req.Body.Role)
	updated, err := s.users.SetRole(ctx, req.UserId, role)
	if err != nil {
		if errors.Is(err, auth.ErrUserNotFound) {
			return gen.SetUserRole404JSONResponse{Message: "그런 사용자가 없다"}, nil
		}
		if errors.Is(err, auth.ErrInvalidRole) {
			return gen.SetUserRole404JSONResponse{Message: "부여할 수 없는 역할이다"}, nil
		}
		return nil, err
	}
	return gen.SetUserRole200JSONResponse(toUserListItem(*updated)), nil
}

func (s *Server) SetProjectStatus(ctx context.Context, req gen.SetProjectStatusRequestObject) (gen.SetProjectStatusResponseObject, error) {
	if req.Body == nil {
		return gen.SetProjectStatus404JSONResponse{Message: "본문이 없다"}, nil
	}
	status := string(req.Body.Status)
	project, err := s.translate.SetProjectStatus(ctx, req.ProjectId, status)
	if err != nil {
		if errors.Is(err, translate.ErrNotFound) {
			return gen.SetProjectStatus404JSONResponse{Message: "그런 프로젝트가 없다"}, nil
		}
		return nil, err
	}
	return gen.SetProjectStatus200JSONResponse(toAPIProject(*project)), nil
}

func toUserListItem(u auth.UserItem) gen.UserListItem {
	return gen.UserListItem{
		Id:          u.ID,
		Handle:      u.Handle,
		DisplayName: u.DisplayName,
		Role:        gen.UserListItemRole(u.Role),
	}
}
