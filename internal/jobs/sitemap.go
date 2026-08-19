package jobs

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/riverqueue/river"

	"github.com/dinnertime/reclassic/internal/storage"
	"github.com/dinnertime/reclassic/internal/translate"
)

// SitemapArgs는 한 번역 프로젝트의 사이트맵을 다시 만들라는 지시다.
type SitemapArgs struct {
	ProjectID int64 `json:"project_id"`
}

func (SitemapArgs) Kind() string { return "sitemap" }

// 사이트맵은 Gutenberg를 부르지 않으므로 parse 큐에서 돌아도 된다.
// fetch 큐는 동시성 1이라 수집을 막게 된다.
func (SitemapArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{Queue: QueueParse}
}

// URL 하나당 50,000개 / 50MB 제한. 넉넉하게 잘라 인덱스 사이트맵으로 묶는다.
const maxURLsPerFile = 45000

// CoverageLister는 프로젝트의 챕터별 진행률을 준다.
// 색인 판정은 도메인(internal/translate)이 한다 — 잡이 임계값을 다시 해석하지 않는다.
type CoverageLister interface {
	ChapterProgress(ctx context.Context, projectID int64) ([]translate.ChapterProgress, error)
}

type SitemapWorker struct {
	river.WorkerDefaults[SitemapArgs]

	lister    CoverageLister
	store     storage.ObjectStore
	baseURL   string
	threshold float64
	log       *slog.Logger
}

func NewSitemapWorker(lister CoverageLister, store storage.ObjectStore, baseURL string, threshold float64, log *slog.Logger) *SitemapWorker {
	return &SitemapWorker{lister: lister, store: store, baseURL: baseURL, threshold: threshold, log: log}
}

func (w *SitemapWorker) Work(ctx context.Context, job *river.Job[SitemapArgs]) error {
	projectID := job.Args.ProjectID

	chapters, err := w.lister.ChapterProgress(ctx, projectID)
	if err != nil {
		return fmt.Errorf("chapter coverage %d: %w", projectID, err)
	}

	// 색인 대상만 넣는다. 승인률이 낮은 챕터는 대부분이 원문이라 thin content다 (ADR-007).
	var urls []string
	for _, c := range chapters {
		if !c.Coverage.Indexable(w.threshold) {
			continue
		}
		urls = append(urls, fmt.Sprintf("%s/projects/%d/chapters/%d", w.baseURL, projectID, c.Idx))
	}

	files := chunk(urls, maxURLsPerFile)
	keys := make([]string, 0, len(files))
	for i, batch := range files {
		key := fmt.Sprintf("sitemaps/project-%d-%d.xml", projectID, i)
		if err := w.store.Put(ctx, key, []byte(urlSet(batch)), "application/xml"); err != nil {
			return fmt.Errorf("put sitemap %s: %w", key, err)
		}
		keys = append(keys, key)
	}

	indexKey := fmt.Sprintf("sitemaps/project-%d.xml", projectID)
	if err := w.store.Put(ctx, indexKey, []byte(sitemapIndex(w.baseURL, keys)), "application/xml"); err != nil {
		return fmt.Errorf("put sitemap index: %w", err)
	}

	w.log.InfoContext(ctx, "사이트맵 생성",
		slog.Int64("project_id", projectID),
		slog.Int("indexable_chapters", len(urls)),
		slog.Int("total_chapters", len(chapters)),
		slog.Int("files", len(files)),
	)
	return nil
}

func chunk(urls []string, size int) [][]string {
	// URL이 하나도 없어도 빈 사이트맵을 남긴다.
	// 파일이 사라지면 크롤러가 404를 받고, 그건 "색인 대상이 없다"와 다른 신호다.
	if len(urls) == 0 {
		return [][]string{{}}
	}
	var out [][]string
	for start := 0; start < len(urls); start += size {
		end := start + size
		if end > len(urls) {
			end = len(urls)
		}
		out = append(out, urls[start:end])
	}
	return out
}

func urlSet(urls []string) string {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	b.WriteString(`<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">` + "\n")
	for _, u := range urls {
		fmt.Fprintf(&b, "  <url><loc>%s</loc></url>\n", escapeXML(u))
	}
	b.WriteString("</urlset>\n")
	return b.String()
}

func sitemapIndex(baseURL string, keys []string) string {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	b.WriteString(`<sitemapindex xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">` + "\n")
	for _, k := range keys {
		fmt.Fprintf(&b, "  <sitemap><loc>%s/%s</loc></sitemap>\n", baseURL, escapeXML(k))
	}
	b.WriteString("</sitemapindex>\n")
	return b.String()
}

func escapeXML(s string) string {
	return strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;", "'", "&apos;").Replace(s)
}
