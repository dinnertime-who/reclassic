package storage

import (
	"bytes"
	"context"
	"os"
	"testing"

	"github.com/dinnertime/reclassic/internal/config"
)

// S3 왕복 테스트. 로컬은 MinIO, 프로덕션 프로토콜은 R2와 같다 (ADR-008).
// 접속할 수 없으면 건너뛴다 — CI가 스토리지 없이 통과해야 한다.
func testStore(t *testing.T) *S3Store {
	t.Helper()
	config.LoadDotEnv("../../.env")

	endpoint := os.Getenv("S3_ENDPOINT")
	if endpoint == "" {
		t.Skip("S3_ENDPOINT 없음 — 스토리지 통합 테스트 건너뜀")
	}

	store, err := NewS3Store(context.Background(), Config{
		Endpoint:  endpoint,
		AccessKey: os.Getenv("R2_ACCESS_KEY_ID"),
		SecretKey: os.Getenv("R2_SECRET_ACCESS_KEY"),
		Bucket:    os.Getenv("R2_BUCKET"),
	})
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	if err := store.EnsureBucket(context.Background()); err != nil {
		t.Skipf("MinIO에 연결할 수 없음 (%v) — `make dev`로 띄울 것", err)
	}
	return store
}

func TestPutGetRoundTrip(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()

	body := []byte("<html><body><p>왕복 테스트</p></body></html>")
	key := SourceKey(999000003, HashContent(body))

	if err := store.Put(ctx, key, body, "text/html; charset=utf-8"); err != nil {
		t.Fatalf("Put: %v", err)
	}

	got, err := store.Get(ctx, key)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !bytes.Equal(got, body) {
		t.Errorf("왕복 후 내용이 다르다: %q", got)
	}

	// 같은 키에 다시 올려도 실패하지 않는다. 잡은 멱등이어야 한다.
	if err := store.Put(ctx, key, body, "text/html; charset=utf-8"); err != nil {
		t.Errorf("같은 키 재업로드: %v", err)
	}
}

func TestGetMissingKeyFails(t *testing.T) {
	store := testStore(t)
	if _, err := store.Get(context.Background(), "sources/999000003/none.html"); err == nil {
		t.Error("없는 키인데 에러가 없다")
	}
}
