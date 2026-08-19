// Package translate는 번역 제안과 검수 도메인이다.
package translate

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	gen "github.com/dinnertime/reclassic/internal/db/gen"
)

var (
	// ErrConflict는 다른 검수자가 먼저 처리했다는 뜻이다. HTTP 409로 나간다.
	ErrConflict = errors.New("이미 처리된 제안이다")
	ErrNotFound = errors.New("찾을 수 없다")
	// ErrForbidden은 권한이 없다는 뜻이다.
	ErrForbidden = errors.New("권한이 없다")
)

type Service struct {
	pool *pgxpool.Pool
}

func NewService(pool *pgxpool.Pool) *Service {
	return &Service{pool: pool}
}

// Propose는 문단 번역을 제안한다. 제안은 쌓인다 — 문단당 N개다.
func (s *Service) Propose(ctx context.Context, projectID int64, stableID, text string, authorID int64) (*gen.TranslationProposal, error) {
	q := gen.New(s.pool)
	p, err := q.CreateProposal(ctx, gen.CreateProposalParams{
		ProjectID:         projectID,
		ParagraphStableID: stableID,
		Text:              text,
		AuthorID:          authorID,
	})
	if err != nil {
		// 같은 사람이 같은 문단에 대기 중인 제안을 둘 이상 두지 않는다 (부분 유니크 인덱스).
		if isUniqueViolation(err) {
			return nil, fmt.Errorf("%w: 이미 대기 중인 제안이 있다", ErrConflict)
		}
		return nil, fmt.Errorf("create proposal: %w", err)
	}
	return &p, nil
}

// Approve는 제안 하나를 확정본으로 올린다.
//
// ARCHITECTURE 불변식 3 — 세 가지를 한 트랜잭션 안에서 함께 한다.
// 2단계의 `AND status='pending'`이 두 검수자가 서로 다른 제안을 동시에 승인하는 것을 막는다.
// 영향 행이 0이면 롤백하고 409다. 이 조건을 빼면 확정본이 조용히 두 번 갈아치워진다.
func (s *Service) Approve(ctx context.Context, proposalID, reviewerID int64, note string) (*gen.ParagraphTranslation, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	q := gen.New(tx)

	proposal, err := q.GetProposal(ctx, proposalID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get proposal %d: %w", proposalID, err)
	}

	// 0. 같은 문단의 승인을 직렬화한다 (ADR-024).
	//    이게 없으면 서로 다른 제안을 동시에 승인했을 때 approved가 둘 남는다.
	//    트랜잭션 종료 시 자동 해제된다.
	if err := q.LockParagraphForReview(ctx, gen.LockParagraphForReviewParams{
		ProjectKey: fmt.Sprint(proposal.ProjectID),
		StableID:   proposal.ParagraphStableID,
	}); err != nil {
		return nil, fmt.Errorf("lock paragraph: %w", err)
	}

	// 1. 기존 확정본의 제안을 superseded로.
	if err := q.SupersedeCurrentProposal(ctx, gen.SupersedeCurrentProposalParams{
		ProjectID:         proposal.ProjectID,
		ParagraphStableID: proposal.ParagraphStableID,
	}); err != nil {
		return nil, fmt.Errorf("supersede current: %w", err)
	}

	// 2. 새 제안 승인. 0 rows면 다른 검수자가 먼저 처리한 것이다.
	approved, err := q.ApproveProposal(ctx, gen.ApproveProposalParams{
		ID:         proposalID,
		ReviewedBy: int8Ptr(reviewerID),
		ReviewNote: note,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrConflict
		}
		return nil, fmt.Errorf("approve proposal %d: %w", proposalID, err)
	}

	// 3. 확정본 교체. 복합 PK가 문단당 1행을 강제한다.
	translation, err := q.UpsertParagraphTranslation(ctx, gen.UpsertParagraphTranslationParams{
		ProjectID:         approved.ProjectID,
		ParagraphStableID: approved.ParagraphStableID,
		Text:              approved.Text,
		ProposalID:        approved.ID,
		ApprovedBy:        reviewerID,
	})
	if err != nil {
		return nil, fmt.Errorf("upsert translation: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}
	return &translation, nil
}

// Reject는 제안을 거절한다. 확정본은 건드리지 않는다.
func (s *Service) Reject(ctx context.Context, proposalID, reviewerID int64, note string) error {
	q := gen.New(s.pool)
	_, err := q.RejectProposal(ctx, gen.RejectProposalParams{
		ID:         proposalID,
		ReviewedBy: int8Ptr(reviewerID),
		ReviewNote: note,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrConflict
		}
		return fmt.Errorf("reject proposal %d: %w", proposalID, err)
	}
	return nil
}

// Withdraw는 제안자가 자기 제안을 거둔다.
func (s *Service) Withdraw(ctx context.Context, proposalID, authorID int64) error {
	q := gen.New(s.pool)
	_, err := q.WithdrawProposal(ctx, gen.WithdrawProposalParams{
		ID:       proposalID,
		AuthorID: authorID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// 남의 제안이거나 이미 처리됐다. 어느 쪽인지 알려주지 않는다.
			return ErrForbidden
		}
		return fmt.Errorf("withdraw proposal %d: %w", proposalID, err)
	}
	return nil
}
