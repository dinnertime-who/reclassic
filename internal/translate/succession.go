package translate

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	gen "github.com/dinnertime/reclassic/internal/db/gen"
)

// SuccessionOutcome은 revision 전환 한 번의 결과다.
type SuccessionOutcome struct {
	FromRevisionID int64
	ToRevisionID   int64
	Matched        int
	Added          int
	Lost           int
	// Orphaned는 소실된 stable_id 중 확정 번역이 붙어 있던 것이다.
	// 사람이 쓴 번역이 갈 곳을 잃은 건수다.
	Orphaned  int
	OrphanIDs []string
}

// ExecuteSuccession은 revision 전환 시 번역 승계를 처리한다.
//
// 놀랍게도 **번역 행을 옮기지 않는다.** paragraph_translations가
// paragraph_stable_id를 참조하므로(ADR-004), 본문이 같으면 새 revision에서도
// 같은 키로 그대로 조회된다. 승계는 자동이고 여기서 할 일은 두 가지다:
//
//  1. 무엇이 승계됐고 무엇이 갈 곳을 잃었는지 기록한다
//  2. 갈 곳을 잃은 번역을 관리자가 볼 수 있게 남긴다 — 조용히 버리지 않는다
//
// ADR-004가 안 B(위치를 해시에 포함)를 기각한 이유가 여기서 드러난다.
// 위치를 넣었다면 이 함수가 수만 건을 옮기고 대부분 실패했을 것이다.
func ExecuteSuccession(ctx context.Context, tx pgx.Tx, bookID, fromRevisionID, toRevisionID int64, matched, added, lost int) (*SuccessionOutcome, error) {
	q := gen.New(tx)

	orphans, err := q.FindOrphanedTranslations(ctx, gen.FindOrphanedTranslationsParams{
		BookID:     bookID,
		RevisionID: toRevisionID,
	})
	if err != nil {
		return nil, fmt.Errorf("find orphaned translations: %w", err)
	}
	if orphans == nil {
		orphans = []string{}
	}

	payload, err := json.Marshal(orphans)
	if err != nil {
		return nil, fmt.Errorf("marshal orphan ids: %w", err)
	}

	from := pgtype.Int8{}
	if fromRevisionID != 0 {
		from = pgtype.Int8{Int64: fromRevisionID, Valid: true}
	}

	if _, err := q.RecordSuccession(ctx, gen.RecordSuccessionParams{
		BookID:         bookID,
		FromRevisionID: from,
		ToRevisionID:   toRevisionID,
		Matched:        int32(matched),
		Added:          int32(added),
		Lost:           int32(lost),
		Orphaned:       int32(len(orphans)),
		OrphanIds:      payload,
	}); err != nil {
		return nil, fmt.Errorf("record succession: %w", err)
	}

	return &SuccessionOutcome{
		FromRevisionID: fromRevisionID,
		ToRevisionID:   toRevisionID,
		Matched:        matched,
		Added:          added,
		Lost:           lost,
		Orphaned:       len(orphans),
		OrphanIDs:      orphans,
	}, nil
}
