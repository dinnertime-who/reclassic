package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/dinnertime/reclassic/internal/api/gen"
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
	UserByHandle(ctx context.Context, handle string) (*gendb.User, error)
	CreateProject(ctx context.Context, gutenbergID int, targetLang string) (*gendb.TranslationProject, error)
}

// userHandleHeader는 임시 신원이다. 인증이 아니다 — 위조가 자명하게 가능하다.
// 슬라이스 5(세션 인증)에서 걷어낸다.
const userHandleHeader = "X-User-Handle"

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
	user, err := s.currentUser(ctx)
	if err != nil {
		return gen.CreateProposal401JSONResponse{Message: "신원을 확인할 수 없다"}, nil
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
	user, err := s.currentUser(ctx)
	if err != nil {
		return gen.ReviewProposal401JSONResponse{Message: "신원을 확인할 수 없다"}, nil
	}
	// 검수는 reviewer 이상만 한다. 제안은 누구나 할 수 있다.
	if user.Role != "reviewer" && user.Role != "admin" {
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
	return gen.CreateProject201JSONResponse{
		Id:         project.ID,
		BookId:     project.BookID,
		TargetLang: project.TargetLang,
		Status:     gen.ProjectStatus(project.Status),
	}, nil
}

// currentUser는 임시 신원 헤더를 사용자로 바꾼다.
// 슬라이스 5에서 세션으로 대체한다.
func (s *Server) currentUser(ctx context.Context) (*gendb.User, error) {
	handle, _ := ctx.Value(userHandleKey{}).(string)
	if handle == "" {
		return nil, fmt.Errorf("신원 헤더가 없다")
	}
	return s.translate.UserByHandle(ctx, handle)
}

type userHandleKey struct{}

// withUserHandle은 임시 신원 헤더를 컨텍스트로 옮긴다.
// 핸들러가 *http.Request를 직접 만지지 않게 하기 위함이다.
func withUserHandle(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if handle := r.Header.Get(userHandleHeader); handle != "" {
			r = r.WithContext(context.WithValue(r.Context(), userHandleKey{}, handle))
		}
		next.ServeHTTP(w, r)
	})
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
