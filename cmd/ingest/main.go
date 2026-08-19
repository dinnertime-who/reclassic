// 적재와 승계 측정 CLI.
//
// 임시다. River를 붙이는 수집 자동화 슬라이스에서 ParseBook 잡이
// internal/book.Ingester를 직접 부르게 되고, 이 진입점은 cmd/worker로 흡수된다.
// 도메인 로직을 여기 두지 않는 이유가 그것이다.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"sort"

	"github.com/dinnertime/reclassic/internal/book"
	"github.com/dinnertime/reclassic/internal/config"
	"github.com/dinnertime/reclassic/internal/db"
	"github.com/dinnertime/reclassic/internal/gutenberg"
	"github.com/dinnertime/reclassic/internal/parse"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))

	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	config.LoadDotEnv(".env")

	var err error
	switch os.Args[1] {
	case "run":
		err = cmdRun(log, os.Args[2:])
	case "succession":
		err = cmdSuccession(os.Args[2:])
	default:
		usage()
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "✗ %v\n", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprint(os.Stderr, `ingest — 파서 결과 적재와 stable_id 승계 측정

  ingest run        -corpus=<path> -cache=<dir> [-only=ID]
      캐시된 원문을 파싱해 DB에 적재한다. 멱등이다.

  ingest succession -corpus=<path> -cache=<dir> [-only=ID]
      저장된 활성 revision과 지금 파서의 결과를 stable_id로 대조한다. DB에 쓰지 않는다.
`)
}

// corpusBooks는 두 서브커맨드가 공유하는 대상 선정이다.
func corpusBooks(corpusPath string, only int) ([]parse.BookSpec, error) {
	c, err := parse.LoadCorpus(corpusPath)
	if err != nil {
		return nil, err
	}
	if only == 0 {
		return c.Books, nil
	}
	for _, b := range c.Books {
		if b.GutenbergID == only {
			return []parse.BookSpec{b}, nil
		}
	}
	return nil, fmt.Errorf("corpus에 %d번 도서가 없다", only)
}

func connect(ctx context.Context) (*pgxpool.Pool, error) {
	databaseURL, err := config.Require("DATABASE_URL")
	if err != nil {
		return nil, err
	}
	p, err := db.Connect(ctx, databaseURL)
	if err != nil {
		return nil, err
	}
	if err := p.Ping(ctx); err != nil {
		return nil, fmt.Errorf("db ping: %w — `make dev`로 Postgres를 먼저 띄울 것", err)
	}
	return p, nil
}

func cmdRun(log *slog.Logger, args []string) error {
	fs := flag.NewFlagSet("run", flag.ExitOnError)
	corpusPath := fs.String("corpus", "internal/parse/testdata/corpus.json", "corpus.json 경로")
	cacheDir := fs.String("cache", ".cache/gutenberg", "원문 캐시 디렉토리")
	only := fs.Int("only", 0, "이 Gutenberg ID 하나만")
	if err := fs.Parse(args); err != nil {
		return err
	}

	books, err := corpusBooks(*corpusPath, *only)
	if err != nil {
		return err
	}

	ctx := context.Background()
	p, err := connect(ctx)
	if err != nil {
		return err
	}
	defer p.Close()

	ing := book.NewIngester(p, log)

	fmt.Printf("%-6s %-34s %-16s %6s %6s %8s  %s\n",
		"권", "제목", "전략", "챕터", "문단", "신뢰도", "상태")

	var failed int
	for _, spec := range books {
		if !gutenberg.Cached(*cacheDir, spec.GutenbergID) {
			fmt.Printf("%-6d %-34s 캐시 없음 — `make fetch-corpus`\n",
				spec.GutenbergID, trunc(spec.ExpectedTitle, 34))
			failed++
			continue
		}
		raw, err := os.ReadFile(gutenberg.HTMLPath(*cacheDir, spec.GutenbergID))
		if err != nil {
			return fmt.Errorf("read cache %d: %w", spec.GutenbergID, err)
		}

		res, err := ing.Ingest(ctx, book.Source{
			GutenbergID: spec.GutenbergID,
			Title:       spec.ExpectedTitle,
			Language:    "en",
			HTML:        raw,
		})
		if err != nil {
			fmt.Printf("%-6d %-34s ✗ %v\n", spec.GutenbergID, trunc(spec.ExpectedTitle, 34), err)
			failed++
			continue
		}

		status := res.Status
		if res.Skipped {
			status += " (이미 적재됨)"
		}
		fmt.Printf("%-6d %-34s %-16s %6d %6d %8.3f  %s\n",
			res.GutenbergID, trunc(spec.ExpectedTitle, 34), res.Strategy,
			res.Chapters, res.Paragraphs, res.Confidence, status)
		for _, reason := range res.Gate.Reasons {
			fmt.Printf("%-6s %s\n", "", "→ "+reason)
		}
	}

	if failed > 0 {
		return fmt.Errorf("%d권 적재 실패", failed)
	}
	return nil
}

