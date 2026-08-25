package api

import (
	"context"
	"errors"

	"github.com/dinnertime/reclassic/internal/api/gen"
	"github.com/dinnertime/reclassic/internal/auth"
	gendb "github.com/dinnertime/reclassic/internal/db/gen"
	"github.com/dinnertime/reclassic/internal/translate"
)

// Translator는 번역 핸들러가 도메인에 대해 알아야 하는 전부다.
type Translator interface {
	Chapter(ctx context.Context, projectID int64, idx int) (*translate.ChapterView, error)
	Proposals(ctx context.Context, projectID int64, stableID string) ([]translate.ProposalView, error)
	Propose(ctx context.Context, projectID int64, stableID, text string, authorID int64) (*gendb.TranslationProposal, error)
	Approve(ctx context.Context, proposalID, reviewerID int64, note string) (*gendb.ParagraphTranslation, error)
	Reject(ctx context.Context, proposalID, reviewerID int64, note string) error
	CreateProject(ctx context.Context, gutenbergID int, targetLang string) (*gendb.TranslationProject, error)
	ListProjects(ctx context.Context, status string) ([]translate.ProjectItem, error)
	SetProjectStatus(ctx context.Context, id int64, status string) (*gendb.TranslationProject, error)
}

func (s *Server) GetProjectChapter(ctx context.Context, req gen.GetProjectChapterRequestObject) (gen.GetProjectChapterResponseObject, error) {
	view, err := s.translate.Chapter(ctx, req.ProjectId, req.Idx)
	if err != nil {
		if errors.Is(err, translate.ErrNotFound) {
			return gen.GetProjectChapter404JSONResponse{Message: "그 챕터를 찾을 수 없다"}, nil
		}
		return nil, err
	}

	paragraphs := make([]gen.TranslatedParagraph, 0, len(view.Paragraphs))
	for _, p := range view.Paragraphs {
		tp := gen.TranslatedParagraph{
			StableId:      p.StableID,
			SourceText:    p.SourceText,
			ProposalCount: p.ProposalCount,
		}
		// 확정본이 없으면 null이다. 빈 문자열로 채우면 "빈 번역"과 구분되지 않는다.
		if p.ApprovedTranslation != "" {
			text := p.ApprovedTranslation
			tp.ApprovedTranslation = &text
		}
		paragraphs = append(paragraphs, tp)
	}

	return gen.GetProjectChapter200JSONResponse{
		Chapter:       gen.Chapter{Idx: view.Idx, Title: view.Title},
		Paragraphs:    paragraphs,
		TotalChapters: view.TotalChapters,
		Coverage: gen.Coverage{
			Total:    view.Coverage.Total,
			Approved: view.Coverage.Approved,
			Ratio:    view.Coverage.Ratio(),
		},
		Indexable: view.Coverage.Indexable(s.indexThreshold),
	}, nil
}

func (s *Server) ListProposals(ctx context.Context, req gen.ListProposalsRequestObject) (gen.ListProposalsResponseObject, error) {
	rows, err := s.translate.Proposals(ctx, req.ProjectId, req.StableId)
	if err != nil {
		return nil, err
	}
	out := make(gen.ListProposals200JSONResponse, 0, len(rows))
	for _, r := range rows {
		out = append(out, toProposal(r.Proposal, r.AuthorHandle))
	}
	return out, nil
}

func (s *Server) CreateProposal(ctx context.Context, req gen.CreateProposalRequestObject) (gen.CreateProposalResponseObject, error) {
	// authGuard가 이미 로그인을 확인했다. 여기 오면 사용자가 있다.
	user, ok := userFrom(ctx)
	if !ok {
		return gen.CreateProposal401JSONResponse{Message: "로그인이 필요하다"}, nil
	}
	if req.Body == nil || req.Body.Text == "" {
		return gen.CreateProposal409JSONResponse{Message: "번역문이 비었다"}, nil
	}

	p, err := s.translate.Propose(ctx, req.ProjectId, req.StableId, req.Body.Text, user.ID)
	if err != nil {
		if errors.Is(err, translate.ErrConflict) {
			return gen.CreateProposal409JSONResponse{Message: "이미 대기 중인 제안이 있다"}, nil
		}
		return nil, err
	}
	return gen.CreateProposal201JSONResponse(toProposal(*p, user.Handle)), nil
}

