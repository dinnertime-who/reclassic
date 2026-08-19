package main

import (
	"flag"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/dinnertime/reclassic/internal/config"
	"github.com/dinnertime/reclassic/internal/gutenberg"
	"github.com/dinnertime/reclassic/internal/parse"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	config.LoadDotEnv(".env")

	var err error
	switch os.Args[1] {
	case "fetch":
		err = runFetch(os.Args[2:])
	case "report":
		err = runReport(os.Args[2:])
	case "golden":
		err = runGolden(os.Args[2:])
	default:
		usage()
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, `usage:
  parsecheck fetch  -corpus=<path> -cache=<dir>
  parsecheck report -corpus=<path> -cache=<dir> [-out=.cache/report.html]
  parsecheck golden -corpus=<path> -cache=<dir> [-update] [-dir=internal/parse/testdata/golden]
`)
}

func runFetch(args []string) error {
	fs := flag.NewFlagSet("fetch", flag.ContinueOnError)
	corpusPath := fs.String("corpus", "internal/parse/testdata/corpus.json", "corpus.json 경로")
	cacheDir := fs.String("cache", ".cache/gutenberg", "캐시 디렉토리")
	if err := fs.Parse(args); err != nil {
		return err
	}

	ua := os.Getenv("GUTENBERG_USER_AGENT")
	if ua == "" {
		return fmt.Errorf("GUTENBERG_USER_AGENT 가 없습니다. .env 를 설정하시오")
	}
	interval, err := minInterval()
	if err != nil {
		return err
	}

	corpus, err := parse.LoadCorpus(*corpusPath)
	if err != nil {
		return err
	}
	client := gutenberg.NewClient(ua, interval)

	var failed int
	for _, book := range corpus.Books {
		if gutenberg.Cached(*cacheDir, book.GutenbergID) {
			fmt.Printf("skip  %4d  %s (cached)\n", book.GutenbergID, book.ExpectedTitle)
			continue
		}
		fmt.Printf("fetch %4d  %s ...\n", book.GutenbergID, book.ExpectedTitle)
		meta, err := gutenberg.FetchBook(client, *cacheDir, book)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  WARN %v\n", err)
			failed++
			continue
		}
		fmt.Printf("  ok   %s  sha256=%s\n", meta.URL, meta.SHA256[:12])
	}
	if failed > 0 {
		return fmt.Errorf("%d권 수집 실패", failed)
	}
	return nil
}

func runReport(args []string) error {
	fs := flag.NewFlagSet("report", flag.ContinueOnError)
	corpusPath := fs.String("corpus", "internal/parse/testdata/corpus.json", "corpus.json 경로")
	cacheDir := fs.String("cache", ".cache/gutenberg", "캐시 디렉토리")
	out := fs.String("out", ".cache/report.html", "HTML 리포트 경로")
	if err := fs.Parse(args); err != nil {
		return err
	}
	evals, err := evaluateCorpus(*corpusPath, *cacheDir)
	if err != nil {
		return err
	}
	parse.PrintSummary(os.Stdout, evals)
	if err := parse.WriteHTMLReport(*out, evals); err != nil {
		return err
	}
	fmt.Printf("\nHTML 리포트: %s\n", *out)
	return nil
}

func runGolden(args []string) error {
	fs := flag.NewFlagSet("golden", flag.ContinueOnError)
	corpusPath := fs.String("corpus", "internal/parse/testdata/corpus.json", "corpus.json 경로")
	cacheDir := fs.String("cache", ".cache/gutenberg", "캐시 디렉토리")
	dir := fs.String("dir", "internal/parse/testdata/golden", "golden 디렉토리")
	update := fs.Bool("update", false, "스냅샷을 현재 결과로 갱신")
	if err := fs.Parse(args); err != nil {
		return err
	}
	evals, err := evaluateCorpus(*corpusPath, *cacheDir)
	if err != nil {
		return err
	}

	var mismatches int
	for _, ev := range evals {
		got := parse.Snapshot(ev)
		if *update {
			if err := parse.WriteGolden(*dir, got); err != nil {
				return err
			}
			fmt.Printf("wrote %d.json\n", ev.GutenbergID)
			continue
		}
		want, err := parse.LoadGolden(*dir, ev.GutenbergID)
		if err != nil {
			fmt.Fprintf(os.Stderr, "missing golden for %d: %v\n", ev.GutenbergID, err)
			mismatches++
			continue
		}
		if err := parse.GoldenEqual(want, got); err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			mismatches++
		} else {
			fmt.Printf("ok    %d  %s\n", ev.GutenbergID, ev.Title)
		}
	}
	if *update {
		return nil
	}
	if mismatches > 0 {
		return fmt.Errorf("%d golden mismatch — -update 전에 diff를 눈으로 확인하시오", mismatches)
	}
	return nil
}

func evaluateCorpus(corpusPath, cacheDir string) ([]*parse.Evaluation, error) {
	corpus, err := parse.LoadCorpus(corpusPath)
	if err != nil {
		return nil, err
	}
	var evals []*parse.Evaluation
	var missing int
	for _, book := range corpus.Books {
		path := gutenberg.HTMLPath(cacheDir, book.GutenbergID)
		raw, err := os.ReadFile(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "missing cache %s\n", path)
			missing++
			continue
		}
		ev, err := parse.EvaluateHTML(raw, nil)
		if err != nil {
			return nil, fmt.Errorf("evaluate %d: %w", book.GutenbergID, err)
		}
		ev.GutenbergID = book.GutenbergID
		ev.Title = book.ExpectedTitle
		ev.Category = book.Category
		if meta, err := gutenberg.LoadMeta(cacheDir, book.GutenbergID); err == nil && meta.SHA256 != "" {
			ev.SourceSHA256 = meta.SHA256
		}
		evals = append(evals, ev)
	}
	if missing > 0 {
		return evals, fmt.Errorf("%d권 캐시 없음 — make fetch-corpus 먼저", missing)
	}
	return evals, nil
}

func minInterval() (time.Duration, error) {
	raw := os.Getenv("GUTENBERG_MIN_INTERVAL_MS")
	if raw == "" {
		return 0, fmt.Errorf("GUTENBERG_MIN_INTERVAL_MS 가 없습니다. .env 를 설정하시오")
	}
	ms, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("GUTENBERG_MIN_INTERVAL_MS: %w", err)
	}
	if ms < 1000 {
		return 0, fmt.Errorf("GUTENBERG_MIN_INTERVAL_MS=%d — 1000 미만으로 낮추지 말 것", ms)
	}
	return time.Duration(ms) * time.Millisecond, nil
}
