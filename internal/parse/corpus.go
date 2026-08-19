package parse

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

type Corpus struct {
	Books []BookSpec `json:"books"`
}

type BookSpec struct {
	GutenbergID   int    `json:"gutenberg_id"`
	ExpectedTitle string `json:"expected_title"`
	Category      string `json:"category"`
	Note          string `json:"note"`
}

func LoadCorpus(path string) (*Corpus, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read corpus: %w", err)
	}
	var c Corpus
	if err := json.Unmarshal(raw, &c); err != nil {
		return nil, fmt.Errorf("parse corpus: %w", err)
	}
	if len(c.Books) == 0 {
		return nil, fmt.Errorf("corpus is empty")
	}
	return &c, nil
}

func TitleMatch(expected, actual string) bool {
	e := titleKey(expected)
	a := titleKey(actual)
	if e == "" || a == "" {
		return false
	}
	return strings.Contains(a, e) || strings.Contains(e, a)
}

func titleKey(s string) string {
	s = strings.ToLower(s)
	s = strings.ReplaceAll(s, "the project gutenberg ebook of ", "")
	s = strings.ReplaceAll(s, "the project gutenberg ebook ", "")
	var b strings.Builder
	prevSpace := false
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			prevSpace = false
		default:
			if !prevSpace {
				b.WriteByte(' ')
				prevSpace = true
			}
		}
	}
	return strings.TrimSpace(b.String())
}

func ExtractTitle(raw []byte) string {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(string(raw)))
	if err != nil {
		return ""
	}
	if v, ok := doc.Find(`meta[name="dc.title"]`).Attr("content"); ok && strings.TrimSpace(v) != "" {
		return strings.TrimSpace(v)
	}
	if v, ok := doc.Find(`meta[name="dcterms.title"]`).Attr("content"); ok && strings.TrimSpace(v) != "" {
		return strings.TrimSpace(v)
	}
	if v, ok := doc.Find(`meta[property="og:title"]`).Attr("content"); ok && strings.TrimSpace(v) != "" {
		return strings.TrimSpace(v)
	}
	if t := strings.TrimSpace(doc.Find("title").First().Text()); t != "" {
		return t
	}
	if t := strings.TrimSpace(doc.Find("h1").First().Text()); t != "" {
		return t
	}
	return ""
}
