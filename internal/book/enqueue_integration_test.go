package book

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
)

// failingQueue는 잡 등록이 실패하는 상황을 만든다.
type failingQueue struct{ err error }

func (f failingQueue) EnqueueFetch(context.Context, pgx.Tx, int64, int, string) error {
	return f.err
}

// countingQueue는 성공하되 호출 여부를 기록한다.
type countingQueue struct{ calls int }

func (c *countingQueue) EnqueueFetch(context.Context, pgx.Tx, int64, int, string) error {
	c.calls++
	return nil
}

const requestTestID = 999000002

// ADR-003이 River를 고른 유일한 근거를 못 박는다.
//
// 잡 등록이 실패하면 books 행도 남지 않아야 한다. Redis 기반 큐라면
// "책은 생성됐는데 잡 등록이 실패"하는 틈이 생긴다.
func TestRequestRollsBackBookWhenEnqueueFails(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	if _, err := pool.Exec(ctx, "DELETE FROM books WHERE gutenberg_id = $1", requestTestID); err != nil {
		t.Fatalf("cleanup: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), "DELETE FROM books WHERE gutenberg_id = $1", requestTestID)
	})

	wantErr := errors.New("큐가 죽었다")
	_, err := NewRequester(pool, failingQueue{err: wantErr}).Request(ctx, requestTestID, "Test", "en")
	if err == nil {
		t.Fatal("잡 등록이 실패했는데 Request가 성공했다")
	}
	if !errors.Is(err, wantErr) {
		t.Errorf("err = %v, 원인이 감싸여 있지 않다", err)
	}

	var count int
	row := pool.QueryRow(ctx, "SELECT count(*) FROM books WHERE gutenberg_id = $1", requestTestID)
	if err := row.Scan(&count); err != nil {
		t.Fatalf("count books: %v", err)
	}
	if count != 0 {
		t.Errorf("books 행이 %d개 남았다 — 잡 등록 실패 시 롤백돼야 한다", count)
	}
}

func TestRequestCreatesBookAndEnqueues(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	if _, err := pool.Exec(ctx, "DELETE FROM books WHERE gutenberg_id = $1", requestTestID); err != nil {
		t.Fatalf("cleanup: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), "DELETE FROM books WHERE gutenberg_id = $1", requestTestID)
	})

	queue := &countingQueue{}
	requester := NewRequester(pool, queue)

	bookID, err := requester.Request(ctx, requestTestID, "Test", "en")
	if err != nil {
		t.Fatalf("Request: %v", err)
	}
	if bookID == 0 {
		t.Error("bookID가 0이다")
	}
	if queue.calls != 1 {
		t.Errorf("EnqueueFetch 호출 %d회, want 1", queue.calls)
	}

	// 두 번째 요청은 거부된다. 같은 책을 두 번 수집하지 않는다.
	if _, err := requester.Request(ctx, requestTestID, "Test", "en"); !errors.Is(err, ErrAlreadyRequested) {
		t.Errorf("두 번째 Request err = %v, want ErrAlreadyRequested", err)
	}
	if queue.calls != 1 {
		t.Errorf("거부됐는데 잡이 %d회 등록됐다", queue.calls)
	}
}
