package parse_test

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/dinnertime/reclassic/internal/parse"
)

func TestGoldenRegression(t *testing.T) {
	corpusPath := filepath.Join("testdata", "corpus.json")
	goldenDir := filepath.Join("testdata", "golden")
	cacheDir := filepath.Join("..", "..", ".cache", "gutenberg")

	corpus, err := parse.LoadCorpus(corpusPath)
	if err != nil {
		t.Fatal(err)
	}

	ran := 0
	for _, book := range corpus.Books {
		book := book
		cache := filepath.Join(cacheDir, strconv.Itoa(book.GutenbergID)+".html")
		if _, err := os.Stat(cache); err != nil {
			continue
		}
		ran++
		t.Run(strconv.Itoa(book.GutenbergID), func(t *testing.T) {
			raw, err := os.ReadFile(cache)
			if err != nil {
				t.Fatal(err)
			}
			ev, err := parse.EvaluateHTML(raw, nil)
			if err != nil {
				t.Fatal(err)
			}
			ev.GutenbergID = book.GutenbergID
			ev.Title = book.ExpectedTitle
			ev.Category = book.Category
			got := parse.Snapshot(ev)
			want, err := parse.LoadGolden(goldenDir, book.GutenbergID)
			if err != nil {
				t.Fatalf("golden missing: %v (run parsecheck golden -update after visual check)", err)
			}
			if err := parse.GoldenEqual(want, got); err != nil {
				t.Fatal(err)
			}
		})
	}
	if ran == 0 {
		t.Skip("Gutenberg cache 없음 — CI에서는 건너뛴다")
	}
}
