package jobs

import (
	"context"
	"strings"
	"testing"

	"github.com/riverqueue/river"

	"github.com/dinnertime/reclassic/internal/translate"
)

type stubLister struct{ chapters []translate.ChapterProgress }

func (s stubLister) ChapterProgress(context.Context, int64) ([]translate.ChapterProgress, error) {
	return s.chapters, nil
}

func progress(idx, total, approved int) translate.ChapterProgress {
	return translate.ChapterProgress{
		Idx:      idx,
		Coverage: translate.Coverage{Total: total, Approved: approved},
	}
}

const threshold = 0.80

// 색인 대상만 사이트맵에 들어간다. 승인률이 낮은 챕터는 thin content다 (ADR-007).
func TestSitemapIncludesOnlyIndexableChapters(t *testing.T) {
	store := newMemStore()
	w := NewSitemapWorker(stubLister{chapters: []translate.ChapterProgress{
		progress(0, 100, 100), // 100% → 포함
		progress(1, 100, 80),  // 80%  → 포함 (경계)
		progress(2, 100, 79),  // 79%  → 제외
		progress(3, 100, 0),   // 미번역 → 제외
		progress(4, 0, 0),     // 빈 챕터 → 제외
	}}, store, "https://reclassic.example", threshold, discardLogger())

	if err := w.Work(context.Background(), &river.Job[SitemapArgs]{Args: SitemapArgs{ProjectID: 7}}); err != nil {
		t.Fatalf("Work: %v", err)
	}

	body := string(store.objects["sitemaps/project-7-0.xml"])
	for _, want := range []string{"/projects/7/chapters/0", "/projects/7/chapters/1"} {
		if !strings.Contains(body, want) {
			t.Errorf("%q가 사이트맵에 없다", want)
		}
	}
	for _, notWant := range []string{"/projects/7/chapters/2", "/projects/7/chapters/3", "/projects/7/chapters/4"} {
		if strings.Contains(body, notWant) {
			t.Errorf("%q가 사이트맵에 있다 — 색인 대상이 아니다", notWant)
		}
	}

	if _, ok := store.objects["sitemaps/project-7.xml"]; !ok {
		t.Error("인덱스 사이트맵이 없다")
	}
}

// 색인 대상이 하나도 없어도 빈 사이트맵을 남긴다.
// 파일이 사라지면 크롤러가 404를 받고, 그건 "색인 대상이 없다"와 다른 신호다.
func TestSitemapWritesEmptyFileWhenNothingIndexable(t *testing.T) {
	store := newMemStore()
	w := NewSitemapWorker(stubLister{chapters: []translate.ChapterProgress{
		progress(0, 100, 10),
	}}, store, "https://reclassic.example", threshold, discardLogger())

	if err := w.Work(context.Background(), &river.Job[SitemapArgs]{Args: SitemapArgs{ProjectID: 7}}); err != nil {
		t.Fatalf("Work: %v", err)
	}
	body, ok := store.objects["sitemaps/project-7-0.xml"]
	if !ok {
		t.Fatal("빈 사이트맵도 남겨야 한다")
	}
	if strings.Contains(string(body), "<url>") {
		t.Errorf("URL이 들어 있다: %s", body)
	}
}

// 파일당 URL 제한을 넘으면 분할한다.
func TestSitemapSplitsLargeProjects(t *testing.T) {
	chapters := make([]translate.ChapterProgress, maxURLsPerFile+10)
	for i := range chapters {
		chapters[i] = progress(i, 1, 1)
	}
	store := newMemStore()
	w := NewSitemapWorker(stubLister{chapters: chapters}, store, "https://reclassic.example", threshold, discardLogger())

	if err := w.Work(context.Background(), &river.Job[SitemapArgs]{Args: SitemapArgs{ProjectID: 7}}); err != nil {
		t.Fatalf("Work: %v", err)
	}
	if _, ok := store.objects["sitemaps/project-7-1.xml"]; !ok {
		t.Errorf("분할되지 않았다 (키: %v)", keys(store))
	}
}
