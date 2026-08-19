package parse

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
)

type Golden struct {
	GutenbergID    int             `json:"gutenberg_id"`
	Title          string          `json:"title"`
	SourceSHA256   string          `json:"source_sha256"`
	Strategy       string          `json:"strategy"`
	Confidence     float64         `json:"confidence"`
	Coverage       float64         `json:"coverage"`
	ChapterCount   int             `json:"chapter_count"`
	ParagraphCount int             `json:"paragraph_count"`
	TotalChars     int             `json:"total_chars"`
	Warnings       []string        `json:"warnings"`
	Chapters       []GoldenChapter `json:"chapters"`
}

type GoldenChapter struct {
	Idx            int    `json:"idx"`
	Title          string `json:"title"`
	ParagraphCount int    `json:"paragraph_count"`
	Chars          int    `json:"chars"`
	TextSHA256     string `json:"text_sha256"`
}

func Snapshot(ev *Evaluation) Golden {
	g := Golden{
		GutenbergID:    ev.GutenbergID,
		Title:          ev.Title,
		SourceSHA256:   ev.SourceSHA256,
		ChapterCount:   ev.ChapterCount(),
		ParagraphCount: ev.ParagraphCount(),
		TotalChars:     ev.TotalChars(),
		Warnings:       ev.Warnings,
	}
	if g.Warnings == nil {
		g.Warnings = []string{}
	}
	if ev.Best != nil && ev.Best.Result != nil {
		g.Strategy = ev.Best.Name
		g.Confidence = round4(float64(ev.Best.Result.Confidence))
		g.Coverage = round4(ev.Best.Signals.Coverage)
		if ev.Best.Result.Warnings != nil {
			g.Warnings = ev.Best.Result.Warnings
		}
		for _, ch := range ev.Best.Result.Chapters {
			var chars int
			var b strings.Builder
			for _, p := range ch.Paragraphs {
				chars += len(p.Text)
				b.WriteString(p.Text)
				b.WriteByte('\n')
			}
			g.Chapters = append(g.Chapters, GoldenChapter{
				Idx:            ch.Idx,
				Title:          ch.Title,
				ParagraphCount: len(ch.Paragraphs),
				Chars:          chars,
				TextSHA256:     sha256String(b.String()),
			})
		}
	}
	if g.Chapters == nil {
		g.Chapters = []GoldenChapter{}
	}
	return g
}

func WriteGolden(dir string, g Golden) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	path := filepath.Join(dir, fmt.Sprintf("%d.json", g.GutenbergID))
	raw, err := marshalGolden(g)
	if err != nil {
		return err
	}
	return os.WriteFile(path, raw, 0o644)
}

func LoadGolden(dir string, id int) (Golden, error) {
	var g Golden
	raw, err := os.ReadFile(filepath.Join(dir, fmt.Sprintf("%d.json", id)))
	if err != nil {
		return g, err
	}
	if err := json.Unmarshal(raw, &g); err != nil {
		return g, err
	}
	return g, nil
}

func GoldenEqual(a, b Golden) error {
	ar, err := marshalGolden(a)
	if err != nil {
		return err
	}
	br, err := marshalGolden(b)
	if err != nil {
		return err
	}
	if !bytes.Equal(ar, br) {
		return fmt.Errorf("golden mismatch for %d (%s)", a.GutenbergID, a.Title)
	}
	return nil
}

func marshalGolden(g Golden) ([]byte, error) {
	g.Confidence = round4(g.Confidence)
	g.Coverage = round4(g.Coverage)
	if g.Warnings == nil {
		g.Warnings = []string{}
	}
	if g.Chapters == nil {
		g.Chapters = []GoldenChapter{}
	}
	buf, err := json.MarshalIndent(g, "", "  ")
	if err != nil {
		return nil, err
	}
	buf = append(buf, '\n')
	return buf, nil
}

func round4(f float64) float64 {
	return math.Round(f*10000) / 10000
}

func sha256String(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}
