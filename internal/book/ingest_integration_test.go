package book

import (
	"context"
	"io"
	"log/slog"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/dinnertime/reclassic/internal/config"
	"github.com/dinnertime/reclassic/internal/db"
	gen "github.com/dinnertime/reclassic/internal/db/gen"
)

// 통합 테스트는 DATABASE_URL이 없으면 건너뛴다.
// parse_test.go가 캐시 없을 때 건너뛰는 것과 같은 이유다 — CI가 DB 없이 통과해야 한다.
func testPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	// 테스트는 패키지 디렉토리에서 돈다. 저장소 루트의 .env를 찾아 올라간다.
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

// 테스트 전용 도서를 쓴다. 코퍼스 도서 번호와 겹치지 않도록 음수를 쓸 수는 없으므로
// 실제로 존재하지 않는 큰 번호를 쓰고, 끝나면 지운다.
const testGutenbergID = 999000001

func cleanupBook(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	_, err := pool.Exec(context.Background(),
		"DELETE FROM books WHERE gutenberg_id = $1", testGutenbergID)
	if err != nil {
		t.Fatalf("cleanup: %v", err)
	}
}

// 게이트를 통과하는 최소 픽스처. 챕터당 문단 3개 이상이어야 chapterSanity가 잡히고
// (confidence.go), 문단 길이 중앙값이 40자를 넘어야 paraSanity가 잡힌다.
const twoChapterHTML = `<html><body>
<h1>Test Book</h1>
<h2>CHAPTER I.</h2>
<p>` + para1 + `</p>
<p>` + para2 + `</p>
<p>` + para3 + `</p>
<h2>CHAPTER II.</h2>
<p>` + para4 + `</p>
<p>` + para5 + `</p>
<p>` + para6 + `</p>
</body></html>`

const (
	para1 = "It is a truth universally acknowledged that a single man in possession of a good fortune must be in want of a wife."
	para2 = "However little known the feelings or views of such a man may be on his first entering a neighbourhood, this truth is well fixed."
	para3 = "My dear Mr. Bennet, said his lady to him one day, have you heard that Netherfield Park is let at last?"
	para4 = "Mr. Bennet was so odd a mixture of quick parts, sarcastic humour, reserve, and caprice, that experience was insufficient."
	para5 = "She was a woman of mean understanding, little information, and uncertain temper, and she fancied herself nervous."
	para6 = "The business of her life was to get her daughters married; its solace was visiting and news of the neighbourhood."
)

func newTestIngester(pool *pgxpool.Pool) *Ingester {
	return NewIngester(pool, slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func testSource() Source {
	return Source{
		GutenbergID: testGutenbergID,
		Title:       "Test Book",
		Language:    "en",
		HTML:        []byte(twoChapterHTML),
	}
}

func TestIngestStoresChaptersAndParagraphs(t *testing.T) {
	pool := testPool(t)
	cleanupBook(t, pool)
	t.Cleanup(func() { cleanupBook(t, pool) })

	ctx := context.Background()
	res, err := newTestIngester(pool).Ingest(ctx, testSource())
	if err != nil {
		t.Fatalf("ingest: %v", err)
	}
	if res.Status != StatusReady {
		t.Fatalf("status = %q, want %q (gate: %v)", res.Status, StatusReady, res.Gate.Reasons)
	}

	q := gen.New(pool)
	chapters, err := q.CountChapters(ctx, res.RevisionID)
	if err != nil {
		t.Fatalf("count chapters: %v", err)
	}
	if int(chapters) != res.Chapters {
		t.Errorf("DB 챕터 %d, 적재 결과 %d", chapters, res.Chapters)
	}

	ids, err := q.ListStableIDs(ctx, res.RevisionID)
	if err != nil {
		t.Fatalf("list stable ids: %v", err)
	}
	if len(ids) != res.Paragraphs {
		t.Errorf("DB 문단 %d, 적재 결과 %d", len(ids), res.Paragraphs)
	}
}

// 같은 원문을 같은 파서 버전으로 다시 넣어도 revision이 늘지 않아야 한다.
func TestIngestIsIdempotent(t *testing.T) {
	pool := testPool(t)
	cleanupBook(t, pool)
	t.Cleanup(func() { cleanupBook(t, pool) })

	ctx := context.Background()
	ing := newTestIngester(pool)

	first, err := ing.Ingest(ctx, testSource())
	if err != nil {
		t.Fatalf("첫 적재: %v", err)
	}
	second, err := ing.Ingest(ctx, testSource())
	if err != nil {
		t.Fatalf("두 번째 적재: %v", err)
	}

	if !second.Skipped {
		t.Error("두 번째 적재가 건너뛰지 않았다")
	}
	if second.RevisionID != first.RevisionID {
		t.Errorf("revision이 새로 생겼다: %d → %d", first.RevisionID, second.RevisionID)
	}

	var count int
	row := pool.QueryRow(ctx, `SELECT count(*) FROM book_revisions r
		JOIN books b ON b.id = r.book_id WHERE b.gutenberg_id = $1`, testGutenbergID)
	if err := row.Scan(&count); err != nil {
		t.Fatalf("count revisions: %v", err)
	}
	if count != 1 {
		t.Errorf("revision %d개, want 1", count)
	}
}

// 게이트에 걸린 책은 활성 revision이 없고, 읽기 조회가 404여야 한다.
func TestGatedBookIsNotReadable(t *testing.T) {
	pool := testPool(t)
	cleanupBook(t, pool)
	t.Cleanup(func() { cleanupBook(t, pool) })

	ctx := context.Background()

	// 신뢰도를 못 넘기는 원문. 챕터도 문단도 없다시피 하다.
	src := testSource()
	src.HTML = []byte(`<html><body><p>짧다</p></body></html>`)

	res, err := newTestIngester(pool).Ingest(ctx, src)
	if err != nil {
		t.Fatalf("ingest: %v", err)
	}
	if res.Status != StatusNeedsReview {
		t.Fatalf("status = %q, want %q", res.Status, StatusNeedsReview)
	}
	if res.RevisionID == 0 {
		t.Error("needs_review여도 revision은 저장돼야 한다")
	}

	if _, err := NewReader(pool).Chapter(ctx, testGutenbergID, 0); err != ErrNotFound {
		t.Errorf("Chapter() err = %v, want ErrNotFound", err)
	}
}

func TestReaderReturnsChapter(t *testing.T) {
	pool := testPool(t)
	cleanupBook(t, pool)
	t.Cleanup(func() { cleanupBook(t, pool) })

	ctx := context.Background()
	res, err := newTestIngester(pool).Ingest(ctx, testSource())
	if err != nil {
		t.Fatalf("ingest: %v", err)
	}

	view, err := NewReader(pool).Chapter(ctx, testGutenbergID, 0)
	if err != nil {
		t.Fatalf("Chapter: %v", err)
	}
	if view.TotalChapters != res.Chapters {
		t.Errorf("totalChapters = %d, want %d", view.TotalChapters, res.Chapters)
	}
	if len(view.Paragraphs) == 0 {
		t.Fatal("문단이 비었다")
	}
	if view.Paragraphs[0].StableID == "" {
		t.Error("stable_id가 비었다")
	}

	// 범위 밖 챕터는 404다.
	if _, err := NewReader(pool).Chapter(ctx, testGutenbergID, 9999); err != ErrNotFound {
		t.Errorf("범위 밖 챕터 err = %v, want ErrNotFound", err)
	}
}
