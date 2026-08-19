package jobs

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	"github.com/riverqueue/river"

	"github.com/dinnertime/reclassic/internal/storage"
)

type stubFetcher struct {
	body   []byte
	status int
	err    error
	calls  int
}

func (s *stubFetcher) GetRetry5xx(string, int) ([]byte, int, error) {
	s.calls++
	return s.body, s.status, s.err
}

type memStore struct {
	objects map[string][]byte
	putErr  error
}

func newMemStore() *memStore { return &memStore{objects: map[string][]byte{}} }

func (m *memStore) Put(_ context.Context, key string, body []byte, _ string) error {
	if m.putErr != nil {
		return m.putErr
	}
	m.objects[key] = body
	return nil
}

func (m *memStore) Get(_ context.Context, key string) ([]byte, error) {
	body, ok := m.objects[key]
	if !ok {
		return nil, errors.New("없는 키")
	}
	return body, nil
}

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func fetchJob() *river.Job[FetchSourceArgs] {
	return &river.Job[FetchSourceArgs]{
		Args: FetchSourceArgs{BookID: 1, GutenbergID: 1342, Title: "Pride and Prejudice"},
	}
}

// 4xx는 재시도해도 결과가 같다. River가 무한히 다시 시도하게 두지 않는다.
func TestFetchSourceCancelsOn4xx(t *testing.T) {
	w := NewFetchSourceWorker(
		&stubFetcher{status: 404, err: errors.New("not found")},
		newMemStore(),
		discardLogger(),
	)

	err := w.Work(context.Background(), fetchJob())
	if err == nil {
		t.Fatal("에러가 없다")
	}
	// river.JobCancel은 센티널이 아니라 래퍼다. 타입으로 확인한다.
	var cancel *river.JobCancelError
	if !errors.As(err, &cancel) {
		t.Errorf("err = %v (%T), want JobCancel", err, err)
	}
}

// 5xx는 재시도 대상이다. 취소하지 않고 그냥 에러를 반환한다.
func TestFetchSourceRetriesOn5xx(t *testing.T) {
	w := NewFetchSourceWorker(
		&stubFetcher{status: 503, err: errors.New("unavailable")},
		newMemStore(),
		discardLogger(),
	)

	err := w.Work(context.Background(), fetchJob())
	if err == nil {
		t.Fatal("에러가 없다")
	}
	var cancel *river.JobCancelError
	if errors.As(err, &cancel) {
		t.Error("5xx를 취소로 처리했다 — 재시도돼야 한다")
	}
}

// 모든 후보 URL을 시도한다. Gutenberg는 전사 시기에 따라 경로가 다르다.
func TestFetchSourceTriesAllURLs(t *testing.T) {
	fetcher := &stubFetcher{status: 404, err: errors.New("not found")}
	w := NewFetchSourceWorker(fetcher, newMemStore(), discardLogger())

	_ = w.Work(context.Background(), fetchJob())

	if fetcher.calls != 3 {
		t.Errorf("URL 시도 %d회, want 3", fetcher.calls)
	}
}

// 저장 키가 내용 주소라 재시도해도 쓰레기가 쌓이지 않는다.
func TestFetchSourceStoresAtContentAddressedKey(t *testing.T) {
	body := []byte("<html><body><p>hello</p></body></html>")
	store := newMemStore()
	w := NewFetchSourceWorker(&stubFetcher{body: body, status: 200}, store, discardLogger())

	// River 클라이언트가 컨텍스트에 없어 enqueue 단계에서 실패하지만,
	// 그 앞의 저장까지는 검증할 수 있다.
	_ = w.Work(context.Background(), fetchJob())

	want := storage.SourceKey(1342, storage.HashContent(body))
	if got, ok := store.objects[want]; !ok {
		t.Errorf("키 %q에 저장되지 않았다 (있는 키: %v)", want, keys(store))
	} else if string(got) != string(body) {
		t.Error("저장된 내용이 다르다")
	}
}

func keys(m *memStore) []string {
	out := make([]string, 0, len(m.objects))
	for k := range m.objects {
		out = append(out, k)
	}
	return out
}
