package gutenberg

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/dinnertime/reclassic/internal/parse"
)

type Meta struct {
	GutenbergID int    `json:"gutenberg_id"`
	URL         string `json:"url"`
	SHA256      string `json:"sha256"`
	Title       string `json:"title"`
}

func HTMLPath(cacheDir string, id int) string {
	return filepath.Join(cacheDir, fmt.Sprintf("%d.html", id))
}

func MetaPath(cacheDir string, id int) string {
	return filepath.Join(cacheDir, fmt.Sprintf("%d.meta.json", id))
}

func Cached(cacheDir string, id int) bool {
	_, err := os.Stat(HTMLPath(cacheDir, id))
	return err == nil
}

func SourceURLs(id int) []string {
	return []string{
		fmt.Sprintf("https://www.gutenberg.org/cache/epub/%d/pg%d-images.html", id, id),
		fmt.Sprintf("https://www.gutenberg.org/cache/epub/%d/pg%d.html", id, id),
		fmt.Sprintf("https://www.gutenberg.org/files/%d/%d-h/%d-h.htm", id, id, id),
	}
}

func FetchBook(c *Client, cacheDir string, book parse.BookSpec) (Meta, error) {
	if Cached(cacheDir, book.GutenbergID) {
		return LoadMeta(cacheDir, book.GutenbergID)
	}

	var lastErr error
	for _, u := range SourceURLs(book.GutenbergID) {
		body, status, err := c.GetRetry5xx(u, 3)
		if err != nil {
			lastErr = fmt.Errorf("%s: %w", u, err)
			continue
		}
		if status >= 400 && status < 500 {
			lastErr = fmt.Errorf("%s: HTTP %d", u, status)
			continue
		}
		if status != 200 {
			lastErr = fmt.Errorf("%s: HTTP %d", u, status)
			continue
		}

		actual := parse.ExtractTitle(body)
		if !parse.TitleMatch(book.ExpectedTitle, actual) {
			return Meta{}, fmt.Errorf("title mismatch for %d: expected %q, got %q — corpus.json을 고친 뒤 다시 받으시오", book.GutenbergID, book.ExpectedTitle, actual)
		}

		if err := os.MkdirAll(cacheDir, 0o755); err != nil {
			return Meta{}, err
		}
		sum := sha256.Sum256(body)
		meta := Meta{
			GutenbergID: book.GutenbergID,
			URL:         u,
			SHA256:      hex.EncodeToString(sum[:]),
			Title:       actual,
		}
		if err := os.WriteFile(HTMLPath(cacheDir, book.GutenbergID), body, 0o644); err != nil {
			return Meta{}, err
		}
		if err := writeMeta(cacheDir, meta); err != nil {
			return Meta{}, err
		}
		return meta, nil
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("no url succeeded")
	}
	return Meta{}, fmt.Errorf("fetch source %d: %w", book.GutenbergID, lastErr)
}

func LoadMeta(cacheDir string, id int) (Meta, error) {
	var m Meta
	raw, err := os.ReadFile(MetaPath(cacheDir, id))
	if err != nil {
		// html만 있고 meta가 없으면 해시를 다시 계산한다.
		html, err2 := os.ReadFile(HTMLPath(cacheDir, id))
		if err2 != nil {
			return m, err
		}
		sum := sha256.Sum256(html)
		m = Meta{
			GutenbergID: id,
			SHA256:      hex.EncodeToString(sum[:]),
			Title:       parse.ExtractTitle(html),
		}
		_ = writeMeta(cacheDir, m)
		return m, nil
	}
	if err := json.Unmarshal(raw, &m); err != nil {
		return m, err
	}
	return m, nil
}

func writeMeta(cacheDir string, m Meta) error {
	raw, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	return os.WriteFile(MetaPath(cacheDir, m.GutenbergID), raw, 0o644)
}