func (s *Server) ReviewProposal(ctx context.Context, req gen.ReviewProposalRequestObject) (gen.ReviewProposalResponseObject, error) {
	user, ok := userFrom(ctx)
	if !ok {
		return gen.ReviewProposal401JSONResponse{Message: "로그인이 필요하다"}, nil
	}
	// 검수는 reviewer 이상만 한다. 제안은 로그인한 누구나 할 수 있다.
	if !auth.CanReview(user.Role) {
		return gen.ReviewProposal403JSONResponse{Message: "검수 권한이 없다"}, nil
	}
	if req.Body == nil {
		return gen.ReviewProposal404JSONResponse{Message: "본문이 없다"}, nil
	}

	note := ""
	if req.Body.Note != nil {
		note = *req.Body.Note
	}

	switch req.Body.Action {
	case gen.Approve:
		if _, err := s.translate.Approve(ctx, req.ProposalId, user.ID, note); err != nil {
			return reviewError(err)
		}
		return gen.ReviewProposal200JSONResponse{ProposalId: req.ProposalId, Status: gen.ReviewResultStatusApproved}, nil
	case gen.Reject:
		if err := s.translate.Reject(ctx, req.ProposalId, user.ID, note); err != nil {
			return reviewError(err)
		}
		return gen.ReviewProposal200JSONResponse{ProposalId: req.ProposalId, Status: gen.ReviewResultStatusRejected}, nil
	default:
		return gen.ReviewProposal404JSONResponse{Message: "알 수 없는 action"}, nil
	}
}

func reviewError(err error) (gen.ReviewProposalResponseObject, error) {
	switch {
	case errors.Is(err, translate.ErrNotFound):
		return gen.ReviewProposal404JSONResponse{Message: "제안을 찾을 수 없다"}, nil
	case errors.Is(err, translate.ErrConflict):
		// 다른 검수자가 먼저 처리했다. 덮어쓰지 않는다.
		return gen.ReviewProposal409JSONResponse{Message: "다른 검수자가 먼저 처리했다"}, nil
	default:
		return nil, err
	}
}

func (s *Server) CreateProject(ctx context.Context, req gen.CreateProjectRequestObject) (gen.CreateProjectResponseObject, error) {
	if req.Body == nil {
		return gen.CreateProject404JSONResponse{Message: "본문이 없다"}, nil
	}
	project, err := s.translate.CreateProject(ctx, req.Body.GutenbergId, req.Body.TargetLang)
	if err != nil {
		if errors.Is(err, translate.ErrNotFound) {
			return gen.CreateProject404JSONResponse{Message: "그런 책이 없다"}, nil
		}
		return nil, err
	}
	return gen.CreateProject201JSONResponse(toAPIProject(*project)), nil
}

func toAPIProject(p gendb.TranslationProject) gen.Project {
	out := gen.Project{
		Id:         p.ID,
		BookId:     p.BookID,
		TargetLang: p.TargetLang,
		Status:     gen.ProjectStatus(p.Status),
	}
	if p.PublishedAt.Valid {
		t := p.PublishedAt.Time
		out.PublishedAt = &t
	}
	return out
}

func toProposal(p gendb.TranslationProposal, authorHandle string) gen.Proposal {
	return gen.Proposal{
		Id:           p.ID,
		ProjectId:    p.ProjectID,
		StableId:     p.ParagraphStableID,
		Text:         p.Text,
		AuthorHandle: authorHandle,
		Status:       gen.ProposalStatus(p.Status),
		CreatedAt:    p.CreatedAt.Time,
	}
}
