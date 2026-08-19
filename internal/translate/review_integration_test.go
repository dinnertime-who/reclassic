package translate

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/dinnertime/reclassic/internal/config"
	"github.com/dinnertime/reclassic/internal/db"
	gen "github.com/dinnertime/reclassic/internal/db/gen"
)

func testPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	config.LoadDotEnv("../../.env")
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skip("DATABASE_URL 없음 — 통합 테스트 건너뜀")
	}
	pool, err := db.Connect(context.Background(), url)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	if err := pool.Ping(context.Background()); err != nil {
		t.Skipf("Postgres에 연결할 수 없음 (%v) — `make dev`로 띄울 것", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

const fixtureGutenbergID = 999000010

// fixture는 책·프로젝트·검수자 둘·제안자 둘을 만든다.
type fixture struct {
	pool      *pgxpool.Pool
	projectID int64
	bookID    int64
	authorA   int64
	authorB   int64
	reviewerA int64
	reviewerB int64
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	pool := testPool(t)
	ctx := context.Background()

	cleanup := func() {
		_, _ = pool.Exec(context.Background(), "DELETE FROM books WHERE gutenberg_id = $1", fixtureGutenbergID)
		_, _ = pool.Exec(context.Background(), "DELETE FROM users WHERE handle LIKE 'fixture-%'")
	}
	cleanup()
	t.Cleanup(cleanup)

	q := gen.New(pool)

	book, err := q.UpsertBook(ctx, gen.UpsertBookParams{
		GutenbergID: fixtureGutenbergID, Title: "Fixture", Language: "en",
	})
	if err != nil {
		t.Fatalf("upsert book: %v", err)
	}

	project, err := q.CreateProject(ctx, gen.CreateProjectParams{BookID: book.ID, TargetLang: "ko"})
	if err != nil {
		t.Fatalf("create project: %v", err)
	}

	user := func(handle, role string) int64 {
		u, err := q.UpsertUser(ctx, gen.UpsertUserParams{Handle: handle, DisplayName: handle, Role: role})
		if err != nil {
			t.Fatalf("upsert user %s: %v", handle, err)
		}
		return u.ID
	}

	return &fixture{
		pool:      pool,
		projectID: project.ID,
		bookID:    book.ID,
		authorA:   user("fixture-author-a", "member"),
		authorB:   user("fixture-author-b", "member"),
		reviewerA: user("fixture-reviewer-a", "reviewer"),
		reviewerB: user("fixture-reviewer-b", "reviewer"),
	}
}

const fixtureStableID = "ffffffffffffffff"

// 이 슬라이스의 핵심 검증이다.
//
// 두 검수자가 같은 문단의 서로 다른 제안을 동시에 승인한다.
// ADR-005는 `WHERE status='pending'`이 이걸 막는다고 적었지만 확인된 적이 없었다.
// 정확히 하나만 성공하고, 확정본은 한 행이어야 한다 (불변식 2).
func TestConcurrentApprovalLetsExactlyOneWin(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	svc := NewService(f.pool)

	first, err := svc.Propose(ctx, f.projectID, fixtureStableID, "첫 번째 번역", f.authorA)
	if err != nil {
		t.Fatalf("propose A: %v", err)
	}
	second, err := svc.Propose(ctx, f.projectID, fixtureStableID, "두 번째 번역", f.authorB)
	if err != nil {
		t.Fatalf("propose B: %v", err)
	}

	// 두 검수자가 서로 다른 제안을 동시에 승인한다.
	var wg sync.WaitGroup
	errs := make([]error, 2)
	start := make(chan struct{})

	wg.Add(2)
	go func() {
		defer wg.Done()
		<-start
		_, errs[0] = svc.Approve(ctx, first.ID, f.reviewerA, "")
	}()
	go func() {
		defer wg.Done()
		<-start
		_, errs[1] = svc.Approve(ctx, second.ID, f.reviewerB, "")
	}()
	close(start)
	wg.Wait()

	// 서로 다른 제안이므로 status 조건만으로는 둘 다 통과할 수 있다.
	// 확정본 행이 하나뿐이라는 것이 불변식 2를 지킨다.
	var rows int
	if err := f.pool.QueryRow(ctx,
		`SELECT count(*) FROM paragraph_translations
		  WHERE project_id = $1 AND paragraph_stable_id = $2`,
		f.projectID, fixtureStableID).Scan(&rows); err != nil {
		t.Fatalf("count translations: %v", err)
	}
	if rows != 1 {
		t.Fatalf("확정본이 %d행이다 — 문단당 정확히 하나여야 한다 (불변식 2)", rows)
	}

	// 확정본이 가리키는 제안만 approved이고, 나머지는 approved로 남아 있으면 안 된다.
	var winner int64
	if err := f.pool.QueryRow(ctx,
		`SELECT proposal_id FROM paragraph_translations
		  WHERE project_id = $1 AND paragraph_stable_id = $2`,
		f.projectID, fixtureStableID).Scan(&winner); err != nil {
		t.Fatalf("get winner: %v", err)
	}

	var approvedCount int
	if err := f.pool.QueryRow(ctx,
		`SELECT count(*) FROM translation_proposals
		  WHERE project_id = $1 AND paragraph_stable_id = $2 AND status = 'approved'`,
		f.projectID, fixtureStableID).Scan(&approvedCount); err != nil {
		t.Fatalf("count approved: %v", err)
	}
	if approvedCount != 1 {
		t.Errorf("approved 제안이 %d개다 — 확정본과 어긋난다", approvedCount)
	}

	var winnerStatus string
	if err := f.pool.QueryRow(ctx,
		`SELECT status FROM translation_proposals WHERE id = $1`, winner).Scan(&winnerStatus); err != nil {
		t.Fatalf("get winner status: %v", err)
	}
	if winnerStatus != "approved" {
		t.Errorf("확정본이 가리키는 제안의 status = %q, want approved", winnerStatus)
	}

	t.Logf("동시 승인 결과: errs=%v, 확정본 제안=%d", errs, winner)
}

// 같은 제안을 두 검수자가 동시에 승인하면 하나는 409다.
func TestConcurrentApprovalOfSameProposalConflicts(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	svc := NewService(f.pool)

	p, err := svc.Propose(ctx, f.projectID, fixtureStableID, "번역", f.authorA)
	if err != nil {
		t.Fatalf("propose: %v", err)
	}

	var wg sync.WaitGroup
	errs := make([]error, 2)
	start := make(chan struct{})
	wg.Add(2)
	for i, reviewer := range []int64{f.reviewerA, f.reviewerB} {
		go func(i int, reviewer int64) {
			defer wg.Done()
			<-start
			_, errs[i] = svc.Approve(ctx, p.ID, reviewer, "")
		}(i, reviewer)
	}
	close(start)
	wg.Wait()

	var conflicts, successes int
	for _, err := range errs {
		switch {
		case err == nil:
			successes++
		case errors.Is(err, ErrConflict):
			conflicts++
		default:
			t.Errorf("예상 못 한 에러: %v", err)
		}
	}
	if successes != 1 || conflicts != 1 {
		t.Errorf("성공 %d / 충돌 %d, want 1 / 1 (errs: %v)", successes, conflicts, errs)
	}
}

// 확정본을 다른 제안으로 교체하면 이전 것이 superseded가 된다.
func TestApproveSupersedesPrevious(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	svc := NewService(f.pool)

	first, err := svc.Propose(ctx, f.projectID, fixtureStableID, "첫 번역", f.authorA)
	if err != nil {
		t.Fatalf("propose A: %v", err)
	}
	if _, err := svc.Approve(ctx, first.ID, f.reviewerA, ""); err != nil {
		t.Fatalf("approve A: %v", err)
	}

	second, err := svc.Propose(ctx, f.projectID, fixtureStableID, "더 나은 번역", f.authorB)
	if err != nil {
		t.Fatalf("propose B: %v", err)
	}
	translation, err := svc.Approve(ctx, second.ID, f.reviewerB, "더 자연스럽다")
	if err != nil {
		t.Fatalf("approve B: %v", err)
	}

	if translation.Text != "더 나은 번역" {
		t.Errorf("확정본 = %q, want %q", translation.Text, "더 나은 번역")
	}

	var firstStatus string
	if err := f.pool.QueryRow(ctx,
		`SELECT status FROM translation_proposals WHERE id = $1`, first.ID).Scan(&firstStatus); err != nil {
		t.Fatalf("get first status: %v", err)
	}
	if firstStatus != "superseded" {
		t.Errorf("이전 제안 status = %q, want superseded", firstStatus)
	}
}

// 같은 사람이 같은 문단에 대기 중인 제안을 둘 이상 두지 않는다.
func TestDuplicatePendingProposalRejected(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	svc := NewService(f.pool)

	if _, err := svc.Propose(ctx, f.projectID, fixtureStableID, "첫 시도", f.authorA); err != nil {
		t.Fatalf("propose: %v", err)
	}
	if _, err := svc.Propose(ctx, f.projectID, fixtureStableID, "두 번째 시도", f.authorA); !errors.Is(err, ErrConflict) {
		t.Errorf("err = %v, want ErrConflict", err)
	}

	// 철회하면 다시 낼 수 있다.
	var id int64
	if err := f.pool.QueryRow(ctx,
		`SELECT id FROM translation_proposals
		  WHERE project_id=$1 AND paragraph_stable_id=$2 AND author_id=$3 AND status='pending'`,
		f.projectID, fixtureStableID, f.authorA).Scan(&id); err != nil {
		t.Fatalf("find pending: %v", err)
	}
	if err := svc.Withdraw(ctx, id, f.authorA); err != nil {
		t.Fatalf("withdraw: %v", err)
	}
	if _, err := svc.Propose(ctx, f.projectID, fixtureStableID, "다시 시도", f.authorA); err != nil {
		t.Errorf("철회 후 재제안 실패: %v", err)
	}
}

// 남의 제안은 철회할 수 없다.
func TestWithdrawOthersProposalForbidden(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	svc := NewService(f.pool)

	p, err := svc.Propose(ctx, f.projectID, fixtureStableID, "내 번역", f.authorA)
	if err != nil {
		t.Fatalf("propose: %v", err)
	}
	if err := svc.Withdraw(ctx, p.ID, f.authorB); !errors.Is(err, ErrForbidden) {
		t.Errorf("err = %v, want ErrForbidden", err)
	}
}

// ADR-024가 막는 것을 반복해서 확인한다.
//
// 이 테스트는 단발로 돌리면 통과해 버린다 — 처음에 그렇게 놓쳤다.
// 인터리빙이 맞아떨어져야 드러나므로 여러 번 돌린다.
// 자문 잠금을 빼면 40라운드 중 거의 전부가 실패한다.
func TestConcurrentApprovalStress(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	svc := NewService(f.pool)

	const rounds = 40
	var bad int

	for i := 0; i < rounds; i++ {
		stableID := fmt.Sprintf("stress%010d", i)

		first, err := svc.Propose(ctx, f.projectID, stableID, "A", f.authorA)
		if err != nil {
			t.Fatalf("라운드 %d propose A: %v", i, err)
		}
		second, err := svc.Propose(ctx, f.projectID, stableID, "B", f.authorB)
		if err != nil {
			t.Fatalf("라운드 %d propose B: %v", i, err)
		}

		var wg sync.WaitGroup
		start := make(chan struct{})
		wg.Add(2)
		go func() { defer wg.Done(); <-start; _, _ = svc.Approve(ctx, first.ID, f.reviewerA, "") }()
		go func() { defer wg.Done(); <-start; _, _ = svc.Approve(ctx, second.ID, f.reviewerB, "") }()
		close(start)
		wg.Wait()

		approved, rows := paragraphState(t, f, stableID)
		if approved != 1 || rows != 1 {
			bad++
			t.Logf("라운드 %d: approved=%d 확정본=%d — 어긋남", i, approved, rows)
		}
	}

	if bad > 0 {
		t.Errorf("%d/%d 라운드에서 approved 제안과 확정본이 어긋났다 (ADR-024)", bad, rounds)
	}
}

func paragraphState(t *testing.T, f *fixture, stableID string) (approved, rows int) {
	t.Helper()
	ctx := context.Background()
	if err := f.pool.QueryRow(ctx,
		`SELECT count(*) FROM translation_proposals
		  WHERE project_id=$1 AND paragraph_stable_id=$2 AND status='approved'`,
		f.projectID, stableID).Scan(&approved); err != nil {
		t.Fatalf("count approved: %v", err)
	}
	if err := f.pool.QueryRow(ctx,
		`SELECT count(*) FROM paragraph_translations
		  WHERE project_id=$1 AND paragraph_stable_id=$2`,
		f.projectID, stableID).Scan(&rows); err != nil {
		t.Fatalf("count translations: %v", err)
	}
	return approved, rows
}