func cmdSuccession(args []string) error {
	fs := flag.NewFlagSet("succession", flag.ExitOnError)
	corpusPath := fs.String("corpus", "internal/parse/testdata/corpus.json", "corpus.json 경로")
	cacheDir := fs.String("cache", ".cache/gutenberg", "원문 캐시 디렉토리")
	only := fs.Int("only", 0, "이 Gutenberg ID 하나만")
	if err := fs.Parse(args); err != nil {
		return err
	}

	books, err := corpusBooks(*corpusPath, *only)
	if err != nil {
		return err
	}

	ctx := context.Background()
	p, err := connect(ctx)
	if err != nil {
		return err
	}
	defer p.Close()

	fmt.Printf("%-6s %-34s %9s %9s %8s %9s %6s %6s\n",
		"권", "제목", "저장 문단", "현재 문단", "일치", "승계율", "신규", "소실")

	var results []*book.Succession
	for _, spec := range books {
		if !gutenberg.Cached(*cacheDir, spec.GutenbergID) {
			continue
		}
		raw, err := os.ReadFile(gutenberg.HTMLPath(*cacheDir, spec.GutenbergID))
		if err != nil {
			return fmt.Errorf("read cache %d: %w", spec.GutenbergID, err)
		}

		s, err := book.MeasureSuccession(ctx, p, spec.GutenbergID, spec.ExpectedTitle, raw)
		if err != nil {
			if errors.Is(err, book.ErrNotFound) {
				fmt.Printf("%-6d %-34s 활성 revision 없음 (적재 안 됐거나 needs_review)\n",
					spec.GutenbergID, trunc(spec.ExpectedTitle, 34))
				continue
			}
			return err
		}
		results = append(results, s)
		fmt.Printf("%-6d %-34s %9d %9d %8d %8.1f%% %6d %6d\n",
			s.GutenbergID, trunc(s.Title, 34), s.Stored, s.Current,
			s.Matched, s.Rate()*100, s.Added, s.Lost)
	}

	summarize(results)
	return nil
}

func summarize(results []*book.Succession) {
	if len(results) == 0 {
		return
	}
	var stored, matched, lost int
	for _, s := range results {
		stored += s.Stored
		matched += s.Matched
		lost += s.Lost
	}
	rate := 0.0
	if stored > 0 {
		rate = float64(matched) / float64(stored) * 100
	}
	fmt.Printf("\n%d권 합계 — 저장 %d문단, 일치 %d, 승계율 %.1f%%, 소실 %d\n",
		len(results), stored, matched, rate, lost)

	// 승계율이 낮은 순으로 보여준다. 낮은 것이 다음 설계 결정의 근거다.
	worst := append([]*book.Succession(nil), results...)
	sort.Slice(worst, func(i, j int) bool { return worst[i].Rate() < worst[j].Rate() })
	if worst[0].Rate() < 1.0 {
		fmt.Println("\n승계율이 100% 미만인 권:")
		for _, s := range worst {
			if s.Rate() >= 1.0 {
				break
			}
			fmt.Printf("  %-6d %-34s %.1f%% (소실 %d)\n",
				s.GutenbergID, trunc(s.Title, 34), s.Rate()*100, s.Lost)
		}
	}
}

func trunc(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}
