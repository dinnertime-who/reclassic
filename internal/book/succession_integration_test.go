package book

import (
	"context"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	gen "github.com/dinnertime/reclassic/internal/db/gen"
)

// 재파싱해도 본문이 같은 문단의 번역은 그대로다.
// paragraph_translations가 stable_id를 참조하므로 실제로는 아무것도 옮기지 않는다 (ADR-004).
func TestSuccessionKeepsTranslationsWhenTextUnchanged(t *testing.T) {
	pool := testPool(t)
	cleanupBook(t, pool)
	t.Cleanup(func() { cleanupBook(t, pool) })

	ctx := context.Background()
	ing := newTestIngester(pool)
	q := gen.New(pool)

	first, err := ing.Ingest(ctx, testSource())
	if err != nil {
		t.Fatalf("첫 적재: %v", err)
	}

	// 문단 하나에 확정 번역을 붙인다.
	stableID, projectID := seedTranslation(t, pool, first.BookID, first.RevisionID)

	// 본문은 그대로 두고 원문만 바꿔 새 revision을 만든다.
	// 챕터 제목 뒤에 공백을 더해도 문단 본문은 같으므로 stable_id가 유지된다.
	src := testSource()
	src.HTML = []byte(strings.Replace(twoChapterHTML, "<h1>Test Book</h1>", "<h1>Test Book </h1>", 1))

	second, err := ing.Ingest(ctx, src)
	if err != nil {
		t.Fatalf("두 번째 적재: %v", err)
	}
	if second.RevisionID == first.RevisionID {
		t.Fatal("새 revision이 생기지 않았다 — 원문 해시가 같다")
	}
	if second.Succession == nil {
		t.Fatal("승계 기록이 없다")
	}
	if second.Succession.Orphaned != 0 {
		t.Errorf("고아 번역 %d건 — 본문이 안 바뀌었으므로 0이어야 한다", second.Succession.Orphaned)
	}
	if second.Succession.Lost != 0 {
		t.Errorf("소실 %d건, want 0", second.Succession.Lost)
	}

	// 번역이 새 revision에서도 그대로 조회된다.
	rows, err := q.ListChapterParagraphsWithTranslation(ctx, gen.ListChapterParagraphsWithTranslationParams{
		ChapterID: chapterIDOf(t, pool, second.RevisionID, 0),
		ProjectID: projectID,
	})
	if err != nil {
		t.Fatalf("list paragraphs: %v", err)
	}
	var found bool
	for _, r := range rows {
		if r.StableID == stableID && r.ApprovedTranslation.Valid {
			found = true
		}
	}
	if !found {
		t.Error("승계 후 번역이 보이지 않는다")
	}
}

// 본문이 바뀌어 stable_id가 사라지면 번역이 갈 곳을 잃는다.
// 조용히 버리지 않고 고아로 기록해야 한다.
func TestSuccessionRecordsOrphanedTranslations(t *testing.T) {
	pool := testPool(t)
	cleanupBook(t, pool)
	t.Cleanup(func() { cleanupBook(t, pool) })

	ctx := context.Background()
	ing := newTestIngester(pool)

	first, err := ing.Ingest(ctx, testSource())
	if err != nil {
		t.Fatalf("첫 적재: %v", err)
	}
	seedTranslation(t, pool, first.BookID, first.RevisionID)

	// 번역이 붙은 문단(para1)의 본문을 바꾼다. stable_id가 달라진다.
	src := testSource()
	src.HTML = []byte(strings.Replace(twoChapterHTML, para1, "완전히 다른 본문으로 바뀐 첫 문단이며 길이도 충분히 길게 만들어 둔다.", 1))

	second, err := ing.Ingest(ctx, src)
	if err != nil {
		t.Fatalf("두 번째 적재: %v", err)
	}
	if second.Succession == nil {
		t.Fatal("승계 기록이 없다")
	}
	if second.Succession.Lost == 0 {
		t.Error("소실이 0이다 — 본문을 바꿨는데 stable_id가 그대로다")
	}
	if second.Succession.Orphaned != 1 {
		t.Errorf("고아 번역 %d건, want 1 (소실 %d, 고아 목록 %v)",
			second.Succession.Orphaned, second.Succession.Lost, second.Succession.OrphanIDs)
	}

	// 로그가 DB에 남아 관리자가 볼 수 있어야 한다.
	var orphaned int
	if err := pool.QueryRow(ctx,
		`SELECT orphaned FROM revision_successions WHERE to_revision_id = $1`,
		second.RevisionID).Scan(&orphaned); err != nil {
		t.Fatalf("read succession log: %v", err)
	}
	if orphaned != 1 {
		t.Errorf("기록된 고아 수 %d, want 1", orphaned)
	}
}

// seedTranslation은 프로젝트·사용자·확정 번역을 하나 만든다.
// 첫 챕터 첫 문단에 붙인다.
func seedTranslation(t *testing.T, pool *pgxpool.Pool, bookID, revisionID int64) (stableID string, projectID int64) {
	t.Helper()
	ctx := context.Background()
	q := gen.New(pool)

	project, err := q.CreateProject(ctx, gen.CreateProjectParams{BookID: bookID, TargetLang: "ko"})
	if err != nil {
		t.Fatalf("create project: %v", err)
	}

	author, err := q.UpsertUser(ctx, gen.UpsertUserParams{
		Handle: "succession-author", DisplayName: "a", Role: "member"})
	if err != nil {
		t.Fatalf("upsert author: %v", err)
	}
	reviewer, err := q.UpsertUser(ctx, gen.UpsertUserParams{
		Handle: "succession-reviewer", DisplayName: "r", Role: "reviewer"})
	if err != nil {
		t.Fatalf("upsert reviewer: %v", err)
	}

	if err := pool.QueryRow(ctx,
		`SELECT stable_id FROM paragraphs WHERE revision_id = $1 ORDER BY chapter_id, idx LIMIT 1`,
		revisionID).Scan(&stableID); err != nil {
		t.Fatalf("first stable id: %v", err)
	}

	proposal, err := q.CreateProposal(ctx, gen.CreateProposalParams{
		ProjectID: project.ID, ParagraphStableID: stableID,
		Text: "확정된 번역문", AuthorID: author.ID,
	})
	if err != nil {
		t.Fatalf("create proposal: %v", err)
	}
	if _, err := q.UpsertParagraphTranslation(ctx, gen.UpsertParagraphTranslationParams{
		ProjectID: project.ID, ParagraphStableID: stableID,
		Text: proposal.Text, ProposalID: proposal.ID, ApprovedBy: reviewer.ID,
	}); err != nil {
		t.Fatalf("upsert translation: %v", err)
	}

	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(),
			"DELETE FROM users WHERE handle LIKE 'succession-%'")
	})
	return stableID, project.ID
}

func chapterIDOf(t *testing.T, pool *pgxpool.Pool, revisionID int64, idx int) int64 {
	t.Helper()
	var id int64
	if err := pool.QueryRow(context.Background(),
		`SELECT id FROM chapters WHERE revision_id = $1 AND idx = $2`, revisionID, idx).Scan(&id); err != nil {
		t.Fatalf("chapter %d: %v", idx, err)
	}
	return id
}
