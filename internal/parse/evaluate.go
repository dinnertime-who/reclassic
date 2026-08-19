package parse

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

type Scored struct {
	Name    string
	Result  *Result
	Signals Signals
	Err     error
}

type Evaluation struct {
	GutenbergID  int
	Title        string
	Category     string
	SourceSHA256 string
	MarkerStart  bool
	MarkerEnd    bool
	BodyChars    int
	Best         *Scored
	All          []Scored
	Warnings     []string
}

func EvaluateHTML(raw []byte, extractors []Extractor) (*Evaluation, error) {
	ev := &Evaluation{
		SourceSHA256: sha256Hex(raw),
	}

	stripped, start, end := StripBoilerplate(string(raw))
	ev.MarkerStart = start
	ev.MarkerEnd = end
	if !start || !end {
		ev.Warnings = append(ev.Warnings, "Gutenberg 시작/끝 마커를 일부 또는 전부 찾지 못함 — 전체 문서로 진행")
	}

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(stripped))
	if err != nil {
		return nil, fmt.Errorf("parse html: %w", err)
	}
	doc.Find(".pg-boilerplate, #pg-header, #pg-footer, #pg-machine-header, #pg-machine-footer").Remove()

	bodyText := Normalize(doc.Find("body").Text())
	if bodyText == "" {
		bodyText = Normalize(doc.Text())
	}
	ev.BodyChars = len(bodyText)

	if extractors == nil {
		extractors = DefaultExtractors()
	}

	var best *Scored
	for _, ex := range extractors {
		sc := Scored{Name: ex.Name()}
		res, err := ex.Extract(doc)
		if err != nil {
			sc.Err = err
			ev.All = append(ev.All, sc)
			continue
		}
		res.Strategy = ex.Name()
		res.Warnings = append(append([]string(nil), ev.Warnings...), res.Warnings...)
		sig := ComputeSignals(res, ev.BodyChars)
		conf := Score(sig)
		if ex.Name() == "single-chapter" && conf > SingleChapterCap {
			conf = SingleChapterCap
		}
		res.Confidence = conf
		sc.Result = res
		sc.Signals = sig
		ev.All = append(ev.All, sc)
		if best == nil || conf > best.Result.Confidence {
			cp := sc
			best = &cp
		}
	}
	ev.Best = best
	if best == nil {
		return nil, fmt.Errorf("모든 전략이 실패함")
	}
	return ev, nil
}

func sha256Hex(raw []byte) string {
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func Preview(text string, n int) string {
	text = Normalize(text)
	runes := []rune(text)
	if len(runes) <= n {
		return text
	}
	return string(runes[:n])
}

func (ev *Evaluation) ChapterCount() int {
	if ev.Best == nil || ev.Best.Result == nil {
		return 0
	}
	return len(ev.Best.Result.Chapters)
}

func (ev *Evaluation) ParagraphCount() int {
	if ev.Best == nil || ev.Best.Result == nil {
		return 0
	}
	n := 0
	for _, ch := range ev.Best.Result.Chapters {
		n += len(ch.Paragraphs)
	}
	return n
}

func (ev *Evaluation) TotalChars() int {
	if ev.Best == nil || ev.Best.Result == nil {
		return 0
	}
	n := 0
	for _, ch := range ev.Best.Result.Chapters {
		for _, p := range ch.Paragraphs {
			n += len(p.Text)
		}
	}
	return n
}
